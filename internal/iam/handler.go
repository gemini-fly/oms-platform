package iam

import (
	"net/http"
	"strings"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/iam/auth/config", authConfig(s))
	s.Mux.HandleFunc("/api/v1/iam/login", login(s))
	s.Mux.HandleFunc("/api/v1/iam/logout", logout(s))
	s.Mux.HandleFunc("/api/v1/iam/profile", profile(s))
	s.Mux.HandleFunc("/api/v1/iam/users", users(s))
	s.Mux.HandleFunc("/api/v1/iam/departments", departments(s))
	s.Mux.HandleFunc("/api/v1/iam/roles", roles(s))
	s.Mux.HandleFunc("/api/v1/iam/menu-permissions", menuPermissions(s))
	s.Mux.HandleFunc("/api/v1/iam/policy-bindings", policyBindings(s))
	s.Mux.HandleFunc("/api/v1/iam/policy-bindings/", policyBindingByID(s))
}

func profile(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		if len(s.Store.Users) == 0 {
			platform.Error(w, http.StatusNotFound, "USER_NOT_FOUND", "profile not found")
			return
		}
		actorID := platform.ActorID(r, s.Store)
		actor := currentUser(s.Store, actorID)
		platform.JSON(w, http.StatusOK, map[string]any{
			"user":            actor,
			"policyBindings":  policyBindingsForUser(s.Store.PolicyBindings, actorID),
			"departments":     s.Store.Departments,
			"availableRoles":  s.Store.Roles,
			"menuPermissions": s.Store.MenuPermissions,
		})
	}
}

func users(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		platform.JSON(w, http.StatusOK, platform.Page[platform.User]{Items: s.Store.Users, Page: 1, PageSize: len(s.Store.Users), Total: int64(len(s.Store.Users))})
	}
}

func departments(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		platform.JSON(w, http.StatusOK, s.Store.Departments)
	}
}

func roles(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		platform.JSON(w, http.StatusOK, s.Store.Roles)
	}
}

func menuPermissions(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, s.Store.MenuPermissions)
			return
		}
		actorID := platform.ActorID(r, s.Store)
		if !s.Store.HasAnyRole(actorID, "platform_admin") {
			platform.Error(w, http.StatusForbidden, "IAM_FORBIDDEN", "platform admin role required")
			return
		}
		var req []platform.MenuPermission
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		normalized := normalizeMenuPermissions(req, s.Store.Roles)
		if err := s.Store.PersistMenuPermissions(normalized); err != nil {
			platform.Error(w, http.StatusInternalServerError, "PERSIST_FAILED", err.Error())
			return
		}
		s.Store.Audit(actorID, "iam.menu_permission.update", "menu_permission", 0, "success", "update role menu access")
		platform.JSON(w, http.StatusOK, normalized)
	}
}

