package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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
	t.Cleanup(service.Close)

	_, err := service.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.exe", []byte("not a cv")),
		LocationID: testLocationID,
		RadiusKm:   25,
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
	t.Cleanup(service.Close)

	_, err := service.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.pdf", []byte("plain text")),
		LocationID: testLocationID,
		RadiusKm:   25,
	})
	if !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("Start() error = %v, want ErrInvalidUpload", err)
	}
}

func TestStartRejectsGenericZIPRenamedAsDOCX(t *testing.T) {
	scanService := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, testConfig(1024*1024))
	t.Cleanup(scanService.Close)

	_, err := scanService.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.docx", makeZIPFixture(t, "readme.txt", "not a Word document")),
		LocationID: testLocationID,
	})
	if !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("Start() error = %v, want ErrInvalidUpload for generic ZIP", err)
	}
}

func TestStartRejectsActualOversizeFile(t *testing.T) {
	service := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, testConfig(5))
	t.Cleanup(service.Close)

	_, err := service.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", []byte("123456")),
		LocationID: testLocationID,
		RadiusKm:   25,
	})
	if !errors.Is(err, ErrUploadTooLarge) {
		t.Fatalf("Start() error = %v, want ErrUploadTooLarge", err)
	}
}

func TestStartRejectsEmptyCVBeforeCreatingScan(t *testing.T) {
	testRepo := newRecordingRepository()
	scanService := NewScanService(testRepo, processor.UnavailableProcessor{}, testConfig(1024))
	t.Cleanup(scanService.Close)

	_, err := scanService.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", nil),
		LocationID: testLocationID,
	})
	if !errors.Is(err, ErrInvalidUpload) {
		t.Fatalf("Start() error = %v, want ErrInvalidUpload for empty raw CV", err)
	}
	if len(testRepo.scans) != 0 {
		t.Fatalf("repository scans = %d, want no lifecycle row for empty upload", len(testRepo.scans))
	}
}

func TestStartDeletesTemporaryFileAfterProcessorReturns(t *testing.T) {
	testRepo := newRecordingRepository()
	processor := &recordingProcessor{done: make(chan struct{})}
	service := NewScanService(testRepo, processor, testConfig(1024))
	t.Cleanup(service.Close)

	id, err := service.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.pdf", []byte("%PDF-1.7")),
		LocationID: testLocationID,
		RadiusKm:   25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("Start() returned uuid.Nil")
	}
	service.Close()
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

func TestStartPreservesCVExtensionForTheRealExtractor(t *testing.T) {
	for _, filename := range []string{"resume.pdf", "resume.docx", "resume.txt"} {
		t.Run(filename, func(t *testing.T) {
			scanProcessor := &recordingProcessor{done: make(chan struct{})}
			scanService := NewScanService(newRecordingRepository(), scanProcessor, testConfig(1024))
			t.Cleanup(scanService.Close)

			content := []byte("plain text cv")
			if filepath.Ext(filename) == ".pdf" {
				content = []byte("%PDF-1.7")
			}
			if filepath.Ext(filename) == ".docx" {
				content = makeZIPFixture(t, "word/document.xml", "<w:document><w:t>Backend Engineer</w:t></w:document>")
			}
			if _, err := scanService.Start(context.Background(), ScanInput{File: makeFileHeader(t, filename, content), LocationID: testLocationID}); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			scanService.Close()
			if got, want := filepath.Ext(scanProcessor.path), filepath.Ext(filename); got != want {
				t.Fatalf("processor temporary extension = %q, want %q so ExtractCVText can select the parser", got, want)
			}
			if got, want := filepath.Clean(filepath.Dir(scanProcessor.path)), filepath.Clean(cvTemporaryDirectory()); got != want {
				t.Fatalf("processor temporary directory = %q, want private CV directory %q", got, want)
			}
		})
	}
}

func makeZIPFixture(t *testing.T, name, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entry, err := archive.Create(name)
	if err != nil {
		t.Fatalf("create ZIP entry: %v", err)
	}
	if _, err := entry.Write([]byte(content)); err != nil {
		t.Fatalf("write ZIP entry: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close ZIP fixture: %v", err)
	}
	return buffer.Bytes()
}

