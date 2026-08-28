package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlatformSettingsAPI(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := NewServer(NewDemoStore())
	RegisterSettings(server)

	getRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/v1/platform/settings", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", getRec.Code, http.StatusOK)
	}
	if !strings.Contains(getRec.Body.String(), "OMS 运维平台") {
		t.Fatalf("GET body should contain default platform name: %s", getRec.Body.String())
	}

	body := bytes.NewBufferString(`{
		"platformName":"Acme 运维平台",
		"logoText":"AC",
		"logoUrl":"data:image/svg+xml;base64,PHN2Zy8+",
		"faviconUrl":"",
		"themeColor":"#0f766e",
		"ldapAuth":{
			"enabled":true,
			"url":"ldaps://ldap.example.com:636",
			"baseDn":"dc=example,dc=com",
			"bindDn":"cn=readonly,dc=example,dc=com",
			"bindPassword":"secret",
			"userFilter":"(&(objectClass=person)(uid=%s))",
			"userAttribute":"uid",
			"displayNameAttribute":"cn",
			"emailAttribute":"mail",
			"defaultRoleCode":"developer"
		}
	}`)
	putRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(putRec, httptest.NewRequest(http.MethodPut, "/api/v1/platform/settings", body))
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d, body=%s", putRec.Code, http.StatusOK, putRec.Body.String())
	}
	if strings.Contains(putRec.Body.String(), "secret") || strings.Contains(putRec.Body.String(), "bindPassword") {
		t.Fatalf("PUT response should redact LDAP bind password: %s", putRec.Body.String())
	}
	if got := server.Store.Settings.PlatformName; got != "Acme 运维平台" {
		t.Fatalf("platform name = %q, want %q", got, "Acme 运维平台")
	}
	if got := server.Store.Settings.LogoText; got != "AC" {
		t.Fatalf("logo text = %q, want %q", got, "AC")
	}
	if got := server.Store.Settings.ThemeColor; got != "#0f766e" {
		t.Fatalf("theme color = %q, want %q", got, "#0f766e")
	}
	if got := server.Store.Settings.LDAPAuth.URL; got != "ldaps://ldap.example.com:636" {
		t.Fatalf("ldap url = %q, want ldaps://ldap.example.com:636", got)
	}
	if !server.Store.Settings.LDAPAuth.Enabled {
		t.Fatalf("ldap auth should be enabled")
	}
	if got := server.Store.Settings.LDAPAuth.BindPassword; got != "secret" {
		t.Fatalf("stored LDAP bind password = %q, want secret", got)
	}

	getSavedRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(getSavedRec, httptest.NewRequest(http.MethodGet, "/api/v1/platform/settings", nil))
	if strings.Contains(getSavedRec.Body.String(), "secret") || strings.Contains(getSavedRec.Body.String(), "bindPassword") {
		t.Fatalf("GET response should redact LDAP bind password: %s", getSavedRec.Body.String())
	}
}

