package config

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress                = "127.0.0.1:8080"
	defaultEnvironment            = "development"
	defaultMaxCVBytes             = int64(10 * 1024 * 1024)
	defaultMaxRadiusKm            = float64(500)
	defaultRateLimitPerMinute     = 10
	defaultReadRateLimitPerMinute = 60
	defaultMaxPromotionImageBytes = int64(5 * 1024 * 1024)
	defaultPromotionRateLimit     = 10
	defaultAdminSessionTTL        = 12 * time.Hour
	defaultAdminLoginRateLimit    = 5
	defaultAdminCookieName        = "ferris_admin_session"
	defaultClientSessionTTL       = 30 * 24 * time.Hour
	defaultClientCookieName       = "ferris_client_session"
	defaultDatabaseHost           = "127.0.0.1"
	defaultDatabasePort           = "5432"
	defaultDatabaseName           = "gogetsomefoodferris"
	defaultDeepSeekBaseURL        = "https://api.deepseek.com"
	defaultDeepSeekPrimaryModel   = "deepseek-v4-flash"
	defaultDeepSeekFallbackModel  = "deepseek-v4-pro"
)

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

// Config contains the validated runtime settings shared by the HTTP server,
// scan service, and promotion upload boundary.
//
// DatabaseURL is kept as one validated connection string so the rest of the
// application never needs to know whether it came from DATABASE_URL or the
// developer-friendly fields in backend/.env.
type Config struct {
	Address                      string
	DatabaseURL                  string
	Environment                  string
	MaxCVBytes                   int64
	MaxRadiusKm                  float64
	AllowedOrigins               []string
	RateLimitPerMinute           int
	ReadRateLimitPerMinute       int
	MaxPromotionImageBytes       int64
	PromotionRateLimitPerMinute  int
	CloudinaryURL                string
	AdminSessionTTL              time.Duration
	AdminCookieName              string
	AdminCookieSecure            bool
	AdminLoginRateLimitPerMinute int
	DeepSeekAPIKey               string
	DeepSeekBaseURL              string
	DeepSeekPrimaryModel         string
	DeepSeekFallbackModel        string
	GoogleClientID               string
	GoogleClientSecret           string
	GoogleRedirectURL            string
	ClientSessionTTL             time.Duration
	ClientCookieName             string
	ClientCookieSecure           bool
	ClientRedirectOrigin         string
}

// Load reads configuration only from the current process environment.
//
// This is the production-safe path: a container, service manager, or secret
// manager owns the values and no local file is implicitly consulted.
func Load() (Config, error) {
	return loadConfig(os.Getenv, false)
}

// LoadLocal reads an optional local .env file and then validates the resulting
// configuration. Explicit process environment variables take precedence over
// values from the file, which lets a deployment override local defaults.
//
// The lookup order supports both commands documented in this repository:
// running from backend/ finds .env, while running from the repository root
// finds backend/.env. BACKEND_ENV_FILE can point to another local file when a
// developer needs a separate database without changing source code.
func LoadLocal() (Config, error) {
	if explicitPath := strings.TrimSpace(os.Getenv("BACKEND_ENV_FILE")); explicitPath != "" {
		return LoadFromDotEnv(explicitPath)
	}

	for _, candidate := range []string{".env", filepath.Join("backend", ".env")} {
		_, err := os.Stat(candidate)
		if err == nil {
			return LoadFromDotEnv(candidate)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("inspect local environment file: %w", err)
		}
	}

	return Load()
}

// LoadFromDotEnv loads one local dotenv-style file without mutating the
// process environment. Keeping file values in a private map prevents a
// development secret from leaking into child processes or unrelated tests.
func LoadFromDotEnv(path string) (Config, error) {
	fileValues, err := readDotEnvFile(path)
	if err != nil {
		return Config{}, err
	}

	lookup := func(key string) string {
		normalizedKey := strings.ToUpper(key)
		// Generic names such as USERNAME and PORT are commonly injected by the
		// operating system or a shell. They are valid local .env aliases but
		// must not silently override the developer's database file.
		if !isGenericLocalDatabaseKey(normalizedKey) {
			if value := strings.TrimSpace(os.Getenv(normalizedKey)); value != "" {
				return value
			}
		}
		return fileValues[normalizedKey]
	}
	return loadConfig(lookup, true)
}

