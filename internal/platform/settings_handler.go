package platform

import (
	"context"
	"net/http"
	"strings"
	"time"
)

func RegisterSettings(s *Server) {
	s.Mux.HandleFunc("/api/v1/platform/settings", platformSettings(s))
	s.Mux.HandleFunc("/api/v1/platform/settings/ldap/test", testLDAPSettings(s))
	s.Mux.HandleFunc("/api/v1/platform/database/status", databaseStatus(s))
}

func testLDAPSettings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		actorID := ActorID(r, s.Store)
		allowed := s.Store.HasAnyRole(actorID, "platform_admin")
		savedPassword := s.Store.Settings.LDAPAuth.BindPassword
		s.Store.Unlock()
		if !allowed {
			Error(w, http.StatusForbidden, "SETTINGS_FORBIDDEN", "platform admin role required")
			return
		}

		var settings LDAPAuthSettings
		if err := Decode(r, &settings); err != nil {
			Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if strings.TrimSpace(settings.BindPassword) == "" {
			settings.BindPassword = savedPassword
		}
		settings = normalizeLDAPAuthSettings(settings, DefaultPlatformSettings().LDAPAuth)
		if err := validateLDAPAuthSettings(settings); err != nil {
			Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if !settings.Enabled {
			JSON(w, http.StatusOK, map[string]string{"status": "disabled", "message": "LDAP 登录认证未启用"})
			return
		}

		tester := s.LDAPSettingsTester
		if tester == nil {
			tester = TestLDAPSettingsConnection
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		if err := tester(ctx, settings); err != nil {
			s.Store.Lock()
			s.Store.Audit(actorID, "platform.settings.ldap.test", "platform_settings", 1, "failed", err.Error())
			s.Store.Unlock()
			Error(w, http.StatusUnprocessableEntity, "LDAP_TEST_FAILED", err.Error())
			return
		}
		s.Store.Lock()
		s.Store.Audit(actorID, "platform.settings.ldap.test", "platform_settings", 1, "success", "LDAP connection validated")
		s.Store.Unlock()
		JSON(w, http.StatusOK, map[string]string{"status": "connected", "message": "LDAP 连接、Bind、Base DN 和用户过滤器验证通过"})
	}
}

func databaseStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		JSON(w, http.StatusOK, s.Store.DatabaseStatus(r.Context()))
	}
}

func platformSettings(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}

		s.Store.Lock()
		defer s.Store.Unlock()

		if s.Store.Settings.PlatformName == "" {
			s.Store.Settings = DefaultPlatformSettings()
		}
		if r.Method == http.MethodGet {
			JSON(w, http.StatusOK, RedactPlatformSettings(s.Store.Settings))
			return
		}
		actorID := ActorID(r, s.Store)
		if !s.Store.HasAnyRole(actorID, "platform_admin") {
			Error(w, http.StatusForbidden, "SETTINGS_FORBIDDEN", "platform admin role required")
			return
		}

		var req PlatformSettings
		if err := Decode(r, &req); err != nil {
			Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if strings.TrimSpace(req.LDAPAuth.BindPassword) == "" {
			req.LDAPAuth.BindPassword = s.Store.Settings.LDAPAuth.BindPassword
		}
		settings, err := NormalizePlatformSettings(req)
		if err != nil {
			Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		settings.UpdatedAt = time.Now()
		if err := s.Store.PersistPlatformSettings(settings); err != nil {
			Error(w, http.StatusInternalServerError, "SETTINGS_SAVE_FAILED", err.Error())
			return
		}
		s.Store.Audit(actorID, "platform.settings.update", "platform_settings", 1, "success", "update platform brand settings")
		JSON(w, http.StatusOK, RedactPlatformSettings(settings))
	}
}

func RedactPlatformSettings(settings PlatformSettings) PlatformSettings {
	settings.LDAPAuth.BindPassword = ""
	return settings
}

func NormalizePlatformSettings(settings PlatformSettings) (PlatformSettings, error) {
	defaults := DefaultPlatformSettings()
	settings.PlatformName = strings.TrimSpace(settings.PlatformName)
	settings.LogoText = strings.TrimSpace(settings.LogoText)
	settings.LogoURL = strings.TrimSpace(settings.LogoURL)
	settings.FaviconURL = strings.TrimSpace(settings.FaviconURL)
	settings.ThemeColor = strings.TrimSpace(settings.ThemeColor)
	settings.LDAPAuth = normalizeLDAPAuthSettings(settings.LDAPAuth, defaults.LDAPAuth)

	if settings.PlatformName == "" {
		return PlatformSettings{}, errInvalidSettings("platformName is required")
	}
	if len([]rune(settings.PlatformName)) > 80 {
		return PlatformSettings{}, errInvalidSettings("platformName is too long")
	}
	if len([]rune(settings.LogoText)) > 12 {
		return PlatformSettings{}, errInvalidSettings("logoText is too long")
	}
	if settings.LogoText == "" {
		settings.LogoText = defaults.LogoText
	}
	if settings.ThemeColor == "" {
		settings.ThemeColor = defaults.ThemeColor
	}
	if !isHexColor(settings.ThemeColor) {
		return PlatformSettings{}, errInvalidSettings("themeColor must be a hex color")
	}
	if !isAllowedAssetURL(settings.LogoURL) {
		return PlatformSettings{}, errInvalidSettings("logoUrl must be empty, http(s), relative path, or image data URL")
	}
	if !isAllowedAssetURL(settings.FaviconURL) {
		return PlatformSettings{}, errInvalidSettings("faviconUrl must be empty, http(s), relative path, or image data URL")
	}
	if err := validateLDAPAuthSettings(settings.LDAPAuth); err != nil {
		return PlatformSettings{}, err
	}
	return settings, nil
}

