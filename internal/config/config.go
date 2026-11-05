// Package config parses the server's configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/rs/zerolog"
)

// Env returns the value of an environment variable, or "" when it is unset.
// os.Getenv satisfies it; tests pass a map lookup.
type Env func(key string) string

// Accepted APP_ENV values.
const (
	EnvDev  = "dev"
	EnvProd = "prod"
)

const (
	defaultPort       = 8080
	defaultDBPath     = "./data/parlay.db"
	defaultPasscode   = "dev"
	devSessionKey     = "dev-session-key-not-for-production"
	minSessionKeySize = 32
)

// Config holds every value the app reads from the environment. LITESTREAM_BUCKET
// is absent on purpose: Litestream reads it, not the app.
type Config struct {
	AppEnv     string
	Port       int
	DBPath     string
	Passcode   string
	SessionKey string
	OddsAPIKey string
	LogLevel   zerolog.Level
}

// IsProd reports whether production rules apply.
func (c Config) IsProd() bool { return c.AppEnv == EnvProd }

// Load builds a Config from env. It reports every problem it finds, joined, so a
// misconfigured deploy shows all of them at once. The caller exits on error.
func Load(env Env) (Config, error) {
	var problems []error

	appEnv := valueOr(env, "APP_ENV", EnvDev)
	if appEnv != EnvDev && appEnv != EnvProd {
		problems = append(problems, fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDev, EnvProd, appEnv))
	}
	prod := appEnv == EnvProd

	cfg := Config{
		AppEnv:     appEnv,
		Port:       portOr(env, &problems),
		DBPath:     valueOr(env, "DB_PATH", defaultDBPath),
		Passcode:   devDefault(env, "PASSCODE", defaultPasscode, prod),
		SessionKey: devDefault(env, "SESSION_KEY", devSessionKey, prod),
		OddsAPIKey: env("ODDS_API_KEY"),
		LogLevel:   levelOr(env, prod, &problems),
	}

	if prod && cfg.Passcode == "" {
		problems = append(problems, errors.New("PASSCODE is required in prod"))
	}
	if prod && len(cfg.SessionKey) < minSessionKeySize {
		problems = append(problems, fmt.Errorf("SESSION_KEY must be at least %d bytes in prod", minSessionKeySize))
	}

	return cfg, errors.Join(problems...)
}

// valueOr returns the variable's value, or fallback when it is unset or empty.
func valueOr(env Env, key, fallback string) string {
	if value := env(key); value != "" {
		return value
	}
	return fallback
}

// devDefault applies a development default only outside prod, so prod must set
// the variable itself.
func devDefault(env Env, key, fallback string, prod bool) string {
	if prod {
		return env(key)
	}
	return valueOr(env, key, fallback)
}

func portOr(env Env, problems *[]error) int {
	raw := env("PORT")
	if raw == "" {
		return defaultPort
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		*problems = append(*problems, fmt.Errorf("PORT must be a number from 1 to 65535, got %q", raw))
		return defaultPort
	}
	return port
}

func levelOr(env Env, prod bool, problems *[]error) zerolog.Level {
	name := valueOr(env, "LOG_LEVEL", "")
	if name == "" {
		if prod {
			name = "info"
		} else {
			name = "debug"
		}
	}
	level, err := zerolog.ParseLevel(name)
	if err != nil {
		*problems = append(*problems, fmt.Errorf("LOG_LEVEL is not a known level, got %q", name))
		return zerolog.InfoLevel
	}
	return level
}
