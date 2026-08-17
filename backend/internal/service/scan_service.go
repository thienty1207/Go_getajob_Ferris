package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"mime/multipart"
	"os"

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
	File       *multipart.FileHeader
	LocationID uuid.UUID
	RadiusKm   float64
}

// ScanService owns temporary CV lifecycle, scan state creation, processor
// invocation, and failure transition. It never persists the raw CV itself.
type ScanService struct {
	repository  repository.ScanRepository
	processor   processor.ScanProcessor
	maxCVBytes  int64
	maxRadiusKm float64
}

// NewScanService wires the processor explicitly. An unavailable processor is
// used as a safe failure mode so the API cannot fabricate completed matches.
func NewScanService(scanRepository repository.ScanRepository, scanProcessor processor.ScanProcessor, cfg config.Config) *ScanService {
	if scanProcessor == nil {
		scanProcessor = processor.UnavailableProcessor{}
	}
	return &ScanService{
		repository:  scanRepository,
		processor:   scanProcessor,
		maxCVBytes:  cfg.MaxCVBytes,
		maxRadiusKm: cfg.MaxRadiusKm,
	}
}

// Start validates the request, copies the upload to a temporary file, creates
// the database lifecycle row, invokes processing, and removes the temporary
// file on every exit path.
func (s *ScanService) Start(ctx context.Context, input ScanInput) (uuid.UUID, error) {
	if err := validateScanInput(input, s.maxRadiusKm); err != nil {
		return uuid.Nil, err
	}

	temporaryPath, err := saveTemporaryCV(input.File, s.maxCVBytes)
	if err != nil {
		return uuid.Nil, err
	}
	var scanID uuid.UUID
	defer func() {
		if cleanupErr := removeTemporaryFile(temporaryPath); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			slog.Error("temporary CV cleanup failed", "scan_id", scanID.String(), "err", cleanupErr)
		}
	}()

	scanID, err = s.repository.CreateScan(ctx, input.LocationID, input.RadiusKm)
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.processor.Process(ctx, scanID, temporaryPath, input.RadiusKm); err != nil {
		code := processor.ErrorCode(err)
		if updateErr := s.setFailedStatus(ctx, scanID, code); updateErr != nil {
			return uuid.Nil, fmt.Errorf("%w: failed transition", ErrScanProcessing)
		}
	}
	return scanID, nil
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

// validateScanInput enforces the user-visible location/radius contract before
// allocating a temporary file or creating a database row.
func validateScanInput(input ScanInput, maxRadiusKm float64) error {
	if input.File == nil || !isFinitePositive(maxRadiusKm) || !isFinitePositive(input.RadiusKm) || input.RadiusKm > maxRadiusKm {
		return ErrInvalidScanInput
	}
	if input.LocationID == uuid.Nil {
		return ErrInvalidScanInput
	}
	return nil
}

func isFinitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// removeTemporaryFile is deliberately small so Start's defer path remains
// easy to audit for the raw-CV retention requirement.
func removeTemporaryFile(path string) error {
	if path == "" {
		return nil
	}
	return os.Remove(path)
}
