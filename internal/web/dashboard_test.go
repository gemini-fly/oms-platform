package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDashboardServesConfiguredFrontendIndex(t *testing.T) {
	frontendDir := t.TempDir()
	index := `<!doctype html><html><body><h1>ops frontend</h1></body></html>`
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte(index), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	t.Setenv("SY_PLATFORM_FRONTEND_DIR", frontendDir)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	dashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "ops frontend") {
		t.Fatalf("dashboard body should contain frontend index: %s", rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
}

func TestDashboardServesEmbeddedFrontend(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	serveEmbeddedFrontend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `id="authScreen"`) {
		t.Fatalf("embedded dashboard should contain the login shell")
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache-control = %q, want no-store", got)
	}
}

func TestDashboardServesConfiguredFrontendStaticAsset(t *testing.T) {
	frontendDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("index"), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	if err := os.WriteFile(filepath.Join(frontendDir, "asset.txt"), []byte("asset"), 0644); err != nil {
		t.Fatalf("write asset: %v", err)
	}
	t.Setenv("SY_PLATFORM_FRONTEND_DIR", frontendDir)

	req := httptest.NewRequest(http.MethodGet, "/asset.txt", nil)
	rec := httptest.NewRecorder()

	dashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "asset" {
		t.Fatalf("body = %q, want asset", body)
	}
}

func TestDashboardDoesNotFallbackForAPIRoutes(t *testing.T) {
	frontendDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("index"), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	t.Setenv("SY_PLATFORM_FRONTEND_DIR", frontendDir)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	rec := httptest.NewRecorder()

	dashboard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestResolveFrontendDirRequiresIndex(t *testing.T) {
	frontendDir := t.TempDir()
	t.Setenv("SY_PLATFORM_FRONTEND_DIR", frontendDir)

	if _, err := resolveFrontendDir(); err == nil {
		t.Fatalf("resolveFrontendDir should fail without index.html")
	}
}

func TestDashboardRejectsPathTraversal(t *testing.T) {
	frontendDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(frontendDir, "index.html"), []byte("index"), 0644); err != nil {
		t.Fatalf("write index: %v", err)
	}
	t.Setenv("SY_PLATFORM_FRONTEND_DIR", frontendDir)

	req := httptest.NewRequest(http.MethodGet, "/../go.mod", nil)
	rec := httptest.NewRecorder()

	dashboard(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
