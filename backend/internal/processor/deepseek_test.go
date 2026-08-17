package processor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepSeekParserFallsBackAndRedactsUnneededPII(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests++
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		body, _ := json.Marshal(payload)
		if strings.Contains(string(body), "person@example.com") || strings.Contains(string(body), "0901234567") {
			t.Fatalf("provider request contains PII: %s", body)
		}
		if requests == 1 {
			writer.WriteHeader(http.StatusBadGateway)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"roles\":[\"Backend Engineer\"],\"skills\":[\"Go\"],\"years_of_experience\":4,\"seniority\":\"MID\",\"domains\":[\"software\"],\"education\":[],\"certifications\":[]}"}}]}`))
	}))
	defer server.Close()

	parser, err := NewDeepSeekParser(DeepSeekConfig{
		APIKey:        "test-key",
		BaseURL:       server.URL,
		PrimaryModel:  "deepseek-v4-flash",
		FallbackModel: "deepseek-v4-pro",
		HTTPClient:    server.Client(),
	})
	if err != nil {
		t.Fatalf("NewDeepSeekParser() error = %v", err)
	}

	profile, modelName, err := parser.Parse(context.Background(), "Name: Nguyen Van A\nEmail: person@example.com\nPhone: 0901234567\nBackend Engineer - Go")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if requests != 2 || modelName != "deepseek-v4-pro" || profile.Roles[0] != "Backend Engineer" {
		t.Fatalf("requests=%d model=%q profile=%#v", requests, modelName, profile)
	}
}

func TestValidateStructuredProfileRejectsUnknownOrUnsafeOutput(t *testing.T) {
	profile := StructuredProfilePayload{Roles: []string{"Backend"}, Skills: []string{"Go"}, YearsOfExperience: 101, Seniority: "MID"}
	if err := profile.Validate(); err == nil {
		t.Fatal("Validate() error = nil for impossible experience")
	}
}
