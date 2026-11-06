// Package config parses the server's configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

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

// AllowedEmail is one ALLOWED_EMAILS entry. Name is empty unless the entry is
// written as email=Name, which links a Google account to a history-import user
// with that name.
type AllowedEmail struct {
	Email string
	Name  string
}

// Config holds every value the app reads from the environment. LITESTREAM_BUCKET
// is absent on purpose: Litestream reads it, not the app.
type Config struct {
	AppEnv        string
	Port          int
	DBPath        string
	Passcode      string
	SessionKey    string
	OddsAPIKey    string
	LogLevel      zerolog.Level
	AllowedEmails []AllowedEmail
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

	allowedEmails, err := parseAllowedEmails(env("ALLOWED_EMAILS"))
	if err != nil {
		problems = append(problems, err)
	}
	cfg.AllowedEmails = allowedEmails

	if prod && cfg.Passcode == "" {
		problems = append(problems, errors.New("PASSCODE is required in prod"))
	}
	if prod && len(cfg.SessionKey) < minSessionKeySize {
		problems = append(problems, fmt.Errorf("SESSION_KEY must be at least %d bytes in prod", minSessionKeySize))
	}

	return cfg, errors.Join(problems...)
}

// parseAllowedEmails reads a comma list of email or email=Name entries. Empty
// segments are ignored so a trailing comma is harmless, and emails are stored
// lowercase so the gate compares them without case.
func parseAllowedEmails(raw string) ([]AllowedEmail, error) {
	var (
		entries  []AllowedEmail
		problems []error
	)
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		email, name, _ := strings.Cut(part, "=")
		email = strings.ToLower(strings.TrimSpace(email))
		name = strings.TrimSpace(name)
		if !looksLikeEmail(email) {
			problems = append(problems,
				fmt.Errorf("ALLOWED_EMAILS entry %q must be email or email=Name", part))
			continue
		}
		entries = append(entries, AllowedEmail{Email: email, Name: name})
	}
	return entries, errors.Join(problems...)
}

// looksLikeEmail rejects the obvious typos an allow list entry can carry
// without pulling in address parsing that would accept display names.
func looksLikeEmail(email string) bool {
	at := strings.LastIndex(email, "@")
	return at > 0 && at < len(email)-1 && !strings.ContainsAny(email, " \t")
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
