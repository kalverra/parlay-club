package tz_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/tz"
)

func TestEasternLoads(t *testing.T) {
	t.Parallel()

	require.NotNil(t, tz.Eastern)
	assert.Equal(t, "America/New_York", tz.Eastern.String())

	// Test a known winter date (EST is UTC-5).
	winter := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC).In(tz.Eastern)
	_, offset := winter.Zone()
	assert.Equal(t, -5*3600, offset)
}
