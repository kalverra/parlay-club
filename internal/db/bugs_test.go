package db_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kalverra/parlay-club/internal/db"
)

func TestCreateAndLookupBugReport(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	created, err := store.CreateBugReport(t.Context(), db.BugReport{
		UserID:      ada.ID,
		PageURL:     "https://parlay.club/slips/1",
		Description: "  Submit button is clipped on mobile  ",
		UserAgent:   "Mozilla/5.0 (iPhone)",
		ScreenSize:  "390x844",
		Screenshot:  []byte("jpeg-bytes"),
	})
	require.NoError(t, err)

	assert.Positive(t, created.ID)
	assert.Len(t, created.Token, 32)
	assert.Equal(t, ada.ID, created.UserID)
	assert.Equal(t, "https://parlay.club/slips/1", created.PageURL)
	assert.Equal(t, "Submit button is clipped on mobile", created.Description)
	assert.Equal(t, "Mozilla/5.0 (iPhone)", created.UserAgent)
	assert.Equal(t, "390x844", created.ScreenSize)
	assert.Equal(t, []byte("jpeg-bytes"), created.Screenshot)
	assert.Equal(t, "image/jpeg", created.ScreenshotMIME)
	assert.Nil(t, created.GitHubIssueURL)
	assert.Nil(t, created.GitHubIssueNum)
	assert.False(t, created.CreatedAt.IsZero())

	byID, err := store.BugReportByID(t.Context(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, byID)
}

func TestCreateBugReportValidation(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	_, err := store.CreateBugReport(t.Context(), db.BugReport{
		UserID:      ada.ID,
		Description: "   ",
	})
	require.ErrorIs(t, err, db.ErrBugReportDescriptionRequired)

	_, err = store.CreateBugReport(t.Context(), db.BugReport{
		UserID:      9999,
		Description: "Unknown user",
	})
	require.Error(t, err)

	first, err := store.CreateBugReport(t.Context(), db.BugReport{
		Token:       "fixed-token-123",
		UserID:      ada.ID,
		Description: "First report",
	})
	require.NoError(t, err)
	assert.Equal(t, "fixed-token-123", first.Token)

	_, err = store.CreateBugReport(t.Context(), db.BugReport{
		Token:       "fixed-token-123",
		UserID:      ada.ID,
		Description: "Duplicate token",
	})
	require.Error(t, err)
}

func TestBugReportNotFound(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)

	_, err := store.BugReportByID(t.Context(), 42)
	require.ErrorIs(t, err, db.ErrBugReportNotFound)

	_, _, err = store.ScreenshotByToken(t.Context(), "missing-token")
	require.ErrorIs(t, err, db.ErrBugReportNotFound)
}

func TestScreenshotByToken(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	withImage, err := store.CreateBugReport(t.Context(), db.BugReport{
		UserID:         ada.ID,
		Description:    "Chart cut off",
		Screenshot:     []byte{0xff, 0xd8, 0xff, 0xe0},
		ScreenshotMIME: "image/png",
	})
	require.NoError(t, err)

	data, mime, err := store.ScreenshotByToken(t.Context(), withImage.Token)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xff, 0xd8, 0xff, 0xe0}, data)
	assert.Equal(t, "image/png", mime)

	withoutImage, err := store.CreateBugReport(t.Context(), db.BugReport{
		UserID:      ada.ID,
		Description: "No screenshot attached",
	})
	require.NoError(t, err)

	data, mime, err = store.ScreenshotByToken(t.Context(), withoutImage.Token)
	require.NoError(t, err)
	assert.Empty(t, data)
	assert.Equal(t, "image/jpeg", mime)
}

func TestSetGitHubIssue(t *testing.T) {
	t.Parallel()

	store, _ := openTestStore(t)
	ada := createUser(t, store, "Ada")

	first, err := store.CreateBugReport(t.Context(), db.BugReport{
		UserID:      ada.ID,
		Description: "First issue",
	})
	require.NoError(t, err)

	require.NoError(t, store.SetGitHubIssue(
		t.Context(),
		first.ID,
		"https://github.com/kalverra/parlay-club/issues/42",
		42,
	))

	updated, err := store.BugReportByID(t.Context(), first.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.GitHubIssueURL)
	require.NotNil(t, updated.GitHubIssueNum)
	assert.Equal(t, "https://github.com/kalverra/parlay-club/issues/42", *updated.GitHubIssueURL)
	assert.Equal(t, 42, *updated.GitHubIssueNum)

	err = store.SetGitHubIssue(t.Context(), 99999, "https://example.com/1", 1)
	require.ErrorIs(t, err, db.ErrBugReportNotFound)
}
