# Final fix

## Commit

Remove the unreachable props redirect

Production's CSRF guard answers 403 to a non-HTMX POST before the props handler runs, so the redirect branch and the doc
lines that promised it described a path nobody could reach. Keeping them would mislead readers and invite a test that
passes only in the harness; the route now documents the refreshed explorer partial that htmx actually receives.
