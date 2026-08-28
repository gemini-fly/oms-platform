package audit

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestLogsIncludeActorAndTime(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	server.Store.Lock()
	server.Store.Audit(0, "test.action", "resource", 99, "success", "operator visible")
	server.Store.Unlock()

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"actorDisplayName", "Platform Admin", "actorUsername", "test.action", "createdAt"} {
		if !strings.Contains(body, want) {
			t.Fatalf("audit response should contain %q: %s", want, body)
		}
	}
}

func TestLogsRequireOpsRole(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.CurrentUserID = store.Users[3].ID
	store.PolicyBindings = []platform.PolicyBinding{{
		ID:        store.Next("binding"),
		UserID:    store.CurrentUserID,
		RoleID:    store.Roles[3].ID,
		RoleCode:  "developer",
		ScopeType: "global",
	}}
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/audit/logs", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}
