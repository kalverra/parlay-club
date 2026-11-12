package bugs

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kalverra/parlay-club/internal/db"
)

const maxTitleRunes = 80

// BuildIssueTitle derives a concise single-line GitHub issue title from a bug description.
func BuildIssueTitle(description string) string {
	for line := range strings.SplitSeq(description, "\n") {
		collapsed := strings.Join(strings.Fields(line), " ")
		if collapsed == "" {
			continue
		}
		if utf8.RuneCountInString(collapsed) > maxTitleRunes {
			runes := []rune(collapsed)
			collapsed = strings.TrimSpace(string(runes[:maxTitleRunes-3])) + "..."
		}
		return "Bug: " + collapsed
	}
	return "Bug report"
}

// BuildIssueBody formats a bug report and reporter context as GitHub-flavored Markdown.
func BuildIssueBody(report db.BugReport, reporter db.User, baseURL string) string {
	var b strings.Builder

	desc := strings.TrimSpace(report.Description)
	b.WriteString("### Description\n")
	b.WriteString(desc)
	b.WriteString("\n\n### Context\n")
	fmt.Fprintf(&b, "- **Reporter:** %s\n", formatReporter(reporter))
	fmt.Fprintf(&b, "- **Page:** `%s`\n", valueOrUnknown(report.PageURL))
	fmt.Fprintf(&b, "- **Screen:** `%s`\n", valueOrUnknown(report.ScreenSize))
	fmt.Fprintf(&b, "- **Device:** `%s`\n", valueOrUnknown(report.UserAgent))

	createdAt := report.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	fmt.Fprintf(&b, "- **Time:** %s", createdAt.UTC().Format(time.RFC3339))

	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if len(report.Screenshot) > 0 && trimmedBase != "" && report.Token != "" {
		fmt.Fprintf(&b, "\n\n### Screenshot\n![Screenshot](%s/screenshots/%s.jpg)", trimmedBase, report.Token)
	}

	return b.String()
}

func formatReporter(reporter db.User) string {
	name := strings.TrimSpace(reporter.Name)
	email := strings.TrimSpace(reporter.Email)
	if name == "" {
		name = "Unknown"
	}
	if email != "" {
		return fmt.Sprintf("%s (%s)", name, email)
	}
	return name
}

func valueOrUnknown(value string) string {
	cleaned := strings.ReplaceAll(value, "`", "")
	collapsed := strings.Join(strings.Fields(cleaned), " ")
	if collapsed != "" {
		return collapsed
	}
	return "unknown"
}
