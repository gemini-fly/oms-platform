package apptree

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/app-tree/tree", tree(s))
	s.Mux.HandleFunc("/api/v1/app-tree/business-groups", createBusinessGroup(s))
	s.Mux.HandleFunc("/api/v1/app-tree/business-centers", createBusinessCenter(s))
	s.Mux.HandleFunc("/api/v1/app-tree/business-lines", createBusinessLine(s))
	s.Mux.HandleFunc("/api/v1/app-tree/systems", createSystem(s))
	s.Mux.HandleFunc("/api/v1/app-tree/repository/inspect", inspectRepository(s))
	s.Mux.HandleFunc("/api/v1/app-tree/applications", createApplication(s))
	s.Mux.HandleFunc("/api/v1/app-tree/services", createService(s))
	s.Mux.HandleFunc("/api/v1/app-tree/services/", serviceAction(s))
	s.Mux.HandleFunc("/api/v1/app-tree/service-members/", serviceMemberAction(s))
	s.Mux.HandleFunc("/api/v1/app-tree/environments", createEnvironment(s))
	s.Mux.HandleFunc("/api/v1/app-tree/environments/", environmentAction(s))
	s.Mux.HandleFunc("/api/v1/app-tree/pod-sessions/", podSessionAction(s))
	s.Mux.HandleFunc("/api/v1/app-tree/applications/", applicationOverview(s))
}

func tree(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		visible := visibleServiceIDs(s.Store, platform.ActorID(r, s.Store))
		platform.JSON(w, http.StatusOK, map[string]any{
			"businessGroups":  s.Store.BusinessGroups,
			"businessCenters": s.Store.BusinessCenters,
			"businessLines":   s.Store.BusinessLines,
			"systems":         s.Store.Systems,
			"applications":    filterVisibleApplications(s.Store.Applications, s.Store.Services, visible),
			"services":        filterVisibleServices(s.Store.Services, visible),
			"serviceMembers":  filterVisibleServiceMembers(s.Store.ServiceMembers, visible),
			"environments":    filterVisibleEnvironments(s.Store.Environments, visible),
		})
	}
}

func createBusinessGroup(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var item platform.BusinessGroup
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canManageTreeStructure(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "APP_TREE_STRUCTURE_FORBIDDEN", "dev owner, ops, or platform admin role required")
			return
		}
		item.ID = s.Store.Next("business_group")
		item.OwnerUserID = actorID
		item.Status = fallback(item.Status, "enabled")
		s.Store.BusinessGroups = append(s.Store.BusinessGroups, item)
		s.Store.Audit(actorID, "app_tree.business_group.create", "business_group", item.ID, "success", "create business group")
		platform.JSON(w, http.StatusCreated, item)
	}
}

func createBusinessCenter(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var item platform.BusinessCenter
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canManageTreeStructure(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "APP_TREE_STRUCTURE_FORBIDDEN", "dev owner, ops, or platform admin role required")
			return
		}
		if !businessGroupExists(s.Store.BusinessGroups, item.BusinessGroupID) {
			platform.Error(w, http.StatusBadRequest, "BUSINESS_GROUP_NOT_FOUND", "business group not found")
			return
		}
		item.ID = s.Store.Next("business_center")
		item.OwnerUserID = actorID
		item.Status = fallback(item.Status, "enabled")
		s.Store.BusinessCenters = append(s.Store.BusinessCenters, item)
		s.Store.Audit(actorID, "app_tree.business_center.create", "business_center", item.ID, "success", "create business center")
		platform.JSON(w, http.StatusCreated, item)
	}
}

func createBusinessLine(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var item platform.BusinessLine
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canManageTreeStructure(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "APP_TREE_STRUCTURE_FORBIDDEN", "dev owner, ops, or platform admin role required")
			return
		}
		if !businessCenterExists(s.Store.BusinessCenters, item.BusinessCenterID) {
			platform.Error(w, http.StatusBadRequest, "BUSINESS_CENTER_NOT_FOUND", "business center not found")
			return
		}
		item.ID = s.Store.Next("business_line")
		item.OwnerUserID = actorID
		item.Status = fallback(item.Status, "enabled")
		s.Store.BusinessLines = append(s.Store.BusinessLines, item)
		s.Store.Audit(actorID, "app_tree.business_line.create", "business_line", item.ID, "success", "create business line")
		platform.JSON(w, http.StatusCreated, item)
	}
}

