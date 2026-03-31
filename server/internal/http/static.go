package http

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:static/dist
var embeddedStaticFiles embed.FS

var staticDistFS fs.FS = mustSubFS(embeddedStaticFiles, "static/dist")

func mustSubFS(root fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(root, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

func hasEmbeddedStaticFile(requestPath string) bool {
	name, ok := normalizeStaticPath(requestPath)
	if !ok || name == "index.html" || strings.HasPrefix(path.Base(name), ".") {
		return false
	}

	info, err := fs.Stat(staticDistFS, name)
	return err == nil && !info.IsDir()
}

func hasEmbeddedIndex() bool {
	info, err := fs.Stat(staticDistFS, "index.html")
	return err == nil && !info.IsDir()
}

func normalizeStaticPath(requestPath string) (string, bool) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(requestPath, "/"))
	if trimmed == "" {
		return "index.html", true
	}

	cleaned := path.Clean(trimmed)
	if cleaned == "." {
		return "index.html", true
	}
	if !fs.ValidPath(cleaned) {
		return "", false
	}

	return cleaned, true
}

func serveEmbeddedPath(w http.ResponseWriter, r *http.Request, name string) {
	http.ServeFileFS(w, r, staticDistFS, name)
}
