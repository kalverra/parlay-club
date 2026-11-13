package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/auth"
)

const (
	testClientID    = "client-id"
	testRedirectURL = "http://localhost:8080/auth/google/callback"
)

// oidcTestServer serves the discovery document, the key set, and a token
// response holding whatever ID token the test put there.
type oidcTestServer struct {
	*httptest.Server
	idToken string
	keys    jose.JSONWebKeySet
}

func newOIDCTestServer(t *testing.T) *oidcTestServer {
	t.Helper()

	s := &oidcTestServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

func (s *oidcTestServer) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		_ = json.NewEncoder(w).Encode(discoveryDocument(s.URL))
	case "/keys":
		_ = json.NewEncoder(w).Encode(s.keys)
	case "/token":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     s.idToken,
		})
	default:
		http.NotFound(w, r)
	}
}

// discoveryDocument is the OpenID Connect metadata a test issuer publishes.
func discoveryDocument(issuer string) map[string]any {
	return map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/auth",
		"token_endpoint":                        issuer + "/token",
		"jwks_uri":                              issuer + "/keys",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
}

// flakyOIDCServer answers discovery with 503 until failures are used up and
// counts every discovery request it sees. onCall, when set, runs on each one.
type flakyOIDCServer struct {
	*httptest.Server
	failures int32
	calls    atomic.Int32
	onCall   func()
}

func newFlakyOIDCServer(t *testing.T, failures int32, onCall func()) *flakyOIDCServer {
	t.Helper()

	s := &flakyOIDCServer{failures: failures, onCall: onCall}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.Close)
	return s
}

func (s *flakyOIDCServer) handle(w http.ResponseWriter, _ *http.Request) {
	if s.onCall != nil {
		s.onCall()
	}
	if s.calls.Add(1) <= s.failures {
		http.Error(w, "issuer is warming up", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(discoveryDocument(s.URL))
}

func TestGoogleProviderRetriesTransientDiscoveryFailures(t *testing.T) {
	t.Parallel()

	server := newFlakyOIDCServer(t, 2, nil)

	provider, err := auth.NewGoogleProvider(
		t.Context(), testClientID, "secret", testRedirectURL, server.URL)
	require.NoError(t, err)
	assert.Equal(t, int32(3), server.calls.Load(), "the third attempt succeeds")

	raw := provider.AuthURL("state-1", "nonce-1", "challenge-1")
	assert.True(t, strings.HasPrefix(raw, server.URL+"/auth?"))
}

func TestGoogleProviderFailsAfterExhaustingDiscoveryAttempts(t *testing.T) {
	t.Parallel()

	server := newFlakyOIDCServer(t, 10, nil)

	_, err := auth.NewGoogleProvider(
		t.Context(), testClientID, "secret", testRedirectURL, server.URL)
	require.Error(t, err)
	require.ErrorContains(t, err, "discover identity provider")
	require.ErrorContains(t, err, "503 Service Unavailable")
	assert.Equal(t, int32(3), server.calls.Load(), "retries stop after three attempts")
}

func TestGoogleProviderStopsDiscoveryWhenCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	server := newFlakyOIDCServer(t, 10, cancel)

	start := time.Now()
	_, err := auth.NewGoogleProvider(
		ctx, testClientID, "secret", testRedirectURL, server.URL)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), server.calls.Load(), "a cancelled context stops retrying")
	assert.Less(t, time.Since(start), 200*time.Millisecond,
		"cancellation ends the request instead of waiting out the backoff")
}

func TestGoogleProviderBoundsAHungDiscoveryAttempt(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	hung := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case hung <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	start := time.Now()
	_, err := auth.NewGoogleProvider(
		ctx, testClientID, "secret", testRedirectURL, server.URL)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second, "a stalled request cannot hang startup")

	select {
	case <-hung:
	default:
		t.Fatal("startup never reached the issuer")
	}
}

