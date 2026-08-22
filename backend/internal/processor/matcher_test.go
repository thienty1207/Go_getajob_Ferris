package processor

import (
	"testing"

	"github.com/gogetsomefoodferris/backend/internal/model"
	"github.com/google/uuid"
)

func TestScoreCandidateUsesTheFiveApprovedWeights(t *testing.T) {
	profile := model.StructuredProfile{
		Roles:             []string{"Backend Engineer"},
		Skills:            []string{"Go", "PostgreSQL"},
		YearsOfExperience: 4,
		Seniority:         "MID",
		Domains:           []string{"Fintech"},
	}
	candidate := model.JobCandidate{
		ID:                uuid.New(),
		Title:             "Backend Engineer",
		Role:              "Backend Engineer",
		RequiredSkills:    []string{"Go", "Rust"},
		PreferredSkills:   []string{"Docker"},
		Seniority:         "MID",
		MinimumExperience: pointer(2.0),
		Domains:           []string{"Fintech"},
		WorkMode:          "ONSITE",
	}

	score, err := ScoreCandidate(profile, candidate)
	if err != nil {
		t.Fatalf("ScoreCandidate() error = %v", err)
	}
	if score.RequiredSkillsPoints != 17.5 || score.RoleRelevancePoints != 25 || score.ExperiencePoints != 15 || score.SeniorityPoints != 15 || score.PreferredSkillsDomainPoints != 5 || score.MatchPercent != 77.5 {
		t.Fatalf("score = %#v, want deterministic weighted components", score)
	}
}

func TestScoreCandidateDoesNotTreatMatchPercentAsHiringProbability(t *testing.T) {
	profile := model.StructuredProfile{Skills: []string{"Go"}, Seniority: "UNSPECIFIED"}
	candidate := model.JobCandidate{ID: uuid.New(), Title: "Go Engineer", Role: "Go Engineer", RequiredSkills: []string{"Go"}, Seniority: "UNSPECIFIED", WorkMode: "REMOTE"}
	score, err := ScoreCandidate(profile, candidate)
	if err != nil {
		t.Fatalf("ScoreCandidate() error = %v", err)
	}
	if score.MatchPercent < 0 || score.MatchPercent > 100 {
		t.Fatalf("score = %#v, want bounded CV Match %%", score)
	}
}

func TestScoreCandidateSeparatesSupportFromSoftwareRoles(t *testing.T) {
	profile := model.StructuredProfile{
		Roles:             []string{"IT Officer"},
		Skills:            []string{"Ticketing", "Troubleshooting"},
		YearsOfExperience: 2,
		Seniority:         "MID",
		Domains:           []string{"IT Support"},
	}
	supportJob := model.JobCandidate{
		ID:                uuid.New(),
		Title:             "IT Helpdesk Support",
		Role:              "IT Support",
		RequiredSkills:    []string{"Ticketing", "Troubleshooting"},
		PreferredSkills:   []string{"ITIL"},
		Seniority:         "MID",
		MinimumExperience: pointer(1),
		Domains:           []string{"IT Support"},
		WorkMode:          "ONSITE",
	}
	softwareJob := model.JobCandidate{
		ID:                uuid.New(),
		Title:             "Backend Software Engineer",
		Role:              "Software Engineer",
		RequiredSkills:    []string{"Go", "PostgreSQL"},
		PreferredSkills:   []string{"Docker"},
		Seniority:         "MID",
		MinimumExperience: pointer(1),
		Domains:           []string{"Software Engineering"},
		WorkMode:          "ONSITE",
	}

	supportScore, err := ScoreCandidate(profile, supportJob)
	if err != nil {
		t.Fatalf("support score error = %v", err)
	}
	softwareScore, err := ScoreCandidate(profile, softwareJob)
	if err != nil {
		t.Fatalf("software score error = %v", err)
	}
	if supportScore.MatchPercent < 70 {
		t.Fatalf("support score = %#v, want strong support-role match", supportScore)
	}
	if softwareScore.MatchPercent > 45 || softwareScore.RoleRelevancePoints != 0 {
		t.Fatalf("software score = %#v, want materially lower unrelated-role match", softwareScore)
	}
}

func TestScoreCandidateDoesNotGiveFreePointsForMissingJobEvidence(t *testing.T) {
	profile := model.StructuredProfile{Roles: []string{"IT Officer"}, Seniority: "UNSPECIFIED"}
	candidate := model.JobCandidate{
		ID:        uuid.New(),
		Title:     "Software Engineer",
		Role:      "Software Engineer",
		Seniority: "UNSPECIFIED",
		WorkMode:  "REMOTE",
	}

	score, err := ScoreCandidate(profile, candidate)
	if err != nil {
		t.Fatalf("ScoreCandidate() error = %v", err)
	}
	if score.MatchPercent != 0 {
		t.Fatalf("score = %#v, want zero when no matching evidence exists", score)
	}
}

func pointer(value float64) *float64 { return &value }
