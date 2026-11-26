// Package web holds the server's templates and static assets.
package web

import "embed"

//go:generate tailwindcss -i ./tailwind/input.css -o ./static/app.css --minify
//go:generate go run ../cmd/genicons -out static

// Static holds the files served under /static: the hand-written app.js and
// vendored HTMX, the generated icons, plus app.css, which `go generate ./...`
// writes and commits.
//
//go:embed static
var Static embed.FS

// ServiceWorker is the script served at /sw.js. It sits outside static because
// the worker's scope follows its URL, and /static/ would only control itself.
//
//go:embed sw.js
var ServiceWorker []byte

// Templates holds the page templates. The layout defines "layout" and each
// page file defines the "content" block it wraps.
//
//go:embed templates
var Templates embed.FS