func TestCleanupStaleTemporaryCVsRemovesOnlyOldOwnedFiles(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	oldOwned := filepath.Join(directory, "gogetsomefood-cv-old.txt")
	recentOwned := filepath.Join(directory, "gogetsomefood-cv-recent.pdf")
	unrelated := filepath.Join(directory, "another-app.txt")
	ownedDirectory := filepath.Join(directory, "gogetsomefood-cv-directory")
	for _, path := range []string{oldOwned, recentOwned, unrelated} {
		if err := os.WriteFile(path, []byte("sensitive test fixture"), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", path, err)
		}
	}
	if err := os.Mkdir(ownedDirectory, 0o700); err != nil {
		t.Fatalf("Mkdir(%q) error = %v", ownedDirectory, err)
	}
	if err := os.Chtimes(oldOwned, now.Add(-staleTemporaryCVAge-time.Minute), now.Add(-staleTemporaryCVAge-time.Minute)); err != nil {
		t.Fatalf("Chtimes(old) error = %v", err)
	}
	if err := os.Chtimes(recentOwned, now.Add(-time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatalf("Chtimes(recent) error = %v", err)
	}

	if err := cleanupStaleTemporaryCVs(directory, now); err != nil {
		t.Fatalf("cleanupStaleTemporaryCVs() error = %v", err)
	}
	if _, err := os.Stat(oldOwned); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old owned CV stat error = %v, want removed", err)
	}
	for _, path := range []string{recentOwned, unrelated, ownedDirectory} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("preserved file %q stat error = %v", path, err)
		}
	}
}

func TestTemporaryCVRecoveryEventuallyRemovesRecentStartupOrphan(t *testing.T) {
	directory := t.TempDir()
	clock := newFakeClock(time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	ticker := newFakeTemporaryCVRecoveryTicker()
	orphan := filepath.Join(directory, "gogetsomefood-cv-recent.txt")
	if err := os.WriteFile(orphan, []byte("sensitive test fixture"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", orphan, err)
	}
	if err := os.Chtimes(orphan, clock.Now().Add(-time.Minute), clock.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("Chtimes(%q) error = %v", orphan, err)
	}

	scanService := newScanService(
		newRecordingRepository(),
		processor.UnavailableProcessor{},
		testConfig(1024),
		scanServiceDependencies{
			temporaryCVDirectory: directory,
			now:                  clock.Now,
			recoveryTicker:       ticker,
		},
	)
	t.Cleanup(scanService.Close)

	if _, err := os.Stat(orphan); err != nil {
		t.Fatalf("recent startup orphan stat error = %v, want initial sweep to preserve it", err)
	}
	clock.Advance(staleTemporaryCVAge)
	ticker.Tick(t)
	scanService.Close()

	if _, err := os.Stat(orphan); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("orphan stat error after scheduled recovery = %v, want removed", err)
	}
}

func TestCloseStopsTemporaryCVRecoveryTask(t *testing.T) {
	ticker := newFakeTemporaryCVRecoveryTicker()
	scanService := newScanService(
		newRecordingRepository(),
		processor.UnavailableProcessor{},
		testConfig(1024),
		scanServiceDependencies{
			temporaryCVDirectory: t.TempDir(),
			now:                  time.Now,
			recoveryTicker:       ticker,
		},
	)

	scanService.Close()

	select {
	case <-ticker.stopped:
	default:
		t.Fatal("Close() returned before the temporary CV recovery ticker stopped")
	}
}

func TestStartReturnsBeforeBackgroundProcessorFinishes(t *testing.T) {
	processor := &blockingProcessor{started: make(chan struct{}), release: make(chan struct{}), finished: make(chan struct{})}
	service := NewScanService(newRecordingRepository(), processor, testConfig(1024))
	t.Cleanup(service.Close)

	if _, err := service.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-processor.started:
	case <-time.After(2 * time.Second):
		t.Fatal("background processor did not start")
	}
	select {
	case <-processor.finished:
		t.Fatal("Start() waited for background processor")
	default:
	}

	close(processor.release)
	service.Close()
}

