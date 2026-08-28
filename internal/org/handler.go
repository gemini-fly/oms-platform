package org

import (
	"net/http"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

type DepartmentView struct {
	ID                 int64  `json:"id"`
	ExternalID         string `json:"externalId"`
	ParentID           int64  `json:"parentId"`
	Name               string `json:"name"`
	ManagerUserID      int64  `json:"managerUserId"`
	ManagerUsername    string `json:"managerUsername"`
	ManagerDisplayName string `json:"managerDisplayName"`
	Source             string `json:"source"`
	Status             string `json:"status"`
}

type DingTalkConfigView struct {
	CorpID       string    `json:"corpId"`
	AppKey       string    `json:"appKey"`
	AgentID      string    `json:"agentId"`
	RootDeptID   string    `json:"rootDeptId"`
	SyncMode     string    `json:"syncMode"`
	Status       string    `json:"status"`
	Configured   bool      `json:"configured"`
	AppSecretSet bool      `json:"appSecretSet"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type GitLabGroupMappingView struct {
	ID                 int64     `json:"id"`
	DepartmentID       int64     `json:"departmentId"`
	DepartmentName     string    `json:"departmentName"`
	ManagerUserID      int64     `json:"managerUserId"`
	ManagerDisplayName string    `json:"managerDisplayName"`
	GitLabGroupPath    string    `json:"gitlabGroupPath"`
	AccessLevel        string    `json:"accessLevel"`
	SyncMode           string    `json:"syncMode"`
	Status             string    `json:"status"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/org/dingtalk/config", dingtalkConfig(s))
	s.Mux.HandleFunc("/api/v1/org/gitlab-mappings", gitLabMappings(s))
	s.Mux.HandleFunc("/api/v1/org/departments", departments(s))
	s.Mux.HandleFunc("/api/v1/org/users", users(s))
	s.Mux.HandleFunc("/api/v1/org/sync/dingtalk", syncDingTalk(s))
}

func dingtalkConfig(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, toConfigView(s.Store.DingTalkConfig))
			return
		}
		if !s.Store.HasAnyRole(actorID, "platform_admin") {
			platform.Error(w, http.StatusForbidden, "ORG_CONFIG_FORBIDDEN", "platform admin role required")
			return
		}
		var req platform.DingTalkOrgConfig
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if strings.TrimSpace(req.AppSecret) == "" {
			req.AppSecret = s.Store.DingTalkConfig.AppSecret
		}
		config := platform.NormalizeDingTalkConfig(req)
		config.UpdatedAt = time.Now()
		if config.Status == "enabled" && (config.CorpID == "" || config.AppKey == "" || config.AppSecret == "") {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "enabled DingTalk config requires corpId, appKey, and appSecret")
			return
		}
		if err := s.Store.PersistDingTalkConfig(config); err != nil {
			platform.Error(w, http.StatusInternalServerError, "PERSIST_FAILED", err.Error())
			return
		}
		s.Store.Audit(actorID, "org.dingtalk.config.update", "org_integration", 0, "success", "update DingTalk org integration config")
		platform.JSON(w, http.StatusOK, toConfigView(config))
	}
}

func gitLabMappings(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, gitLabMappingViews(s.Store.GitLabMappings, s.Store.Departments, s.Store.Users))
			return
		}
		if !s.Store.HasAnyRole(actorID, "platform_admin") {
			platform.Error(w, http.StatusForbidden, "ORG_CONFIG_FORBIDDEN", "platform admin role required")
			return
		}
		var req []platform.GitLabGroupMapping
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		items, err := normalizeGitLabMappings(req, s.Store)
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if err := s.Store.PersistGitLabMappings(items); err != nil {
			platform.Error(w, http.StatusInternalServerError, "PERSIST_FAILED", err.Error())
			return
		}
		s.Store.Audit(actorID, "org.gitlab_mapping.update", "gitlab_group_mapping", 0, "success", "update department to GitLab group mappings")
		platform.JSON(w, http.StatusOK, gitLabMappingViews(items, s.Store.Departments, s.Store.Users))
	}
}

