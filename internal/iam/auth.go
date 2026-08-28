package iam

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func authConfig(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		settings := platform.RedactPlatformSettings(s.Store.Settings)
		s.Store.Unlock()
		platform.JSON(w, http.StatusOK, map[string]any{
			"ldapEnabled":  settings.LDAPAuth.Enabled,
			"platformName": settings.PlatformName,
			"logoUrl":      settings.LogoURL,
			"faviconUrl":   settings.FaviconURL,
			"themeColor":   settings.ThemeColor,
		})
	}
}

func login(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var req loginRequest
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "请输入 LDAP 用户名和密码")
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || req.Password == "" || len([]rune(req.Username)) > 256 || len(req.Password) > 1024 {
			platform.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
			return
		}

		s.Store.Lock()
		settings := s.Store.Settings.LDAPAuth
		s.Store.Unlock()
		if !settings.Enabled {
			platform.Error(w, http.StatusConflict, "LDAP_DISABLED", "LDAP 登录认证未启用")
			return
		}
		authenticator := s.LDAPAuthenticator
		if authenticator == nil {
			authenticator = platform.AuthenticateLDAP
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		identity, err := authenticator(ctx, settings, req.Username, req.Password)
		req.Password = ""
		if err != nil {
			s.Store.Lock()
			s.Store.AuditSystem("iam.ldap.login", "user", 0, "failed", "LDAP login failed for "+req.Username)
			s.Store.Unlock()
			platform.Error(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误")
			return
		}

		s.Store.Lock()
		user := upsertLDAPUser(s.Store, settings, identity)
		s.Store.Audit(user.ID, "iam.ldap.login", "user", user.ID, "success", "LDAP login succeeded")
		s.Store.Unlock()
		now := time.Now()
		token, expiresAt, err := s.CreateSession(user.ID, now)
		if err != nil {
			platform.Error(w, http.StatusInternalServerError, "SESSION_CREATE_FAILED", "无法创建登录会话")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     platform.SessionCookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
			Expires:  expiresAt,
			MaxAge:   int(time.Until(expiresAt).Seconds()),
		})
		platform.JSON(w, http.StatusOK, map[string]any{"user": user, "expiresAt": expiresAt})
	}
}

func logout(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		if cookie, err := r.Cookie(platform.SessionCookieName); err == nil {
			s.DeleteSession(cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:     platform.SessionCookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteStrictMode,
			Expires:  time.Unix(1, 0),
			MaxAge:   -1,
		})
		platform.JSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	}
}

func upsertLDAPUser(store *platform.Store, settings platform.LDAPAuthSettings, identity platform.LDAPIdentity) platform.User {
	username := strings.TrimSpace(identity.Username)
	for i := range store.Users {
		matchedUsername := strings.EqualFold(store.Users[i].Username, username)
		matchedEmail := identity.Email != "" && strings.EqualFold(store.Users[i].Email, identity.Email)
		if !matchedUsername && !matchedEmail {
			continue
		}
		store.Users[i].AuthSource = "ldap"
		store.Users[i].Username = username
		store.Users[i].DisplayName = fallbackIdentity(identity.DisplayName, username)
		store.Users[i].Email = identity.Email
		store.Users[i].Status = "enabled"
		ensureLDAPRole(store, store.Users[i].ID, settings, username)
		return store.Users[i]
	}
	user := platform.User{
		ID:          store.Next("user"),
		ExternalID:  "ldap:" + username,
		AuthSource:  "ldap",
		Username:    username,
		DisplayName: fallbackIdentity(identity.DisplayName, username),
		Email:       identity.Email,
		Status:      "enabled",
	}
	store.Users = append(store.Users, user)
	ensureLDAPRole(store, user.ID, settings, username)
	return user
}

func ensureLDAPRole(store *platform.Store, userID int64, settings platform.LDAPAuthSettings, username string) {
	roleCode := settings.DefaultRoleCode
	if adminUsername(settings) != "" && strings.EqualFold(adminUsername(settings), username) {
		roleCode = "platform_admin"
	}
	if roleCode == "" {
		roleCode = "developer"
	}
	for _, binding := range store.PolicyBindings {
		if binding.UserID == userID && binding.RoleCode == roleCode && binding.ScopeType == "global" {
			return
		}
	}
	for _, role := range store.Roles {
		if role.Code == roleCode && role.Status == "enabled" {
			store.PolicyBindings = append(store.PolicyBindings, platform.PolicyBinding{
				ID: store.Next("binding"), UserID: userID, RoleID: role.ID, RoleCode: role.Code, ScopeType: "global",
			})
			return
		}
	}
}

func adminUsername(settings platform.LDAPAuthSettings) string {
	if value := strings.TrimSpace(settings.AdminUsername); value != "" {
		return value
	}
	prefix := strings.ToLower(strings.TrimSpace(settings.UserAttribute)) + "="
	firstRDN := strings.TrimSpace(strings.SplitN(settings.BindDN, ",", 2)[0])
	if strings.HasPrefix(strings.ToLower(firstRDN), prefix) {
		return strings.TrimSpace(firstRDN[len(prefix):])
	}
	return ""
}

func fallbackIdentity(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}
