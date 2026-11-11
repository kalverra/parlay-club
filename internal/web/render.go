package web

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
)

// pageFiles maps a page name to the template file that defines its content.
// The shell and the auth screens share the layout, so they share this map.
var pageFiles = map[string]string{
	"home":      "templates/home.html",
	"slip":      "templates/slip.html",
	"games":     "templates/games.html",
	"research":  "templates/research.html",
	"record":    "templates/record.html",
	"login":     "templates/login.html",
	"dev-login": "templates/dev-login.html",
	"join":      "templates/join.html",
	"groups":    "templates/groups.html",
	"profile":   "templates/profile.html",
	"sandbox":   "templates/sandbox.html",
}

// partialsFile holds the fragments HTMX swaps into a page. Each fragment is a
// named template in the file.
const partialsFile = "templates/partials.html"

// partialFiles names the fragments the slip screen can render on its own.
var partialFiles = map[string]string{
	"group-list":      partialsFile,
	"slip-panel":      partialsFile,
	"slip-export":     partialsFile,
	"slip-activity":   partialsFile,
	"slip-payout":     partialsFile,
	"leg-form":        partialsFile,
	"leg-added":       partialsFile,
	"game-props":      partialsFile,
	"research-game":   partialsFile,
	"research-player": partialsFile,
}

// Renderer parses and runs the page templates. It is safe to use from several
// requests at once.
type Renderer struct {
	fsys   fs.FS
	reload bool
	pages  map[string]*template.Template
}

// NewRenderer loads the layout and every page from fsys. With reload set it
// reads the templates again on each Render, so a dev edit shows up on the next
// request. Either way it parses once up front, so a broken template fails at
// startup.
func NewRenderer(fsys fs.FS, reload bool) (*Renderer, error) {
	renderer := &Renderer{
		fsys:   fsys,
		reload: reload,
		pages:  make(map[string]*template.Template, len(pageFiles)),
	}

	for name, file := range pageFiles {
		tmpl, err := renderer.parse(file)
		if err != nil {
			return nil, err
		}
		if !reload {
			renderer.pages[name] = tmpl
		}
	}

	return renderer, nil
}

// Render returns the page's full HTML for data, which must satisfy the layout's
// fields. It buffers, so a failed render writes nothing.
func (r *Renderer) Render(name string, data any) ([]byte, error) {
	tmpl, err := r.template(name)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, "layout", data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}

	return out.Bytes(), nil
}

// RenderPartial returns one HTMX fragment for data. It buffers, so a failed
// render writes nothing.
func (r *Renderer) RenderPartial(name string, data any) ([]byte, error) {
	file, ok := partialFiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown partial %q", name)
	}

	tmpl, err := template.New("root").
		Funcs(templateFuncs()).
		ParseFS(r.fsys, "templates/avatar.html", file)
	if err != nil {
		return nil, fmt.Errorf("parse partial %s: %w", name, err)
	}

	var out bytes.Buffer
	if err := tmpl.ExecuteTemplate(&out, name, data); err != nil {
		return nil, fmt.Errorf("render partial %s: %w", name, err)
	}
	return out.Bytes(), nil
}

// template returns the parsed page, reloading it from the file system in dev.
func (r *Renderer) template(name string) (*template.Template, error) {
	file, ok := pageFiles[name]
	if !ok {
		return nil, fmt.Errorf("unknown page %q", name)
	}
	if !r.reload {
		return r.pages[name], nil
	}

	return r.parse(file)
}

// templateFuncs are the helpers every template set shares.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"asset":       AssetURL,
		"list":        func(values ...string) []string { return values },
		"inc":         func(value int) int { return value + 1 },
		"resultClass": resultClass,
	}
}

// parse builds one template set from the layout and a page.
func (r *Renderer) parse(file string) (*template.Template, error) {
	tmpl, err := template.New("root").
		Funcs(templateFuncs()).
		ParseFS(r.fsys, "templates/layout.html", "templates/avatar.html", partialsFile, file)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return tmpl, nil
}
