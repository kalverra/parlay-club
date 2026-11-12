package bugs

import (
	"context"
	"strings"

	"github.com/rs/zerolog"

	"github.com/kalverra/parlay-club/internal/db"
)

// ReportStore persists bug reports and records created GitHub issue metadata.
type ReportStore interface {
	CreateBugReport(ctx context.Context, report db.BugReport) (db.BugReport, error)
	SetGitHubIssue(ctx context.Context, id int64, issueURL string, issueNum int) error
}

// Deps holds dependencies for constructing a Service.
type Deps struct {
	Store       ReportStore
	GitHub      GitHubClient
	Repo        string
	BaseURL     string
	GitHubToken string
	Logger      zerolog.Logger
}

// SubmitInput captures the user-submitted fields for a bug report.
type SubmitInput struct {
	PageURL        string
	Description    string
	UserAgent      string
	ScreenSize     string
	Screenshot     []byte
	ScreenshotMIME string
}

// ReportResult summarizes the persisted bug report and any linked GitHub issue.
type ReportResult struct {
	Report   db.BugReport
	IssueURL string
	IssueNum int
}

// Service persists bug reports locally and optionally files GitHub issues.
type Service struct {
	store   ReportStore
	github  GitHubClient
	repo    string
	baseURL string
	logger  zerolog.Logger
}

// NewService constructs a bug reporting Service. If GitHub is nil and GitHubToken
// is non-empty, a default RESTClient is created.
func NewService(deps Deps) *Service {
	gh := deps.GitHub
	if gh == nil && strings.TrimSpace(deps.GitHubToken) != "" {
		gh = NewRESTClient(deps.GitHubToken)
	}
	return &Service{
		store:   deps.Store,
		github:  gh,
		repo:    strings.TrimSpace(deps.Repo),
		baseURL: strings.TrimRight(strings.TrimSpace(deps.BaseURL), "/"),
		logger:  deps.Logger,
	}
}

// SubmitReport saves the bug report in SQLite first, then attempts to create a
// GitHub issue when configured. GitHub errors are logged and never fail the submission.
func (s *Service) SubmitReport(ctx context.Context, reporter db.User, input SubmitInput) (ReportResult, error) {
	saved, err := s.store.CreateBugReport(ctx, db.BugReport{
		UserID:         reporter.ID,
		PageURL:        strings.TrimSpace(input.PageURL),
		Description:    input.Description,
		UserAgent:      strings.TrimSpace(input.UserAgent),
		ScreenSize:     strings.TrimSpace(input.ScreenSize),
		Screenshot:     input.Screenshot,
		ScreenshotMIME: strings.TrimSpace(input.ScreenshotMIME),
	})
	if err != nil {
		return ReportResult{}, err
	}

	if s.github == nil || s.repo == "" {
		s.logger.Warn().
			Int64("report_id", saved.ID).
			Msg("GitHub issue reporting not configured; bug report saved locally")
		return ReportResult{Report: saved}, nil
	}

	issue, err := s.github.CreateIssue(ctx, s.repo, CreateIssueRequest{
		Title:  BuildIssueTitle(saved.Description),
		Body:   BuildIssueBody(saved, reporter, s.baseURL),
		Labels: []string{"bug"},
	})
	if err != nil {
		s.logger.Warn().
			Err(err).
			Int64("report_id", saved.ID).
			Str("repo", s.repo).
			Msg("failed to create GitHub issue; bug report saved locally")
		return ReportResult{Report: saved}, nil
	}

	if err := s.store.SetGitHubIssue(ctx, saved.ID, issue.HTMLURL, issue.Number); err != nil {
		s.logger.Warn().
			Err(err).
			Int64("report_id", saved.ID).
			Int("issue_num", issue.Number).
			Msg("failed to record GitHub issue on bug report")
	}

	saved.GitHubIssueURL = &issue.HTMLURL
	saved.GitHubIssueNum = &issue.Number
	return ReportResult{
		Report:   saved,
		IssueURL: issue.HTMLURL,
		IssueNum: issue.Number,
	}, nil
}