func departments(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		platform.JSON(w, http.StatusOK, departmentViews(s.Store.Departments, s.Store.Users))
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

func syncDingTalk(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !s.Store.HasAnyRole(actorID, "platform_admin") {
			platform.Error(w, http.StatusForbidden, "ORG_CONFIG_FORBIDDEN", "platform admin role required")
			return
		}
		ensureDingTalkOrg(s.Store)
		departments := departmentViews(s.Store.Departments, s.Store.Users)
		managerCount := 0
		for _, item := range departments {
			if item.Source == "dingtalk" && item.ManagerUserID != 0 {
				managerCount++
			}
		}
		reason := "sync departments and supervisors from DingTalk"
		if !isConfigured(s.Store.DingTalkConfig) {
			reason = "demo sync; DingTalk config is not completed"
		}
		s.Store.Audit(actorID, "org.dingtalk.sync", "department", 0, "success", reason)
		platform.JSON(w, http.StatusOK, map[string]any{
			"source":       "dingtalk",
			"syncedAt":     time.Now(),
			"config":       toConfigView(s.Store.DingTalkConfig),
			"departments":  departments,
			"users":        s.Store.Users,
			"managerCount": managerCount,
		})
	}
}

func toConfigView(config platform.DingTalkOrgConfig) DingTalkConfigView {
	return DingTalkConfigView{
		CorpID:       config.CorpID,
		AppKey:       config.AppKey,
		AgentID:      config.AgentID,
		RootDeptID:   config.RootDeptID,
		SyncMode:     config.SyncMode,
		Status:       config.Status,
		Configured:   isConfigured(config),
		AppSecretSet: config.AppSecret != "",
		UpdatedAt:    config.UpdatedAt,
	}
}

func isConfigured(config platform.DingTalkOrgConfig) bool {
	return config.Status == "enabled" && config.CorpID != "" && config.AppKey != "" && config.AppSecret != ""
}

func normalizeGitLabMappings(req []platform.GitLabGroupMapping, store *platform.Store) ([]platform.GitLabGroupMapping, error) {
	departments := map[int64]platform.Department{}
	for _, dept := range store.Departments {
		departments[dept.ID] = dept
	}
	seenDepartments := map[int64]bool{}
	items := make([]platform.GitLabGroupMapping, 0, len(req))
	for _, item := range req {
		departmentID := item.DepartmentID
		if departmentID == 0 {
			continue
		}
		if _, ok := departments[departmentID]; !ok {
			return nil, errInvalidOrgMapping("department not found")
		}
		if seenDepartments[departmentID] {
			continue
		}
		path := strings.Trim(strings.TrimSpace(item.GitLabGroupPath), "/")
		if path == "" {
			continue
		}
		if len([]rune(path)) > 200 || strings.ContainsAny(path, " \t\r\n") {
			return nil, errInvalidOrgMapping("gitlabGroupPath must be a slash separated path without spaces")
		}
		accessLevel := strings.ToLower(strings.TrimSpace(item.AccessLevel))
		if accessLevel != "owner" && accessLevel != "developer" {
			accessLevel = "maintainer"
		}
		syncMode := strings.ToLower(strings.TrimSpace(item.SyncMode))
		if syncMode != "manual" {
			syncMode = "dingtalk_manager_owner"
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status != "disabled" {
			status = "enabled"
		}
		if item.ID == 0 {
			item.ID = store.Next("gitlab_mapping")
		}
		updatedAt := item.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = time.Now()
		}
		items = append(items, platform.GitLabGroupMapping{
			ID:              item.ID,
			DepartmentID:    departmentID,
			GitLabGroupPath: path,
			AccessLevel:     accessLevel,
			SyncMode:        syncMode,
			Status:          status,
			UpdatedAt:       updatedAt,
		})
		seenDepartments[departmentID] = true
	}
	return items, nil
}

type errInvalidOrgMapping string

func (e errInvalidOrgMapping) Error() string {
	return string(e)
}

func gitLabMappingViews(items []platform.GitLabGroupMapping, departments []platform.Department, users []platform.User) []GitLabGroupMappingView {
	deptMap := map[int64]platform.Department{}
	for _, dept := range departments {
		deptMap[dept.ID] = dept
	}
	userMap := map[int64]platform.User{}
	for _, user := range users {
		userMap[user.ID] = user
	}
	views := make([]GitLabGroupMappingView, 0, len(items))
	for _, item := range items {
		dept := deptMap[item.DepartmentID]
		manager := userMap[dept.ManagerUserID]
		views = append(views, GitLabGroupMappingView{
			ID:                 item.ID,
			DepartmentID:       item.DepartmentID,
			DepartmentName:     dept.Name,
			ManagerUserID:      dept.ManagerUserID,
			ManagerDisplayName: manager.DisplayName,
			GitLabGroupPath:    item.GitLabGroupPath,
			AccessLevel:        item.AccessLevel,
			SyncMode:           item.SyncMode,
			Status:             item.Status,
			UpdatedAt:          item.UpdatedAt,
		})
	}
	return views
}

