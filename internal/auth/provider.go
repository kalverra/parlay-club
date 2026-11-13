package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// GoogleIssuer is Google's OpenID Connect issuer.
const GoogleIssuer = "https://accounts.google.com"

const (
	// discoverAttempts is how many times startup asks the issuer for its
	// discovery document before giving up.
	discoverAttempts = 3
	// discoverAttemptTimeout bounds one discovery request, so a stalled DNS
	// lookup or TLS handshake cannot hold startup for the whole retry budget.
	discoverAttemptTimeout = 5 * time.Second
)

// discoverBackoff is the pause before each retry after a failed attempt. The
// two entries sit before the second and third attempts, so a short blip at boot
// does not fail a cold start.
var discoverBackoff = [...]time.Duration{200 * time.Millisecond, 500 * time.Millisecond}

// ErrEmailNotVerified is returned when Google says the signed-in address is not
// verified, so it cannot prove who the person is.
var ErrEmailNotVerified = errors.New("google email is not verified")

// Provider is Google in prod and a fake in tests.
type Provider interface {
	AuthURL(state, nonce, challenge string) string
	Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error)
}

// GoogleProvider runs the authorization code flow against an OpenID Connect
// issuer.
type GoogleProvider struct {
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

// NewGoogleProvider discovers issuer and builds the Google provider. An empty
// issuer means Google; tests pass a local server. The redirect URL must match
// one registered on the Google client.
func NewGoogleProvider(
	ctx context.Context,
	clientID, clientSecret, redirectURL, issuer string,
) (*GoogleProvider, error) {
	if issuer == "" {
		issuer = GoogleIssuer
	}

	discovered, err := discoverIssuer(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("discover identity provider: %w", err)
	}

	return &GoogleProvider{
		oauth: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     discovered.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: discovered.Verifier(&oidc.Config{ClientID: clientID}),
	}, nil
}

// discoverIssuer fetches issuer's discovery document, retrying a failed attempt
// with a short backoff. It returns the first success, the last failure after
// discoverAttempts tries, or ctx's error as soon as ctx is cancelled.
func discoverIssuer(ctx context.Context, issuer string) (*oidc.Provider, error) {
	var lastErr error

	for attempt := range discoverAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("wait to retry discovery: %w", ctx.Err())
			case <-time.After(discoverBackoff[min(attempt-1, len(discoverBackoff)-1)]):
			}
		}

		// A per-attempt deadline keeps one hung request from spending the
		// whole startup budget the retries are meant to cover.
		attemptCtx, cancel := context.WithTimeout(ctx, discoverAttemptTimeout)
		discovered, err := oidc.NewProvider(attemptCtx, issuer)
		cancel()
		if err == nil {
			return discovered, nil
		}
		lastErr = err

		if ctx.Err() != nil {
			return nil, fmt.Errorf("discover while stopping: %w", ctx.Err())
		}
	}

	return nil, lastErr
}

// AuthURL builds the authorization request. It asks for online access, carries
// the nonce that binds the ID token to this attempt, and uses PKCE.
func (p *GoogleProvider) AuthURL(state, nonce, challenge string) string {
	return p.oauth.AuthCodeURL(
		state,
		oauth2.AccessTypeOnline,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange trades the code for tokens, verifies the ID token's signature,
// issuer, audience, expiry, and nonce, and returns the identity it proves.
func (p *GoogleProvider) Exchange(
	ctx context.Context,
	code, verifier, nonce string,
) (Identity, error) {
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return Identity{}, errors.New("token response has no id token")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id token: %w", err)
	}
	if idToken.Nonce != nonce {
		return Identity{}, errors.New("id token nonce does not match")
	}

	var claims struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("read id token claims: %w", err)
	}
	if !claims.EmailVerified {
		return Identity{}, ErrEmailNotVerified
	}

	return Identity{
		Sub:     claims.Sub,
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
	}, nil
}

// compile-time check that the real provider satisfies the interface.
var _ Provider = (*GoogleProvider)(nil)
