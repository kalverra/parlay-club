package config_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/config"
)

// envFrom turns a map into a config.Env lookup.
func envFrom(values map[string]string) config.Env {
	return func(key string) string { return values[key] }
}

func TestLoadDevDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(nil))
	require.NoError(t, err)

	assert.Equal(t, config.EnvDev, cfg.AppEnv)
	assert.False(t, cfg.IsProd())
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "./data/parlay.db", cfg.DBPath)
	assert.NotEmpty(t, cfg.SessionKey)
	assert.Empty(t, cfg.GoogleClientID)
	assert.Empty(t, cfg.GoogleClientSecret)
	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
	assert.Empty(t, cfg.OddsAPIKey)
	assert.Empty(t, cfg.AllowedEmails)
	assert.Equal(t, zerolog.DebugLevel, cfg.LogLevel)
}

func TestLoadDevDefaultBaseURLFollowsThePort(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{"PORT": "9090"}))
	require.NoError(t, err)

	assert.Equal(t, "http://localhost:9090", cfg.BaseURL)
}

func TestLoadOverridesEveryValue(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"APP_ENV":              "dev",
		"PORT":                 "9090",
		"DB_PATH":              "/tmp/custom.db",
		"SESSION_KEY":          "short-is-fine-in-dev",
		"GOOGLE_CLIENT_ID":     "client-id",
		"GOOGLE_CLIENT_SECRET": "client-secret",
		"BASE_URL":             "https://parlay.example.com/",
		"ODDS_API_KEY":         "odds-key",
		"LOG_LEVEL":            "warn",
	}))
	require.NoError(t, err)

	assert.Equal(t, "dev", cfg.AppEnv)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "/tmp/custom.db", cfg.DBPath)
	assert.Equal(t, "short-is-fine-in-dev", cfg.SessionKey)
	assert.Equal(t, "client-id", cfg.GoogleClientID)
	assert.Equal(t, "client-secret", cfg.GoogleClientSecret)
	assert.Equal(t, "https://parlay.example.com", cfg.BaseURL, "the trailing slash is trimmed")
	assert.Equal(t, "odds-key", cfg.OddsAPIKey)
	assert.Equal(t, zerolog.WarnLevel, cfg.LogLevel)
}

func TestLoadParsesAllowedEmails(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"ALLOWED_EMAILS": " Ada@Example.com , bob@example.com=Bob Smith,,",
	}))
	require.NoError(t, err)

	assert.Equal(t, []config.AllowedEmail{
		{Email: "ada@example.com"},
		{Email: "bob@example.com", Name: "Bob Smith"},
	}, cfg.AllowedEmails)
}

func TestLoadRejectsMalformedAllowedEmails(t *testing.T) {
	t.Parallel()

	_, err := config.Load(envFrom(map[string]string{
		"ALLOWED_EMAILS": "ada@example.com,not-an-email,bob=Bob",
	}))
	require.Error(t, err)

	assert.Contains(t, err.Error(), `ALLOWED_EMAILS entry "not-an-email"`)
	assert.Contains(t, err.Error(), `ALLOWED_EMAILS entry "bob=Bob"`)
	assert.NotContains(t, err.Error(), "ada@example.com")
}

func TestLoadProdDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"APP_ENV":              config.EnvProd,
		"SESSION_KEY":          strings.Repeat("k", 32),
		"GOOGLE_CLIENT_ID":     "client-id",
		"GOOGLE_CLIENT_SECRET": "client-secret",
		"BASE_URL":             "https://parlay.example.com",
		"ALLOWED_EMAILS":       "ada@example.com",
	}))
	require.NoError(t, err)

	assert.Equal(t, config.EnvProd, cfg.AppEnv)
	assert.True(t, cfg.IsProd())
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, "./data/parlay.db", cfg.DBPath)
	assert.Equal(t, strings.Repeat("k", 32), cfg.SessionKey)
	assert.Equal(t, "client-id", cfg.GoogleClientID)
	assert.Equal(t, "client-secret", cfg.GoogleClientSecret)
	assert.Equal(t, "https://parlay.example.com", cfg.BaseURL)
	assert.Len(t, cfg.AllowedEmails, 1)
	assert.Equal(t, zerolog.InfoLevel, cfg.LogLevel)
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()

	valid := map[string]string{
		"APP_ENV":              config.EnvProd,
		"SESSION_KEY":          strings.Repeat("k", 32),
		"GOOGLE_CLIENT_ID":     "client-id",
		"GOOGLE_CLIENT_SECRET": "client-secret",
		"BASE_URL":             "https://parlay.example.com",
		"ALLOWED_EMAILS":       "ada@example.com",
	}

	tests := map[string]struct {
		overrides map[string]string
		wantErr   []string
	}{
		"prod missing google client id": {
			overrides: map[string]string{"GOOGLE_CLIENT_ID": ""},
			wantErr:   []string{"GOOGLE_CLIENT_ID is required in prod"},
		},
		"prod missing google client secret": {
			overrides: map[string]string{"GOOGLE_CLIENT_SECRET": ""},
			wantErr:   []string{"GOOGLE_CLIENT_SECRET is required in prod"},
		},
		"prod missing base url": {
			overrides: map[string]string{"BASE_URL": ""},
			wantErr:   []string{"BASE_URL is required in prod"},
		},
		"prod blank base url": {
			overrides: map[string]string{"BASE_URL": "   "},
			wantErr:   []string{"BASE_URL is required in prod"},
		},
		"prod missing allowed emails": {
			overrides: map[string]string{"ALLOWED_EMAILS": ""},
			wantErr:   []string{"ALLOWED_EMAILS must list at least one email in prod"},
		},
		"prod missing session key": {
			overrides: map[string]string{"SESSION_KEY": ""},
			wantErr:   []string{"SESSION_KEY must be at least 32 bytes in prod"},
		},
		"prod short session key": {
			overrides: map[string]string{"SESSION_KEY": strings.Repeat("k", 31)},
			wantErr:   []string{"SESSION_KEY must be at least 32 bytes in prod"},
		},
		"prod missing everything lists every problem": {
			overrides: map[string]string{
				"GOOGLE_CLIENT_ID":     "",
				"GOOGLE_CLIENT_SECRET": "",
				"BASE_URL":             "",
				"ALLOWED_EMAILS":       "",
				"SESSION_KEY":          "",
			},
			wantErr: []string{
				"GOOGLE_CLIENT_ID is required in prod",
				"GOOGLE_CLIENT_SECRET is required in prod",
				"BASE_URL is required in prod",
				"ALLOWED_EMAILS must list at least one email in prod",
				"SESSION_KEY must be at least 32 bytes in prod",
			},
		},
		"unknown app env": {
			overrides: map[string]string{"APP_ENV": "staging"},
			wantErr:   []string{`APP_ENV must be "dev" or "prod", got "staging"`},
		},
		"port not a number": {
			overrides: map[string]string{"PORT": "eighty"},
			wantErr:   []string{`PORT must be a number from 1 to 65535, got "eighty"`},
		},
		"port zero": {
			overrides: map[string]string{"PORT": "0"},
			wantErr:   []string{"PORT must be a number from 1 to 65535"},
		},
		"port too large": {
			overrides: map[string]string{"PORT": "65536"},
			wantErr:   []string{"PORT must be a number from 1 to 65535"},
		},
		"unknown log level": {
			overrides: map[string]string{"LOG_LEVEL": "loud"},
			wantErr:   []string{`LOG_LEVEL is not a known level, got "loud"`},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			values := make(map[string]string, len(valid)+len(test.overrides))
			maps.Copy(values, valid)
			maps.Copy(values, test.overrides)

			_, err := config.Load(envFrom(values))
			require.Error(t, err)
			for _, want := range test.wantErr {
				assert.Contains(t, err.Error(), want)
			}
		})
	}
}

func TestLoadDevStartsWithNoSecrets(t *testing.T) {
	t.Parallel()

	// An empty list allows everyone and no Google values means the dev login.
	cfg, err := config.Load(envFrom(map[string]string{"APP_ENV": config.EnvDev}))
	require.NoError(t, err)

	assert.Empty(t, cfg.GoogleClientID)
	assert.Empty(t, cfg.AllowedEmails)
	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
}
