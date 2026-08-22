package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
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
	Summary           model.CVSummary             `json:"summary"`
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
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" && !isLoopbackHTTP(parsed) {
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
	// A successful first Decode does not guarantee that the provider returned
	// exactly one JSON value. Require EOF so a second object/value cannot be
	// silently ignored and later treated as validated structured profile data.
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return model.StructuredProfile{}, errors.New("DeepSeek completion contains trailing JSON content")
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
	if err := payload.Summary.Validate(); err != nil {
		return err
	}
	if summary := strings.Join(append(append(append([]string{payload.Summary.Headline, payload.Summary.Overview}, payload.Summary.TargetRoles...), payload.Summary.Strengths...), payload.Summary.Gaps...), " "); emailPattern.MatchString(summary) || phonePattern.MatchString(summary) || urlPattern.MatchString(summary) {
		return errors.New("summary contains disallowed personal data")
	}
	return nil
}

func (payload StructuredProfilePayload) ToModel() model.StructuredProfile {
	summary := payload.Summary
	return model.StructuredProfile{
		Roles:             payload.Roles,
		Skills:            payload.Skills,
		YearsOfExperience: payload.YearsOfExperience,
		Seniority:         strings.ToUpper(strings.TrimSpace(payload.Seniority)),
		Domains:           payload.Domains,
		Education:         payload.Education,
		Certifications:    payload.Certifications,
		Summary:           &summary,
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
	emailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	// The provider does not need any telephone number. This broader pattern
	// covers international formats instead of assuming every candidate uses a
	// Vietnamese prefix; eight digits avoids matching ordinary CV years.
	phonePattern        = regexp.MustCompile(`(?i)\+?\d(?:[\s().\-]*\d){7,14}`)
	urlPattern          = regexp.MustCompile(`(?i)(?:https?://|www\.)[^\s]+|(?:linkedin|github)\.com/[^\s]+`)
	identityLinePattern = regexp.MustCompile(`(?im)^\s*(?:name|full name|candidate|họ tên|ho va ten|email|e-mail|phone|mobile|telephone|điện thoại|dien thoai|date of birth|dob|birth date|ngày sinh|ngay sinh|id|identity|national id|cccd|cmnd|passport|address|địa chỉ|dia chi|photo|avatar|website|portfolio|linkedin|github)\s*[:\-].*$`)
	addressLinePattern  = regexp.MustCompile(`(?im)^\s*\d{1,5}\s+.*(?:street|road|avenue|district|ward|đường|duong|phường|phuong|quận|quan|thành phố|thanh pho|tp\.?\s*hcm|hà nội|ha noi).*$`)
)

func sanitizeCVText(value string) string {
	value = redactLikelyHeaderName(value)
	value = identityLinePattern.ReplaceAllString(value, "")
	value = addressLinePattern.ReplaceAllString(value, "")
	value = emailPattern.ReplaceAllString(value, "[REDACTED_EMAIL]")
	value = phonePattern.ReplaceAllString(value, "[REDACTED_PHONE]")
	value = urlPattern.ReplaceAllString(value, "[REDACTED_URL]")
	value = compactBlankLines(value)
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxCVTextRunes {
		runes := []rune(value)
		value = string(runes[:maxCVTextRunes])
	}
	return value
}

// redactLikelyHeaderName removes an unlabeled candidate name only when nearby
// contact details make the first line look like an identity header. Job titles
// are deliberately excluded so matching-relevant text remains available.
func redactLikelyHeaderName(value string) string {
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	first := -1
	contactNearby := false
	for index, line := range lines {
		if index >= 10 {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if first == -1 {
			first = index
		}
		if emailPattern.MatchString(trimmed) || phonePattern.MatchString(trimmed) || identityLinePattern.MatchString(trimmed) || urlPattern.MatchString(trimmed) {
			contactNearby = true
		}
	}
	if first >= 0 && contactNearby && looksLikePersonName(lines[first]) {
		lines[first] = ""
	}
	return strings.Join(lines, "\n")
}

func looksLikePersonName(value string) bool {
	words := strings.Fields(strings.TrimSpace(value))
	if len(words) < 2 || len(words) > 6 {
		return false
	}
	roleKeywords := []string{"engineer", "developer", "manager", "designer", "analyst", "consultant", "specialist", "intern", "director", "officer", "architect", "kỹ sư", "nhân viên", "chuyên viên", "quản lý", "thực tập"}
	lower := strings.ToLower(value)
	for _, keyword := range roleKeywords {
		if strings.Contains(lower, keyword) {
			return false
		}
	}
	for _, word := range words {
		hasLetter := false
		for _, character := range strings.Trim(word, "-'’.") {
			if !unicode.IsLetter(character) {
				return false
			}
			hasLetter = true
		}
		if !hasLetter {
			return false
		}
	}
	return true
}

func compactBlankLines(value string) string {
	lines := strings.Split(value, "\n")
	compacted := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		compacted = append(compacted, line)
	}
	return strings.Join(compacted, "\n")
}

func isLoopbackHTTP(parsed *url.URL) bool {
	if parsed == nil || parsed.Scheme != "http" {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
{"roles":["string"],"skills":["string"],"years_of_experience":0,"seniority":"JUNIOR|MID|SENIOR|UNSPECIFIED","domains":["string"],"education":[{"institution":"string","degree":"string","field_of_study":"string","start_year":2020,"end_year":2024,"grade":"string"}],"certifications":[{"certificate_name":"string","issuer":"string","issued_year":2020,"expires_year":2025}],"summary":{"headline":"string","overview":"string","target_roles":["string"],"strengths":["string"],"gaps":["string"]}}
The summary must be concise and useful for a job seeker: one headline, one short overview, one to five target roles, one to five strengths, and one to four gaps or next areas to improve. Do not return name, email, phone, address, photo, raw CV text, URLs, or any field outside the schema. Use empty arrays only for education or certifications; summary lists must contain useful bounded items. Keep years_of_experience between 0 and 100.`
