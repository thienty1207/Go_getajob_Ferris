package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want missing DATABASE_URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("Load() error = %q, want DATABASE_URL in configuration error", err)
	}
}

func TestLoadUsesSafeLocalDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:secret@127.0.0.1:5432/jobs")
	for _, key := range []string{
		"API_ADDR",
		"APP_ENV",
		"MAX_CV_BYTES",
		"MAX_RADIUS_KM",
		"CORS_ALLOWED_ORIGINS",
		"RATE_LIMIT_PER_MINUTE",
		"READ_RATE_LIMIT_PER_MINUTE",
		"MAX_PROMOTION_IMAGE_BYTES",
		"PROMOTION_RATE_LIMIT_PER_MINUTE",
		"CLOUDINARY_URL",
		"ADMIN_SESSION_TTL",
		"ADMIN_COOKIE_NAME",
		"ADMIN_COOKIE_SECURE",
		"ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE",
		"DEEPSEEK_API_KEY",
		"DEEPSEEK_BASE_URL",
		"DEEPSEEK_PRIMARY_MODEL",
		"DEEPSEEK_FALLBACK_MODEL",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != "127.0.0.1:8080" {
		t.Fatalf("Address = %q, want local default", cfg.Address)
	}
	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development", cfg.Environment)
	}
	if cfg.MaxCVBytes != 10*1024*1024 {
		t.Fatalf("MaxCVBytes = %d, want 10 MiB", cfg.MaxCVBytes)
	}
	if cfg.MaxRadiusKm != 500 {
		t.Fatalf("MaxRadiusKm = %v, want 500", cfg.MaxRadiusKm)
	}
	if cfg.RateLimitPerMinute != 10 {
		t.Fatalf("RateLimitPerMinute = %d, want 10", cfg.RateLimitPerMinute)
	}
	if cfg.ReadRateLimitPerMinute != 60 {
		t.Fatalf("ReadRateLimitPerMinute = %d, want 60", cfg.ReadRateLimitPerMinute)
	}
	if cfg.MaxPromotionImageBytes != 5*1024*1024 {
		t.Fatalf("MaxPromotionImageBytes = %d, want 5 MiB", cfg.MaxPromotionImageBytes)
	}
	if cfg.PromotionRateLimitPerMinute != 10 {
		t.Fatalf("PromotionRateLimitPerMinute = %d, want 10", cfg.PromotionRateLimitPerMinute)
	}
	if cfg.AdminSessionTTL != 12*time.Hour || cfg.AdminCookieName != "ferris_admin_session" || cfg.AdminCookieSecure || cfg.AdminLoginRateLimitPerMinute != 5 {
		t.Fatalf("admin config defaults = ttl:%v cookie:%q secure:%v limit:%d", cfg.AdminSessionTTL, cfg.AdminCookieName, cfg.AdminCookieSecure, cfg.AdminLoginRateLimitPerMinute)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "http://localhost:5173" || cfg.AllowedOrigins[1] != "http://127.0.0.1:5173" {
		t.Fatalf("AllowedOrigins = %#v, want the two local frontend origins", cfg.AllowedOrigins)
	}
}

func TestLoadParsesConfiguredValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:secret@localhost:5432/jobs")
	t.Setenv("API_ADDR", "127.0.0.1:9090")
	t.Setenv("APP_ENV", "test")
	t.Setenv("MAX_CV_BYTES", "2048")
	t.Setenv("MAX_RADIUS_KM", "75.5")
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://client.example , https://admin.example ")
	t.Setenv("RATE_LIMIT_PER_MINUTE", "25")
	t.Setenv("READ_RATE_LIMIT_PER_MINUTE", "90")
	t.Setenv("MAX_PROMOTION_IMAGE_BYTES", "4096")
	t.Setenv("PROMOTION_RATE_LIMIT_PER_MINUTE", "7")
	t.Setenv("CLOUDINARY_URL", "cloudinary://key:secret@cloud")
	t.Setenv("ADMIN_SESSION_TTL", "2h")
	t.Setenv("ADMIN_COOKIE_NAME", "custom_admin")
	t.Setenv("ADMIN_COOKIE_SECURE", "true")
	t.Setenv("ADMIN_LOGIN_RATE_LIMIT_PER_MINUTE", "8")
	t.Setenv("DEEPSEEK_API_KEY", "deepseek-test-key")
	t.Setenv("DEEPSEEK_BASE_URL", "https://deepseek.example")
	t.Setenv("DEEPSEEK_PRIMARY_MODEL", "deepseek-test-flash")
	t.Setenv("DEEPSEEK_FALLBACK_MODEL", "deepseek-test-pro")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != "127.0.0.1:9090" || cfg.Environment != "test" || cfg.MaxCVBytes != 2048 || cfg.MaxRadiusKm != 75.5 || cfg.RateLimitPerMinute != 25 || cfg.ReadRateLimitPerMinute != 90 || cfg.MaxPromotionImageBytes != 4096 || cfg.PromotionRateLimitPerMinute != 7 || cfg.CloudinaryURL != "cloudinary://key:secret@cloud" || cfg.AdminSessionTTL != 2*time.Hour || cfg.AdminCookieName != "custom_admin" || !cfg.AdminCookieSecure || cfg.AdminLoginRateLimitPerMinute != 8 || cfg.DeepSeekAPIKey != "deepseek-test-key" || cfg.DeepSeekBaseURL != "https://deepseek.example" || cfg.DeepSeekPrimaryModel != "deepseek-test-flash" || cfg.DeepSeekFallbackModel != "deepseek-test-pro" {
		t.Fatalf("Load() parsed unexpected config: %#v", cfg)
	}
	wantOrigins := []string{"https://client.example", "https://admin.example"}
	if len(cfg.AllowedOrigins) != len(wantOrigins) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", cfg.AllowedOrigins, wantOrigins)
	}
	for i, want := range wantOrigins {
		if cfg.AllowedOrigins[i] != want {
			t.Fatalf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], want)
		}
	}
}

func TestLoadRejectsNonLocalAPIAddressUntilAuthenticationExists(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://app:secret@localhost:5432/jobs")
	t.Setenv("API_ADDR", "0.0.0.0:9090")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil for a public bind address")
	}
}

func TestLoadRejectsNonFiniteRadiusLimit(t *testing.T) {
	for _, raw := range []string{"NaN", "+Inf", "-Inf"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://app:secret@localhost:5432/jobs")
			t.Setenv("MAX_RADIUS_KM", raw)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil for MAX_RADIUS_KM=%q", raw)
			}
		})
	}
}

func TestLoadFromDotEnvBuildsLocalPostgresURLAndKeepsProcessEnvironmentPrecedence(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	envContent := "port=5432\nusername=postgres\npassword=file-secret\nhost=127.0.0.1\ndatabase=gogetsomefoodferris\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("write test .env: %v", err)
	}

	for _, key := range []string{
		"DATABASE_URL",
		"DATABASE_HOST",
		"DATABASE_PORT",
		"DATABASE_USER",
		"DATABASE_PASSWORD",
		"DATABASE_NAME",
		"DB_HOST",
		"DB_PORT",
		"DB_USER",
		"DB_PASSWORD",
		"DB_NAME",
		"HOST",
		"PORT",
		"USERNAME",
		"PASSWORD",
		"DATABASE",
	} {
		t.Setenv(key, "")
	}

	cfg, err := LoadFromDotEnv(envPath)
	if err != nil {
		t.Fatalf("LoadFromDotEnv() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://postgres:file-secret@127.0.0.1:5432/gogetsomefoodferris?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want URL built from local .env fields", cfg.DatabaseURL)
	}

	t.Setenv("DATABASE_URL", "postgres://override:secret@127.0.0.1:5432/override")
	cfg, err = LoadFromDotEnv(envPath)
	if err != nil {
		t.Fatalf("LoadFromDotEnv() with process override error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://override:secret@127.0.0.1:5432/override" {
		t.Fatalf("DatabaseURL = %q, want process environment to override .env", cfg.DatabaseURL)
	}
}

func TestLoadFromDotEnvIgnoresGenericOperatingSystemVariablesForDatabaseFields(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	envContent := "port=5432\nusername=postgres\npassword=file-secret\ndatabase=gogetsomefoodferris\n"
	if err := os.WriteFile(envPath, []byte(envContent), 0o600); err != nil {
		t.Fatalf("write test .env: %v", err)
	}

	t.Setenv("USERNAME", "operating-system-user")
	t.Setenv("PASSWORD", "")
	t.Setenv("HOST", "")
	t.Setenv("PORT", "")
	t.Setenv("DATABASE", "")
	t.Setenv("DATABASE_URL", "")

	cfg, err := LoadFromDotEnv(envPath)
	if err != nil {
		t.Fatalf("LoadFromDotEnv() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://postgres:file-secret@127.0.0.1:5432/gogetsomefoodferris?sslmode=disable" {
		t.Fatalf("DatabaseURL = %q, want generic operating-system variables ignored", cfg.DatabaseURL)
	}
}
