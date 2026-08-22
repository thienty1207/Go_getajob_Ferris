package service

import (
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"os"
	"sync"
	"time"

	"github.com/gogetsomefoodferris/backend/internal/config"
	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/gogetsomefoodferris/backend/internal/processor"
	"github.com/gogetsomefoodferris/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	// These stable categories let the HTTP layer show safe messages without
	// exposing parser, filesystem, or database details.
	ErrInvalidScanInput = errors.New("invalid scan input")
	ErrScanProcessing   = errors.New("scan processing state unavailable")
)

// ScanInput is the validated-at-the-boundary request shape passed from the
// HTTP handler to the scan lifecycle service.
type ScanInput struct {
	File         *multipart.FileHeader
	LocationID   uuid.UUID
	ClientUserID *uuid.UUID
	// RadiusKm is retained only for source compatibility with old internal
	// callers. It is ignored by the active location-only scan flow.
	RadiusKm float64
}

// ScanService owns temporary CV lifecycle, scan state creation, processor
// invocation, and failure transition. It never persists the raw CV itself.
type ScanService struct {
	repository           repository.ScanRepository
	processor            processor.ScanProcessor
	maxCVBytes           int64
	processingContext    context.Context
	cancelProcessing     context.CancelFunc
	processingTimeout    time.Duration
	failedStatusTimeout  time.Duration
	temporaryCVDirectory string
	now                  func() time.Time
	recoveryTicker       temporaryCVRecoveryTicker
	processingWG         sync.WaitGroup
	recoveryWG           sync.WaitGroup
	closeOnce            sync.Once
}

const (
	scanProcessingTimeout       = 2 * time.Minute
	scanFailedStatusTimeout     = 5 * time.Second
	temporaryCVRecoveryInterval = time.Minute
)

type temporaryCVRecoveryTicker interface {
	C() <-chan time.Time
	Stop()
}

type systemTemporaryCVRecoveryTicker struct {
	ticker *time.Ticker
}

func (t systemTemporaryCVRecoveryTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t systemTemporaryCVRecoveryTicker) Stop() {
	t.ticker.Stop()
}

type scanServiceDependencies struct {
	temporaryCVDirectory string
	now                  func() time.Time
	recoveryTicker       temporaryCVRecoveryTicker
	processingTimeout    time.Duration
	failedStatusTimeout  time.Duration
}

// NewScanService wires the processor explicitly. An unavailable processor is
// used as a safe failure mode so the API cannot fabricate completed matches.
func NewScanService(scanRepository repository.ScanRepository, scanProcessor processor.ScanProcessor, cfg config.Config) *ScanService {
	return newScanService(scanRepository, scanProcessor, cfg, scanServiceDependencies{})
}

func newScanService(scanRepository repository.ScanRepository, scanProcessor processor.ScanProcessor, cfg config.Config, dependencies scanServiceDependencies) *ScanService {
	if scanProcessor == nil {
		scanProcessor = processor.UnavailableProcessor{}
	}
	if dependencies.temporaryCVDirectory == "" {
		dependencies.temporaryCVDirectory = cvTemporaryDirectory()
	}
	if dependencies.now == nil {
		dependencies.now = time.Now
	}
	if dependencies.recoveryTicker == nil {
		dependencies.recoveryTicker = systemTemporaryCVRecoveryTicker{ticker: time.NewTicker(temporaryCVRecoveryInterval)}
	}
	if dependencies.processingTimeout <= 0 {
		dependencies.processingTimeout = scanProcessingTimeout
	}
	if dependencies.failedStatusTimeout <= 0 {
		dependencies.failedStatusTimeout = scanFailedStatusTimeout
	}
	processingContext, cancelProcessing := context.WithCancel(context.Background())
	scanService := &ScanService{
		repository:           scanRepository,
		processor:            scanProcessor,
		maxCVBytes:           cfg.MaxCVBytes,
		processingContext:    processingContext,
		cancelProcessing:     cancelProcessing,
		processingTimeout:    dependencies.processingTimeout,
		failedStatusTimeout:  dependencies.failedStatusTimeout,
		temporaryCVDirectory: dependencies.temporaryCVDirectory,
		now:                  dependencies.now,
		recoveryTicker:       dependencies.recoveryTicker,
	}
	scanService.recoverTemporaryCVs()
	scanService.recoveryWG.Add(1)
	go scanService.runTemporaryCVRecovery()
	return scanService
}

