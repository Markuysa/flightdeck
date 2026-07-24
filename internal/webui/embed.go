// Package webui embeds the built React SPA and serves it from the same
// process and port as the API (ADR-002: single binary, no external asset
// fetch at runtime).
//
// Build step (also needed by the README/demo ticket):
//
//  1. npm run build                        (in ui/)
//  2. cp -r ui/dist/. internal/webui/dist/  (from the repo root)
//  3. go build -o bin/flightdeck ./cmd/flightdeck
//
// ui/dist is gitignored and does not exist on a clean checkout, but
// //go:embed fails at *compile time* if its pattern matches no files — a
// bare `//go:embed dist` pointed at ui/dist would break `go build ./...` on
// every clean checkout, including CI's Go job. So this package instead
// embeds a directory that lives inside the package itself,
// internal/webui/dist/, and internal/webui/dist/index.html is committed as
// a small placeholder that keeps the embed pattern always resolving. The
// real build step above copies the built assets on top of that placeholder
// (overwriting index.html along with everything else); every other file
// under internal/webui/dist/ is gitignored, so a real build never gets
// committed by accident. See the repo root .gitignore for the exact rule.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// FS returns the embedded UI's root: internal/webui/dist with the "dist"
// prefix stripped, so paths inside it match what the SPA expects at "/".
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		// Only fails on a malformed path, which "dist" (embedded above) is
		// not — this can only happen if the source itself is broken.
		panic(err)
	}
	return sub
}

// Handler serves the embedded SPA over the process's real assets.
func Handler() http.Handler { return HandlerFS(FS()) }

// HandlerFS builds an SPA-serving handler over assets: a request naming a
// real file is served as itself, with a content type http.FileServer
// derives from its extension; any other path (an unknown route, or a hard
// refresh on a client-side route like /p/:id or /p/:id/t/:tid) falls back
// to index.html so the SPA's own router can take over. Exported (not just
// Handler) so tests can drive the routing logic against a small in-memory
// fs.FS instead of the real embedded build.
func HandlerFS(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isRealFile(assets, r.URL.Path) {
			// Rewrite to "/", not "/index.html": http.FileServer 301s any
			// request whose path literally ends in "/index.html" (it
			// canonicalizes that to the directory root), so falling back to
			// the file itself would redirect instead of serving 200 — "/"
			// hits its normal directory-index behavior and serves the same
			// content directly.
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}

// isRealFile reports whether urlPath names an actual, non-directory file in
// assets.
func isRealFile(assets fs.FS, urlPath string) bool {
	p := strings.TrimPrefix(path.Clean(urlPath), "/")
	if p == "" || p == "." {
		return false
	}
	info, err := fs.Stat(assets, p)
	return err == nil && !info.IsDir()
}
