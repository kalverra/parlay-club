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
	assert.Empty(t, cfg.TheOddsAPIKey)
	assert.Empty(t, cfg.GitHubToken)
	assert.Equal(t, "kalverra/parlay-club", cfg.GitHubRepo)
	assert.Empty(t, cfg.PrefillToken)
	assert.Empty(t, cfg.ReminderToken)
	assert.Empty(t, cfg.ReminderQueue)
	assert.Empty(t, cfg.VAPIDPublicKey)
	assert.Empty(t, cfg.VAPIDPrivateKey)
	assert.False(t, cfg.PushEnabled())
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
		"THE_ODDS_API_KEY":     "the-odds-key",
		"GITHUB_TOKEN":         "  ghp_secret  ",
		"GITHUB_REPO":          "  other/repo  ",
		"PREFILL_TOKEN":        "  prefill-secret  ",
		"REMINDER_TOKEN":       "  reminder-secret  ",
		"REMINDER_QUEUE":       "  projects/p/locations/r/queues/q  ",
		"VAPID_PUBLIC_KEY":     "  public-key  ",
		"VAPID_PRIVATE_KEY":    "  private-key  ",
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
	assert.Equal(t, "the-odds-key", cfg.TheOddsAPIKey)
	assert.Equal(t, "ghp_secret", cfg.GitHubToken)
	assert.Equal(t, "other/repo", cfg.GitHubRepo)
	assert.Equal(t, "prefill-secret", cfg.PrefillToken)
	assert.Equal(t, "reminder-secret", cfg.ReminderToken)
	assert.Equal(t, "projects/p/locations/r/queues/q", cfg.ReminderQueue)
	assert.Equal(t, "public-key", cfg.VAPIDPublicKey)
	assert.Equal(t, "private-key", cfg.VAPIDPrivateKey)
	assert.True(t, cfg.PushEnabled())
	assert.Equal(t, zerolog.WarnLevel, cfg.LogLevel)
}

func TestLoadProdDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"APP_ENV":              config.EnvProd,
		"SESSION_KEY":          strings.Repeat("k", 32),
		"GOOGLE_CLIENT_ID":     "client-id",
		"GOOGLE_CLIENT_SECRET": "client-secret",
		"BASE_URL":             "https://parlay.example.com",
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
	assert.Empty(t, cfg.OddsAPIKey)
	assert.Empty(t, cfg.TheOddsAPIKey)
	assert.Empty(t, cfg.GitHubToken)
	assert.Equal(t, "kalverra/parlay-club", cfg.GitHubRepo)
	assert.Empty(t, cfg.PrefillToken)
	assert.Empty(t, cfg.ReminderToken)
	assert.Empty(t, cfg.ReminderQueue)
	assert.Empty(t, cfg.VAPIDPublicKey)
	assert.Empty(t, cfg.VAPIDPrivateKey)
	assert.False(t, cfg.PushEnabled())
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
				"SESSION_KEY":          "",
			},
			wantErr: []string{
				"GOOGLE_CLIENT_ID is required in prod",
				"GOOGLE_CLIENT_SECRET is required in prod",
				"BASE_URL is required in prod",
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

func TestLoadValidatesBaseURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseURL string
		want    string
		wantErr string
	}{
		"https with host": {
			baseURL: "https://parlay.example.com",
			want:    "https://parlay.example.com",
		},
		"http with host": {
			baseURL: "http://localhost:9090",
			want:    "http://localhost:9090",
		},
		"trailing slashes are trimmed": {
			baseURL: "https://parlay.example.com//",
			want:    "https://parlay.example.com",
		},
		"path is allowed": {
			baseURL: "https://parlay.example.com/parlay",
			want:    "https://parlay.example.com/parlay",
		},
		"ftp scheme": {
			baseURL: "ftp://parlay.example.com",
			wantErr: `BASE_URL must be an http or https URL with a host, got "ftp://parlay.example.com"`,
		},
		"bare hostname": {
			baseURL: "parlay.example.com",
			wantErr: `BASE_URL must be an http or https URL with a host, got "parlay.example.com"`,
		},
		"scheme without host": {
			baseURL: "https:/parlay.example.com",
			wantErr: `BASE_URL must be an http or https URL with a host, got "https:/parlay.example.com"`,
		},
		"url parse error": {
			baseURL: "http://exa mple.com",
			wantErr: `BASE_URL must be an http or https URL with a host, got "http://exa mple.com"`,
		},
	}

	// Each case runs in both envs. Prod carries the other required values so
	// BASE_URL is the only variable under test.
	envs := map[string]map[string]string{
		config.EnvDev: {},
		config.EnvProd: {
			"SESSION_KEY":          strings.Repeat("k", 32),
			"GOOGLE_CLIENT_ID":     "client-id",
			"GOOGLE_CLIENT_SECRET": "client-secret",
		},
	}

	for name, test := range tests {
		for appEnv, base := range envs {
			t.Run(appEnv+"/"+name, func(t *testing.T) {
				t.Parallel()

				values := make(map[string]string, len(base)+2)
				maps.Copy(values, base)
				values["APP_ENV"] = appEnv
				values["BASE_URL"] = test.baseURL

				cfg, err := config.Load(envFrom(values))
				if test.wantErr != "" {
					require.Error(t, err)
					assert.Contains(t, err.Error(), test.wantErr)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, test.want, cfg.BaseURL)
			})
		}
	}
}