func createSystem(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var item platform.System
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canManageTreeStructure(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "APP_TREE_STRUCTURE_FORBIDDEN", "dev owner, ops, or platform admin role required")
			return
		}
		if !businessLineExists(s.Store.BusinessLines, item.BusinessLineID) {
			platform.Error(w, http.StatusBadRequest, "BUSINESS_LINE_NOT_FOUND", "business line not found")
			return
		}
		item.ID = s.Store.Next("system")
		item.OwnerUserID = actorID
		item.Status = fallback(item.Status, "enabled")
		s.Store.Systems = append(s.Store.Systems, item)
		s.Store.Audit(actorID, "app_tree.system.create", "system", item.ID, "success", "create system")
		platform.JSON(w, http.StatusCreated, item)
	}
}

func createApplication(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var item platform.Application
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		item.AppID = platform.NormalizeAppID(item.AppID)
		item.Name = strings.TrimSpace(item.Name)
		if item.RepositoryProvider == "" {
			item.RepositoryProvider = platform.RepositoryProviderFromURL(item.RepositoryURL)
		} else {
			item.RepositoryProvider = platform.NormalizeRepositoryProvider(item.RepositoryProvider)
		}
		item.RepositoryFullName = platform.NormalizeRepositoryFullName(item.RepositoryFullName)
		item.RepositoryURL = strings.TrimSpace(item.RepositoryURL)
		if item.RepositoryFullName == "" {
			item.RepositoryFullName = platform.RepositoryFullNameFromURL(item.RepositoryURL)
		}
		if item.AppID == "" {
			item.AppID = platform.NormalizeAppID(platform.RepositoryProjectName(item.RepositoryFullName, item.RepositoryURL))
		}
		if err := platform.ValidateAppID(item.AppID); err != nil {
			platform.Error(w, http.StatusBadRequest, "APPID_INVALID", err.Error())
			return
		}
		if item.Name == "" {
			item.Name = item.AppID
		}
		if err := platform.ValidateRepositoryForAppID(item.AppID, item.RepositoryFullName, item.RepositoryURL); err != nil {
			platform.Error(w, http.StatusBadRequest, "APPID_REPOSITORY_MISMATCH", err.Error())
			return
		}
		if item.SystemID == 0 {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "systemId is required")
			return
		}
		if !systemExists(s.Store.Systems, item.SystemID) {
			platform.Error(w, http.StatusBadRequest, "SYSTEM_NOT_FOUND", "system not found")
			return
		}
		if applicationAppIDExists(s.Store.Applications, item.AppID) {
			platform.Error(w, http.StatusConflict, "APPID_EXISTS", "appId already exists")
			return
		}
		if !canApplyApplication(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "APPLICATION_FORBIDDEN", "current user cannot apply application")
			return
		}
		item.ID = s.Store.Next("application")
		item.OwnerUserID = actorID
		item.LifecycleStatus = fallback(item.LifecycleStatus, "applying")
		s.Store.Applications = append(s.Store.Applications, item)
		s.Store.Audit(actorID, "app_tree.application.create", "application", item.ID, "success", "apply application")
		platform.JSON(w, http.StatusCreated, item)
	}
}

func createService(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var item platform.Service
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "service name is required")
			return
		}
		if item.ApplicationID == 0 {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "applicationId is required")
			return
		}
		app, ok := findApplication(s.Store.Applications, item.ApplicationID)
		if !ok {
			platform.Error(w, http.StatusBadRequest, "APPLICATION_NOT_FOUND", "application not found")
			return
		}
		if !canManageApplication(s.Store, actorID, app) {
			platform.Error(w, http.StatusForbidden, "SERVICE_FORBIDDEN", "current user cannot create service for this application")
			return
		}
		if item.RepositoryProvider == "" {
			item.RepositoryProvider = platform.RepositoryProviderFromURL(item.RepositoryURL)
		} else {
			item.RepositoryProvider = platform.NormalizeRepositoryProvider(item.RepositoryProvider)
		}
		item.RepositoryFullName = platform.NormalizeRepositoryFullName(item.RepositoryFullName)
		item.RepositoryURL = strings.TrimSpace(item.RepositoryURL)
		if item.RepositoryFullName == "" {
			item.RepositoryFullName = platform.RepositoryFullNameFromURL(item.RepositoryURL)
		}
		if item.RepositoryFullName == "" && item.RepositoryURL == "" {
			item.RepositoryProvider = app.RepositoryProvider
			item.RepositoryFullName = app.RepositoryFullName
			item.RepositoryURL = app.RepositoryURL
		}
		if app.AppID != "" && (item.RepositoryFullName != "" || item.RepositoryURL != "") {
			if err := platform.ValidateRepositoryForAppID(app.AppID, item.RepositoryFullName, item.RepositoryURL); err != nil {
				platform.Error(w, http.StatusBadRequest, "SERVICE_REPOSITORY_MISMATCH", err.Error())
				return
			}
		}
		item.ServiceType = platform.NormalizeServiceType(item.ServiceType)
		item = platform.NormalizeRuntimeProfile(item)
		if item.PipelineTemplateID != 0 {
			if _, ok := s.Store.PipelineTemplateByID(item.PipelineTemplateID); !ok {
				platform.Error(w, http.StatusBadRequest, "PIPELINE_TEMPLATE_NOT_FOUND", "pipeline template not found")
				return
			}
		} else {
			item.PipelineTemplateID = s.Store.RecommendedPipelineTemplateID(item)
		}
		item.ID = s.Store.Next("service")
		item.OwnerUserID = actorID
		item.Status = fallback(item.Status, "enabled")
		s.Store.Services = append(s.Store.Services, item)
		s.Store.ServiceMembers = append(s.Store.ServiceMembers, platform.ServiceMember{
			ID:        s.Store.Next("service_member"),
			ServiceID: item.ID,
			UserID:    item.OwnerUserID,
			Role:      "owner",
			Status:    "enabled",
			CreatedAt: time.Now(),
		})
		pipeline := s.Store.DefaultPipeline(item)
		s.Store.Pipelines = append(s.Store.Pipelines, pipeline)
		s.Store.Audit(item.OwnerUserID, "app_tree.service.create", "service", item.ID, "success", "create service and default pipeline")
		platform.JSON(w, http.StatusCreated, map[string]any{"service": item, "pipeline": pipeline})
	}
}

