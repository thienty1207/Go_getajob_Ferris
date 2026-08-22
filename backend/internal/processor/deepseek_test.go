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
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"roles\":[\"Backend Engineer\"],\"skills\":[\"Go\"],\"years_of_experience\":4,\"seniority\":\"MID\",\"domains\":[\"software\"],\"education\":[],\"certifications\":[],\"summary\":{\"headline\":\"Backend Engineer\",\"overview\":\"Builds backend services.\",\"target_roles\":[\"Backend Engineer\"],\"strengths\":[\"Go\"],\"gaps\":[\"Add cloud exposure\"]}}"}}]}`))
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

func TestDeepSeekParserRedactsPIIMatrixBeforeProviderRequest(t *testing.T) {
	forbidden := []string{
		"Nguyen Van An",
		"person@example.com",
		"+1 (415) 555-0198",
		"01/02/1990",
		"079123456789",
		"12 Nguyen Hue",
		"linkedin.com/in/nguyenvanan",
		"github.com/nguyenvanan",
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload deepSeekCompletionRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		if len(payload.Messages) != 2 {
			t.Fatalf("provider messages = %#v, want system and sanitized user messages", payload.Messages)
		}
		for _, marker := range forbidden {
			if strings.Contains(payload.Messages[1].Content, marker) {
				t.Fatalf("provider request still contains PII marker %q: %q", marker, payload.Messages[1].Content)
			}
		}
		if !strings.Contains(payload.Messages[1].Content, "Backend Engineer") {
			t.Fatalf("sanitizer removed job-relevant text: %q", payload.Messages[1].Content)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"roles\":[\"Backend Engineer\"],\"skills\":[\"Go\"],\"years_of_experience\":4,\"seniority\":\"MID\",\"domains\":[\"software\"],\"education\":[],\"certifications\":[],\"summary\":{\"headline\":\"Backend Engineer\",\"overview\":\"Builds backend services.\",\"target_roles\":[\"Backend Engineer\"],\"strengths\":[\"Go\"],\"gaps\":[\"Add cloud exposure\"]}}"}}]}`))
	}))
	defer server.Close()

	parser, err := NewDeepSeekParser(DeepSeekConfig{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewDeepSeekParser() error = %v", err)
	}

	cv := strings.Join([]string{
		"Nguyen Van An",
		"Backend Engineer",
		"Email: person@example.com",
		"Mobile: +1 (415) 555-0198",
		"Ngày sinh: 01/02/1990",
		"CCCD: 079123456789",
		"Địa chỉ: 12 Nguyen Hue, Quan 1, TP HCM",
		"LinkedIn: https://linkedin.com/in/nguyenvanan",
		"GitHub: https://github.com/nguyenvanan",
		"4 years building Go services",
	}, "\n")
	if _, _, err := parser.Parse(context.Background(), cv); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
}

func TestNewDeepSeekParserRejectsPlainHTTPOutsideLoopback(t *testing.T) {
	if _, err := NewDeepSeekParser(DeepSeekConfig{APIKey: "test-key", BaseURL: "http://example.com"}); err == nil {
		t.Fatal("NewDeepSeekParser() error = nil for non-loopback plaintext HTTP")
	}
}

func TestValidateStructuredProfileRejectsUnknownOrUnsafeOutput(t *testing.T) {
	profile := StructuredProfilePayload{Roles: []string{"Backend"}, Skills: []string{"Go"}, YearsOfExperience: 101, Seniority: "MID"}
	if err := profile.Validate(); err == nil {
		t.Fatal("Validate() error = nil for impossible experience")
	}
}

func TestDeepSeekParserRejectsTrailingJSONValue(t *testing.T) {
	const validProfile = `{"roles":["Backend Engineer"],"skills":["Go"],"years_of_experience":4,"seniority":"MID","domains":["software"],"education":[],"certifications":[],"summary":{"headline":"Backend Engineer","overview":"Builds backend services.","target_roles":["Backend Engineer"],"strengths":["Go"],"gaps":["Add cloud exposure"]}}`

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": validProfile + ` {"unexpected":true}`},
			}},
		})
	}))
	defer server.Close()

	parser, err := NewDeepSeekParser(DeepSeekConfig{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewDeepSeekParser() error = %v", err)
	}

	if _, err := parser.parseWithModel(context.Background(), "deepseek-v4-flash", "Backend Engineer - Go"); err == nil {
		t.Fatal("parseWithModel() error = nil for a second trailing JSON value")
	}
}

func TestDeepSeekParserRejectsUnknownProfileField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": `{"roles":[],"skills":[],"years_of_experience":0,"seniority":"JUNIOR","domains":[],"education":[],"certifications":[],"summary":{"headline":"IT Candidate","overview":"Support professional.","target_roles":["IT Support"],"strengths":["Troubleshooting"],"gaps":["Add certification"]},"raw_cv":"must not persist"}`},
			}},
		})
	}))
	defer server.Close()

	parser, err := NewDeepSeekParser(DeepSeekConfig{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewDeepSeekParser() error = %v", err)
	}

	if _, err := parser.parseWithModel(context.Background(), "deepseek-v4-flash", "Backend Engineer - Go"); err == nil {
		t.Fatal("parseWithModel() error = nil for an unknown structured-profile field")
	}
}

func TestDeepSeekParserRejectsMalformedSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message": map[string]any{"content": `{"roles":["IT Officer"],"skills":[],"years_of_experience":2,"seniority":"MID","domains":[],"education":[],"certifications":[],"summary":{"headline":"","overview":"","target_roles":[],"strengths":[],"gaps":[]}}`},
			}},
		})
	}))
	defer server.Close()

	parser, err := NewDeepSeekParser(DeepSeekConfig{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatalf("NewDeepSeekParser() error = %v", err)
	}
	if _, err := parser.parseWithModel(context.Background(), "deepseek-v4-flash", "IT Officer"); err == nil {
		t.Fatal("parseWithModel() error = nil for an empty summary")
	}
}
