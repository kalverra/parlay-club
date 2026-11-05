// Package web holds the server's templates and static assets.
package web

import "embed"

//go:generate tailwindcss -i ./tailwind/input.css -o ./static/app.css --minify

// Static holds the files served under /static: the hand-written app.js and
// vendored HTMX, plus app.css, which `go generate ./...` writes and commits.
//
//go:embed static
var Static embed.FS

// Templates holds the page templates. The layout defines "layout" and each
// page file defines the "content" block it wraps.
//
//go:embed templates
var Templates embed.FS