func serviceAction(s *platform.Server) http.HandlerFunc {
	type request struct {
		UserID int64  `json:"userId"`
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/members") {
			platform.Error(w, http.StatusNotFound, "NOT_FOUND", "route not found")
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/app-tree/services/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		service, ok := findService(s.Store.Services, id)
		if !ok {
			platform.Error(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found")
			return
		}
		if !canSeeService(s.Store, actorID, service) {
			platform.Error(w, http.StatusForbidden, "SERVICE_FORBIDDEN", "current user cannot see this service")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, filterMembersByService(s.Store.ServiceMembers, id))
			return
		}
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		if !canManageServiceMembers(s.Store, actorID, service) {
			platform.Error(w, http.StatusForbidden, "SERVICE_MEMBER_FORBIDDEN", "current user cannot configure service members")
			return
		}
		var req request
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if req.UserID == 0 || !userExists(s.Store.Users, req.UserID) {
			platform.Error(w, http.StatusBadRequest, "USER_NOT_FOUND", "user not found")
			return
		}
		role := normalizeServiceMemberRole(req.Role)
		status := normalizeServiceMemberStatus(req.Status)
		for i := range s.Store.ServiceMembers {
			if s.Store.ServiceMembers[i].ServiceID == id && s.Store.ServiceMembers[i].UserID == req.UserID {
				s.Store.ServiceMembers[i].Role = role
				s.Store.ServiceMembers[i].Status = status
				s.Store.Audit(actorID, "app_tree.service_member.update", "service_member", s.Store.ServiceMembers[i].ID, "success", fmt.Sprintf("service:%d user:%d role:%s", id, req.UserID, role))
				platform.JSON(w, http.StatusOK, s.Store.ServiceMembers[i])
				return
			}
		}
		member := platform.ServiceMember{
			ID:        s.Store.Next("service_member"),
			ServiceID: id,
			UserID:    req.UserID,
			Role:      role,
			Status:    status,
			CreatedAt: time.Now(),
		}
		s.Store.ServiceMembers = append(s.Store.ServiceMembers, member)
		s.Store.Audit(actorID, "app_tree.service_member.create", "service_member", member.ID, "success", fmt.Sprintf("service:%d user:%d role:%s", id, req.UserID, role))
		platform.JSON(w, http.StatusCreated, member)
	}
}

func serviceMemberAction(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodDelete) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/app-tree/service-members/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		for i, member := range s.Store.ServiceMembers {
			if member.ID != id {
				continue
			}
			service, ok := findService(s.Store.Services, member.ServiceID)
			if !ok {
				platform.Error(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found")
				return
			}
			if !canManageServiceMembers(s.Store, actorID, service) {
				platform.Error(w, http.StatusForbidden, "SERVICE_MEMBER_FORBIDDEN", "current user cannot configure service members")
				return
			}
			s.Store.ServiceMembers = append(s.Store.ServiceMembers[:i], s.Store.ServiceMembers[i+1:]...)
			s.Store.Audit(actorID, "app_tree.service_member.delete", "service_member", id, "success", fmt.Sprintf("service:%d user:%d", member.ServiceID, member.UserID))
			platform.JSON(w, http.StatusOK, map[string]any{"deleted": id})
			return
		}
		platform.Error(w, http.StatusNotFound, "SERVICE_MEMBER_NOT_FOUND", "service member not found")
	}
}

