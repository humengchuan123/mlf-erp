package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App         AppConfig
	HTTP        HTTPConfig
	DB          DBConfig
	Redis       RedisConfig
	Auth        AuthConfig
	Idempotency IdempotencyConfig
	Log         LogConfig
}

type AppConfig struct {
	Name            string
	Env             string
	DeliveryMode    string
	DefaultTenantID string
}

type HTTPConfig struct {
	Addr               string
	ShutdownTimeout    time.Duration
	CORSAllowedOrigins []string
	TrustedProxies     []string
}

type DBConfig struct {
	Enabled        bool
	Required       bool
	DSN            string
	MaxConns       int32
	ConnectTimeout time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AuthConfig struct {
	JWTSecret       string
	JWTIssuer       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type IdempotencyConfig struct {
	TTL              time.Duration
	Required         bool
	MaxBodyBytes     int64
	MaxResponseBytes int64
	CleanupInterval  time.Duration
}

type LogConfig struct {
	Level string
}

func Load() (*Config, error) {
	env := getString("APP_ENV", "dev")

	cfg := &Config{
		App: AppConfig{
			Name:            getString("APP_NAME", "mlf-erp"),
			Env:             env,
			DeliveryMode:    getString("APP_DELIVERY_MODE", "saas"),
			DefaultTenantID: getString("DEFAULT_TENANT_ID", "00000000-0000-0000-0000-000000000001"),
		},
		HTTP: HTTPConfig{
			Addr:               getString("HTTP_ADDR", ":8080"),
			ShutdownTimeout:    getDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
			CORSAllowedOrigins: getCSV("CORS_ALLOWED_ORIGINS", []string{"http://localhost:5173"}),
			TrustedProxies:     getCSV("HTTP_TRUSTED_PROXIES", nil),
		},
		DB: DBConfig{
			Enabled:        getBool("DB_ENABLED", true),
			Required:       getBool("DB_REQUIRED", true),
			DSN:            getString("DATABASE_URL", "postgres://mlf:mlf@localhost:5432/mlf_erp?sslmode=disable"),
			MaxConns:       int32(getInt("DB_MAX_CONNS", 10)),
			ConnectTimeout: getDuration("DB_CONNECT_TIMEOUT", 5*time.Second),
		},
		Redis: RedisConfig{
			Addr:     getString("REDIS_ADDR", "localhost:6379"),
			Password: getString("REDIS_PASSWORD", ""),
			DB:       getInt("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			JWTSecret:       getString("JWT_SECRET", "dev-only-change-me"),
			JWTIssuer:       getString("JWT_ISSUER", "mlf-erp"),
			AccessTokenTTL:  getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
			RefreshTokenTTL: getDuration("REFRESH_TOKEN_TTL", 168*time.Hour),
		},
		Idempotency: IdempotencyConfig{
			TTL:              getDuration("IDEMPOTENCY_TTL", 24*time.Hour),
			Required:         getBool("IDEMPOTENCY_REQUIRED", false),
			MaxBodyBytes:     int64(getInt("IDEMPOTENCY_MAX_BODY_BYTES", 1<<20)),
			MaxResponseBytes: int64(getInt("IDEMPOTENCY_MAX_RESPONSE_BYTES", 4<<20)),
			CleanupInterval:  getDuration("IDEMPOTENCY_CLEANUP_INTERVAL", time.Hour),
		},
		Log: LogConfig{
			Level: getString("LOG_LEVEL", "info"),
		},
	}

	if cfg.App.Name == "" {
		return nil, fmt.Errorf("APP_NAME must not be empty")
	}
	if cfg.HTTP.Addr == "" {
		return nil, fmt.Errorf("HTTP_ADDR must not be empty")
	}
	if cfg.DB.Enabled && cfg.DB.DSN == "" {
		return nil, fmt.Errorf("DATABASE_URL must not be empty when DB_ENABLED=true")
	}
	if cfg.DB.MaxConns <= 0 {
		return nil, fmt.Errorf("DB_MAX_CONNS must be greater than zero")
	}
	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET must not be empty")
	}
	if env != "dev" && env != "test" {
		if cfg.Auth.JWTSecret == "dev-only-change-me" {
			return nil, fmt.Errorf("JWT_SECRET must be changed outside development")
		}
		if len(cfg.Auth.JWTSecret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters outside development")
		}
	}
	if cfg.Auth.AccessTokenTTL <= 0 {
		return nil, fmt.Errorf("ACCESS_TOKEN_TTL must be greater than zero")
	}
	if cfg.Auth.RefreshTokenTTL <= 0 {
		return nil, fmt.Errorf("REFRESH_TOKEN_TTL must be greater than zero")
	}
	if cfg.Idempotency.TTL <= 0 {
		return nil, fmt.Errorf("IDEMPOTENCY_TTL must be greater than zero")
	}
	if cfg.Idempotency.MaxBodyBytes <= 0 {
		return nil, fmt.Errorf("IDEMPOTENCY_MAX_BODY_BYTES must be greater than zero")
	}
	if cfg.Idempotency.MaxResponseBytes <= 0 {
		return nil, fmt.Errorf("IDEMPOTENCY_MAX_RESPONSE_BYTES must be greater than zero")
	}
	if cfg.Idempotency.CleanupInterval <= 0 {
		return nil, fmt.Errorf("IDEMPOTENCY_CLEANUP_INTERVAL must be greater than zero")
	}

	return cfg, nil
}

func getString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
