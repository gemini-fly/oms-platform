package iam

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestMenuPermissionsCanBeUpdated(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	t.Setenv("SY_PLATFORM_MENU_PERMISSION_FILE", filepath.Join(t.TempDir(), "menus.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := strings.NewReader(`[
		{"menuKey":"overview","roleCodes":["developer"]},
		{"menuKey":"cmdb","roleCodes":["ops_owner"]},
		{"menuKey":"settings","roleCodes":[]},
		{"menuKey":"unknown","roleCodes":["developer"]}
	]`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/iam/menu-permissions", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, want := range []string{`"menuKey":"overview"`, `"roleCodes":["platform_admin","developer"]`, `"menuKey":"cmdb"`, `"roleCodes":["platform_admin","ops_owner"]`, `"menuKey":"settings"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("menu permission response should contain %q: %s", want, rec.Body.String())
		}
	}
	for _, want := range []string{`"menuKey":"unknown"`, `"menuName":"unknown"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("new menu should be persisted with %q: %s", want, rec.Body.String())
		}
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "iam.menu_permission.update" {
		t.Fatalf("audit action = %q, want iam.menu_permission.update", last.Action)
	}
}

func TestMenuPermissionsKeepNewMenuName(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	t.Setenv("SY_PLATFORM_MENU_PERMISSION_FILE", filepath.Join(t.TempDir(), "menus.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := strings.NewReader(`[{"menuKey":"lab","menuName":"实验室","roleCodes":["developer"]}]`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/iam/menu-permissions", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, want := range []string{`"menuKey":"lab"`, `"menuName":"实验室"`, `"roleCodes":["platform_admin","developer"]`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("new menu response should contain %q: %s", want, rec.Body.String())
		}
	}
}

func TestProfileIncludesMenuPermissions(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/iam/profile", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"menuPermissions"`) {
		t.Fatalf("profile should include menu permissions: %s", rec.Body.String())
	}
}

func TestProfileUsesCurrentActor(t *testing.T) {
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
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/iam/profile", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, want := range []string{`"username":"developer"`, `"roleCode":"developer"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("profile should contain %q: %s", want, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), `"username":"admin"`) {
		t.Fatalf("profile user should not be hard-coded to admin: %s", rec.Body.String())
	}
}

func TestMenuPermissionUpdateRequiresPlatformAdmin(t *testing.T) {
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

	body := strings.NewReader(`[{"menuKey":"cmdb","roleCodes":["developer"]}]`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/iam/menu-permissions", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestPolicyBindingsRequirePlatformAdmin(t *testing.T) {
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

	getRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/v1/iam/policy-bindings", nil))
	if getRec.Code != http.StatusForbidden {
		t.Fatalf("GET status = %d, want %d, body=%s", getRec.Code, http.StatusForbidden, getRec.Body.String())
	}

	postRec := httptest.NewRecorder()
	body := strings.NewReader(`{"userId":4,"roleCode":"ops_owner","scopeType":"global"}`)
	server.Mux.ServeHTTP(postRec, httptest.NewRequest(http.MethodPost, "/api/v1/iam/policy-bindings", body))
	if postRec.Code != http.StatusForbidden {
		t.Fatalf("POST status = %d, want %d, body=%s", postRec.Code, http.StatusForbidden, postRec.Body.String())
	}
}

func TestPolicyBindingRejectsUnknownUser(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := strings.NewReader(`{"userId":999,"roleCode":"developer","scopeType":"global"}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/iam/policy-bindings", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "USER_NOT_FOUND") {
		t.Fatalf("response should explain unknown user: %s", rec.Body.String())
	}
}