func createEnvironment(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var item platform.Environment
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		service, ok := findService(s.Store.Services, item.ServiceID)
		if !ok {
			platform.Error(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found")
			return
		}
		if !canManageServiceMembers(s.Store, actorID, service) {
			platform.Error(w, http.StatusForbidden, "ENVIRONMENT_FORBIDDEN", "current user cannot create environment for this service")
			return
		}
		if isProductionEnvironment(item) && !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner") {
			platform.Error(w, http.StatusForbidden, "PRODUCTION_ENVIRONMENT_FORBIDDEN", "only ops or platform admin can create production environment")
			return
		}
		if item.K8sNamespaceID != 0 {
			namespace, ok := findK8sNamespaceOK(s.Store.K8sNamespaces, item.K8sNamespaceID)
			if !ok {
				platform.Error(w, http.StatusBadRequest, "K8S_NAMESPACE_NOT_FOUND", "k8s namespace not found")
				return
			}
			if namespace.ScopeType == "service" && namespace.ScopeID != item.ServiceID {
				platform.Error(w, http.StatusForbidden, "K8S_NAMESPACE_FORBIDDEN", "namespace does not belong to this service")
				return
			}
		}
		item.ID = s.Store.Next("environment")
		item.Status = fallback(item.Status, "enabled")
		s.Store.Environments = append(s.Store.Environments, item)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func environmentAction(s *platform.Server) http.HandlerFunc {
	type request struct {
		PodName string `json:"podName"`
		Account string `json:"account"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/pod-sessions") {
			platform.Error(w, http.StatusNotFound, "NOT_FOUND", "route not found")
			return
		}
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/app-tree/environments/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		var req request
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}

		s.Store.Lock()
		defer s.Store.Unlock()
		env, ok := findEnvironment(s.Store.Environments, id)
		if !ok {
			platform.Error(w, http.StatusNotFound, "ENVIRONMENT_NOT_FOUND", "environment not found")
			return
		}
		service, ok := findService(s.Store.Services, env.ServiceID)
		if !ok {
			platform.Error(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found")
			return
		}
		actorID := platform.ActorID(r, s.Store)
		roleCodes := s.Store.RoleCodesForUser(actorID)
		if !canSeeService(s.Store, actorID, service) {
			s.Store.Audit(actorID, "app_tree.pod.login", "environment", env.ID, "denied", "service member access required")
			platform.Error(w, http.StatusForbidden, "SERVICE_FORBIDDEN", "current user cannot access this service")
			return
		}
		cluster := findK8sCluster(s.Store.K8sClusters, env.K8sClusterID)
		namespace := findK8sNamespace(s.Store.K8sNamespaces, env.K8sNamespaceID)
		if !canLoginPod(roleCodes, env) {
			s.Store.Audit(actorID, "app_tree.pod.login", "environment", env.ID, "denied", "production pod login requires ops role")
			platform.Error(w, http.StatusForbidden, "POD_LOGIN_FORBIDDEN", "研发角色只能登录非生产环境 Pod，生产环境只允许运维或平台管理员登录")
			return
		}
		req.PodName = defaultPodName(service, env)
		req.Account = defaultPodAccount(roleCodes)
		now := time.Now()
		session := platform.PodSession{
			ID:              s.Store.Next("pod_session"),
			ActorUserID:     actorID,
			EnvironmentID:   env.ID,
			EnvironmentName: env.Name,
			ServiceID:       service.ID,
			ServiceName:     service.Name,
			ClusterName:     cluster.Name,
			Namespace:       namespace.Name,
			PodName:         req.PodName,
			Account:         req.Account,
			Status:          "connected",
			StartedAt:       now,
			LastActiveAt:    now,
			Lines: []platform.TerminalLine{
				terminalLine("system", fmt.Sprintf("connected to pod %s in %s/%s", req.PodName, cluster.Name, namespace.Name)),
				terminalLine("output", fmt.Sprintf("kubectl -n %s exec -it %s -- /bin/sh", namespace.Name, req.PodName)),
			},
		}
		s.Store.PodSessions = append(s.Store.PodSessions, session)
		s.Store.Audit(actorID, "app_tree.pod.login", "pod_session", session.ID, "success", fmt.Sprintf("%s/%s %s$ login", namespace.Name, req.PodName, req.Account))
		platform.JSON(w, http.StatusCreated, session)
	}
}

func podSessionAction(s *platform.Server) http.HandlerFunc {
	type request struct {
		Command string `json:"command"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/commands") {
			platform.Error(w, http.StatusNotFound, "NOT_FOUND", "route not found")
			return
		}
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/app-tree/pod-sessions/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		var req request
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		command := strings.TrimSpace(req.Command)
		if command == "" {
			platform.Error(w, http.StatusBadRequest, "EMPTY_COMMAND", "command is required")
			return
		}
		if len([]rune(command)) > 200 {
			platform.Error(w, http.StatusBadRequest, "COMMAND_TOO_LONG", "command is too long")
			return
		}

		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		session := findPodSession(s.Store.PodSessions, id)
		if session == nil {
			platform.Error(w, http.StatusNotFound, "SESSION_NOT_FOUND", "pod session not found")
			return
		}
		if session.Status == "closed" {
			platform.Error(w, http.StatusConflict, "SESSION_CLOSED", "pod session is closed")
			return
		}
		env, ok := findEnvironment(s.Store.Environments, session.EnvironmentID)
		if !ok {
			platform.Error(w, http.StatusNotFound, "ENVIRONMENT_NOT_FOUND", "environment not found")
			return
		}
		service, ok := findService(s.Store.Services, session.ServiceID)
		if !ok {
			platform.Error(w, http.StatusNotFound, "SERVICE_NOT_FOUND", "service not found")
			return
		}
		if !canSeeService(s.Store, actorID, service) {
			s.Store.Audit(actorID, "app_tree.pod.command", "pod_session", session.ID, "denied", "service member access required")
			platform.Error(w, http.StatusForbidden, "SERVICE_FORBIDDEN", "current user cannot access this service")
			return
		}
		if !canControlPodSession(s.Store, actorID, *session) {
			s.Store.Audit(actorID, "app_tree.pod.command", "pod_session", session.ID, "denied", "pod session owner or ops role required")
			platform.Error(w, http.StatusForbidden, "POD_SESSION_FORBIDDEN", "current user cannot control this pod session")
			return
		}
		if !canLoginPod(s.Store.RoleCodesForUser(actorID), env) {
			s.Store.Audit(actorID, "app_tree.pod.command", "pod_session", session.ID, "denied", "production pod command requires ops role")
			platform.Error(w, http.StatusForbidden, "POD_COMMAND_FORBIDDEN", "当前角色无权在该环境 Pod 执行命令")
			return
		}
		now := time.Now()
		session.Lines = append(session.Lines, platform.TerminalLine{Kind: "input", Content: command, CreatedAt: now})
		output := simulatePodCommand(*session, command)
		session.Lines = append(session.Lines, output...)
		session.LastActiveAt = now
		if command == "exit" || command == "logout" {
			session.Status = "closed"
		}
		reason := fmt.Sprintf("%s/%s %s$ %s", session.Namespace, session.PodName, session.Account, command)
		s.Store.Audit(actorID, "app_tree.pod.command", "pod_session", session.ID, "success", reason)
		platform.JSON(w, http.StatusOK, map[string]any{"session": session, "output": output})
	}
}

