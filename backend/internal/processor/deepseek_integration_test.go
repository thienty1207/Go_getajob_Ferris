package processor

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestDeepSeekLiveStructuredProfile is opt-in because it calls the paid
// provider. It sends a synthetic, PII-free CV summary and verifies the same
// strict JSON/profile contract used by the production scan processor.
func TestDeepSeekLiveStructuredProfile(t *testing.T) {
	if os.Getenv("FERRIS_DEEPSEEK_INTEGRATION") != "1" {
		t.Skip("set FERRIS_DEEPSEEK_INTEGRATION=1 to run the live DeepSeek parser proof")
	}
	parser, err := NewDeepSeekParser(DeepSeekConfig{
		APIKey:        os.Getenv("DEEPSEEK_API_KEY"),
		BaseURL:       os.Getenv("DEEPSEEK_BASE_URL"),
		PrimaryModel:  os.Getenv("DEEPSEEK_PRIMARY_MODEL"),
		FallbackModel: os.Getenv("DEEPSEEK_FALLBACK_MODEL"),
	})
	if err != nil {
		t.Fatalf("NewDeepSeekParser() live error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	profile, modelName, err := parser.Parse(ctx, `
Backend Engineer
Skills: Go, PostgreSQL, Docker, REST API
Experience: 4 years building web services and relational data systems
Seniority: Mid
Domain: Recruitment technology
Education: Bachelor of Computer Science
`)
	if err != nil {
		t.Fatalf("Parse() live error = %v", err)
	}
	if modelName == "" || len(profile.Roles) == 0 || len(profile.Skills) == 0 || profile.YearsOfExperience < 0 || profile.Seniority == "" {
		t.Fatalf("live structured profile is incomplete: model=%q roles=%d skills=%d years=%v seniority=%q",
			modelName, len(profile.Roles), len(profile.Skills), profile.YearsOfExperience, profile.Seniority)
	}
	t.Logf("DeepSeek structured profile verified with model=%q roles=%d skills=%d", modelName, len(profile.Roles), len(profile.Skills))
}
