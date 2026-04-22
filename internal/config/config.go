package config

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           int
	PokeAPIBaseURL string
	HTTPTimeout    time.Duration
	CacheTTL       time.Duration
	RateLimitRPS   float64
	LogLevel       slog.Level

	YodaTranslatorURL        string
	ShakespeareTranslatorURL string
}

func Load() Config {
	cfg := Config{
		Port:           envInt("PORT", 8080),
		PokeAPIBaseURL: envStr("POKEAPI_BASE_URL", "https://pokeapi.co"),
		HTTPTimeout:    envDuration("HTTP_TIMEOUT", 10*time.Second),
		CacheTTL:       envDuration("CACHE_TTL", 5*time.Minute),
		RateLimitRPS:   envFloat("RATE_LIMIT_RPS", 10),
		LogLevel:       envLogLevel("LOG_LEVEL", slog.LevelInfo),

		YodaTranslatorURL:        os.Getenv("YODA_TRANSLATOR_URL"),
		ShakespeareTranslatorURL: os.Getenv("SHAKESPEARE_TRANSLATOR_URL"),
	}
	return cfg
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envLogLevel(key string, fallback slog.Level) slog.Level {
	v := os.Getenv(key)
	switch v {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "info", "INFO":
		return slog.LevelInfo
	case "warn", "WARN":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return fallback
	}
}
