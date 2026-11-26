package push

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/parlay"
)

// notifySlip is the slip every message test builds on.
var notifySlip = db.Slip{ID: 42, Name: "Sunday Slate"}

func TestLegsAddedMessageNamesOnePickOrCountsABatch(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Message{
		Title: "Sunday Slate",
		Body:  "Ben added Bills -3.5",
		URL:   "/slips/42",
		Tag:   "slip-42",
	}, legsAddedMessage(notifySlip, "Ben", 1, "Bills -3.5"))

	assert.Equal(t, "Ben added 3 legs", legsAddedMessage(notifySlip, "Ben", 3, "").Body)
	assert.Equal(t, "Ben added a leg", legsAddedMessage(notifySlip, "Ben", 1, "").Body)
}

func TestSlipLockedMessageInvitesThePlacement(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Message{
		Title: "Sunday Slate",
		Body:  "Everyone's locked in. Time to place it.",
		URL:   "/slips/42",
		Tag:   "slip-42",
	}, slipLockedMessage(notifySlip))
}

func TestSlipGradedMessageMatchesTheResult(t *testing.T) {
	t.Parallel()

	tests := map[parlay.Result]string{
		parlay.Won:  "Sunday Slate cashed!",
		parlay.Lost: "Sunday Slate came up short.",
		parlay.Push: "Sunday Slate pushed. Stakes back.",
		parlay.Void: "Sunday Slate pushed. Stakes back.",
	}
	for result, body := range tests {
		t.Run(string(result), func(t *testing.T) {
			t.Parallel()

			slip := notifySlip
			slip.Result = result
			message, ok := slipGradedMessage(slip)
			assert.True(t, ok)
			assert.Equal(t, body, message.Body)
			assert.Equal(t, "slip-42", message.Tag)
		})
	}

	slip := notifySlip
	slip.Result = parlay.Pending
	_, ok := slipGradedMessage(slip)
	assert.False(t, ok, "an ungraded slip sends nothing")
}