func loadConfig(lookup func(string) string, includeLegacyDatabaseKeys bool) (Config, error) {
	databaseURL, err := databaseURLFrom(lookup, includeLegacyDatabaseKeys)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Address:                      valueOrDefault(lookup, "API_ADDR", defaultAddress),
		DatabaseURL:                  databaseURL,
		Environment:                  valueOrDefault(lookup, "APP_ENV", defaultEnvironment),
		MaxCVBytes:                   defaultMaxCVBytes,
		MaxRadiusKm:                  defaultMaxRadiusKm,
		AllowedOrigins:               append([]string(nil), defaultAllowedOrigins...),
		RateLimitPerMinute:           defaultRateLimitPerMinute,
		ReadRateLimitPerMinute:       defaultReadRateLimitPerMinute,
		MaxPromotionImageBytes:       defaultMaxPromotionImageBytes,
		PromotionRateLimitPerMinute:  defaultPromotionRateLimit,
		CloudinaryURL:                strings.TrimSpace(lookup("CLOUDINARY_URL")),
		AdminSessionTTL:              defaultAdminSessionTTL,
		AdminCookieName:              valueOrDefault(lookup, "ADMIN_COOKIE_NAME", defaultAdminCookieName),
		AdminCookieSecure:            cfgCookieSecure(lookup, valueOrDefault(lookup, "APP_ENV", defaultEnvironment)),
		AdminLoginRateLimitPerMinute: defaultAdminLoginRateLimit,
		DeepSeekAPIKey:               strings.TrimSpace(lookup("DEEPSEEK_API_KEY")),
		DeepSeekBaseURL:              valueOrDefault(lookup, "DEEPSEEK_BASE_URL", defaultDeepSeekBaseURL),
		DeepSeekPrimaryModel:         valueOrDefault(lookup, "DEEPSEEK_PRIMARY_MODEL", defaultDeepSeekPrimaryModel),
		DeepSeekFallbackModel:        valueOrDefault(lookup, "DEEPSEEK_FALLBACK_MODEL", defaultDeepSeekFallbackModel),
		GoogleClientID:               strings.TrimSpace(lookup("GOOGLE_CLIENT_ID")),
		GoogleClientSecret:           strings.TrimSpace(lookup("GOOGLE_CLIENT_SECRET")),
		GoogleRedirectURL:            strings.TrimSpace(lookup("GOOGLE_REDIRECT_URL")),
		ClientSessionTTL:             defaultClientSessionTTL,
		ClientCookieName:             valueOrDefault(lookup, "CLIENT_COOKIE_NAME", defaultClientCookieName),
		ClientCookieSecure:           cfgClientCookieSecure(lookup, valueOrDefault(lookup, "APP_ENV", defaultEnvironment)),
		ClientRedirectOrigin:         strings.TrimSpace(lookup("CLIENT_REDIRECT_ORIGIN")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if strings.TrimSpace(cfg.AdminCookieName) == "" || strings.ContainsAny(cfg.AdminCookieName, " \t\r\n;=") {
		return Config{}, fmt.Errorf("invalid ADMIN_COOKIE_NAME")
	}
	if !isLoopbackAddress(cfg.Address) {
		return Config{}, fmt.Errorf("API_ADDR must use a loopback address until scan authentication exists")
	}

	if raw := strings.TrimSpace(lookup("MAX_CV_BYTES")); raw != "" {
		cfg.MaxCVBytes, err = parsePositiveInt64("MAX_CV_BYTES", raw)
		if err != nil || cfg.MaxCVBytes > 100*1024*1024 {
			return Config{}, fmt.Errorf("invalid MAX_CV_BYTES")
		}
	}
	if raw := strings.TrimSpace(lookup("MAX_RADIUS_KM")); raw != "" {
		cfg.MaxRadiusKm, err = parsePositiveFloat("MAX_RADIUS_KM", raw)
		if err != nil || cfg.MaxRadiusKm > 10000 {
			return Config{}, fmt.Errorf("invalid MAX_RADIUS_KM")
		}
	}
	if raw := strings.TrimSpace(lookup("RATE_LIMIT_PER_MINUTE")); raw != "" {
		cfg.RateLimitPerMinute, err = parsePositiveInt("RATE_LIMIT_PER_MINUTE", raw)
		if err != nil || cfg.RateLimitPerMinute > 10000 {
			return Config{}, fmt.Errorf("invalid RATE_LIMIT_PER_MINUTE")
		}
	}
	if raw := strings.TrimSpace(lookup("READ_RATE_LIMIT_PER_MINUTE")); raw != "" {
		cfg.ReadRateLimitPerMinute, err = parsePositiveInt("READ_RATE_LIMIT_PER_MINUTE", raw)
		if err != nil || cfg.ReadRateLimitPerMinute > 100000 {
			return Config{}, fmt.Errorf("invalid READ_RATE_LIMIT_PER_MINUTE")
		}
	}
	if raw := strings.TrimSpace(lookup("MAX_PROMOTION_IMAGE_BYTES")); raw != "" {
		cfg.MaxPromotionImageBytes, err = parsePositiveInt64("MAX_PROMOTION_IMAGE_BYTES", raw)
		if err != nil || cfg.MaxPromotionImageBytes > 50*1024*1024 {
			return Config{}, fmt.Errorf("invalid MAX_PROMOTION_IMAGE_BYTES")
		}
	}
	if raw := strings.TrimSpace(lookup("PROMOTION_RATE_LIMIT_PER_MINUTE")); raw != "" {
		cfg.PromotionRateLimitPerMinute, err = parsePositiveInt("PROMOTION_RATE_LIMIT_PER_MINUTE", raw)
		if err != nil || cfg.PromotionRateLimitPerMinute > 10000 {
			return Config{}, fmt.Errorf("invalid PROMOTION_RATE_LIMIT_PER_MINUTE")
		}
	}
	if raw := strings.TrimSpace(lookup("ADMIN_SESSION_TTL")); raw != "" {
		cfg.AdminSessionTTL, err = time.ParseDuration(raw)
		if err != nil || cfg.AdminSessionTTL < 15*time.Minute || cfg.AdminSessionTTL > 7*24*time.Hour {
			return Config{}, fmt.Errorf("invalid ADMIN_SESSION_TTL")
		}
	}
	if raw := strings.TrimSpace(lookup("ADMIN_COOKIE_SECURE")); raw != "" {
		cfg.AdminCookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid ADMIN_COOKIE_SECURE")
		}
	}
	if raw := strings.TrimSpace(lookup("ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE")); raw != "" {
		cfg.AdminLoginRateLimitPerMinute, err = parsePositiveInt("ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE", raw)
		if err != nil || cfg.AdminLoginRateLimitPerMinute > 1000 {
			return Config{}, fmt.Errorf("invalid ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE")
		}
	}
	if raw := strings.TrimSpace(lookup("CORS_ALLOWED_ORIGINS")); raw != "" {
		cfg.AllowedOrigins = splitOrigins(raw)
	}
	if len(cfg.AllowedOrigins) == 0 || containsWildcard(cfg.AllowedOrigins) {
		return Config{}, fmt.Errorf("invalid CORS_ALLOWED_ORIGINS")
	}
	if err := validateProviderURL(cfg.DeepSeekBaseURL); err != nil {
		return Config{}, fmt.Errorf("invalid DEEPSEEK_BASE_URL")
	}
	if !validModelName(cfg.DeepSeekPrimaryModel) || !validModelName(cfg.DeepSeekFallbackModel) {
		return Config{}, fmt.Errorf("invalid DeepSeek model")
	}
	if err := validateGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL, cfg.Environment, lookup); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(cfg.ClientCookieName) == "" || strings.ContainsAny(cfg.ClientCookieName, " \t\r\n;=") {
		return Config{}, fmt.Errorf("invalid CLIENT_COOKIE_NAME")
	}
	if cfg.ClientCookieName == cfg.AdminCookieName {
		return Config{}, fmt.Errorf("admin and client cookie names must be different")
	}
	if raw := strings.TrimSpace(lookup("CLIENT_SESSION_TTL")); raw != "" {
		cfg.ClientSessionTTL, err = time.ParseDuration(raw)
		if err != nil || cfg.ClientSessionTTL < 15*time.Minute || cfg.ClientSessionTTL > 7*24*time.Hour {
			return Config{}, fmt.Errorf("invalid CLIENT_SESSION_TTL")
		}
	}
	if raw := strings.TrimSpace(lookup("CLIENT_COOKIE_SECURE")); raw != "" {
		cfg.ClientCookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CLIENT_COOKIE_SECURE")
		}
	}

	return cfg, nil
}

func validateProviderURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return errors.New("invalid provider URL")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" {
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		if strings.EqualFold(host, "localhost") || ip != nil && ip.IsLoopback() {
			return nil
		}
	}
	return errors.New("invalid provider URL")
}

func validModelName(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 120 && !strings.ContainsAny(value, " \t\r\n")
}

// cfgCookieSecure defaults to a local HTTP-compatible cookie while making a
// production deployment use HTTPS-only session cookies. An explicit setting is
// parsed again below so it always wins over the environment default.
func cfgCookieSecure(lookup func(string) string, environment string) bool {
	if raw := strings.TrimSpace(lookup("ADMIN_COOKIE_SECURE")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		return err == nil && parsed
	}
	return strings.EqualFold(strings.TrimSpace(environment), "production")
}

// cfgClientCookieSecure mirrors cfgCookieSecure for the separate client
// session cookie so dev local HTTP works while production stays Secure.
func cfgClientCookieSecure(lookup func(string) string, environment string) bool {
	if raw := strings.TrimSpace(lookup("CLIENT_COOKIE_SECURE")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		return err == nil && parsed
	}
	return strings.EqualFold(strings.TrimSpace(environment), "production")
}

// validateGoogleOAuth requires all three Google OAuth values when client login
// is enabled. Google client login is considered enabled whenever a client ID is
// present; the secret and redirect URL must then be complete. In production the
// redirect URL must be HTTPS so the authorization code never transits plaintext;
// local development may use plain HTTP for the loopback callback.
func validateGoogleOAuth(clientID, clientSecret, redirectURL string, environment string, lookup func(string) string) error {
	if strings.TrimSpace(clientID) == "" {
		return nil // Google client login not configured; endpoints will 503.
	}
	if strings.TrimSpace(clientSecret) == "" {
		return fmt.Errorf("GOOGLE_CLIENT_SECRET is required when GOOGLE_CLIENT_ID is set")
	}
	parsed, err := url.Parse(strings.TrimSpace(redirectURL))
	if err != nil {
		return fmt.Errorf("invalid GOOGLE_REDIRECT_URL")
	}
	validScheme := parsed.Scheme == "https"
	if environment == "" || !strings.EqualFold(strings.TrimSpace(environment), "production") {
		validScheme = validScheme || parsed.Scheme == "http"
	}
	if !validScheme || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("invalid GOOGLE_REDIRECT_URL")
	}
	return nil
}

