-- Bug reports stores user-submitted bug and diagnostic captures with an
-- unguessable token for capability screenshot delivery and GitHub issue linking.
CREATE TABLE IF NOT EXISTS bug_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    page_url TEXT NOT NULL,
    description TEXT NOT NULL,
    user_agent TEXT NOT NULL,
    screen_size TEXT NOT NULL,
    screenshot BLOB,
    screenshot_mime TEXT NOT NULL DEFAULT 'image/jpeg',
    github_issue_url TEXT,
    github_issue_num INTEGER,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
