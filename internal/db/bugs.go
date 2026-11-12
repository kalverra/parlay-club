package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrBugReportNotFound is returned when a bug report lookup matches no row.
var ErrBugReportNotFound = errors.New("bug report not found")

// ErrBugReportDescriptionRequired is returned when creating a report with no description.
var ErrBugReportDescriptionRequired = errors.New("bug report description required")

// BugReport represents a user-submitted diagnostic capture.
type BugReport struct {
	ID             int64
	Token          string
	UserID         int64
	PageURL        string
	Description    string
	UserAgent      string
	ScreenSize     string
	Screenshot     []byte
	ScreenshotMIME string
	GitHubIssueURL *string
	GitHubIssueNum *int
	CreatedAt      time.Time
}

// bugReportColumns is the column list for bug report queries, matching scanBugReport order.
const bugReportColumns = `
	id, token, user_id, page_url, description, user_agent, screen_size,
	screenshot, screenshot_mime, github_issue_url, github_issue_num, created_at`

// GenerateBugReportToken generates a 16-byte cryptographically secure random token (32 hex chars).
func GenerateBugReportToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate bug report token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateBugReport inserts a new bug report. If Token is empty, an unguessable 16-byte
// random hex token is generated. If ScreenshotMIME is empty, it defaults to image/jpeg.
func (s *Store) CreateBugReport(ctx context.Context, report BugReport) (BugReport, error) {
	report.Description = strings.TrimSpace(report.Description)
	if report.Description == "" {
		return BugReport{}, ErrBugReportDescriptionRequired
	}

	if report.Token == "" {
		token, err := GenerateBugReportToken()
		if err != nil {
			return BugReport{}, err
		}
		report.Token = token
	}

	if report.ScreenshotMIME == "" {
		report.ScreenshotMIME = "image/jpeg"
	}

	result, err := s.conn().ExecContext(ctx, `
		INSERT INTO bug_reports (
			token, user_id, page_url, description, user_agent, screen_size,
			screenshot, screenshot_mime, github_issue_url, github_issue_num
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		report.Token,
		report.UserID,
		report.PageURL,
		report.Description,
		report.UserAgent,
		report.ScreenSize,
		report.Screenshot,
		report.ScreenshotMIME,
		nullableString(report.GitHubIssueURL),
		nullableInt(report.GitHubIssueNum),
	)
	if err != nil {
		return BugReport{}, fmt.Errorf("create bug report: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return BugReport{}, fmt.Errorf("create bug report: %w", err)
	}

	var createdAt string
	if err := s.conn().QueryRowContext(ctx,
		"SELECT created_at FROM bug_reports WHERE id = ?", id).Scan(&createdAt); err != nil {
		return BugReport{}, fmt.Errorf("create bug report: %w", err)
	}
	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return BugReport{}, fmt.Errorf("create bug report: %w", err)
	}

	report.ID = id
	report.CreatedAt = parsed
	return report, nil
}

// BugReportByID looks up a bug report by its primary key ID.
func (s *Store) BugReportByID(ctx context.Context, id int64) (BugReport, error) {
	report, err := scanBugReport(s.conn().QueryRowContext(ctx,
		"SELECT"+bugReportColumns+" FROM bug_reports WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BugReport{}, fmt.Errorf("bug report %d: %w", id, ErrBugReportNotFound)
		}
		return BugReport{}, fmt.Errorf("bug report %d: %w", id, err)
	}
	return report, nil
}

// ScreenshotByToken returns the raw screenshot blob and mime type for a capability token.
func (s *Store) ScreenshotByToken(ctx context.Context, token string) ([]byte, string, error) {
	var (
		screenshot []byte
		mime       string
	)
	err := s.conn().QueryRowContext(ctx,
		"SELECT screenshot, screenshot_mime FROM bug_reports WHERE token = ?", token).
		Scan(&screenshot, &mime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", fmt.Errorf("screenshot token %s: %w", token, ErrBugReportNotFound)
		}
		return nil, "", fmt.Errorf("screenshot token %s: %w", token, err)
	}
	return screenshot, mime, nil
}

// SetGitHubIssue updates the GitHub issue URL and issue number for a bug report by ID.
func (s *Store) SetGitHubIssue(ctx context.Context, id int64, issueURL string, issueNum int) error {
	result, err := s.conn().ExecContext(ctx, `
		UPDATE bug_reports
		SET github_issue_url = ?, github_issue_num = ?
		WHERE id = ?`,
		issueURL, issueNum, id,
	)
	if err != nil {
		return fmt.Errorf("set github issue for bug report %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("set github issue for bug report %d: %w", id, err)
	}
	if rows == 0 {
		return fmt.Errorf("set github issue for bug report %d: %w", id, ErrBugReportNotFound)
	}
	return nil
}

// scanBugReport reads the columns bugReportColumns selects, in order.
func scanBugReport(row rowScanner) (BugReport, error) {
	var (
		report         BugReport
		githubIssueURL sql.NullString
		githubIssueNum sql.NullInt64
		createdAt      string
	)

	if err := row.Scan(
		&report.ID,
		&report.Token,
		&report.UserID,
		&report.PageURL,
		&report.Description,
		&report.UserAgent,
		&report.ScreenSize,
		&report.Screenshot,
		&report.ScreenshotMIME,
		&githubIssueURL,
		&githubIssueNum,
		&createdAt,
	); err != nil {
		return BugReport{}, err
	}

	parsed, err := scanTimestamp(createdAt)
	if err != nil {
		return BugReport{}, err
	}

	report.GitHubIssueURL = nullableStringPtr(githubIssueURL)
	report.GitHubIssueNum = nullableIntPtr(githubIssueNum)
	report.CreatedAt = parsed
	return report, nil
}