func valueOrDefault(lookup func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(lookup(key)); value != "" {
		return value
	}
	return fallback
}

func databaseURLFrom(lookup func(string) string, includeLegacyDatabaseKeys bool) (string, error) {
	if value := strings.TrimSpace(lookup("DATABASE_URL")); value != "" {
		return value, nil
	}

	username := firstNonEmpty(lookup, "DATABASE_USER", "DB_USER")
	password := firstNonEmpty(lookup, "DATABASE_PASSWORD", "DB_PASSWORD")
	if includeLegacyDatabaseKeys {
		username = firstNonEmpty(lookup, "DATABASE_USER", "DB_USER", "USERNAME")
		password = firstNonEmpty(lookup, "DATABASE_PASSWORD", "DB_PASSWORD", "PASSWORD")
	}
	if username == "" && password == "" {
		return "", fmt.Errorf("DATABASE_URL is required")
	}
	if username == "" || password == "" {
		return "", fmt.Errorf("DATABASE_URL or complete local PostgreSQL connection fields are required")
	}

	host := firstNonEmpty(lookup, "DATABASE_HOST", "DB_HOST")
	port := firstNonEmpty(lookup, "DATABASE_PORT", "DB_PORT")
	if includeLegacyDatabaseKeys {
		host = firstNonEmpty(lookup, "DATABASE_HOST", "DB_HOST", "HOST")
		port = firstNonEmpty(lookup, "DATABASE_PORT", "DB_PORT", "PORT")
	}
	if host == "" {
		host = defaultDatabaseHost
	}
	if port == "" {
		port = defaultDatabasePort
	}
	if err := validateDatabaseHostPort(host, port); err != nil {
		return "", err
	}

	databaseName := firstNonEmpty(lookup, "DATABASE_NAME", "DB_NAME")
	if includeLegacyDatabaseKeys {
		databaseName = firstNonEmpty(lookup, "DATABASE_NAME", "DB_NAME", "DATABASE")
	}
	if databaseName == "" {
		databaseName = defaultDatabaseName
	}
	if strings.ContainsAny(databaseName, "/?#\x00\r\n") || strings.TrimSpace(databaseName) == "" {
		return "", fmt.Errorf("invalid database name")
	}

	// url.UserPassword performs the escaping needed when a local password
	// contains characters such as @ or :. The password never appears in an
	// application log; it exists only in the in-memory connection config.
	connectionURL := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + databaseName,
		User:   url.UserPassword(username, password),
	}
	query := connectionURL.Query()
	query.Set("sslmode", "disable")
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String(), nil
}

