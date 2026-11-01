// Package web holds the server's templates and static assets.
package web

import "embed"

//go:generate tailwindcss -i ./tailwind/input.css -o ./static/app.css --minify

// Static holds the files served under /static: the hand-written app.js and
// vendored HTMX, plus app.css, which `go generate ./...` writes and commits.
//
//go:embed static
var Static embed.FS
