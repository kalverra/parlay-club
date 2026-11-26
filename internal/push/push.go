// Package push stores browser push subscriptions and delivers Web Push
// notifications through the browser vendors' push services.
//
// It wraps github.com/SherClockHolmes/webpush-go for the RFC 8291 encryption
// and VAPID signing. The package owns the subscription policy: which endpoints
// it will talk to, how many devices one person may register, and how a failed
// send retires an endpoint.
package push

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
)

const (
	// MaxSubscriptionsPerUser bounds how many devices one person may register.
	// A new device past the cap replaces the oldest one.
	MaxSubscriptionsPerUser = 10
	// sendTimeout caps one SendToUsers call, sends included. A hung push
	// service delays the request that caused the send, so the cap stays short.
	sendTimeout = 3 * time.Second
	// messageTTL is how long a push service keeps an undelivered message.
	messageTTL = 6 * 60 * 60
	// maxTopicLength is RFC 8030's limit on a Topic header value.
	maxTopicLength = 32
	// maxEndpointLength refuses an absurd stored endpoint. Real browser
	// endpoints run a couple hundred characters.
	maxEndpointLength = 2048
)

var (
	// ErrDisabled reports a push operation on a server with no VAPID key pair.
	ErrDisabled = errors.New("push is not configured")
	// ErrInvalidSubscription reports a subscription the server refuses to store.
	ErrInvalidSubscription = errors.New("invalid push subscription")
)

// knownPushHosts are the push services browsers use. An endpoint must sit on
// one of them (or a subdomain), so a signed-in person cannot make the server
// POST to an arbitrary URL.
var knownPushHosts = []string{
	"fcm.googleapis.com",
	"push.apple.com",
	"push.services.mozilla.com",
	"notify.windows.com",
}

// Keys is the VAPID key pair that identifies this server to push services. It
// is stored base64url-encoded, as webpush-go expects.
type Keys struct {
	Public  string
	Private string
}

// Set reports whether both halves of the pair are present.
func (k Keys) Set() bool {
	return k.Public != "" && k.Private != ""
}

// Subscription mirrors the browser's PushSubscription.toJSON() shape, which is
// what app.js posts.
type Subscription struct {
	Endpoint string           `json:"endpoint"`
	Keys     SubscriptionKeys `json:"keys"`
}

// SubscriptionKeys are the per-device encryption keys from the browser.
type SubscriptionKeys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}

// Options tune a Service. The zero value is production behavior.
type Options struct {
	// Subject is the VAPID contact, the app's public URL. Apple requires it.
	Subject string
	// Client sends the requests. Nil uses a default HTTP client.
	Client webpush.HTTPClient
	// Timeout caps one SendToUsers call. Zero uses the 3 second default.
	Timeout time.Duration
}

// Service stores subscriptions and sends notifications for one server.
type Service struct {
	store   *db.Store
	logger  zerolog.Logger
	keys    Keys
	subject string
	client  webpush.HTTPClient
	timeout time.Duration
}

// New returns a Service backed by store. A missing key pair leaves it disabled:
// Enabled reports false and every method is a no-op or ErrDisabled.
func New(store *db.Store, logger zerolog.Logger, keys Keys, opts Options) *Service {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = sendTimeout
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{}
	}
	return &Service{
		store:   store,
		logger:  logger,
		keys:    keys,
		subject: opts.Subject,
		client:  client,
		timeout: timeout,
	}
}

// Enabled reports whether the server has a key pair to send with.
func (s *Service) Enabled() bool {
	return s != nil && s.keys.Set()
}

// PublicKey returns the VAPID public key the browser subscribes with.
func (s *Service) PublicKey() string {
	if s == nil {
		return ""
	}
	return s.keys.Public
}

// Subscribe stores sub for userID. An endpoint that already exists is
// reassigned to userID and its keys are refreshed. A user's oldest devices are
// dropped past MaxSubscriptionsPerUser.
func (s *Service) Subscribe(ctx context.Context, userID int64, sub Subscription) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	sub.Endpoint = strings.TrimSpace(sub.Endpoint)
	if err := validateSubscription(sub); err != nil {
		return err
	}
	if err := s.store.SavePushSubscription(ctx, db.PushSubscription{
		UserID:   userID,
		Endpoint: sub.Endpoint,
		P256dh:   sub.Keys.P256dh,
		Auth:     sub.Keys.Auth,
	}, MaxSubscriptionsPerUser); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	return nil
}

// Unsubscribe removes endpoint when it belongs to userID. A missing row is not
// an error, so turning notifications off twice is harmless.
func (s *Service) Unsubscribe(ctx context.Context, userID int64, endpoint string) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return fmt.Errorf("%w: endpoint is required", ErrInvalidSubscription)
	}
	if _, err := s.store.DeletePushSubscription(ctx, userID, endpoint); err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}
	return nil
}

// validateSubscription reports why the server will not store sub.
func validateSubscription(sub Subscription) error {
	if err := validateEndpoint(sub.Endpoint); err != nil {
		return err
	}
	if sub.Keys.P256dh == "" || sub.Keys.Auth == "" {
		return fmt.Errorf("%w: keys are required", ErrInvalidSubscription)
	}
	return nil
}

// validateEndpoint requires an https URL on a known push service.
func validateEndpoint(endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("%w: endpoint is required", ErrInvalidSubscription)
	}
	if len(endpoint) > maxEndpointLength {
		return fmt.Errorf("%w: endpoint is too long", ErrInvalidSubscription)
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("%w: endpoint is not a URL", ErrInvalidSubscription)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%w: endpoint must be an https URL", ErrInvalidSubscription)
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	for _, known := range knownPushHosts {
		if host == known || strings.HasSuffix(host, "."+known) {
			return nil
		}
	}
	return fmt.Errorf("%w: unknown push host %q", ErrInvalidSubscription, host)
}
