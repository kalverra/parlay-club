package push

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDefaultsToTheThreeSecondSendCap(t *testing.T) {
	t.Parallel()

	service := New(nil, zerolog.Nop(), Keys{Public: "public", Private: "private"}, Options{})

	assert.Equal(t, 3*time.Second, service.timeout)
	assert.True(t, service.Enabled())
}

func TestTopicForKeepsOnlyTheURLSafeAlphabet(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"":                      "",
		"slip-42":               "slip-42",
		"slip 42!":              "slip42",
		"slip-42-reminder":      "slip-42-reminder",
		"tag_with_underscores":  "tag_with_underscores",
		strings.Repeat("a", 40): strings.Repeat("a", maxTopicLength),
	}

	for tag, want := range tests {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()

			topic := topicFor(tag)
			assert.Equal(t, want, topic)
			assert.LessOrEqual(t, len(topic), maxTopicLength)
		})
	}
}

func TestEncodeMessageMatchesTheServiceWorkerPayload(t *testing.T) {
	t.Parallel()

	payload, err := encodeMessage(Message{
		Title: "Sunday Slate",
		Body:  "Ben added 3 legs",
		URL:   "/slips/42",
		Tag:   "slip-42",
	})
	require.NoError(t, err)

	var decoded map[string]string
	require.NoError(t, json.Unmarshal(payload, &decoded))
	assert.Equal(t, map[string]string{
		"title": "Sunday Slate",
		"body":  "Ben added 3 legs",
		"url":   "/slips/42",
		"tag":   "slip-42",
	}, decoded)
}