// Start validates the request, copies the upload to a temporary file, creates
// the database lifecycle row, and queues processing. The caller receives the
// scan ID immediately so the dedicated result route can show real polling
// state while parsing and matching run in the background.
func (s *ScanService) Start(ctx context.Context, input ScanInput) (uuid.UUID, error) {
	if err := validateScanInput(input); err != nil {
		return uuid.Nil, err
	}

	temporaryPath, err := saveTemporaryCV(input.File, s.maxCVBytes)
	if err != nil {
		return uuid.Nil, err
	}
	var scanID uuid.UUID

	if ownedRepository, ok := s.repository.(repository.ClientOwnedScanRepository); ok && input.ClientUserID != nil {
		scanID, err = ownedRepository.CreateScanForClient(ctx, input.LocationID, input.ClientUserID)
	} else {
		scanID, err = s.repository.CreateScan(ctx, input.LocationID, 0)
	}
	if err != nil {
		if cleanupErr := removeTemporaryFile(temporaryPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			slog.Error("temporary CV cleanup failed", "error_code", "scan_create_failed")
		}
		return uuid.Nil, err
	}

	s.processingWG.Add(1)
	go s.processInBackground(scanID, temporaryPath)
	return scanID, nil
}

func (s *ScanService) processInBackground(scanID uuid.UUID, temporaryPath string) {
	defer s.processingWG.Done()
	defer func() {
		if cleanupErr := removeTemporaryFile(temporaryPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			slog.Error("temporary CV cleanup failed", "scan_id", scanID.String(), "error_code", "temporary_cv_cleanup_failed")
		}
	}()

	baseContext := s.processingContext
	if baseContext == nil {
		baseContext = context.Background()
	}
	processingTimeout := s.processingTimeout
	if processingTimeout <= 0 {
		processingTimeout = scanProcessingTimeout
	}
	processingContext, cancel := context.WithTimeout(baseContext, processingTimeout)
	defer cancel()

	if err := s.processor.Process(processingContext, scanID, temporaryPath, 0); err != nil {
		code := processor.ErrorCode(err)
		failedStatusTimeout := s.failedStatusTimeout
		if failedStatusTimeout <= 0 {
			failedStatusTimeout = scanFailedStatusTimeout
		}
		failedStatusContext, cancelFailedStatus := context.WithTimeout(context.Background(), failedStatusTimeout)
		updateErr := s.setFailedStatus(failedStatusContext, scanID, code)
		cancelFailedStatus()
		if updateErr != nil {
			slog.Error("scan failure transition failed", "scan_id", scanID.String(), "error_code", code)
		}
	}
}

// Close stops recovery, cancels queued scan work, and waits for temporary
// files and terminal scan states. The API calls it before closing PostgreSQL.
func (s *ScanService) Close() {
	s.closeOnce.Do(func() {
		if s.cancelProcessing != nil {
			s.cancelProcessing()
		}
		s.processingWG.Wait()
		s.recoveryWG.Wait()
	})
}

func (s *ScanService) runTemporaryCVRecovery() {
	defer s.recoveryWG.Done()
	defer s.recoveryTicker.Stop()

	ticks := s.recoveryTicker.C()
	for {
		select {
		case <-s.processingContext.Done():
			return
		case _, ok := <-ticks:
			if !ok {
				return
			}
			s.recoverTemporaryCVs()
		}
	}
}

func (s *ScanService) recoverTemporaryCVs() {
	if err := cleanupStaleTemporaryCVs(s.temporaryCVDirectory, s.now()); err != nil {
		// Recovery stays best-effort so a transient filesystem error cannot make
		// the API unavailable; future ticks retry without logging paths or bytes.
		slog.Warn("stale temporary CV cleanup incomplete", "error_code", "temporary_cv_recovery_failed")
	}
}

func (s *ScanService) setFailedStatus(ctx context.Context, scanID uuid.UUID, code string) error {
	// A short retry covers a transient database write failure while keeping the
	// request bounded; the caller still receives an explicit unavailable error
	// if the lifecycle transition cannot be recorded.
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		err = s.repository.SetStatus(ctx, scanID, model.StatusFailed, &code)
		if err == nil {
			return nil
		}
	}
	return err
}

// Get reads the current scan lifecycle and any completed public matches.
func (s *ScanService) Get(ctx context.Context, scanID uuid.UUID) (model.Scan, error) {
	return s.repository.GetScan(ctx, scanID)
}

// GetOwned keeps the authenticated client read path separate from the legacy
// anonymous repository method. Production repositories must implement the
// ownership-aware extension before this method can return a scan.
func (s *ScanService) GetOwned(ctx context.Context, scanID, clientUserID uuid.UUID) (model.Scan, error) {
	ownedRepository, ok := s.repository.(repository.ClientOwnedScanRepository)
	if !ok || clientUserID == uuid.Nil {
		return model.Scan{}, repository.ErrScanNotFound
	}
	return ownedRepository.GetScanForClient(ctx, scanID, clientUserID)
}

// validateScanInput enforces the user-visible file/location contract before
// allocating a temporary file or creating a database row. Radius is no longer
// part of the active client behavior.
func validateScanInput(input ScanInput) error {
	if input.File == nil {
		return ErrInvalidScanInput
	}
	if input.LocationID == uuid.Nil {
		return ErrInvalidScanInput
	}
	return nil
}

// removeTemporaryFile is deliberately small so Start's defer path remains
// easy to audit for the raw-CV retention requirement.
func removeTemporaryFile(path string) error {
	if path == "" {
		return nil
	}
	return os.Remove(path)
}
