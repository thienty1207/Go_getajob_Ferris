package processor

import (
	"context"
	"errors"
	"sort"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

type ProfileParser interface {
	Parse(context.Context, string) (model.StructuredProfile, string, error)
}

type MatchingStore interface {
	LoadScanContext(context.Context, uuid.UUID) (model.Scan, error)
	ListMatchCandidates(context.Context, model.Scan) ([]model.JobCandidate, error)
	CompleteScan(context.Context, uuid.UUID, model.StructuredProfile, string, []model.ScoredJobMatch) error
}

type MatchingProcessor struct {
	store  MatchingStore
	parser ProfileParser
}

func NewMatchingProcessor(store MatchingStore, parser ProfileParser) *MatchingProcessor {
	return &MatchingProcessor{store: store, parser: parser}
}

func (p *MatchingProcessor) Process(ctx context.Context, scanID uuid.UUID, temporaryPath string, _ float64) error {
	if p.store == nil || p.parser == nil {
		return &ProcessingError{Code: "matching_not_configured", Err: errors.New("matching dependencies are missing")}
	}
	scan, err := p.store.LoadScanContext(ctx, scanID)
	if err != nil {
		return &ProcessingError{Code: "scan_context_failed", Err: err}
	}
	// The fourth argument is retained on the processor interface for source
	// compatibility with older callers. Location-only matching now scopes
	// candidates by the scan's canonical location, so zero is valid and no
	// radius gate belongs in this layer anymore.
	if scan.Status != model.StatusParsing || scan.ID == uuid.Nil {
		return &ProcessingError{Code: "invalid_scan_state", Err: errors.New("scan is not ready for parsing")}
	}
	text, err := ExtractCVText(temporaryPath)
	if err != nil {
		return &ProcessingError{Code: "cv_extract_failed", Err: err}
	}
	profile, parserModel, err := p.parser.Parse(ctx, text)
	if err != nil {
		return err
	}
	candidates, err := p.store.ListMatchCandidates(ctx, scan)
	if err != nil {
		return &ProcessingError{Code: "job_candidates_failed", Err: err}
	}
	matches := make([]model.ScoredJobMatch, 0, len(candidates))
	for _, candidate := range candidates {
		score, err := ScoreCandidate(profile, candidate)
		if err != nil {
			return &ProcessingError{Code: "job_candidate_invalid", Err: err}
		}
		matches = append(matches, score)
	}
	sort.SliceStable(matches, func(left, right int) bool {
		if matches[left].MatchPercent != matches[right].MatchPercent {
			return matches[left].MatchPercent > matches[right].MatchPercent
		}
		if matches[left].DistanceKm == nil && matches[right].DistanceKm != nil {
			return false
		}
		if matches[left].DistanceKm != nil && matches[right].DistanceKm == nil {
			return true
		}
		if matches[left].DistanceKm != nil && matches[right].DistanceKm != nil && *matches[left].DistanceKm != *matches[right].DistanceKm {
			return *matches[left].DistanceKm < *matches[right].DistanceKm
		}
		return matches[left].JobID.String() < matches[right].JobID.String()
	})
	if err := p.store.CompleteScan(ctx, scanID, profile, parserModel, matches); err != nil {
		return &ProcessingError{Code: "scan_results_failed", Err: err}
	}
	return nil
}
