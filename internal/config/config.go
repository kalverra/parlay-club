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
	defaultGitHubRepo = "kalverra/parlay-club"
	devSessionKey     = "dev-session-key-not-for-production"
	minSessionKeySize = 32

	// SandboxDBPath is the fixed database the local Sunday sandbox uses. It
	// never reads DB_PATH, so a sandbox run cannot touch the normal dev file.
	SandboxDBPath = "./data/sunday-sandbox.db"
)

// Config holds every value the app reads from the environment. LITESTREAM_BUCKET
// is absent on purpose: Litestream reads it, not the app.
type Config struct {
	AppEnv             string
	Port               int
	DBPath             string
	SessionKey         string
	GoogleClientID     string
	GoogleClientSecret string
	BaseURL            string
	OddsAPIKey         string
	TheOddsAPIKey      string
	GitHubToken        string
	GitHubRepo         string
	LogLevel           zerolog.Level
	Sandbox            bool
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
	port := portOr(env, &problems)
	sandbox := sandboxOr(env, prod, &problems)

	dbPath := valueOr(env, "DB_PATH", defaultDBPath)
	if sandbox {
		dbPath = SandboxDBPath
	}

	cfg := Config{
		AppEnv:             appEnv,
		Port:               port,
		DBPath:             dbPath,
		SessionKey:         devDefault(env, "SESSION_KEY", devSessionKey, prod),
		GoogleClientID:     devDefault(env, "GOOGLE_CLIENT_ID", "", prod),
		GoogleClientSecret: devDefault(env, "GOOGLE_CLIENT_SECRET", "", prod),
		BaseURL:            baseURLOr(env, port, prod),
		OddsAPIKey:         oddsAPIKeyOr(env),
		TheOddsAPIKey:      theOddsAPIKeyOr(env),
		GitHubToken:        strings.TrimSpace(env("GITHUB_TOKEN")),
		GitHubRepo:         gitHubRepoOr(env),
		LogLevel:           levelOr(env, prod, &problems),
		Sandbox:            sandbox,
	}

	if prod && len(cfg.SessionKey) < minSessionKeySize {
		problems = append(problems, fmt.Errorf("SESSION_KEY must be at least %d bytes in prod", minSessionKeySize))
	}
	if prod {
		problems = append(problems, requiredInProd(env, "GOOGLE_CLIENT_ID")...)
		problems = append(problems, requiredInProd(env, "GOOGLE_CLIENT_SECRET")...)
		problems = append(problems, requiredInProd(env, "BASE_URL")...)
	}

	return cfg, errors.Join(problems...)
}

// requiredInProd reports a missing required variable.
func requiredInProd(env Env, key string) []error {
	if strings.TrimSpace(env(key)) == "" {
		return []error{fmt.Errorf("%s is required in prod", key)}
	}
	return nil
}

// baseURLOr returns the public base URL without a trailing slash. Dev defaults
// to the local server; prod must set it.
func baseURLOr(env Env, port int, prod bool) string {
	if value := strings.TrimRight(strings.TrimSpace(env("BASE_URL")), "/"); value != "" {
		return value
	}
	if prod {
		return ""
	}
	return fmt.Sprintf("http://localhost:%d", port)
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

// sandboxOr parses SUNDAY_SANDBOX. Sandbox mode is dev-only; prod rejects it
// instead of silently serving replay data.
func sandboxOr(env Env, prod bool, problems *[]error) bool {
	raw := strings.TrimSpace(env("SUNDAY_SANDBOX"))
	if raw == "" {
		return false
	}
	sandbox, err := strconv.ParseBool(raw)
	if err != nil {
		*problems = append(*problems, fmt.Errorf("SUNDAY_SANDBOX must be a boolean, got %q", raw))
		return false
	}
	if prod && sandbox {
		*problems = append(*problems, errors.New("SUNDAY_SANDBOX is not allowed in prod"))
	}
	return sandbox
}

// oddsAPIKeyOr returns the Parlay API key. It checks PARLAY_API_KEY first,
// falling back to ODDS_API_KEY.
func oddsAPIKeyOr(env Env) string {
	if key := env("PARLAY_API_KEY"); key != "" {
		return key
	}
	return env("ODDS_API_KEY")
}

// theOddsAPIKeyOr returns The Odds API key. It checks THE_ODDS_API_KEY first;
// if unset and PARLAY_API_KEY is provided, it accepts ODDS_API_KEY as The Odds API key.
func theOddsAPIKeyOr(env Env) string {
	if key := env("THE_ODDS_API_KEY"); key != "" {
		return key
	}
	if env("PARLAY_API_KEY") != "" {
		return env("ODDS_API_KEY")
	}
	return ""
}

// gitHubRepoOr returns the target GitHub repository, defaulting to kalverra/parlay-club.
func gitHubRepoOr(env Env) string {
	if repo := strings.TrimSpace(env("GITHUB_REPO")); repo != "" {
		return repo
	}
	return defaultGitHubRepo
}
