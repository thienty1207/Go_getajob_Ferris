package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogetsomefoodferris/backend/internal/model"
)

const (
	DefaultDeepSeekPrimaryModel  = "deepseek-v4-flash"
	DefaultDeepSeekFallbackModel = "deepseek-v4-pro"
	DefaultDeepSeekBaseURL       = "https://api.deepseek.com"
	maxProviderResponseBytes     = 512 * 1024
	maxCVTextRunes               = 80_000
)

type DeepSeekConfig struct {
	APIKey        string
	BaseURL       string
	PrimaryModel  string
	FallbackModel string
	HTTPClient    *http.Client
}

type DeepSeekParser struct {
	apiKey        string
	endpoint      string
	primaryModel  string
	fallbackModel string
	httpClient    *http.Client
}

type StructuredProfilePayload struct {
	Roles             []string                    `json:"roles"`
	Skills            []string                    `json:"skills"`
	YearsOfExperience float64                     `json:"years_of_experience"`
	Seniority         string                      `json:"seniority"`
	Domains           []string                    `json:"domains"`
	Education         []model.EducationRecord     `json:"education"`
	Certifications    []model.CertificationRecord `json:"certifications"`
}

type deepSeekCompletionRequest struct {
	Model          string                 `json:"model"`
	Messages       []deepSeekMessage      `json:"messages"`
	Temperature    float64                `json:"temperature"`
	ResponseFormat deepSeekResponseFormat `json:"response_format"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekResponseFormat struct {
	Type string `json:"type"`
}

type deepSeekCompletionResponse struct {
	Choices []struct {
		Message deepSeekMessage `json:"message"`
	} `json:"choices"`
}

func NewDeepSeekParser(config DeepSeekConfig) (*DeepSeekParser, error) {
	apiKey := strings.TrimSpace(config.APIKey)
	if apiKey == "" {
		return nil, errors.New("DeepSeek API key is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultDeepSeekBaseURL
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "https" && parsed.Scheme != "http" || parsed.Host == "" {
		return nil, errors.New("DeepSeek base URL is invalid")
	}
	endpoint := baseURL
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/v1/chat/completions"
	}
	primaryModel := strings.TrimSpace(config.PrimaryModel)
	if primaryModel == "" {
		primaryModel = DefaultDeepSeekPrimaryModel
	}
	fallbackModel := strings.TrimSpace(config.FallbackModel)
	if fallbackModel == "" {
		fallbackModel = DefaultDeepSeekFallbackModel
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	return &DeepSeekParser{apiKey: apiKey, endpoint: endpoint, primaryModel: primaryModel, fallbackModel: fallbackModel, httpClient: httpClient}, nil
}

func (p *DeepSeekParser) Parse(ctx context.Context, cvText string) (model.StructuredProfile, string, error) {
	sanitized := sanitizeCVText(cvText)
	if sanitized == "" {
		return model.StructuredProfile{}, "", &ProcessingError{Code: "cv_text_empty", Err: errors.New("CV text is empty")}
	}
	var lastErr error
	for _, modelName := range []string{p.primaryModel, p.fallbackModel} {
		profile, err := p.parseWithModel(ctx, modelName, sanitized)
		if err == nil {
			return profile, modelName, nil
		}
		lastErr = err
	}
	return model.StructuredProfile{}, "", &ProcessingError{Code: "deepseek_parse_failed", Err: lastErr}
}

func (p *DeepSeekParser) parseWithModel(ctx context.Context, modelName, cvText string) (model.StructuredProfile, error) {
	payload := deepSeekCompletionRequest{
		Model: modelName,
		Messages: []deepSeekMessage{
			{Role: "system", Content: profileSchemaPrompt},
			{Role: "user", Content: cvText},
		},
		Temperature:    0,
		ResponseFormat: deepSeekResponseFormat{Type: "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return model.StructuredProfile{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return model.StructuredProfile{}, err
	}
	request.Header.Set("Authorization", "Bearer "+p.apiKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := p.httpClient.Do(request)
	if err != nil {
		return model.StructuredProfile{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxProviderResponseBytes+1))
	if err != nil {
		return model.StructuredProfile{}, err
	}
	if len(responseBody) > maxProviderResponseBytes {
		return model.StructuredProfile{}, errors.New("DeepSeek response is too large")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return model.StructuredProfile{}, fmt.Errorf("DeepSeek returned HTTP %d", response.StatusCode)
	}
	var completion deepSeekCompletionResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil || len(completion.Choices) == 0 {
		return model.StructuredProfile{}, errors.New("DeepSeek response has no completion")
	}
	content := stripJSONFence(completion.Choices[0].Message.Content)
	var profile StructuredProfilePayload
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&profile); err != nil {
		return model.StructuredProfile{}, errors.New("DeepSeek completion is not valid profile JSON")
	}
	if err := profile.Validate(); err != nil {
		return model.StructuredProfile{}, err
	}
	return profile.ToModel(), nil
}

func (payload StructuredProfilePayload) Validate() error {
	if !finiteBetween(payload.YearsOfExperience, 0, 100) || !isSupportedSeniority(payload.Seniority) {
		return errors.New("structured profile scalar is invalid")
	}
	if err := validateStringList(payload.Roles, 30, 160); err != nil {
		return err
	}
	if err := validateStringList(payload.Skills, 80, 120); err != nil {
		return err
	}
	if err := validateStringList(payload.Domains, 40, 120); err != nil {
		return err
	}
	if len(payload.Education) > 20 || len(payload.Certifications) > 20 {
		return errors.New("structured profile records exceed limit")
	}
	for _, record := range payload.Education {
		if len(record.Institution) > 240 || len(record.Degree) > 160 || len(record.FieldOfStudy) > 160 || len(record.Grade) > 80 {
			return errors.New("education field exceeds limit")
		}
	}
	for _, record := range payload.Certifications {
		if len(record.CertificateName) > 200 || len(record.Issuer) > 200 {
			return errors.New("certification field exceeds limit")
		}
	}
	return nil
}

func (payload StructuredProfilePayload) ToModel() model.StructuredProfile {
	return model.StructuredProfile{
		Roles:             payload.Roles,
		Skills:            payload.Skills,
		YearsOfExperience: payload.YearsOfExperience,
		Seniority:         strings.ToUpper(strings.TrimSpace(payload.Seniority)),
		Domains:           payload.Domains,
		Education:         payload.Education,
		Certifications:    payload.Certifications,
	}
}

func validateStringList(values []string, maxItems, maxLength int) error {
	if len(values) > maxItems {
		return errors.New("structured profile list exceeds limit")
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > maxLength {
			return errors.New("structured profile list value is invalid")
		}
	}
	return nil
}

func finiteBetween(value, minimum, maximum float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= minimum && value <= maximum
}

func isSupportedSeniority(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "JUNIOR", "MID", "SENIOR", "UNSPECIFIED":
		return true
	default:
		return false
	}
}

var (
	emailPattern        = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	phonePattern        = regexp.MustCompile(`(?i)(?:\+?84|0)[\s().\-]*\d[\d\s().\-]{7,}\d`)
	identityLinePattern = regexp.MustCompile(`(?im)^\s*(?:name|full name|họ tên|ho va ten|email|phone|điện thoại|dien thoai|address|địa chỉ|dia chi)\s*[:\-].*$`)
)

func sanitizeCVText(value string) string {
	value = identityLinePattern.ReplaceAllString(value, "")
	value = emailPattern.ReplaceAllString(value, "[REDACTED_EMAIL]")
	value = phonePattern.ReplaceAllString(value, "[REDACTED_PHONE]")
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxCVTextRunes {
		runes := []rune(value)
		value = string(runes[:maxCVTextRunes])
	}
	return value
}

func stripJSONFence(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "```") {
		if newline := strings.IndexByte(value, '\n'); newline >= 0 {
			value = value[newline+1:]
		}
		value = strings.TrimSuffix(strings.TrimSpace(value), "```")
	}
	return strings.TrimSpace(value)
}

const profileSchemaPrompt = `Return exactly one JSON object and no markdown. Use this schema:
{"roles":["string"],"skills":["string"],"years_of_experience":0,"seniority":"JUNIOR|MID|SENIOR|UNSPECIFIED","domains":["string"],"education":[{"institution":"string","degree":"string","field_of_study":"string","start_year":2020,"end_year":2024,"grade":"string"}],"certifications":[{"certificate_name":"string","issuer":"string","issued_year":2020,"expires_year":2025}]}
Do not return name, email, phone, address, photo, raw CV text, or any field outside the schema. Use empty arrays when a field is not present. Keep years_of_experience between 0 and 100.`
