package service

import (
	"bytes"
	"context"
	"errors"
	"math"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/processor"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

var testLocationID = uuid.MustParse("11111111-1111-4111-8111-111111111111")

func TestStartRejectsUnsupportedExtension(t *testing.T) {
	testRepo := newRecordingRepository()
	service := NewScanService(testRepo, processor.UnavailableProcessor{}, testConfig(1024))

	_, err := service.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.exe", []byte("not a cv")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("Start() error = %v, want ErrInvalidUpload", err)
	}
	if len(testRepo.statuses) != 0 {
		t.Fatalf("repository statuses = %#v, want no persisted scan", testRepo.statuses)
	}
}

func TestStartRejectsInvalidPDFSignature(t *testing.T) {
	service := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, testConfig(1024))

	_, err := service.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.pdf", []byte("plain text")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("Start() error = %v, want ErrInvalidUpload", err)
	}
}

func TestStartRejectsActualOversizeFile(t *testing.T) {
	service := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, testConfig(5))

	_, err := service.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.txt", []byte("123456")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("Start() error = %v, want ErrUploadTooLarge", err)
	}
}

func TestStartDeletesTemporaryFileAfterProcessorReturns(t *testing.T) {
	testRepo := newRecordingRepository()
	processor := &recordingProcessor{}
	service := NewScanService(testRepo, processor, testConfig(1024))

	id, err := service.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.pdf", []byte("%PDF-1.7")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("Start() returned uuid.Nil")
	}
	if processor.path == "" {
		t.Fatal("processor did not receive a temporary path")
	}
	if _, err := os.Stat(processor.path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary path stat error = %v, want os.ErrNotExist", err)
	}
	if got, want := testRepo.statuses, []model.ScanStatus{model.StatusParsing}; !equalStatuses(got, want) {
		t.Fatalf("statuses = %#v, want %#v", got, want)
	}
}

func TestStartPersistsProcessorFailureWithoutFabricatingResult(t *testing.T) {
	testRepo := newRecordingRepository()
	failureProcessor := recordingProcessor{err: &processor.ProcessingError{Code: "parser_failed"}}
	service := NewScanService(testRepo, &failureProcessor, testConfig(1024))

	id, err := service.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("Start() returned uuid.Nil")
	}
	if got, want := testRepo.statuses, []model.ScanStatus{model.StatusParsing, model.StatusFailed}; !equalStatuses(got, want) {
		t.Fatalf("statuses = %#v, want %#v", got, want)
	}
	if testRepo.failedCode != "parser_failed" {
		t.Fatalf("failedCode = %q, want parser_failed", testRepo.failedCode)
	}
	if len(testRepo.matches) != 0 {
		t.Fatalf("matches = %#v, want no fabricated matches", testRepo.matches)
	}
}

func TestStartRejectsInvalidLocationAndRadius(t *testing.T) {
	service := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, testConfig(1024))
	validFile := makeFileHeader(t, "resume.txt", []byte("plain text cv"))

	for name, input := range map[string]ScanInput{
		"missing location": {File: validFile, RadiusKm: 25},
		"zero radius":     {File: validFile, LocationID: testLocationID, RadiusKm: 0},
		"large radius":    {File: validFile, LocationID: testLocationID, RadiusKm: 501},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.Start(context.Background(), input)
			if !errors.Is(err, ErrInvalidScanInput) {
				t.Fatalf("Start() error = %v, want ErrInvalidScanInput", err)
			}
		})
	}
}

func TestStartRejectsNonFiniteRadiusAndLimit(t *testing.T) {
	for name, radius := range map[string]float64{
		"NaN":  math.NaN(),
		"+Inf": math.Inf(1),
		"-Inf": math.Inf(-1),
	} {
		t.Run("radius_"+name, func(t *testing.T) {
			scanService := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, testConfig(1024))
			_, err := scanService.Start(context.Background(), ScanInput{
				File:     makeFileHeader(t, "resume.txt", []byte("plain text cv")),
				LocationID: testLocationID,
				RadiusKm: radius,
			})
			if !errors.Is(err, ErrInvalidScanInput) {
				t.Fatalf("Start() error = %v, want ErrInvalidScanInput", err)
			}
		})
	}

	scanService := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, config.Config{
		MaxCVBytes:  1024,
		MaxRadiusKm: math.NaN(),
	})
	_, err := scanService.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if !errors.Is(err, ErrInvalidScanInput) {
		t.Fatalf("Start() with NaN limit error = %v, want ErrInvalidScanInput", err)
	}
}

