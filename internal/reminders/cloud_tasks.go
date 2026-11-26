package reminders

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// defaultTasksEndpoint is the Cloud Tasks REST root.
	defaultTasksEndpoint = "https://cloudtasks.googleapis.com/v2"
	// defaultMetadataURL is the runtime service account's token endpoint.
	defaultMetadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"
	// metadataFlavor is the header the metadata server requires.
	metadataFlavor = "Google"
	// tokenRefreshMargin refreshes a token this long before it expires.
	tokenRefreshMargin = time.Minute
	// taskNameHashLen is how many hex characters prefix a task name.
	taskNameHashLen = 4
	// cloudTasksTimeout bounds one enqueue, token fetch included.
	cloudTasksTimeout = 10 * time.Second
	// maxCloudTasksBodyBytes caps how much of a response is read.
	maxCloudTasksBodyBytes = 2048
)

// CloudTasksConfig names the queue and the endpoint a task calls.
type CloudTasksConfig struct {
	// Queue is the full path, projects/{project}/locations/{region}/queues/{queue}.
	Queue string
	// BaseURL is the app's public root, without a trailing slash.
	BaseURL string
	// Token is the bearer value POST /internal/reminders expects.
	Token string
}

// CloudTasksOption configures a CloudTasks scheduler, mostly for tests.
type CloudTasksOption func(*CloudTasks)

// WithTasksEndpoint points the scheduler at another Cloud Tasks REST root.
func WithTasksEndpoint(endpoint string) CloudTasksOption {
	return func(c *CloudTasks) {
		if trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/"); trimmed != "" {
			c.tasksURL = trimmed + "/" + c.queue + "/tasks"
		}
	}
}

// WithMetadataURL points the token cache at another metadata endpoint.
func WithMetadataURL(endpoint string) CloudTasksOption {
	return func(c *CloudTasks) {
		if trimmed := strings.TrimSpace(endpoint); trimmed != "" {
			c.tokens.url = trimmed
		}
	}
}

// WithHTTPClient replaces the client that calls both endpoints.
func WithHTTPClient(client *http.Client) CloudTasksOption {
	return func(c *CloudTasks) {
		if client != nil {
			c.client = client
			c.tokens.client = client
		}
	}
}

// CloudTasks schedules reminders through the Cloud Tasks REST API. It needs no
// client library: one JSON POST creates the task, and the runtime service
// account's token comes from the metadata server and is cached until it nears
// expiry. Sync enforces MaxHorizon, which is Cloud Tasks' own 30-day limit.
type CloudTasks struct {
	queue    string
	baseURL  string
	token    string
	tasksURL string
	client   *http.Client
	tokens   *metadataTokenCache
}

// NewCloudTasks builds a scheduler. It rejects a queue path that Google would
// not accept, a base URL that cannot be called, and an empty token, so a
// misconfigured deploy fails at boot instead of at kickoff.
func NewCloudTasks(cfg CloudTasksConfig, opts ...CloudTasksOption) (*CloudTasks, error) {
	queue := strings.Trim(strings.TrimSpace(cfg.Queue), "/")
	if !validQueuePath(queue) {
		return nil, fmt.Errorf(
			"reminders: REMINDER_QUEUE must be projects/{project}/locations/{region}/queues/{queue}, got %q",
			cfg.Queue,
		)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, fmt.Errorf("reminders: BASE_URL must be an http or https URL, got %q", cfg.BaseURL)
	}
	token := strings.TrimSpace(cfg.Token)
	if token == "" {
		return nil, errors.New("reminders: REMINDER_TOKEN is required to schedule reminders")
	}

	client := &http.Client{Timeout: cloudTasksTimeout}
	scheduler := &CloudTasks{
		queue:    queue,
		baseURL:  baseURL,
		token:    token,
		tasksURL: defaultTasksEndpoint + "/" + queue + "/tasks",
		client:   client,
		tokens:   &metadataTokenCache{url: defaultMetadataURL, client: client, now: time.Now},
	}
	for _, opt := range opts {
		opt(scheduler)
	}
	return scheduler, nil
}