func departmentViews(departments []platform.Department, users []platform.User) []DepartmentView {
	userMap := map[int64]platform.User{}
	for _, user := range users {
		userMap[user.ID] = user
	}
	items := make([]DepartmentView, 0, len(departments))
	for _, dept := range departments {
		manager := userMap[dept.ManagerUserID]
		items = append(items, DepartmentView{
			ID:                 dept.ID,
			ExternalID:         dept.ExternalID,
			ParentID:           dept.ParentID,
			Name:               dept.Name,
			ManagerUserID:      dept.ManagerUserID,
			ManagerUsername:    manager.Username,
			ManagerDisplayName: manager.DisplayName,
			Source:             defaultString(dept.Source, "manual"),
			Status:             dept.Status,
		})
	}
	return items
}

func ensureDingTalkOrg(store *platform.Store) {
	root := ensureDepartment(store, "dt-dept-root", 0, "平台组织")
	devDept := ensureDepartment(store, "dt-dept-dev", root.ID, "研发中心")
	opsDept := ensureDepartment(store, "dt-dept-ops", root.ID, "运维中心")

	admin := ensureUser(store, "dt-user-admin", "admin", "Platform Admin", "admin@example.com", root.ID, 0)
	devLead := ensureUser(store, "dt-user-dev-lead", "dev.lead", "研发主管", "dev.lead@example.com", devDept.ID, admin.ID)
	opsLead := ensureUser(store, "dt-user-ops-lead", "ops.lead", "运维主管", "ops.lead@example.com", opsDept.ID, admin.ID)
	ensureUser(store, "dt-user-developer", "developer", "研发同学", "developer@example.com", devDept.ID, devLead.ID)

	setDepartmentManager(store, root.ID, admin.ID)
	setDepartmentManager(store, devDept.ID, devLead.ID)
	setDepartmentManager(store, opsDept.ID, opsLead.ID)
}

func ensureDepartment(store *platform.Store, externalID string, parentID int64, name string) platform.Department {
	for i := range store.Departments {
		if store.Departments[i].ExternalID == externalID {
			store.Departments[i].ParentID = parentID
			store.Departments[i].Name = name
			store.Departments[i].Source = "dingtalk"
			store.Departments[i].Status = defaultString(store.Departments[i].Status, "enabled")
			return store.Departments[i]
		}
	}
	item := platform.Department{
		ID:         store.Next("department"),
		ExternalID: externalID,
		ParentID:   parentID,
		Name:       name,
		Source:     "dingtalk",
		Status:     "enabled",
	}
	store.Departments = append(store.Departments, item)
	return item
}

func ensureUser(store *platform.Store, externalID, username, displayName, email string, departmentID, managerUserID int64) platform.User {
	for i := range store.Users {
		if store.Users[i].ExternalID == externalID {
			store.Users[i].Username = username
			store.Users[i].DisplayName = displayName
			store.Users[i].Email = email
			store.Users[i].DepartmentID = departmentID
			store.Users[i].ManagerUserID = managerUserID
			store.Users[i].Status = defaultString(store.Users[i].Status, "enabled")
			return store.Users[i]
		}
	}
	item := platform.User{
		ID:            store.Next("user"),
		ExternalID:    externalID,
		Username:      username,
		DisplayName:   displayName,
		Email:         email,
		DepartmentID:  departmentID,
		ManagerUserID: managerUserID,
		Status:        "enabled",
	}
	store.Users = append(store.Users, item)
	return item
}

func setDepartmentManager(store *platform.Store, departmentID, managerUserID int64) {
	for i := range store.Departments {
		if store.Departments[i].ID == departmentID {
			store.Departments[i].ManagerUserID = managerUserID
			store.Departments[i].Source = "dingtalk"
			store.Departments[i].Status = defaultString(store.Departments[i].Status, "enabled")
			return
		}
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
