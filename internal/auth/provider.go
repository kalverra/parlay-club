package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// GoogleIssuer is Google's OpenID Connect issuer.
const GoogleIssuer = "https://accounts.google.com"

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

	discovered, err := oidc.NewProvider(ctx, issuer)
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
