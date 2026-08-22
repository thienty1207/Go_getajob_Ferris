package processor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

type testProfileParser struct{}

func (testProfileParser) Parse(context.Context, string) (model.StructuredProfile, string, error) {
	return model.StructuredProfile{Roles: []string{"Backend Engineer"}, Skills: []string{"Go"}, Seniority: "MID"}, "test-parser", nil
}

type testMatchingStore struct {
	scan      model.Scan
	candidate []model.JobCandidate
	profile   model.StructuredProfile
	model     string
	matches   []model.ScoredJobMatch
}

func (store *testMatchingStore) LoadScanContext(context.Context, uuid.UUID) (model.Scan, error) {
	return store.scan, nil
}

func (store *testMatchingStore) ListMatchCandidates(context.Context, model.Scan) ([]model.JobCandidate, error) {
	return store.candidate, nil
}

func (store *testMatchingStore) CompleteScan(_ context.Context, _ uuid.UUID, profile model.StructuredProfile, parserModel string, matches []model.ScoredJobMatch) error {
	store.profile = profile
	store.model = parserModel
	store.matches = matches
	return nil
}

func TestMatchingProcessorPersistsStructuredProfileAndDeterministicMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resume.txt")
	if err := os.WriteFile(path, []byte("Backend Engineer\nGo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	store := &testMatchingStore{
		scan: model.Scan{ID: uuid.New(), Status: model.StatusParsing},
		candidate: []model.JobCandidate{{
			ID:             uuid.New(),
			Title:          "Backend Engineer",
			Role:           "Backend Engineer",
			RequiredSkills: []string{"Go"},
			Seniority:      "MID",
			WorkMode:       "REMOTE",
		}},
	}
	engine := NewMatchingProcessor(store, testProfileParser{})

	// Location-only matching deliberately passes zero for the deprecated radius
	// argument; the processor must still parse and score real candidates.
	if err := engine.Process(context.Background(), store.scan.ID, path, 0); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if store.model != "test-parser" || len(store.profile.Skills) != 1 || len(store.matches) != 1 || store.matches[0].MatchPercent <= 0 {
		t.Fatalf("stored profile/model/matches = %#v/%q/%#v", store.profile, store.model, store.matches)
	}
}

func TestMatchingProcessorDoesNotInventResultsWhenCVTextCannotBeExtracted(t *testing.T) {
	store := &testMatchingStore{scan: model.Scan{ID: uuid.New(), Status: model.StatusParsing}}
	engine := NewMatchingProcessor(store, testProfileParser{})
	if err := engine.Process(context.Background(), store.scan.ID, filepath.Join(t.TempDir(), "missing.txt"), 0); err == nil {
		t.Fatal("Process() error = nil for missing CV file")
	}
	if store.model != "" || len(store.matches) != 0 {
		t.Fatalf("store mutated after extraction failure: model=%q matches=%#v", store.model, store.matches)
	}
}
