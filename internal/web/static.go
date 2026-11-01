package web

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"sync"

	assets "github.com/kalverra/parlay-club/web"
)

// staticCache is safe because AssetURL puts the file's content hash in every
// URL a template renders.
const staticCache = "public, max-age=31536000, immutable"

// assetHashes maps each file in web/static to a short content hash. The files
// are embedded, so this cannot fail.
var assetHashes = sync.OnceValue(func() map[string]string {
	entries, err := fs.ReadDir(assets.Static, "static")
	if err != nil {
		panic("web: read embedded static files: " + err.Error())
	}

	hashes := make(map[string]string, len(entries))
	for _, entry := range entries {
		content, err := fs.ReadFile(assets.Static, path.Join("static", entry.Name()))
		if err != nil {
			panic("web: read embedded static file " + entry.Name() + ": " + err.Error())
		}
		hashes[entry.Name()] = contentHash(content)
	}

	return hashes
})

// AssetURL returns the URL for a file in web/static. The content hash in the
// query makes the long cache in StaticHandler safe.
func AssetURL(name string) string {
	hash, ok := assetHashes()[name]
	if !ok {
		return "/static/" + name
	}

	return "/static/" + name + "?v=" + hash
}

// StaticHandler serves web/static with a one-year cache.
func StaticHandler() http.Handler {
	sub, err := fs.Sub(assets.Static, "static")
	if err != nil {
		panic(fmt.Sprintf("web: embedded static dir: %v", err))
	}

	files := http.StripPrefix("/static/", http.FileServerFS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", staticCache)
		files.ServeHTTP(w, request)
	})
}

// contentHash returns a short, stable fingerprint of a file.
func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:5])
}