// Enqueue creates one named task that POSTs the reminder endpoint at at. A
// duplicate name comes back as a 409, which means the task already exists and
// is not an error.
func (c *CloudTasks) Enqueue(ctx context.Context, slipID int64, start, at time.Time) error {
	if slipID <= 0 {
		return fmt.Errorf("reminders: slip id %d is not positive", slipID)
	}
	token, err := c.tokens.get(ctx)
	if err != nil {
		return fmt.Errorf("reminders: cloud tasks token: %w", err)
	}
	payload, err := json.Marshal(c.task(slipID, start, at))
	if err != nil {
		return fmt.Errorf("reminders: marshal cloud task: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tasksURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("reminders: build cloud task request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("reminders: create cloud task: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusConflict {
		return ErrAlreadyExists
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxCloudTasksBodyBytes))
		return fmt.Errorf(
			"reminders: create cloud task: status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}
	return nil
}

// cloudTask is the tasks.create request body.
type cloudTask struct {
	Task taskPayload `json:"task"`
}

// taskPayload is the task itself: a stable name, the schedule, and the HTTP
// target that runs the reminder.
type taskPayload struct {
	Name         string      `json:"name"`
	ScheduleTime string      `json:"scheduleTime"`
	HTTPRequest  taskRequest `json:"httpRequest"`
}

// taskRequest is the HTTP target Cloud Tasks calls at the schedule time.
type taskRequest struct {
	URL        string            `json:"url"`
	HTTPMethod string            `json:"httpMethod"`
	Headers    map[string]string `json:"headers"`
}

// task builds the request body for one slip start.
func (c *CloudTasks) task(slipID int64, start, at time.Time) cloudTask {
	unix := start.UTC().Unix()
	return cloudTask{Task: taskPayload{
		Name:         c.taskName(slipID, unix),
		ScheduleTime: at.UTC().Format(time.RFC3339),
		HTTPRequest: taskRequest{
			URL:        fmt.Sprintf("%s/internal/reminders?slip=%d&start=%d", c.baseURL, slipID, unix),
			HTTPMethod: http.MethodPost,
			Headers:    map[string]string{"Authorization": "Bearer " + c.token},
		},
	}}
}

// taskName is deterministic, so Cloud Tasks rejects a second task for the same
// slip and start with ALREADY_EXISTS. The short hash prefix spreads names, as
// Cloud Tasks recommends for named tasks.
func (c *CloudTasks) taskName(slipID, startUnix int64) string {
	key := fmt.Sprintf("slip-%d-%d", slipID, startUnix)
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	prefix := fmt.Sprintf("%0*x", taskNameHashLen, hash.Sum32()&0xffff)
	return c.queue + "/tasks/" + prefix + "-" + key
}

// validQueuePath requires the full resource name Google's API expects.
func validQueuePath(queue string) bool {
	parts := strings.Split(queue, "/")
	if len(parts) != 6 {
		return false
	}
	if parts[0] != "projects" || parts[2] != "locations" || parts[4] != "queues" {
		return false
	}
	return parts[1] != "" && parts[3] != "" && parts[5] != ""
}

// metadataTokenCache holds the service account token between enqueues. The
// metadata server issues tokens for about an hour, so fetching one per task
// would be wasteful on a burst of slips.
type metadataTokenCache struct {
	mu     sync.Mutex
	url    string
	client *http.Client
	now    func() time.Time

	token  string
	expiry time.Time
}

// get returns a cached token, or fetches a new one when the cached token is
// missing or about to expire.
func (t *metadataTokenCache) get(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	if t.token != "" && now.Add(tokenRefreshMargin).Before(t.expiry) {
		return t.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return "", fmt.Errorf("build metadata token request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", metadataFlavor)
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch metadata token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxCloudTasksBodyBytes))
	if err != nil {
		return "", fmt.Errorf("read metadata token: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"metadata token: status %d: %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var doc struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("decode metadata token: %w", err)
	}
	if doc.AccessToken == "" {
		return "", errors.New("metadata token: empty access token")
	}

	ttl := time.Duration(doc.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Minute
	}
	t.token = doc.AccessToken
	t.expiry = now.Add(ttl)
	return t.token, nil
}