func policyBindings(s *platform.Server) http.HandlerFunc {
	type request struct {
		UserID    int64  `json:"userId"`
		RoleCode  string `json:"roleCode"`
		ScopeType string `json:"scopeType"`
		ScopeID   int64  `json:"scopeId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodGet {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !s.Store.HasAnyRole(actorID, "platform_admin") {
			platform.Error(w, http.StatusForbidden, "IAM_FORBIDDEN", "platform admin role required")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, s.Store.PolicyBindings)
			return
		}
		var req request
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		role := findRole(s.Store.Roles, req.RoleCode)
		if role.ID == 0 {
			platform.Error(w, http.StatusBadRequest, "ROLE_NOT_FOUND", "role not found")
			return
		}
		if !userExists(s.Store.Users, req.UserID) {
			platform.Error(w, http.StatusBadRequest, "USER_NOT_FOUND", "user not found")
			return
		}
		binding := platform.PolicyBinding{ID: s.Store.Next("binding"), UserID: req.UserID, RoleID: role.ID, RoleCode: role.Code, ScopeType: req.ScopeType, ScopeID: req.ScopeID}
		s.Store.PolicyBindings = append(s.Store.PolicyBindings, binding)
		s.Store.Audit(actorID, "iam.policy_binding.create", "policy_binding", binding.ID, "success", "create application tree scoped binding")
		platform.JSON(w, http.StatusCreated, binding)
	}
}

func policyBindingByID(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodDelete) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/iam/policy-bindings/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !s.Store.HasAnyRole(actorID, "platform_admin") {
			platform.Error(w, http.StatusForbidden, "IAM_FORBIDDEN", "platform admin role required")
			return
		}
		for i, binding := range s.Store.PolicyBindings {
			if binding.ID == id {
				s.Store.PolicyBindings = append(s.Store.PolicyBindings[:i], s.Store.PolicyBindings[i+1:]...)
				s.Store.Audit(actorID, "iam.policy_binding.delete", "policy_binding", id, "success", "delete application tree scoped binding")
				platform.JSON(w, http.StatusOK, map[string]any{"deleted": id})
				return
			}
		}
		platform.Error(w, http.StatusNotFound, "BINDING_NOT_FOUND", "binding not found")
	}
}

func currentUser(store *platform.Store, userID int64) platform.User {
	for _, user := range store.Users {
		if user.ID == userID {
			return user
		}
	}
	if len(store.Users) == 0 {
		return platform.User{}
	}
	return store.Users[0]
}

func userExists(items []platform.User, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func findRole(roles []platform.Role, code string) platform.Role {
	for _, role := range roles {
		if role.Code == code {
			return role
		}
	}
	return platform.Role{}
}

func policyBindingsForUser(items []platform.PolicyBinding, userID int64) []platform.PolicyBinding {
	filtered := make([]platform.PolicyBinding, 0)
	for _, item := range items {
		if item.UserID == userID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func normalizeMenuPermissions(req []platform.MenuPermission, roles []platform.Role) []platform.MenuPermission {
	defaults := platform.DefaultMenuPermissions()
	defaultByKey := make(map[string]platform.MenuPermission, len(defaults))
	order := make([]string, 0, len(defaults)+len(req))
	for _, item := range defaults {
		defaultByKey[item.MenuKey] = item
		order = append(order, item.MenuKey)
	}
	roleCodes := make(map[string]bool, len(roles))
	for _, role := range roles {
		if role.Status == "enabled" {
			roleCodes[role.Code] = true
		}
	}
	byMenu := make(map[string]platform.MenuPermission, len(req))
	for _, item := range req {
		menuKey := strings.TrimSpace(item.MenuKey)
		if menuKey == "" {
			continue
		}
		menuName := strings.TrimSpace(item.MenuName)
		if menuName == "" {
			if def, ok := defaultByKey[menuKey]; ok {
				menuName = def.MenuName
			} else {
				menuName = menuKey
			}
		}
		seen := map[string]bool{}
		normalizedRoles := []string{}
		for _, code := range item.RoleCodes {
			code = strings.TrimSpace(code)
			if !roleCodes[code] || seen[code] {
				continue
			}
			seen[code] = true
			normalizedRoles = append(normalizedRoles, code)
		}
		if !seen["platform_admin"] {
			normalizedRoles = append([]string{"platform_admin"}, normalizedRoles...)
		}
		if _, knownDefault := defaultByKey[menuKey]; !knownDefault {
			if _, alreadyAdded := byMenu[menuKey]; !alreadyAdded {
				order = append(order, menuKey)
			}
		}
		byMenu[menuKey] = platform.MenuPermission{MenuKey: menuKey, MenuName: menuName, RoleCodes: normalizedRoles}
	}
	items := make([]platform.MenuPermission, 0, len(order))
	for _, key := range order {
		if item, ok := byMenu[key]; ok {
			items = append(items, item)
			continue
		}
		if item, ok := defaultByKey[key]; ok {
			items = append(items, item)
		}
	}
	return items
}