func isGenericLocalDatabaseKey(key string) bool {
	switch key {
	case "DATABASE", "HOST", "PASSWORD", "PORT", "USER", "USERNAME":
		return true
	default:
		return false
	}
}

func firstNonEmpty(lookup func(string) string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(lookup(key)); value != "" {
			return value
		}
	}
	return ""
}

func validateDatabaseHostPort(host, port string) error {
	if strings.ContainsAny(host, " \t\r\n/?#\x00") {
		return fmt.Errorf("invalid database host")
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return fmt.Errorf("invalid database port")
	}
	return nil
}

func readDotEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read local environment file: %w", err)
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 64*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		separator := strings.IndexByte(line, '=')
		if separator <= 0 {
			return nil, fmt.Errorf("invalid local environment entry at line %d", lineNumber)
		}
		key := strings.ToUpper(strings.TrimSpace(line[:separator]))
		if !validEnvironmentKey(key) {
			return nil, fmt.Errorf("invalid local environment key at line %d", lineNumber)
		}
		values[key] = strings.Trim(strings.TrimSpace(line[separator+1:]), "\"'")
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read local environment file: %w", err)
	}
	return values, nil
}

func validEnvironmentKey(value string) bool {
	if value == "" || (value[0] != '_' && (value[0] < 'A' || value[0] > 'Z')) {
		return false
	}
	for _, character := range value[1:] {
		if character != '_' && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func parsePositiveInt64(key, raw string) (int64, error) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func parsePositiveInt(key, raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func parsePositiveFloat(key, raw string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func splitOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if origin := strings.TrimSpace(part); origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func containsWildcard(origins []string) bool {
	for _, origin := range origins {
		if origin == "*" {
			return true
		}
	}
	return false
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