func TestStartRetriesFailedTransitionOnce(t *testing.T) {
	testRepo := newRecordingRepository()
	testRepo.failStatusAttempts = 1
	failureProcessor := recordingProcessor{err: &processor.ProcessingError{Code: "parser_failed"}}
	scanService := NewScanService(testRepo, &failureProcessor, testConfig(1024))

	_, err := scanService.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v, want retry to persist failed state", err)
	}
	if got, want := testRepo.statuses, []model.ScanStatus{model.StatusParsing, model.StatusFailed}; !equalStatuses(got, want) {
		t.Fatalf("statuses = %#v, want %#v", got, want)
	}
}

func TestStartTreatsCommittedFailedTransitionAsSuccess(t *testing.T) {
	testRepo := newRecordingRepository()
	testRepo.applyThenFailStatus = true
	failureProcessor := recordingProcessor{err: &processor.ProcessingError{Code: "parser_failed"}}
	scanService := NewScanService(testRepo, &failureProcessor, testConfig(1024))

	_, err := scanService.Start(context.Background(), ScanInput{
		File:     makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
		RadiusKm: 25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v, want idempotent retry success", err)
	}
	if testRepo.failedCode != "parser_failed" {
		t.Fatalf("failedCode = %q, want parser_failed", testRepo.failedCode)
	}
}

func testConfig(maxCVBytes int64) config.Config {
	return config.Config{
		MaxCVBytes:         maxCVBytes,
		MaxRadiusKm:        500,
		RateLimitPerMinute: 10,
	}
}

func makeFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("cv", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("part.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1024 * 1024); err != nil {
		t.Fatalf("ParseMultipartForm() error = %v", err)
	}
	return request.MultipartForm.File["cv"][0]
}

func equalStatuses(got, want []model.ScanStatus) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

type recordingProcessor struct {
	path string
	err  error
}

func (p *recordingProcessor) Process(_ context.Context, _ uuid.UUID, tempPath string, _ float64) error {
	p.path = tempPath
	if _, err := os.Stat(tempPath); err != nil {
		return err
	}
	return p.err
}

type recordingRepository struct {
	scans               map[uuid.UUID]model.Scan
	statuses            []model.ScanStatus
	failedCode          string
	matches             []model.JobMatch
	failStatusAttempts  int
	applyThenFailStatus bool
}

func newRecordingRepository() *recordingRepository {
	return &recordingRepository{scans: make(map[uuid.UUID]model.Scan)}
}

func (r *recordingRepository) CreateScan(_ context.Context, locationID uuid.UUID, radiusKm float64) (uuid.UUID, error) {
	id := uuid.New()
	r.scans[id] = model.Scan{ID: id, Status: model.StatusParsing, LocationID: &locationID, RadiusKm: radiusKm}
	r.statuses = append(r.statuses, model.StatusParsing)
	return id, nil
}

func (r *recordingRepository) SetStatus(_ context.Context, id uuid.UUID, status model.ScanStatus, errorCode *string) error {
	if status == model.StatusFailed && r.applyThenFailStatus {
		r.applyThenFailStatus = false
		scan := r.scans[id]
		scan.Status = status
		r.scans[id] = scan
		r.statuses = append(r.statuses, status)
		if errorCode != nil {
			r.failedCode = *errorCode
		}
		return errors.New("ambiguous post-commit failure")
	}
	if r.failStatusAttempts > 0 {
		r.failStatusAttempts--
		return errors.New("temporary status update failure")
	}
	if status == model.StatusFailed && r.scans[id].Status == model.StatusFailed {
		return nil
	}
	scan := r.scans[id]
	scan.Status = status
	r.scans[id] = scan
	r.statuses = append(r.statuses, status)
	if errorCode != nil {
		r.failedCode = *errorCode
	}
	return nil
}

func (r *recordingRepository) GetScan(_ context.Context, id uuid.UUID) (model.Scan, error) {
	scan, ok := r.scans[id]
	if !ok {
		return model.Scan{}, repository.ErrScanNotFound
	}
	scan.Matches = append([]model.JobMatch(nil), r.matches...)
	return scan, nil
}