func applicationOverview(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/move") {
			moveApplication(s, w, r)
			return
		}
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/app-tree/applications/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		visible := visibleServiceIDs(s.Store, platform.ActorID(r, s.Store))
		services := filterServices(filterVisibleServices(s.Store.Services, visible), id)
		serviceIDs := serviceIDSet(services)
		platform.JSON(w, http.StatusOK, map[string]any{
			"applicationId": id,
			"services":      services,
			"serviceMembers": filterVisibleServiceMembers(
				s.Store.ServiceMembers,
				serviceIDs,
			),
			"environments": filterVisibleEnvironments(s.Store.Environments, serviceIDs),
			"assets":       filterVisibleAssets(s.Store.Assets, serviceIDs),
			"pipelines":    filterVisiblePipelines(s.Store.Pipelines, serviceIDs),
			"releases":     filterVisibleReleases(s.Store.Releases, serviceIDs),
			"tickets":      filterVisibleTickets(s.Store.Tickets, id, serviceIDs),
		})
	}
}

func moveApplication(s *platform.Server, w http.ResponseWriter, r *http.Request) {
	type request struct {
		TargetType string `json:"targetType"`
		TargetID   int64  `json:"targetId"`
	}
	if !platform.Method(w, r, http.MethodPut) {
		return
	}
	id, err := platform.PathID(r.URL.Path, "/api/v1/app-tree/applications/")
	if err != nil {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	var req request
	if err := platform.Decode(r, &req); err != nil {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	req.TargetType = strings.TrimSpace(req.TargetType)
	if req.TargetID == 0 {
		platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "targetId is required")
		return
	}

	s.Store.Lock()
	defer s.Store.Unlock()
	actorID := platform.ActorID(r, s.Store)
	appIndex := -1
	for i := range s.Store.Applications {
		if s.Store.Applications[i].ID == id {
			appIndex = i
			break
		}
	}
	if appIndex < 0 {
		platform.Error(w, http.StatusNotFound, "APPLICATION_NOT_FOUND", "application not found")
		return
	}
	if !canManageApplication(s.Store, actorID, s.Store.Applications[appIndex]) {
		platform.Error(w, http.StatusForbidden, "APPLICATION_FORBIDDEN", "current user cannot move this application")
		return
	}
	oldSystemID := s.Store.Applications[appIndex].SystemID
	targetSystem, err := resolveTargetSystem(s.Store, req.TargetType, req.TargetID, s.Store.Applications[appIndex].OwnerUserID)
	if err != nil {
		platform.Error(w, http.StatusBadRequest, "TARGET_NOT_FOUND", err.Error())
		return
	}
	s.Store.Applications[appIndex].SystemID = targetSystem.ID
	s.Store.Audit(actorID, "app_tree.application.move", "application", id, "success", fmt.Sprintf("system:%d->system:%d via %s:%d", oldSystemID, targetSystem.ID, req.TargetType, req.TargetID))
	platform.JSON(w, http.StatusOK, map[string]any{
		"application":  s.Store.Applications[appIndex],
		"targetSystem": targetSystem,
	})
}

func resolveTargetSystem(store *platform.Store, targetType string, targetID int64, ownerID int64) (platform.System, error) {
	switch targetType {
	case "system":
		for _, item := range store.Systems {
			if item.ID == targetID {
				return item, nil
			}
		}
		return platform.System{}, errors.New("system not found")
	case "business_line":
		for _, item := range store.BusinessLines {
			if item.ID == targetID {
				return ensureSystem(store, item.ID, ownerID), nil
			}
		}
		return platform.System{}, errors.New("business line not found")
	case "business_center":
		for _, item := range store.BusinessCenters {
			if item.ID == targetID {
				line := ensureBusinessLine(store, item.ID, ownerID)
				return ensureSystem(store, line.ID, ownerID), nil
			}
		}
		return platform.System{}, errors.New("business center not found")
	case "business_group":
		for _, item := range store.BusinessGroups {
			if item.ID == targetID {
				center := ensureBusinessCenter(store, item.ID, ownerID)
				line := ensureBusinessLine(store, center.ID, ownerID)
				return ensureSystem(store, line.ID, ownerID), nil
			}
		}
		return platform.System{}, errors.New("business group not found")
	default:
		return platform.System{}, errors.New("targetType must be business_group, business_center, business_line, or system")
	}
}

func findEnvironment(items []platform.Environment, id int64) (platform.Environment, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return platform.Environment{}, false
}

func findService(items []platform.Service, id int64) (platform.Service, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return platform.Service{}, false
}

func findApplication(items []platform.Application, id int64) (platform.Application, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return platform.Application{}, false
}

func applicationAppIDExists(items []platform.Application, appID string) bool {
	for _, item := range items {
		if item.AppID == appID {
			return true
		}
	}
	return false
}

func findK8sCluster(items []platform.K8sCluster, id int64) platform.K8sCluster {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return platform.K8sCluster{Name: "-"}
}

func findK8sNamespace(items []platform.K8sNamespace, id int64) platform.K8sNamespace {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return platform.K8sNamespace{Name: "-"}
}

func findK8sNamespaceOK(items []platform.K8sNamespace, id int64) (platform.K8sNamespace, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return platform.K8sNamespace{}, false
}

func findPodSession(items []platform.PodSession, id int64) *platform.PodSession {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func canLoginPod(roleCodes []string, env platform.Environment) bool {
	if hasAnyRole(roleCodes, "platform_admin", "ops_owner") {
		return true
	}
	return !isProductionEnvironment(env) && hasAnyRole(roleCodes, "dev_owner", "developer")
}

func isProductionEnvironment(env platform.Environment) bool {
	name := strings.ToLower(strings.TrimSpace(env.Name))
	level := strings.ToLower(strings.TrimSpace(env.ReleaseLevel))
	return name == "prod" || name == "production" || level == "prod" || level == "production"
}

func hasAnyRole(roleCodes []string, candidates ...string) bool {
	allowed := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		allowed[candidate] = true
	}
	for _, code := range roleCodes {
		if allowed[code] {
			return true
		}
	}
	return false
}

func defaultPodName(service platform.Service, env platform.Environment) string {
	serviceName := strings.TrimSpace(service.Name)
	if serviceName == "" {
		serviceName = fmt.Sprintf("service-%d", service.ID)
	}
	envName := strings.TrimSpace(env.Name)
	if envName == "" {
		envName = "default"
	}
	return sanitizeK8sName(serviceName + "-" + envName + "-7f9c8d")
}

func sanitizeK8sName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	previousDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			previousDash = false
			continue
		}
		if !previousDash {
			b.WriteByte('-')
			previousDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "pod-local"
	}
	return out
}

