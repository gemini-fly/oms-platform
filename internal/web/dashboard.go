package web

import (
	"bytes"
	"embed"
	"errors"
	"mime"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

//go:embed static/*
var embeddedFrontend embed.FS

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/", dashboard)
}

func dashboard(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	if configured := strings.TrimSpace(os.Getenv("SY_PLATFORM_FRONTEND_DIR")); configured != "" {
		frontendDir, err := validFrontendDir(configured)
		if err != nil {
			http.Error(w, "configured frontend directory not found", http.StatusInternalServerError)
			return
		}
		serveFrontendDir(w, r, frontendDir)
		return
	}
	if frontendDir, err := resolveFrontendDir(); err == nil {
		serveFrontendDir(w, r, frontendDir)
		return
	}
	serveEmbeddedFrontend(w, r)
}

func serveFrontendDir(w http.ResponseWriter, r *http.Request, frontendDir string) {
	path := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
	if path == "." || path == "" {
		serveFrontendFile(w, r, filepath.Join(frontendDir, "index.html"), "text/html; charset=utf-8")
		return
	}

	filePath := filepath.Join(frontendDir, path)
	if !isPathInside(frontendDir, filePath) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filePath)
}

func serveEmbeddedFrontend(w http.ResponseWriter, r *http.Request) {
	requestPath := strings.TrimPrefix(pathpkg.Clean("/"+r.URL.Path), "/")
	if requestPath == "." || requestPath == "" {
		requestPath = "index.html"
	}
	content, err := embeddedFrontend.ReadFile("static/" + requestPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if contentType := mime.TypeByExtension(filepath.Ext(requestPath)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if requestPath == "index.html" {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeContent(w, r, requestPath, time.Time{}, bytes.NewReader(content))
}

func serveFrontendFile(w http.ResponseWriter, r *http.Request, path string, contentType string) {
	w.Header().Set("Cache-Control", "no-store")
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeFile(w, r, path)
}

func resolveFrontendDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("SY_PLATFORM_FRONTEND_DIR")); dir != "" {
		return validFrontendDir(dir)
	}
	if cwd, err := os.Getwd(); err == nil {
		if dir, err := findFrontendDir(cwd); err == nil {
			return dir, nil
		}
	}
	_, file, _, ok := runtime.Caller(0)
	if ok {
		if dir, err := findFrontendDir(filepath.Dir(file)); err == nil {
			return dir, nil
		}
	}
	return "", errors.New("frontend directory not found")
}

func findFrontendDir(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "frontend")
		if valid, err := validFrontendDir(candidate); err == nil {
			return valid, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("frontend directory not found")
}

func validFrontendDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(filepath.Join(abs, "index.html"))
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", errors.New("frontend index.html is a directory")
	}
	return abs, nil
}

func isPathInside(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel))
}
