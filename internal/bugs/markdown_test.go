package bugs_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kalverra/parlay-club/internal/bugs"
	"github.com/kalverra/parlay-club/internal/db"
)

func TestBuildIssueTitle(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		desc string
		want string
	}{
		"single line": {
			desc: "Parlay slip button is clipped",
			want: "Bug: Parlay slip button is clipped",
		},
		"multiline uses first non-empty line": {
			desc: "\n\n  First line   with   spaces  \nSecond line details",
			want: "Bug: First line with spaces",
		},
		"long line truncates": {
			desc: strings.Repeat("a", 100),
			want: "Bug: " + strings.Repeat("a", 77) + "...",
		},
		"empty falls back": {
			desc: "   \n  ",
			want: "Bug report",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, bugs.BuildIssueTitle(tc.desc))
		})
	}
}

func TestBuildIssueBody(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 11, 12, 18, 30, 0, 0, time.UTC)

	t.Run("with screenshot and full reporter", func(t *testing.T) {
		t.Parallel()

		report := db.BugReport{
			Token:       "0123456789abcdef0123456789abcdef",
			Description: "Picks list overflows horizontally",
			PageURL:     "https://parlay.example.com/slips/4",
			ScreenSize:  "390x844",
			UserAgent:   "Mozilla/5.0 (iPhone)",
			Screenshot:  []byte{0xff, 0xd8, 0xff},
			CreatedAt:   createdAt,
		}
		reporter := db.User{
			Name:  "Alex",
			Email: "alex@example.com",
		}

		body := bugs.BuildIssueBody(report, reporter, "https://parlay.example.com/")
		expected := strings.Join([]string{
			"### Description",
			"Picks list overflows horizontally",
			"",
			"### Context",
			"- **Reporter:** Alex (alex@example.com)",
			"- **Page:** `https://parlay.example.com/slips/4`",
			"- **Screen:** `390x844`",
			"- **Device:** `Mozilla/5.0 (iPhone)`",
			"- **Time:** 2026-11-12T18:30:00Z",
			"",
			"### Screenshot",
			"![Screenshot](https://parlay.example.com/screenshots/0123456789abcdef0123456789abcdef.jpg)",
		}, "\n")

		assert.Equal(t, expected, body)
	})

	t.Run("without screenshot or email", func(t *testing.T) {
		t.Parallel()

		report := db.BugReport{
			Token:       "0123456789abcdef0123456789abcdef",
			Description: "Refresh spinner never stops",
			CreatedAt:   createdAt,
		}
		reporter := db.User{
			Name: "Sam",
		}

		body := bugs.BuildIssueBody(report, reporter, "https://parlay.example.com")
		assert.Contains(t, body, "- **Reporter:** Sam\n")
		assert.Contains(t, body, "- **Page:** `unknown`\n")
		assert.Contains(t, body, "- **Screen:** `unknown`\n")
		assert.Contains(t, body, "- **Device:** `unknown`\n")
		assert.NotContains(t, body, "### Screenshot")
	})

	t.Run("sanitizes newlines and backticks in context fields", func(t *testing.T) {
		t.Parallel()

		report := db.BugReport{
			Token:       "0123456789abcdef0123456789abcdef",
			Description: "Broken",
			PageURL:     "https://parlay.example.com/\n`injected`\n@everyone",
			ScreenSize:  "390x844\n`bad`",
			UserAgent:   "Safari\r\n`Mozilla`",
			CreatedAt:   createdAt,
		}
		reporter := db.User{Name: "Sam"}

		body := bugs.BuildIssueBody(report, reporter, "")
		assert.Contains(t, body, "- **Page:** `https://parlay.example.com/ injected @everyone`")
		assert.Contains(t, body, "- **Screen:** `390x844 bad`")
		assert.Contains(t, body, "- **Device:** `Safari Mozilla`")
	})
}