func defaultPodAccount(roleCodes []string) string {
	if hasAnyRole(roleCodes, "platform_admin", "ops_owner") {
		return "ops"
	}
	return "developer"
}

func terminalLine(kind, content string) platform.TerminalLine {
	return platform.TerminalLine{Kind: kind, Content: content, CreatedAt: time.Now()}
}

func simulatePodCommand(session platform.PodSession, command string) []platform.TerminalLine {
	switch command {
	case "help":
		return []platform.TerminalLine{terminalLine("output", "可用命令：hostname, whoami, pwd, ls, env, ps, cat /etc/hostname, exit")}
	case "hostname":
		return []platform.TerminalLine{terminalLine("output", session.PodName)}
	case "whoami":
		return []platform.TerminalLine{terminalLine("output", session.Account)}
	case "pwd":
		return []platform.TerminalLine{terminalLine("output", "/app")}
	case "ls":
		return []platform.TerminalLine{terminalLine("output", "app  config  logs  tmp")}
	case "env":
		return []platform.TerminalLine{terminalLine("output", fmt.Sprintf("APP_NAME=%s\nAPP_ENV=%s\nK8S_NAMESPACE=%s", session.ServiceName, session.EnvironmentName, session.Namespace))}
	case "ps":
		return []platform.TerminalLine{terminalLine("output", "PID   USER       COMMAND\n1     app        /app/server\n18    app        /bin/sh")}
	case "cat /etc/hostname":
		return []platform.TerminalLine{terminalLine("output", session.PodName)}
	case "exit", "logout":
		return []platform.TerminalLine{terminalLine("system", "session closed")}
	default:
		if strings.HasPrefix(command, "kubectl ") {
			return []platform.TerminalLine{terminalLine("output", "当前已在 Pod 内部终端；kubectl 命令应在平台侧编排执行。")}
		}
		return []platform.TerminalLine{terminalLine("output", "命令已记录。当前为本地演示 Pod 终端；接入 K8s Exec Gateway 后会返回真实容器输出。")}
	}
}

