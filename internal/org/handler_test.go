package org

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestDingTalkConfigCanBeSavedWithoutReturningSecret(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	t.Setenv("SY_PLATFORM_ORG_CONFIG_FILE", filepath.Join(t.TempDir(), "org.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := strings.NewReader(`{"corpId":"dingcorp","appKey":"dingkey","appSecret":"secret-value","agentId":"123456","rootDeptId":"1","syncMode":"scheduled","status":"enabled"}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/org/dingtalk/config", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, want := range []string{`"corpId":"dingcorp"`, `"configured":true`, `"appSecretSet":true`, `"syncMode":"scheduled"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("config response should contain %q: %s", want, rec.Body.String())
		}
	}
	if strings.Contains(rec.Body.String(), "secret-value") {
		t.Fatalf("config response should not return app secret: %s", rec.Body.String())
	}
	if got := server.Store.DingTalkConfig.AppSecret; got != "secret-value" {
		t.Fatalf("stored app secret = %q, want secret-value", got)
	}
}

func TestDepartmentsExposeDingTalkManagers(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/org/departments", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{`"source":"dingtalk"`, `"managerUserId"`, `"managerDisplayName":"研发主管"`, `"managerDisplayName":"运维主管"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("departments response should contain %q: %s", want, body)
		}
	}
}

func TestGitLabMappingsCanBeSaved(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	t.Setenv("SY_PLATFORM_GITLAB_MAPPING_FILE", filepath.Join(t.TempDir(), "gitlab-mappings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	departmentID := server.Store.Departments[1].ID
	body := strings.NewReader(fmt.Sprintf(`[
		{"departmentId":%d,"gitlabGroupPath":"platform/payment","accessLevel":"owner","syncMode":"manual","status":"enabled"}
	]`, departmentID))
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/org/gitlab-mappings", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, want := range []string{`"departmentName":"研发中心"`, `"gitlabGroupPath":"platform/payment"`, `"accessLevel":"owner"`, `"syncMode":"manual"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("mapping response should contain %q: %s", want, rec.Body.String())
		}
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "org.gitlab_mapping.update" {
		t.Fatalf("audit action = %q, want org.gitlab_mapping.update", last.Action)
	}
}

func TestSyncDingTalkIsAudited(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/org/sync/dingtalk", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, want := range []string{`"source":"dingtalk"`, `"managerCount"`, `"users"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("sync response should contain %q: %s", want, rec.Body.String())
		}
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "org.dingtalk.sync" {
		t.Fatalf("audit action = %q, want org.dingtalk.sync", last.Action)
	}
}

func TestOrgConfigMutationsRequirePlatformAdmin(t *testing.T) {
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

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "dingtalk_config", method: http.MethodPut, path: "/api/v1/org/dingtalk/config", body: `{"corpId":"dingcorp","appKey":"dingkey","appSecret":"secret-value","agentId":"123456","rootDeptId":"1","status":"enabled"}`},
		{name: "gitlab_mappings", method: http.MethodPut, path: "/api/v1/org/gitlab-mappings", body: `[]`},
		{name: "dingtalk_sync", method: http.MethodPost, path: "/api/v1/org/sync/dingtalk", body: ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			server.Mux.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))

			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
		})
	}
}