func normalizeLDAPAuthSettings(settings, defaults LDAPAuthSettings) LDAPAuthSettings {
	settings.URL = strings.TrimSpace(settings.URL)
	settings.BaseDN = strings.TrimSpace(settings.BaseDN)
	settings.BindDN = strings.TrimSpace(settings.BindDN)
	settings.BindPassword = strings.TrimSpace(settings.BindPassword)
	settings.UserFilter = strings.TrimSpace(settings.UserFilter)
	settings.UserAttribute = strings.TrimSpace(settings.UserAttribute)
	settings.DisplayNameAttribute = strings.TrimSpace(settings.DisplayNameAttribute)
	settings.EmailAttribute = strings.TrimSpace(settings.EmailAttribute)
	settings.DefaultRoleCode = strings.TrimSpace(settings.DefaultRoleCode)
	settings.AdminUsername = strings.TrimSpace(settings.AdminUsername)
	if settings.UserFilter == "" {
		settings.UserFilter = defaults.UserFilter
	}
	if settings.UserAttribute == "" {
		settings.UserAttribute = defaults.UserAttribute
	}
	if settings.DisplayNameAttribute == "" {
		settings.DisplayNameAttribute = defaults.DisplayNameAttribute
	}
	if settings.EmailAttribute == "" {
		settings.EmailAttribute = defaults.EmailAttribute
	}
	if settings.DefaultRoleCode == "" {
		settings.DefaultRoleCode = defaults.DefaultRoleCode
	}
	return settings
}

func validateLDAPAuthSettings(settings LDAPAuthSettings) error {
	if settings.URL != "" && !isLDAPURL(settings.URL) {
		return errInvalidSettings("ldapAuth.url must start with ldap:// or ldaps://")
	}
	for name, value := range map[string]string{
		"ldapAuth.url":                  settings.URL,
		"ldapAuth.baseDn":               settings.BaseDN,
		"ldapAuth.bindDn":               settings.BindDN,
		"ldapAuth.userFilter":           settings.UserFilter,
		"ldapAuth.userAttribute":        settings.UserAttribute,
		"ldapAuth.displayNameAttribute": settings.DisplayNameAttribute,
		"ldapAuth.emailAttribute":       settings.EmailAttribute,
		"ldapAuth.defaultRoleCode":      settings.DefaultRoleCode,
		"ldapAuth.adminUsername":        settings.AdminUsername,
	} {
		if len([]rune(value)) > 500 {
			return errInvalidSettings(name + " is too long")
		}
	}
	if len([]rune(settings.BindPassword)) > 1000 {
		return errInvalidSettings("ldapAuth.bindPassword is too long")
	}
	if !strings.Contains(settings.UserFilter, "%s") {
		return errInvalidSettings("ldapAuth.userFilter must contain %s")
	}
	if settings.Enabled {
		if settings.URL == "" {
			return errInvalidSettings("ldapAuth.url is required when LDAP auth is enabled")
		}
		if settings.BaseDN == "" {
			return errInvalidSettings("ldapAuth.baseDn is required when LDAP auth is enabled")
		}
		if settings.BindDN == "" {
			return errInvalidSettings("ldapAuth.bindDn is required when LDAP auth is enabled")
		}
		if settings.BindPassword == "" {
			return errInvalidSettings("ldapAuth.bindPassword is required when LDAP auth is enabled")
		}
		if settings.StartTLS && strings.HasPrefix(settings.URL, "ldaps://") {
			return errInvalidSettings("ldapAuth.startTls cannot be used with ldaps://")
		}
	}
	return nil
}

type errInvalidSettings string

func (e errInvalidSettings) Error() string {
	return string(e)
}

func isHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, ch := range value[1:] {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

func isAllowedAssetURL(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 500000 {
		return false
	}
	return strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "/") ||
		strings.HasPrefix(value, "data:image/")
}

func isLDAPURL(value string) bool {
	return strings.HasPrefix(value, "ldap://") || strings.HasPrefix(value, "ldaps://")
}