func TestStartPersistsProcessorFailureWithoutFabricatingResult(t *testing.T) {
	testRepo := newRecordingRepository()
	failureProcessor := recordingProcessor{err: &processor.ProcessingError{Code: "parser_failed"}, done: make(chan struct{})}
	service := NewScanService(testRepo, &failureProcessor, testConfig(1024))
	t.Cleanup(service.Close)

	id, err := service.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
		RadiusKm:   25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if id == uuid.Nil {
		t.Fatal("Start() returned uuid.Nil")
	}
	service.Close()
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

func TestShutdownPersistsFailedWithContextAwareRepository(t *testing.T) {
	testRepo := newContextAwareRecordingRepository()
	scanProcessor := &contextBlockingProcessor{started: make(chan struct{}), finished: make(chan error, 1)}
	scanService := NewScanService(testRepo, scanProcessor, testConfig(1024))
	t.Cleanup(scanService.Close)

	if _, err := scanService.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	select {
	case <-scanProcessor.started:
	case <-time.After(2 * time.Second):
		t.Fatal("processor did not start")
	}

	scanService.Close()

	if err := <-scanProcessor.finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("processor error = %v, want context.Canceled", err)
	}
	if got, want := testRepo.statuses, []model.ScanStatus{model.StatusParsing, model.StatusFailed}; !equalStatuses(got, want) {
		t.Fatalf("statuses = %#v, want %#v", got, want)
	}
	if testRepo.canceledStatusAttempts != 0 {
		t.Fatalf("canceled SetStatus attempts = %d, want 0", testRepo.canceledStatusAttempts)
	}
	if testRepo.unboundedFailedStatusAttempts != 0 {
		t.Fatalf("unbounded FAILED SetStatus attempts = %d, want 0", testRepo.unboundedFailedStatusAttempts)
	}
}

func TestProcessorTimeoutPersistsFailedFromMatchingWithContextAwareRepository(t *testing.T) {
	testRepo := newContextAwareRecordingRepository()
	scanProcessor := &contextBlockingProcessor{
		started:  make(chan struct{}),
		finished: make(chan error, 1),
		beforeWait: func(ctx context.Context, scanID uuid.UUID) error {
			return testRepo.SetStatus(ctx, scanID, model.StatusMatching, nil)
		},
	}
	processingContext, cancelProcessing := context.WithTimeout(context.Background(), 20*time.Millisecond)
	scanService := &ScanService{
		repository:        testRepo,
		processor:         scanProcessor,
		maxCVBytes:        1024,
		processingContext: processingContext,
		cancelProcessing:  cancelProcessing,
	}
	t.Cleanup(scanService.Close)

	if _, err := scanService.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	scanService.processingWG.Wait()

	if err := <-scanProcessor.finished; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("processor error = %v, want context.DeadlineExceeded", err)
	}
	if got, want := testRepo.statuses, []model.ScanStatus{model.StatusParsing, model.StatusMatching, model.StatusFailed}; !equalStatuses(got, want) {
		t.Fatalf("statuses = %#v, want %#v", got, want)
	}
	if testRepo.canceledStatusAttempts != 0 {
		t.Fatalf("canceled SetStatus attempts = %d, want 0", testRepo.canceledStatusAttempts)
	}
	if testRepo.unboundedFailedStatusAttempts != 0 {
		t.Fatalf("unbounded FAILED SetStatus attempts = %d, want 0", testRepo.unboundedFailedStatusAttempts)
	}
}

func TestStartRejectsMissingLocation(t *testing.T) {
	service := NewScanService(newRecordingRepository(), processor.UnavailableProcessor{}, testConfig(1024))
	t.Cleanup(service.Close)
	validFile := makeFileHeader(t, "resume.txt", []byte("plain text cv"))

	_, err := service.Start(context.Background(), ScanInput{File: validFile, RadiusKm: 25})
	if !errors.Is(err, ErrInvalidScanInput) {
		t.Fatalf("Start() error = %v, want ErrInvalidScanInput", err)
	}
}

func TestStartIgnoresLegacyRadius(t *testing.T) {
	for _, radius := range []float64{0, 25, 501} {
		t.Run(fmt.Sprintf("radius_%v", radius), func(t *testing.T) {
			scanProcessor := &recordingProcessor{done: make(chan struct{})}
			scanService := NewScanService(newRecordingRepository(), scanProcessor, testConfig(1024))
			t.Cleanup(scanService.Close)
			_, err := scanService.Start(context.Background(), ScanInput{
				File:       makeFileHeader(t, "resume.txt", []byte("plain text cv")),
				LocationID: testLocationID,
				RadiusKm:   radius,
			})
			if err != nil {
				t.Fatalf("Start() error = %v, want legacy radius ignored", err)
			}
			scanService.Close()
		})
	}
}