func TestPlatformSettingsRejectsBlankName(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := NewServer(NewDemoStore())
	RegisterSettings(server)

	body := bytes.NewBufferString(`{"platformName":"","logoText":"AC","themeColor":"#0f766e"}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/platform/settings", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestPlatformSettingsPersistAcrossStores(t *testing.T) {
	settingsFile := filepath.Join(t.TempDir(), "settings.json")
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", settingsFile)

	server := NewServer(NewDemoStore())
	RegisterSettings(server)
	body := bytes.NewBufferString(`{"platformName":"Acme 运维平台","logoText":"AC","themeColor":"#0f766e"}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/platform/settings", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	reloaded := NewDemoStore()
	if got := reloaded.Settings.PlatformName; got != "Acme 运维平台" {
		t.Fatalf("reloaded platform name = %q, want %q", got, "Acme 运维平台")
	}
	if got := reloaded.Settings.LogoText; got != "AC" {
		t.Fatalf("reloaded logo text = %q, want %q", got, "AC")
	}
}

func TestPlatformSettingsRejectsInvalidLDAPAuth(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := NewServer(NewDemoStore())
	RegisterSettings(server)

	body := bytes.NewBufferString(`{
		"platformName":"Acme 运维平台",
		"logoText":"AC",
		"themeColor":"#0f766e",
		"ldapAuth":{
			"enabled":true,
			"url":"https://ldap.example.com",
			"baseDn":"dc=example,dc=com",
			"userFilter":"uid"
		}
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/platform/settings", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLDAPSettingsTestUsesSavedPassword(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := NewServer(NewDemoStore())
	server.Store.Settings.LDAPAuth.BindPassword = "saved-secret"
	var tested LDAPAuthSettings
	server.LDAPSettingsTester = func(_ context.Context, settings LDAPAuthSettings) error {
		tested = settings
		return nil
	}
	RegisterSettings(server)

	body := bytes.NewBufferString(`{
		"enabled":true,
		"url":"ldaps://ldap.example.com:636",
		"baseDn":"dc=example,dc=com",
		"bindDn":"cn=readonly,dc=example,dc=com",
		"bindPassword":"",
		"userFilter":"(&(objectClass=person)(uid=%s))",
		"userAttribute":"uid",
		"displayNameAttribute":"cn",
		"emailAttribute":"mail",
		"defaultRoleCode":"developer"
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/platform/settings/ldap/test", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if tested.BindPassword != "saved-secret" {
		t.Fatalf("tester should receive the saved bind password")
	}
	if strings.Contains(rec.Body.String(), "saved-secret") {
		t.Fatalf("response should not expose the bind password: %s", rec.Body.String())
	}
}

func TestLDAPSettingsTestReturnsConnectionFailure(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := NewServer(NewDemoStore())
	server.LDAPSettingsTester = func(_ context.Context, _ LDAPAuthSettings) error {
		return errors.New("Bind DN 或密码错误")
	}
	RegisterSettings(server)

	body := bytes.NewBufferString(`{
		"enabled":true,
		"url":"ldaps://ldap.example.com:636",
		"baseDn":"dc=example,dc=com",
		"bindDn":"cn=readonly,dc=example,dc=com",
		"bindPassword":"wrong-secret",
		"userFilter":"uid=%s",
		"userAttribute":"uid",
		"displayNameAttribute":"cn",
		"emailAttribute":"mail",
		"defaultRoleCode":"developer"
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/platform/settings/ldap/test", body))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnprocessableEntity, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Bind DN 或密码错误") {
		t.Fatalf("response should explain the LDAP validation failure: %s", rec.Body.String())
	}
}

func TestLDAPSettingsTestRejectsStartTLSWithLDAPS(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := NewServer(NewDemoStore())
	called := false
	server.LDAPSettingsTester = func(_ context.Context, _ LDAPAuthSettings) error {
		called = true
		return nil
	}
	RegisterSettings(server)

	body := bytes.NewBufferString(`{
		"enabled":true,
		"url":"ldaps://ldap.example.com:636",
		"startTls":true,
		"baseDn":"dc=example,dc=com",
		"bindDn":"cn=readonly,dc=example,dc=com",
		"bindPassword":"secret",
		"userFilter":"uid=%s"
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/platform/settings/ldap/test", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if called {
		t.Fatal("tester should not run for an invalid TLS configuration")
	}
}

func TestPlatformSettingsRequiresPlatformAdmin(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := NewDemoStore()
	store.CurrentUserID = store.Users[3].ID
	store.PolicyBindings = []PolicyBinding{{
		ID:        store.Next("binding"),
		UserID:    store.CurrentUserID,
		RoleID:    store.Roles[3].ID,
		RoleCode:  "developer",
		ScopeType: "global",
	}}
	server := NewServer(store)
	RegisterSettings(server)

	body := bytes.NewBufferString(`{"platformName":"Acme 运维平台","logoText":"AC","themeColor":"#0f766e"}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/platform/settings", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestPlatformSettingsPreservesLDAPPasswordWhenBlank(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := NewServer(NewDemoStore())
	RegisterSettings(server)
	server.Store.Settings = PlatformSettings{
		PlatformName: "Acme 运维平台",
		LogoText:     "AC",
		ThemeColor:   "#0f766e",
		LDAPAuth: LDAPAuthSettings{
			Enabled:              true,
			URL:                  "ldaps://ldap.example.com:636",
			BaseDN:               "dc=example,dc=com",
			BindDN:               "cn=readonly,dc=example,dc=com",
			BindPassword:         "saved-secret",
			UserFilter:           "uid=%s",
			UserAttribute:        "uid",
			DisplayNameAttribute: "cn",
			EmailAttribute:       "mail",
			DefaultRoleCode:      "developer",
		},
	}

	body := bytes.NewBufferString(`{
		"platformName":"Acme 运维平台",
		"logoText":"AC",
		"themeColor":"#0f766e",
		"ldapAuth":{
			"enabled":true,
			"url":"ldaps://ldap.example.com:636",
			"baseDn":"dc=example,dc=com",
			"bindDn":"cn=readonly,dc=example,dc=com",
			"bindPassword":"",
			"userFilter":"uid=%s",
			"userAttribute":"uid",
			"displayNameAttribute":"cn",
			"emailAttribute":"mail",
			"defaultRoleCode":"developer"
		}
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/platform/settings", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := server.Store.Settings.LDAPAuth.BindPassword; got != "saved-secret" {
		t.Fatalf("stored LDAP bind password = %q, want saved-secret", got)
	}
	if strings.Contains(rec.Body.String(), "saved-secret") || strings.Contains(rec.Body.String(), "bindPassword") {
		t.Fatalf("response should redact preserved LDAP bind password: %s", rec.Body.String())
	}
}

func TestDatabaseStatusAPIWithoutDatabase(t *testing.T) {
	t.Setenv("SY_PLATFORM_DB_DSN", "")
	t.Setenv("DATABASE_URL", "")
	server := NewServer(NewDemoStore())
	RegisterSettings(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/platform/database/status", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response struct {
		Data DatabaseStatus `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Enabled {
		t.Fatalf("database should be disabled without postgres connection")
	}
	if response.Data.Provider != "memory" {
		t.Fatalf("provider = %q, want memory", response.Data.Provider)
	}
	if response.Data.Status != "memory_only" {
		t.Fatalf("status = %q, want memory_only", response.Data.Status)
	}
	if response.Data.TableName != "platform_store_snapshots" {
		t.Fatalf("table = %q, want platform_store_snapshots", response.Data.TableName)
	}
}

func TestDatabaseStatusRedactsDSNPassword(t *testing.T) {
	t.Setenv("SY_PLATFORM_DB_DSN", "postgres://sy_platform:secret@127.0.0.1:15432/sy_platform?sslmode=disable")
	t.Setenv("DATABASE_URL", "")
	status := NewDemoStore().DatabaseStatus(context.Background())

	if strings.Contains(status.DSN, "secret") {
		t.Fatalf("dsn should redact password: %s", status.DSN)
	}
	if !strings.Contains(status.DSN, "******") {
		t.Fatalf("dsn should show password redaction marker: %s", status.DSN)
	}
	if !strings.Contains(status.DSN, "sy_platform") {
		t.Fatalf("dsn should keep username and database name: %s", status.DSN)
	}
}
