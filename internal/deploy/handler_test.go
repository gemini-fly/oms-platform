package deploy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestReleaseCreationRequiresServiceManager(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	body := bytes.NewBufferString(`{"serviceId":1,"environmentId":1,"version":"v1.2.4"}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/deploy/releases", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestReleaseOperationsRequireOpsRole(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/deploy/releases/1/start", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "RELEASE_OPERATE_FORBIDDEN") {
		t.Fatalf("response should explain release operation permission: %s", rec.Body.String())
	}
}

func setOnlyRole(t *testing.T, store *platform.Store, userID int64, roleCode string) {
	t.Helper()
	store.CurrentUserID = userID
	store.PolicyBindings = []platform.PolicyBinding{roleBinding(t, store, userID, roleCode)}
}

func roleBinding(t *testing.T, store *platform.Store, userID int64, roleCode string) platform.PolicyBinding {
	t.Helper()
	for _, role := range store.Roles {
		if role.Code == roleCode {
			return platform.PolicyBinding{ID: store.Next("binding"), UserID: userID, RoleID: role.ID, RoleCode: role.Code, ScopeType: "global"}
		}
	}
	t.Fatalf("role %q not found", roleCode)
	return platform.PolicyBinding{}
}