func TestGoogleProviderAuthURL(t *testing.T) {
	t.Parallel()

	server := newOIDCTestServer(t)
	provider, err := auth.NewGoogleProvider(
		t.Context(), testClientID, "secret", testRedirectURL, server.URL)
	require.NoError(t, err)

	raw := provider.AuthURL("state-1", "nonce-1", "challenge-1")
	assert.True(t, strings.HasPrefix(raw, server.URL+"/auth?"))

	parsed, err := url.Parse(raw)
	require.NoError(t, err)
	query := parsed.Query()
	assert.Equal(t, testClientID, query.Get("client_id"))
	assert.Equal(t, testRedirectURL, query.Get("redirect_uri"))
	assert.Equal(t, "state-1", query.Get("state"))
	assert.Equal(t, "nonce-1", query.Get("nonce"))
	assert.Equal(t, "challenge-1", query.Get("code_challenge"))
	assert.Equal(t, "S256", query.Get("code_challenge_method"))
	assert.Equal(t, "code", query.Get("response_type"))
	assert.Equal(t, "online", query.Get("access_type"))
	assert.Contains(t, query.Get("scope"), "openid")
	assert.Contains(t, query.Get("scope"), "email")
	assert.Contains(t, query.Get("scope"), "profile")
}

func TestGoogleProviderExchange(t *testing.T) {
	t.Parallel()

	publishedKey := generateKey(t)
	otherKey := generateKey(t)

	tests := map[string]struct {
		key       *rsa.PrivateKey
		kid       string
		claims    func(issuer string) map[string]any
		wantErr   error
		wantError string
	}{
		"valid identity": {
			key: publishedKey,
			kid: "key-1",
			claims: func(issuer string) map[string]any {
				return validClaims(issuer, "nonce-1")
			},
		},
		"unverified email": {
			key: publishedKey,
			kid: "key-1",
			claims: func(issuer string) map[string]any {
				claims := validClaims(issuer, "nonce-1")
				claims["email_verified"] = false
				return claims
			},
			wantErr: auth.ErrEmailNotVerified,
		},
		"bad signature": {
			key: otherKey,
			kid: "key-1",
			claims: func(issuer string) map[string]any {
				return validClaims(issuer, "nonce-1")
			},
			wantError: "signature",
		},
		"wrong audience": {
			key: publishedKey,
			kid: "key-1",
			claims: func(issuer string) map[string]any {
				claims := validClaims(issuer, "nonce-1")
				claims["aud"] = "another-client"
				return claims
			},
			wantError: "audience",
		},
		"wrong nonce": {
			key: publishedKey,
			kid: "key-1",
			claims: func(issuer string) map[string]any {
				return validClaims(issuer, "another-nonce")
			},
			wantError: "nonce",
		},
		"expired token": {
			key: publishedKey,
			kid: "key-1",
			claims: func(issuer string) map[string]any {
				claims := validClaims(issuer, "nonce-1")
				claims["exp"] = time.Now().Add(-time.Minute).Unix()
				return claims
			},
			wantError: "expired",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := newOIDCTestServer(t)
			server.keys = jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
				Key:       &publishedKey.PublicKey,
				KeyID:     "key-1",
				Algorithm: string(jose.RS256),
				Use:       "sig",
			}}}
			server.idToken = signToken(t, test.key, test.kid, test.claims(server.URL))

			provider, err := auth.NewGoogleProvider(
				t.Context(), testClientID, "secret", testRedirectURL, server.URL)
			require.NoError(t, err)

			identity, err := provider.Exchange(t.Context(), "code-1", "verifier-1", "nonce-1")
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				return
			}
			if test.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, strings.ToLower(err.Error()), test.wantError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, auth.Identity{
				Sub:     "google-1",
				Email:   "ada@example.com",
				Name:    "Ada",
				Picture: "ada.png",
			}, identity)
		})
	}
}

// validClaims builds a signed-in Google claim set for issuer.
func validClaims(issuer, nonce string) map[string]any {
	now := time.Now()
	return map[string]any{
		"iss":            issuer,
		"aud":            testClientID,
		"sub":            "google-1",
		"exp":            now.Add(time.Hour).Unix(),
		"iat":            now.Unix(),
		"nonce":          nonce,
		"email":          "ada@example.com",
		"email_verified": true,
		"name":           "Ada",
		"picture":        "ada.png",
	}
}

// signToken signs claims as a compact JWT with kid.
func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)
	require.NoError(t, err)

	payload, err := json.Marshal(claims)
	require.NoError(t, err)

	object, err := signer.Sign(payload)
	require.NoError(t, err)

	raw, err := object.CompactSerialize()
	require.NoError(t, err)
	return raw
}

// generateKey makes an RSA key for signing test tokens.
func generateKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}
