package iam

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func newLDAPAuthTestServer(t *testing.T) *platform.Server {
	t.Helper()
	store := platform.NewDemoStore()
	store.Settings.LDAPAuth = platform.LDAPAuthSettings{
		Enabled: true, URL: "ldaps://ldap.example.com:636", BaseDN: "ou=users,dc=example,dc=com",
		BindDN: "uid=svc,ou=services,dc=example,dc=com", BindPassword: "bind-secret",
		UserFilter: "(&(objectClass=person)(uid=%s))", UserAttribute: "uid",
		DisplayNameAttribute: "cn", EmailAttribute: "mail", DefaultRoleCode: "developer",
		AdminUsername: "ops-admin",
	}
	server := platform.NewServer(store)
	server.LDAPAuthenticator = func(_ context.Context, _ platform.LDAPAuthSettings, username, password string) (platform.LDAPIdentity, error) {
		if password != "correct-password" || (username != "ops-admin" && username != "developer") {
			return platform.LDAPIdentity{}, errTestInvalidCredentials{}
		}
		if username == "developer" {
			return platform.LDAPIdentity{
				DN: "uid=developer,ou=users,dc=example,dc=com", Username: "developer",
				DisplayName: "Developer", Email: "developer@example.com",
			}, nil
		}
		return platform.LDAPIdentity{
			DN: "uid=ops-admin,ou=users,dc=example,dc=com", Username: "ops-admin",
			DisplayName: "Ops Admin", Email: "ops-admin@example.com",
		}, nil
	}
	Register(server)
	return server
}

type errTestInvalidCredentials struct{}

func (errTestInvalidCredentials) Error() string { return "invalid credentials" }

func TestLDAPEnabledRequiresSessionForProfile(t *testing.T) {
	server := newLDAPAuthTestServer(t)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/iam/profile", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "AUTH_REQUIRED") {
		t.Fatalf("response should require authentication: %s", rec.Body.String())
	}
}

func TestLDAPLoginCreatesSessionAndUsesLDAPProfile(t *testing.T) {
	server := newLDAPAuthTestServer(t)
	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/iam/login", strings.NewReader(`{"username":"ops-admin","password":"correct-password"}`))
	server.Handler().ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d, body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}
	cookies := loginRec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != platform.SessionCookieName || !cookies[0].HttpOnly {
		t.Fatalf("login should return HttpOnly session cookie: %#v", cookies)
	}

	profileRec := httptest.NewRecorder()
	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/iam/profile", nil)
	profileReq.AddCookie(cookies[0])
	server.Handler().ServeHTTP(profileRec, profileReq)
	if profileRec.Code != http.StatusOK {
		t.Fatalf("profile status = %d, want %d, body=%s", profileRec.Code, http.StatusOK, profileRec.Body.String())
	}
	for _, want := range []string{`"username":"ops-admin"`, `"authSource":"ldap"`, `"roleCode":"platform_admin"`} {
		if !strings.Contains(profileRec.Body.String(), want) {
			t.Fatalf("profile should contain %s: %s", want, profileRec.Body.String())
		}
	}
}

func TestLDAPLoginRejectsInvalidCredentials(t *testing.T) {
	server := newLDAPAuthTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/iam/login", strings.NewReader(`{"username":"ops-admin","password":"wrong"}`))
	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "invalid credentials") {
		t.Fatalf("response should not expose LDAP error: %s", rec.Body.String())
	}
}

func TestLDAPDeveloperSessionDoesNotInheritBootstrapAdmin(t *testing.T) {
	server := newLDAPAuthTestServer(t)
	platform.RegisterSettings(server)
	loginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/iam/login", strings.NewReader(`{"username":"developer","password":"correct-password"}`)))
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body=%s", loginRec.Code, loginRec.Body.String())
	}
	cookie := loginRec.Result().Cookies()[0]

	profileRec := httptest.NewRecorder()
	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/iam/profile", nil)
	profileReq.AddCookie(cookie)
	server.Handler().ServeHTTP(profileRec, profileReq)
	if !strings.Contains(profileRec.Body.String(), `"username":"developer"`) || !strings.Contains(profileRec.Body.String(), `"roleCode":"developer"`) {
		t.Fatalf("developer profile is wrong: %s", profileRec.Body.String())
	}
	if strings.Contains(profileRec.Body.String(), `"roleCode":"platform_admin"`) {
		t.Fatalf("developer session inherited platform admin: %s", profileRec.Body.String())
	}

	settingsRec := httptest.NewRecorder()
	settingsReq := httptest.NewRequest(http.MethodPut, "/api/v1/platform/settings", strings.NewReader(`{}`))
	settingsReq.AddCookie(cookie)
	server.Handler().ServeHTTP(settingsRec, settingsReq)
	if settingsRec.Code != http.StatusForbidden {
		t.Fatalf("developer settings update status = %d, want %d, body=%s", settingsRec.Code, http.StatusForbidden, settingsRec.Body.String())
	}
}

func TestLDAPLogoutInvalidatesSession(t *testing.T) {
	server := newLDAPAuthTestServer(t)
	loginRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/iam/login", strings.NewReader(`{"username":"ops-admin","password":"correct-password"}`)))
	cookie := loginRec.Result().Cookies()[0]

	logoutRec := httptest.NewRecorder()
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/iam/logout", nil)
	logoutReq.AddCookie(cookie)
	server.Handler().ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout status = %d, want %d", logoutRec.Code, http.StatusOK)
	}

	profileRec := httptest.NewRecorder()
	profileReq := httptest.NewRequest(http.MethodGet, "/api/v1/iam/profile", nil)
	profileReq.AddCookie(cookie)
	server.Handler().ServeHTTP(profileRec, profileReq)
	if profileRec.Code != http.StatusUnauthorized {
		t.Fatalf("profile after logout status = %d, want %d", profileRec.Code, http.StatusUnauthorized)
	}
}

func TestAuthConfigIsPublicAndRedacted(t *testing.T) {
	server := newLDAPAuthTestServer(t)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/iam/auth/config", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"ldapEnabled":true`) {
		t.Fatalf("public auth config status/body = %d %s", rec.Code, rec.Body.String())
	}
	for _, secret := range []string{"bind-secret", "bindDn", "baseDn", "adminUsername"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("public auth config should not expose %q: %s", secret, rec.Body.String())
		}
	}
}

func TestLDAPDisabledKeepsBootstrapProfile(t *testing.T) {
	store := platform.NewDemoStore()
	store.Settings.LDAPAuth.Enabled = false
	server := platform.NewServer(store)
	Register(server)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/iam/profile", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
}
