package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	webpush "github.com/SherClockHolmes/webpush-go"

	"github.com/kalverra/parlay-club/internal/db"
)

// Message is the notification payload the service worker shows. URL is the path
// to open when someone taps it, and Tag replaces an older notification with the
// same tag on the device.
type Message struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Tag   string `json:"tag"`
}

// SendToUsers delivers m to every device of the given active users. Sends run
// concurrently under one deadline. Failures are logged and dropped: a missed
// nudge is better than a request that waits on a push service.
//
// Endpoints the push service reports as gone (404 or 410) are deleted.
func (s *Service) SendToUsers(ctx context.Context, userIDs []int64, m Message) {
	if !s.Enabled() || len(userIDs) == 0 {
		return
	}
	payload, err := encodeMessage(m)
	if err != nil {
		s.logger.Error().Err(err).Msg("push: encode message")
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
	defer cancel()

	subs, err := s.store.PushSubscriptionsForUsers(ctx, userIDs)
	if err != nil {
		s.logger.Error().Err(err).Msg("push: load subscriptions")
		return
	}

	var wg sync.WaitGroup
	for _, sub := range subs {
		// webpush-go grows its buffer from the caller's slice, so each send
		// needs its own copy or concurrent sends race on the payload.
		wg.Go(func() { s.send(ctx, sub, bytes.Clone(payload), m) })
	}
	wg.Wait()
}

// send delivers one payload and retires the endpoint when it is gone.
func (s *Service) send(ctx context.Context, sub db.PushSubscription, payload []byte, m Message) {
	resp, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{Auth: sub.Auth, P256dh: sub.P256dh},
	}, &webpush.Options{
		HTTPClient:      s.client,
		Subscriber:      s.subject,
		Topic:           topicFor(m.Tag),
		TTL:             messageTTL,
		Urgency:         webpush.UrgencyNormal,
		VAPIDPublicKey:  s.keys.Public,
		VAPIDPrivateKey: s.keys.Private,
	})
	if err != nil {
		s.logger.Warn().Err(err).Int64("subscription", sub.ID).Msg("push: send")
		return
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent:
		return
	case http.StatusNotFound, http.StatusGone:
		if err := s.store.DeletePushSubscriptionByID(ctx, sub.ID); err != nil {
			s.logger.Warn().Err(err).Int64("subscription", sub.ID).Msg("push: prune")
		}
	default:
		s.logger.Warn().
			Int("status", resp.StatusCode).
			Int64("subscription", sub.ID).
			Msg("push: send")
	}
}

// encodeMessage builds the JSON body the service worker reads.
func encodeMessage(m Message) ([]byte, error) {
	payload, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal message: %w", err)
	}
	return payload, nil
}

// topicFor turns a tag into a Topic header value. RFC 8030 allows at most 32
// characters from the URL-safe base64 alphabet, and a push service replaces an
// undelivered message that shares a topic.
func topicFor(tag string) string {
	var topic strings.Builder
	for _, r := range tag {
		if topic.Len() == maxTopicLength {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			topic.WriteRune(r)
		}
	}
	return topic.String()
}