func TestLoadDevStartsWithNoSecrets(t *testing.T) {
	t.Parallel()

	// No Google values means the dev login, and sign-up is open.
	cfg, err := config.Load(envFrom(map[string]string{"APP_ENV": config.EnvDev}))
	require.NoError(t, err)

	assert.Empty(t, cfg.GoogleClientID)
	assert.Empty(t, cfg.TheOddsAPIKey)
	assert.Equal(t, "http://localhost:8080", cfg.BaseURL)
}

func TestLoadSandboxUsesTheFixedDatabase(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"SUNDAY_SANDBOX": "1",
		"DB_PATH":        "/tmp/normal-dev.db",
	}))
	require.NoError(t, err)

	assert.True(t, cfg.Sandbox)
	assert.False(t, cfg.IsProd())
	assert.Equal(t, config.SandboxDBPath, cfg.DBPath, "sandbox mode ignores DB_PATH")
}

func TestLoadSandboxLeavesNormalDevDatabaseAlone(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"DB_PATH": "/tmp/normal-dev.db",
	}))
	require.NoError(t, err)

	assert.False(t, cfg.Sandbox)
	assert.Equal(t, "/tmp/normal-dev.db", cfg.DBPath)
}

func TestLoadRejectsSandboxInProd(t *testing.T) {
	t.Parallel()

	_, err := config.Load(envFrom(map[string]string{
		"APP_ENV":              config.EnvProd,
		"SUNDAY_SANDBOX":       "1",
		"SESSION_KEY":          strings.Repeat("k", 32),
		"GOOGLE_CLIENT_ID":     "client-id",
		"GOOGLE_CLIENT_SECRET": "client-secret",
		"BASE_URL":             "https://parlay.example.com",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SUNDAY_SANDBOX is not allowed in prod")
}

func TestLoadRejectsUnknownSandboxValue(t *testing.T) {
	t.Parallel()

	_, err := config.Load(envFrom(map[string]string{"SUNDAY_SANDBOX": "sometimes"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), `SUNDAY_SANDBOX must be a boolean, got "sometimes"`)
}

func TestLoadOddsAPIKeysWithParlayAndOddsAPI(t *testing.T) {
	t.Parallel()

	t.Run("parlay key aliases odds api key", func(t *testing.T) {
		t.Parallel()
		cfg, err := config.Load(envFrom(map[string]string{
			"PARLAY_API_KEY": "parlay-key",
		}))
		require.NoError(t, err)
		assert.Equal(t, "parlay-key", cfg.OddsAPIKey)
		assert.Empty(t, cfg.TheOddsAPIKey)
	})

	t.Run("parlay key and odds key both set", func(t *testing.T) {
		t.Parallel()
		cfg, err := config.Load(envFrom(map[string]string{
			"PARLAY_API_KEY": "parlay-key",
			"ODDS_API_KEY":   "the-odds-key",
		}))
		require.NoError(t, err)
		assert.Equal(t, "parlay-key", cfg.OddsAPIKey)
		assert.Equal(t, "the-odds-key", cfg.TheOddsAPIKey)
	})

	t.Run("explicit the odds api key takes precedence", func(t *testing.T) {
		t.Parallel()
		cfg, err := config.Load(envFrom(map[string]string{
			"PARLAY_API_KEY":   "parlay-key",
			"ODDS_API_KEY":     "other-odds-key",
			"THE_ODDS_API_KEY": "the-odds-key",
		}))
		require.NoError(t, err)
		assert.Equal(t, "parlay-key", cfg.OddsAPIKey)
		assert.Equal(t, "the-odds-key", cfg.TheOddsAPIKey)
	})
}

func TestLoadPushIsOffUnlessBothVAPIDKeysAreSet(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		values map[string]string
		want   bool
	}{
		"both halves": {
			values: map[string]string{"VAPID_PUBLIC_KEY": "public", "VAPID_PRIVATE_KEY": "private"},
			want:   true,
		},
		"public only": {
			values: map[string]string{"VAPID_PUBLIC_KEY": "public"},
		},
		"private only": {
			values: map[string]string{"VAPID_PRIVATE_KEY": "private"},
		},
		"neither": {},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.Load(envFrom(test.values))
			require.NoError(t, err)
			assert.Equal(t, test.want, cfg.PushEnabled())
		})
	}
}

func TestLoadBlankGitHubRepoUsesDefault(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(envFrom(map[string]string{
		"GITHUB_REPO": "   ",
	}))
	require.NoError(t, err)
	assert.Equal(t, "kalverra/parlay-club", cfg.GitHubRepo)
}