func TestStartRetriesFailedTransitionOnce(t *testing.T) {
	testRepo := newRecordingRepository()
	testRepo.failStatusAttempts = 1
	failureProcessor := recordingProcessor{err: &processor.ProcessingError{Code: "parser_failed"}, done: make(chan struct{})}
	scanService := NewScanService(testRepo, &failureProcessor, testConfig(1024))
	t.Cleanup(scanService.Close)

	_, err := scanService.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
		RadiusKm:   25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v, want retry to persist failed state", err)
	}
	scanService.Close()
	if got, want := testRepo.statuses, []model.ScanStatus{model.StatusParsing, model.StatusFailed}; !equalStatuses(got, want) {
		t.Fatalf("statuses = %#v, want %#v", got, want)
	}
}

func TestStartTreatsCommittedFailedTransitionAsSuccess(t *testing.T) {
	testRepo := newRecordingRepository()
	testRepo.applyThenFailStatus = true
	failureProcessor := recordingProcessor{err: &processor.ProcessingError{Code: "parser_failed"}, done: make(chan struct{})}
	scanService := NewScanService(testRepo, &failureProcessor, testConfig(1024))
	t.Cleanup(scanService.Close)

	_, err := scanService.Start(context.Background(), ScanInput{
		File:       makeFileHeader(t, "resume.txt", []byte("plain text cv")),
		LocationID: testLocationID,
		RadiusKm:   25,
	})
	if err != nil {
		t.Fatalf("Start() error = %v, want idempotent retry success", err)
	}
	scanService.Close()
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
	done chan struct{}
}

type blockingProcessor struct {
	started  chan struct{}
	release  chan struct{}
	finished chan struct{}
}

type contextBlockingProcessor struct {
	started    chan struct{}
	finished   chan error
	beforeWait func(context.Context, uuid.UUID) error
}

func (p *blockingProcessor) Process(_ context.Context, _ uuid.UUID, temporaryPath string, _ float64) error {
	if _, err := os.Stat(temporaryPath); err != nil {
		return err
	}
	close(p.started)
	<-p.release
	close(p.finished)
	return nil
}

func (p *contextBlockingProcessor) Process(ctx context.Context, scanID uuid.UUID, temporaryPath string, _ float64) error {
	if _, err := os.Stat(temporaryPath); err != nil {
		return err
	}
	if p.beforeWait != nil {
		if err := p.beforeWait(ctx, scanID); err != nil {
			return err
		}
	}
	close(p.started)
	<-ctx.Done()
	p.finished <- ctx.Err()
	return ctx.Err()
}

func (p *recordingProcessor) Process(_ context.Context, _ uuid.UUID, tempPath string, _ float64) error {
	p.path = tempPath
	if p.done != nil {
		defer close(p.done)
	}
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

type contextAwareRecordingRepository struct {
	*recordingRepository
	canceledStatusAttempts        int
	unboundedFailedStatusAttempts int
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{now: now}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(duration)
}

type fakeTemporaryCVRecoveryTicker struct {
	ticks    chan time.Time
	stopped  chan struct{}
	stopOnce sync.Once
}

func newFakeTemporaryCVRecoveryTicker() *fakeTemporaryCVRecoveryTicker {
	return &fakeTemporaryCVRecoveryTicker{
		ticks:   make(chan time.Time),
		stopped: make(chan struct{}),
	}
}

func (t *fakeTemporaryCVRecoveryTicker) C() <-chan time.Time {
	return t.ticks
}

func (t *fakeTemporaryCVRecoveryTicker) Stop() {
	t.stopOnce.Do(func() { close(t.stopped) })
}

func (t *fakeTemporaryCVRecoveryTicker) Tick(testingT *testing.T) {
	testingT.Helper()
	select {
	case t.ticks <- time.Time{}:
	case <-time.After(2 * time.Second):
		testingT.Fatal("temporary CV recovery task did not receive a scheduled tick")
	}
}

func newRecordingRepository() *recordingRepository {
	return &recordingRepository{scans: make(map[uuid.UUID]model.Scan)}
}

func newContextAwareRecordingRepository() *contextAwareRecordingRepository {
	return &contextAwareRecordingRepository{recordingRepository: newRecordingRepository()}
}

func (r *contextAwareRecordingRepository) SetStatus(ctx context.Context, id uuid.UUID, status model.ScanStatus, errorCode *string) error {
	if err := ctx.Err(); err != nil {
		r.canceledStatusAttempts++
		return err
	}
	if status == model.StatusFailed {
		if _, bounded := ctx.Deadline(); !bounded {
			r.unboundedFailedStatusAttempts++
		}
	}
	return r.recordingRepository.SetStatus(ctx, id, status, errorCode)
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