func canManageTreeStructure(store *platform.Store, actorID int64) bool {
	return store.HasAnyRole(actorID, "platform_admin", "ops_owner", "dev_owner")
}

func canApplyApplication(store *platform.Store, actorID int64) bool {
	return store.HasAnyRole(actorID, "platform_admin", "ops_owner", "dev_owner", "developer")
}

func canManageApplication(store *platform.Store, actorID int64, app platform.Application) bool {
	if store.HasAnyRole(actorID, "platform_admin", "ops_owner", "dev_owner") {
		return true
	}
	if app.OwnerUserID == actorID {
		return true
	}
	for _, service := range store.Services {
		if service.ApplicationID == app.ID && store.CanManageService(actorID, service.ID) {
			return true
		}
	}
	return false
}

func businessGroupExists(items []platform.BusinessGroup, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func businessCenterExists(items []platform.BusinessCenter, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func businessLineExists(items []platform.BusinessLine, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func systemExists(items []platform.System, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func visibleServiceIDs(store *platform.Store, actorID int64) map[int64]bool {
	return store.VisibleServiceIDsForUser(actorID)
}

func canSeeService(store *platform.Store, actorID int64, service platform.Service) bool {
	return visibleServiceIDs(store, actorID)[service.ID]
}

func canManageServiceMembers(store *platform.Store, actorID int64, service platform.Service) bool {
	return store.CanManageService(actorID, service.ID)
}

func canControlPodSession(store *platform.Store, actorID int64, session platform.PodSession) bool {
	return store.HasAnyRole(actorID, "platform_admin", "ops_owner") || session.ActorUserID == actorID
}

func filterVisibleServices(items []platform.Service, visible map[int64]bool) []platform.Service {
	out := make([]platform.Service, 0)
	for _, item := range items {
		if visible[item.ID] {
			out = append(out, item)
		}
	}
	return out
}

func filterVisibleApplications(applications []platform.Application, services []platform.Service, visible map[int64]bool) []platform.Application {
	appIDs := map[int64]bool{}
	for _, service := range services {
		if visible[service.ID] {
			appIDs[service.ApplicationID] = true
		}
	}
	out := make([]platform.Application, 0)
	for _, item := range applications {
		if appIDs[item.ID] {
			out = append(out, item)
		}
	}
	return out
}

func filterVisibleEnvironments(items []platform.Environment, visible map[int64]bool) []platform.Environment {
	out := make([]platform.Environment, 0)
	for _, item := range items {
		if visible[item.ServiceID] {
			out = append(out, item)
		}
	}
	return out
}

func filterVisibleServiceMembers(items []platform.ServiceMember, visible map[int64]bool) []platform.ServiceMember {
	out := make([]platform.ServiceMember, 0)
	for _, item := range items {
		if visible[item.ServiceID] {
			out = append(out, item)
		}
	}
	return out
}

func filterVisiblePipelines(items []platform.Pipeline, visible map[int64]bool) []platform.Pipeline {
	out := make([]platform.Pipeline, 0)
	for _, item := range items {
		if visible[item.ServiceID] {
			out = append(out, item)
		}
	}
	return out
}

func filterVisibleReleases(items []platform.Release, visible map[int64]bool) []platform.Release {
	out := make([]platform.Release, 0)
	for _, item := range items {
		if visible[item.ServiceID] {
			out = append(out, item)
		}
	}
	return out
}

func filterVisibleAssets(items []platform.Asset, visible map[int64]bool) []platform.Asset {
	out := make([]platform.Asset, 0)
	for _, item := range items {
		if item.ScopeType == "service" && visible[item.ScopeID] {
			out = append(out, item)
		}
	}
	return out
}

func filterVisibleTickets(items []platform.Ticket, applicationID int64, visible map[int64]bool) []platform.Ticket {
	out := make([]platform.Ticket, 0)
	for _, item := range items {
		switch item.ScopeType {
		case "service":
			if visible[item.ScopeID] {
				out = append(out, item)
			}
		case "application":
			if item.ScopeID == applicationID && len(visible) > 0 {
				out = append(out, item)
			}
		}
	}
	return out
}

func serviceIDSet(services []platform.Service) map[int64]bool {
	out := map[int64]bool{}
	for _, service := range services {
		out[service.ID] = true
	}
	return out
}

func filterMembersByService(items []platform.ServiceMember, serviceID int64) []platform.ServiceMember {
	out := make([]platform.ServiceMember, 0)
	for _, item := range items {
		if item.ServiceID == serviceID {
			out = append(out, item)
		}
	}
	return out
}

func userExists(items []platform.User, id int64) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func normalizeServiceMemberRole(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "owner", "maintainer", "developer", "viewer":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "developer"
	}
}

func normalizeServiceMemberStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "disabled":
		return "disabled"
	default:
		return "enabled"
	}
}

func ensureBusinessCenter(store *platform.Store, groupID int64, ownerID int64) platform.BusinessCenter {
	for _, item := range store.BusinessCenters {
		if item.BusinessGroupID == groupID {
			return item
		}
	}
	item := platform.BusinessCenter{ID: store.Next("business_center"), BusinessGroupID: groupID, Name: "默认业务中心", OwnerUserID: ownerID, Status: "enabled"}
	store.BusinessCenters = append(store.BusinessCenters, item)
	return item
}

func ensureBusinessLine(store *platform.Store, centerID int64, ownerID int64) platform.BusinessLine {
	for _, item := range store.BusinessLines {
		if item.BusinessCenterID == centerID {
			return item
		}
	}
	item := platform.BusinessLine{ID: store.Next("business_line"), BusinessCenterID: centerID, Name: "默认业务线", OwnerUserID: ownerID, Status: "enabled"}
	store.BusinessLines = append(store.BusinessLines, item)
	return item
}

func ensureSystem(store *platform.Store, lineID int64, ownerID int64) platform.System {
	for _, item := range store.Systems {
		if item.BusinessLineID == lineID {
			return item
		}
	}
	item := platform.System{ID: store.Next("system"), BusinessLineID: lineID, Name: "默认系统", OwnerUserID: ownerID, Status: "enabled"}
	store.Systems = append(store.Systems, item)
	return item
}

func filterServices(items []platform.Service, appID int64) []platform.Service {
	out := make([]platform.Service, 0)
	for _, item := range items {
		if item.ApplicationID == appID {
			out = append(out, item)
		}
	}
	return out
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func fallbackID(value, defaultValue int64) int64 {
	if value == 0 {
		return defaultValue
	}
	return value
}
