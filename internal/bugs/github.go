// Package bugs handles persisting bug reports and filing GitHub issues.
package bugs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultGitHubBaseURL = "https://api.github.com"
	defaultHTTPTimeout   = 10 * time.Second
	maxErrorBodyBytes    = 4096
)

var (
	// ErrMissingGitHubToken is returned when a GitHub request is attempted without a token.
	ErrMissingGitHubToken = errors.New("github token is required")

	// ErrInvalidGitHubRepo is returned when the target repository is not in owner/repo format.
	ErrInvalidGitHubRepo = errors.New("github repo must be in owner/repo format")
)

// GitHubClient creates issues in a target GitHub repository.
type GitHubClient interface {
	CreateIssue(ctx context.Context, repo string, req CreateIssueRequest) (*IssueResult, error)
}

// CreateIssueRequest is the payload sent to GitHub's create-issue endpoint.
type CreateIssueRequest struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
}

// IssueResult holds the created GitHub issue metadata returned by the API.
type IssueResult struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
}

// RESTClientOption configures a RESTClient.
type RESTClientOption func(*RESTClient)

// WithBaseURL overrides the GitHub API base URL (used in tests).
func WithBaseURL(baseURL string) RESTClientOption {
	return func(c *RESTClient) {
		if trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/"); trimmed != "" {
			c.baseURL = trimmed
		}
	}
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(client *http.Client) RESTClientOption {
	return func(c *RESTClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// RESTClient calls the GitHub REST API v3 to create issues.
type RESTClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewRESTClient constructs a GitHub REST API client authenticated with token.
func NewRESTClient(token string, opts ...RESTClientOption) *RESTClient {
	client := &RESTClient{
		token:      strings.TrimSpace(token),
		baseURL:    defaultGitHubBaseURL,
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// CreateIssue posts a new issue to POST /repos/{owner}/{repo}/issues.
func (c *RESTClient) CreateIssue(
	ctx context.Context,
	repo string,
	req CreateIssueRequest,
) (*IssueResult, error) {
	if c.token == "" {
		return nil, ErrMissingGitHubToken
	}

	owner, name, ok := parseRepo(repo)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidGitHubRepo, repo)
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal github issue request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/issues", c.baseURL, owner, name)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build github issue request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Github-Api-Version", "2022-11-28")
	httpReq.Header.Set("User-Agent", "parlay-club")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("post github issue: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read github issue response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github api returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result IssueResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode github issue response: %w", err)
	}
	return &result, nil
}

func parseRepo(repo string) (owner, name string, ok bool) {
	trimmed := strings.Trim(strings.TrimSpace(repo), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
