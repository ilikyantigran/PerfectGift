// Package web embeds the log-viewer SPA and serves it with an index.html
// fallback for client-side routes.
//
// INTEGRATION: the real UI is built separately in
// services/frontend/log-viewer. During integration, replace the contents of
// internal/web/dist/ with that project's `dist/` output (index.html + assets).
// The go:embed directive below picks up whatever lives in dist/.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var embedded embed.FS

// Handler serves the embedded SPA: real files are served as-is; any other
// (non-/api) path falls back to index.html so client-side routing works.
func Handler() (http.Handler, error) {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(dist))

	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil, err
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			serveIndex(w, index)
			return
		}
		if _, err := fs.Stat(dist, p); err != nil {
			// Unknown path: SPA fallback to index.html.
			serveIndex(w, index)
			return
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func serveIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}
