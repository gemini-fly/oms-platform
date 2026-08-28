package platform

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	sync.Mutex
	next               map[string]int64
	CurrentUserID      int64
	settingsFile       string
	orgConfigFile      string
	menuPermissionFile string
	gitLabMappingFile  string
	db                 *sql.DB
	dbSnapshotID       string

	Settings       PlatformSettings
	DingTalkConfig DingTalkOrgConfig

	Users           []User
	Departments     []Department
	Roles           []Role
	PolicyBindings  []PolicyBinding
	MenuPermissions []MenuPermission
	GitLabMappings  []GitLabGroupMapping

	BusinessGroups  []BusinessGroup
	BusinessCenters []BusinessCenter
	BusinessLines   []BusinessLine
	Systems         []System
	Applications    []Application
	Services        []Service
	ServiceMembers  []ServiceMember
	Environments    []Environment

	Tickets   []Ticket
	Approvals []Approval
	Knowledge []Knowledge

	CloudAccounts      []CloudAccount
	CloudResourceTypes []CloudResourceType
	Assets             []Asset
	K8sClusters        []K8sCluster
	K8sNamespaces      []K8sNamespace
	SyncJobs           []SyncJob
	ServerSessions     []ServerSession
	PodSessions        []PodSession
	FileSnapshots      []FileSnapshot

	PipelineTemplates []PipelineTemplate
	PipelineModules   []PipelineModule
	Pipelines         []Pipeline
	PipelineRuns      []PipelineRun
	PipelineLogs      []PipelineLog

	Releases     []Release
	HealthChecks []HealthCheck

	AuditLogs     []AuditLog
	Notifications []Notification
}

func NewStore() *Store {
	store := newStore()
	store.seedCore()
	return store
}

func NewDemoStore() *Store {
	store := newStore()
	store.seed()
	return store
}

func newStore() *Store {
	store := &Store{next: map[string]int64{}, settingsFile: SettingsFilePath(), orgConfigFile: OrgConfigFilePath(), menuPermissionFile: MenuPermissionFilePath(), gitLabMappingFile: GitLabMappingFilePath()}
	return store
}

func SettingsFilePath() string {
	if value := os.Getenv("SY_PLATFORM_SETTINGS_FILE"); value != "" {
		return value
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "sy-platform", "settings.json")
	}
	return filepath.Join(os.TempDir(), "sy-platform-settings.json")
}

func OrgConfigFilePath() string {
	if value := os.Getenv("SY_PLATFORM_ORG_CONFIG_FILE"); value != "" {
		return value
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "sy-platform", "dingtalk-org.json")
	}
	return filepath.Join(os.TempDir(), "sy-platform-dingtalk-org.json")
}

func MenuPermissionFilePath() string {
	if value := os.Getenv("SY_PLATFORM_MENU_PERMISSION_FILE"); value != "" {
		return value
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "sy-platform", "menu-permissions.json")
	}
	return filepath.Join(os.TempDir(), "sy-platform-menu-permissions.json")
}

func GitLabMappingFilePath() string {
	if value := os.Getenv("SY_PLATFORM_GITLAB_MAPPING_FILE"); value != "" {
		return value
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "sy-platform", "gitlab-group-mappings.json")
	}
	return filepath.Join(os.TempDir(), "sy-platform-gitlab-group-mappings.json")
}

func DefaultPlatformSettings() PlatformSettings {
	return PlatformSettings{
		PlatformName: "OMS 运维平台",
		LogoText:     "OMS",
		ThemeColor:   "#2563eb",
		LDAPAuth: LDAPAuthSettings{
			UserFilter:           "(&(objectClass=person)(uid=%s))",
			UserAttribute:        "uid",
			DisplayNameAttribute: "cn",
			EmailAttribute:       "mail",
			DefaultRoleCode:      "developer",
		},
		UpdatedAt: time.Now(),
	}
}

func DefaultDingTalkOrgConfig() DingTalkOrgConfig {
	return DingTalkOrgConfig{
		RootDeptID: "1",
		SyncMode:   "manual",
		Status:     "disabled",
		UpdatedAt:  time.Now(),
	}
}

func DefaultMenuPermissions() []MenuPermission {
	return []MenuPermission{
		{MenuKey: "overview", MenuName: "总览", RoleCodes: []string{"platform_admin", "ops_owner", "dev_owner", "developer", "approver", "viewer"}},
		{MenuKey: "apptree", MenuName: "应用树", RoleCodes: []string{"platform_admin", "ops_owner", "dev_owner", "developer"}},
		{MenuKey: "itsm", MenuName: "工单", RoleCodes: []string{"platform_admin", "ops_owner", "dev_owner", "developer", "approver"}},
		{MenuKey: "cmdb", MenuName: "资产", RoleCodes: []string{"platform_admin", "ops_owner"}},
		{MenuKey: "org", MenuName: "组织", RoleCodes: []string{"platform_admin", "ops_owner"}},
		{MenuKey: "cicd", MenuName: "流水线", RoleCodes: []string{"platform_admin", "ops_owner", "dev_owner", "developer"}},
		{MenuKey: "deploy", MenuName: "发布", RoleCodes: []string{"platform_admin", "ops_owner", "approver"}},
		{MenuKey: "audit", MenuName: "审计", RoleCodes: []string{"platform_admin", "ops_owner"}},
		{MenuKey: "settings", MenuName: "设置", RoleCodes: []string{"platform_admin"}},
	}
}

func (s *Store) Next(kind string) int64 {
	s.next[kind]++
	return s.next[kind]
}

func (s *Store) CurrentActorID() int64 {
	if s.CurrentUserID != 0 {
		for _, user := range s.Users {
			if user.ID == s.CurrentUserID {
				return s.CurrentUserID
			}
		}
	}
	if len(s.Users) == 0 {
		return 0
	}
	return s.Users[0].ID
}

func (s *Store) CurrentUser() User {
	actorID := s.CurrentActorID()
	for _, user := range s.Users {
		if user.ID == actorID {
			return user
		}
	}
	return User{}
}

func (s *Store) CurrentRoleCodes() []string {
	return s.RoleCodesForUser(s.CurrentActorID())
}

func (s *Store) RoleCodesForUser(userID int64) []string {
	codes := make([]string, 0)
	seen := map[string]bool{}
	for _, binding := range s.PolicyBindings {
		if binding.UserID != userID || binding.RoleCode == "" || seen[binding.RoleCode] {
			continue
		}
		seen[binding.RoleCode] = true
		codes = append(codes, binding.RoleCode)
	}
	return codes
}

func (s *Store) HasAnyCurrentRole(candidates ...string) bool {
	return s.HasAnyRole(s.CurrentActorID(), candidates...)
}

func (s *Store) HasAnyRole(userID int64, candidates ...string) bool {
	allowed := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		allowed[candidate] = true
	}
	for _, code := range s.RoleCodesForUser(userID) {
		if allowed[code] {
			return true
		}
	}
	return false
}

func (s *Store) UserByID(userID int64) User {
	for _, user := range s.Users {
		if user.ID == userID {
			return user
		}
	}
	return User{}
}

func (s *Store) VisibleServiceIDs() map[int64]bool {
	return s.VisibleServiceIDsForUser(s.CurrentActorID())
}

func (s *Store) VisibleServiceIDsForUser(actorID int64) map[int64]bool {
	out := map[int64]bool{}
	if s.HasAnyRole(actorID, "platform_admin", "ops_owner") {
		for _, service := range s.Services {
			out[service.ID] = true
		}
		return out
	}
	for _, service := range s.Services {
		if service.OwnerUserID == actorID {
			out[service.ID] = true
		}
	}
	for _, member := range s.ServiceMembers {
		if member.UserID == actorID && member.Status != "disabled" {
			out[member.ServiceID] = true
		}
	}
	return out
}

func (s *Store) HasCurrentServiceAccess(serviceID int64) bool {
	return s.VisibleServiceIDs()[serviceID]
}

func (s *Store) HasServiceAccess(actorID, serviceID int64) bool {
	return s.VisibleServiceIDsForUser(actorID)[serviceID]
}

func (s *Store) CanManageCurrentService(serviceID int64) bool {
	return s.CanManageService(s.CurrentActorID(), serviceID)
}

func (s *Store) CanManageService(actorID, serviceID int64) bool {
	if s.HasAnyRole(actorID, "platform_admin", "ops_owner") {
		return true
	}
	for _, service := range s.Services {
		if service.ID == serviceID && service.OwnerUserID == actorID {
			return true
		}
	}
	for _, member := range s.ServiceMembers {
		if member.ServiceID == serviceID && member.UserID == actorID && member.Status != "disabled" && (member.Role == "owner" || member.Role == "maintainer") {
			return true
		}
	}
	return false
}

func (s *Store) VisibleApplicationIDs() map[int64]bool {
	return s.VisibleApplicationIDsForUser(s.CurrentActorID())
}

func (s *Store) VisibleApplicationIDsForUser(actorID int64) map[int64]bool {
	out := map[int64]bool{}
	visibleServices := s.VisibleServiceIDsForUser(actorID)
	for _, service := range s.Services {
		if visibleServices[service.ID] {
			out[service.ApplicationID] = true
		}
	}
	if s.HasAnyRole(actorID, "platform_admin", "ops_owner") {
		for _, app := range s.Applications {
			out[app.ID] = true
		}
	}
	for _, app := range s.Applications {
		if app.OwnerUserID == actorID {
			out[app.ID] = true
		}
	}
	return out
}

func (s *Store) HasCurrentApplicationAccess(applicationID int64) bool {
	return s.VisibleApplicationIDs()[applicationID]
}

func (s *Store) HasApplicationAccess(actorID, applicationID int64) bool {
	return s.VisibleApplicationIDsForUser(actorID)[applicationID]
}

func (s *Store) Audit(actorID int64, action, resourceType string, resourceID int64, result, reason string) {
	if actorID == 0 {
		actorID = s.CurrentActorID()
	}
	s.AuditLogs = append(s.AuditLogs, AuditLog{
		ID:           s.Next("audit"),
		ActorUserID:  actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       result,
		Reason:       reason,
		CreatedAt:    time.Now(),
	})
}

func (s *Store) AuditSystem(action, resourceType string, resourceID int64, result, reason string) {
	s.AuditLogs = append(s.AuditLogs, AuditLog{
		ID:           s.Next("audit"),
		ActorUserID:  0,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Result:       result,
		Reason:       reason,
		CreatedAt:    time.Now(),
	})
}

func (s *Store) ensureDemoEnvironmentTargetsLocked() {
	service, ok := s.findServiceByNameLocked("order-api")
	if !ok {
		return
	}
	cluster, ok := s.findClusterByNameLocked("prod-hz-ack-01")
	if !ok && len(s.K8sClusters) > 0 {
		cluster = s.K8sClusters[0]
		ok = true
	}
	if !ok {
		return
	}
	targets := []struct {
		envName      string
		releaseLevel string
		namespace    string
	}{
		{envName: "dev", releaseLevel: "development", namespace: "order-dev"},
		{envName: "test", releaseLevel: "testing", namespace: "order-test"},
		{envName: "prod", releaseLevel: "production", namespace: "order-prod"},
	}
	for _, target := range targets {
		namespace := s.ensureDemoNamespaceLocked(cluster.ID, service.ID, target.namespace)
		if s.hasEnvironmentLocked(service.ID, target.envName) {
			continue
		}
		s.Environments = append(s.Environments, Environment{
			ID:             s.Next("environment"),
			ServiceID:      service.ID,
			Name:           target.envName,
			ReleaseLevel:   target.releaseLevel,
			K8sClusterID:   cluster.ID,
			K8sNamespaceID: namespace.ID,
			Status:         "enabled",
		})
	}
}

func (s *Store) ensureBuiltinCloudResourceTypesLocked() bool {
	changed := false
	now := time.Now()
	existing := make(map[string]int, len(s.CloudResourceTypes))
	for index, item := range s.CloudResourceTypes {
		provider := NormalizeCloudProvider(item.Provider)
		resourceType := strings.ToLower(strings.TrimSpace(item.ResourceType))
		if provider != item.Provider {
			s.CloudResourceTypes[index].Provider = provider
			changed = true
		}
		if resourceType != item.ResourceType {
			s.CloudResourceTypes[index].ResourceType = resourceType
			changed = true
		}
		if s.CloudResourceTypes[index].UpdatedAt.IsZero() {
			s.CloudResourceTypes[index].UpdatedAt = now
			changed = true
		}
		existing[provider+"/"+resourceType] = index
	}
	for _, item := range BuiltinCloudResourceTypes(now) {
		key := item.Provider + "/" + item.ResourceType
		if index, ok := existing[key]; ok {
			if s.CloudResourceTypes[index].DisplayName == "" {
				s.CloudResourceTypes[index].DisplayName = item.DisplayName
				changed = true
			}
			if s.CloudResourceTypes[index].Category == "" {
				s.CloudResourceTypes[index].Category = item.Category
				changed = true
			}
			if s.CloudResourceTypes[index].SyncMode == "" {
				s.CloudResourceTypes[index].SyncMode = item.SyncMode
				changed = true
			}
			if s.CloudResourceTypes[index].SyncMode == "generic" {
				s.CloudResourceTypes[index].SyncMode = item.SyncMode
				changed = true
			}
			if s.CloudResourceTypes[index].Status == "" {
				s.CloudResourceTypes[index].Status = item.Status
				changed = true
			}
			continue
		}
		item.ID = s.Next("cloud_resource_type")
		s.CloudResourceTypes = append(s.CloudResourceTypes, item)
		changed = true
	}
	return changed
}

func (s *Store) ensureApplicationRepositoryIdentityLocked() bool {
	changed := false
	appByID := make(map[int64]int, len(s.Applications))
	for index := range s.Applications {
		appByID[s.Applications[index].ID] = index
		normalizedAppID := NormalizeAppID(s.Applications[index].AppID)
		if normalizedAppID == "" {
			normalizedAppID = NormalizeAppID(s.Applications[index].Name)
		}
		if s.Applications[index].AppID != normalizedAppID {
			s.Applications[index].AppID = normalizedAppID
			changed = true
		}
		normalizedProvider := NormalizeRepositoryProvider(s.Applications[index].RepositoryProvider)
		if s.Applications[index].RepositoryProvider != "" && s.Applications[index].RepositoryProvider != normalizedProvider {
			s.Applications[index].RepositoryProvider = normalizedProvider
			changed = true
		}
		normalizedFullName := NormalizeRepositoryFullName(s.Applications[index].RepositoryFullName)
		if s.Applications[index].RepositoryFullName != normalizedFullName {
			s.Applications[index].RepositoryFullName = normalizedFullName
			changed = true
		}
		trimmedURL := strings.TrimSpace(s.Applications[index].RepositoryURL)
		if s.Applications[index].RepositoryURL != trimmedURL {
			s.Applications[index].RepositoryURL = trimmedURL
			changed = true
		}
	}
	for index := range s.Services {
		normalizedProvider := NormalizeRepositoryProvider(s.Services[index].RepositoryProvider)
		if s.Services[index].RepositoryProvider != "" && s.Services[index].RepositoryProvider != normalizedProvider {
			s.Services[index].RepositoryProvider = normalizedProvider
			changed = true
		}
		normalizedFullName := NormalizeRepositoryFullName(s.Services[index].RepositoryFullName)
		if s.Services[index].RepositoryFullName != normalizedFullName {
			s.Services[index].RepositoryFullName = normalizedFullName
			changed = true
		}
		trimmedURL := strings.TrimSpace(s.Services[index].RepositoryURL)
		if s.Services[index].RepositoryURL != trimmedURL {
			s.Services[index].RepositoryURL = trimmedURL
			changed = true
		}
		appIndex, ok := appByID[s.Services[index].ApplicationID]
		if !ok {
			continue
		}
		app := s.Applications[appIndex]
		if s.Services[index].RepositoryFullName == "" && s.Services[index].RepositoryURL == "" && (app.RepositoryFullName != "" || app.RepositoryURL != "") {
			s.Services[index].RepositoryProvider = app.RepositoryProvider
			s.Services[index].RepositoryFullName = app.RepositoryFullName
			s.Services[index].RepositoryURL = app.RepositoryURL
			changed = true
		}
		normalizedService := NormalizeRuntimeProfile(s.Services[index])
		if s.Services[index].RuntimeLanguage != normalizedService.RuntimeLanguage ||
			s.Services[index].RuntimeVersion != normalizedService.RuntimeVersion ||
			s.Services[index].BuildTool != normalizedService.BuildTool {
			s.Services[index].RuntimeLanguage = normalizedService.RuntimeLanguage
			s.Services[index].RuntimeVersion = normalizedService.RuntimeVersion
			s.Services[index].BuildTool = normalizedService.BuildTool
			changed = true
		}
	}
	return changed
}

func (s *Store) ensureServiceOwnerMembersLocked() {
	for _, service := range s.Services {
		if service.OwnerUserID == 0 || s.hasServiceMemberLocked(service.ID, service.OwnerUserID) {
			continue
		}
		s.ServiceMembers = append(s.ServiceMembers, ServiceMember{
			ID:        s.Next("service_member"),
			ServiceID: service.ID,
			UserID:    service.OwnerUserID,
			Role:      "owner",
			Status:    "enabled",
			CreatedAt: time.Now(),
		})
	}
}

func (s *Store) hasServiceMemberLocked(serviceID, userID int64) bool {
	for _, item := range s.ServiceMembers {
		if item.ServiceID == serviceID && item.UserID == userID && item.Status != "disabled" {
			return true
		}
	}
	return false
}

func (s *Store) findServiceByNameLocked(name string) (Service, bool) {
	for _, item := range s.Services {
		if item.Name == name {
			return item, true
		}
	}
	return Service{}, false
}

func (s *Store) findClusterByNameLocked(name string) (K8sCluster, bool) {
	for _, item := range s.K8sClusters {
		if item.Name == name {
			return item, true
		}
	}
	return K8sCluster{}, false
}

func (s *Store) ensureDemoNamespaceLocked(clusterID, serviceID int64, name string) K8sNamespace {
	for _, item := range s.K8sNamespaces {
		if item.ClusterID == clusterID && item.Name == name {
			return item
		}
	}
	item := K8sNamespace{ID: s.Next("k8s_namespace"), ClusterID: clusterID, Name: name, ScopeType: "service", ScopeID: serviceID, Status: "enabled"}
	s.K8sNamespaces = append(s.K8sNamespaces, item)
	return item
}

func (s *Store) hasEnvironmentLocked(serviceID int64, name string) bool {
	for _, item := range s.Environments {
		if item.ServiceID == serviceID && item.Name == name {
			return true
		}
	}
	return false
}

func (s *Store) PersistPlatformSettings(settings PlatformSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.settingsFile), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(s.settingsFile, data, 0600); err != nil {
		return err
	}
	s.Settings = settings
	return nil
}

func (s *Store) PersistDingTalkConfig(config DingTalkOrgConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.orgConfigFile), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(s.orgConfigFile, data, 0600); err != nil {
		return err
	}
	s.DingTalkConfig = config
	return nil
}

func (s *Store) PersistMenuPermissions(items []MenuPermission) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.menuPermissionFile), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(s.menuPermissionFile, data, 0600); err != nil {
		return err
	}
	s.MenuPermissions = items
	return nil
}

func (s *Store) PersistGitLabMappings(items []GitLabGroupMapping) error {
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.gitLabMappingFile), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(s.gitLabMappingFile, data, 0600); err != nil {
		return err
	}
	s.GitLabMappings = items
	return nil
}

func (s *Store) loadPlatformSettings() {
	data, err := os.ReadFile(s.settingsFile)
	if err != nil {
		return
	}
	var settings PlatformSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}
	normalized, err := NormalizePlatformSettings(settings)
	if err != nil {
		return
	}
	if !settings.UpdatedAt.IsZero() {
		normalized.UpdatedAt = settings.UpdatedAt
	}
	s.Settings = normalized
}

func (s *Store) loadDingTalkConfig() {
	data, err := os.ReadFile(s.orgConfigFile)
	if err != nil {
		return
	}
	var config DingTalkOrgConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return
	}
	s.DingTalkConfig = NormalizeDingTalkConfig(config)
}

func (s *Store) loadMenuPermissions() {
	data, err := os.ReadFile(s.menuPermissionFile)
	if err != nil {
		return
	}
	var items []MenuPermission
	if err := json.Unmarshal(data, &items); err != nil {
		return
	}
	if len(items) > 0 {
		s.MenuPermissions = items
	}
}

func (s *Store) loadGitLabMappings() {
	data, err := os.ReadFile(s.gitLabMappingFile)
	if err != nil {
		return
	}
	var items []GitLabGroupMapping
	if err := json.Unmarshal(data, &items); err != nil {
		return
	}
	for _, item := range items {
		if item.ID > s.next["gitlab_mapping"] {
			s.next["gitlab_mapping"] = item.ID
		}
	}
	s.GitLabMappings = items
}

func NormalizeDingTalkConfig(config DingTalkOrgConfig) DingTalkOrgConfig {
	config.CorpID = strings.TrimSpace(config.CorpID)
	config.AppKey = strings.TrimSpace(config.AppKey)
	config.AppSecret = strings.TrimSpace(config.AppSecret)
	config.AgentID = strings.TrimSpace(config.AgentID)
	config.RootDeptID = strings.TrimSpace(config.RootDeptID)
	config.SyncMode = strings.ToLower(strings.TrimSpace(config.SyncMode))
	config.Status = strings.ToLower(strings.TrimSpace(config.Status))
	if config.RootDeptID == "" {
		config.RootDeptID = "1"
	}
	if config.SyncMode != "scheduled" {
		config.SyncMode = "manual"
	}
	if config.Status != "enabled" {
		config.Status = "disabled"
	}
	if config.UpdatedAt.IsZero() {
		config.UpdatedAt = time.Now()
	}
	return config
}

func NormalizeServiceType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "frontend", "front-end", "web", "node", "vue", "react":
		return "frontend"
	default:
		return "backend"
	}
}

func DefaultPipelineDefinition(serviceType string) PipelineDef {
	switch NormalizeServiceType(serviceType) {
	case "frontend":
		return PipelineDef{
			Triggers: []string{"manual", "git_push"},
			Steps: []PipelineStep{
				DefaultPipelineStep("checkout", "git"),
				DefaultPipelineStep("install", "node_install"),
				DefaultPipelineStep("lint", "npm_lint"),
				DefaultPipelineStep("build", "npm_build"),
				DefaultPipelineStep("artifact", "static_artifact"),
				DefaultPipelineStep("image", "nginx_image"),
				DefaultPipelineStep("deploy", "k8s_deploy"),
				DefaultPipelineStep("notify", "notification"),
			},
		}
	default:
		return PipelineDef{
			Triggers: []string{"manual", "git_push"},
			Steps: []PipelineStep{
				DefaultPipelineStep("checkout", "git"),
				DefaultPipelineStep("unit_test", "go_test"),
				DefaultPipelineStep("build", "go_build"),
				DefaultPipelineStep("image", "docker_build"),
				DefaultPipelineStep("deploy", "k8s_deploy"),
				DefaultPipelineStep("notify", "notification"),
			},
		}
	}
}

func JenkinsK8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			PipelineStepWithCommand("prepare", "jenkins_env", `export DEPLOY_ENV="${DEPLOY_ENV}" KUBECONFIG="${KUBECONFIG_PATH}" BUILD_ROOT="${BUILD_ROOT}" GIT_HOST="${GIT_HOST}" GIT_GROUP="${GIT_GROUP}" GIT_PROJECT="${GIT_PROJECT}" APP_ID="${APP_ID}" HARBOR_REGISTRY="${HARBOR_REGISTRY}" HARBOR_PROJECT="${HARBOR_PROJECT}" DEPARTMENT="${DEPARTMENT}" && echo "action=${DEPLOY_ACTION} env=${DEPLOY_ENV} app=${APP_ID} department=${DEPARTMENT}"`),
			PipelineStepWithCommand("checkout", "git", `git -C "${BUILD_ROOT}/${DEPLOY_ENV}/src/${GIT_HOST}/${GIT_GROUP}/${GIT_PROJECT}" fetch --all --prune && git -C "${BUILD_ROOT}/${DEPLOY_ENV}/src/${GIT_HOST}/${GIT_GROUP}/${GIT_PROJECT}" checkout "${GIT_REF}"`),
			PipelineStepWithCommand("build", "go_build", `case "${DEPLOY_ACTION}" in update-first|update-all) export GOPATH="${BUILD_ROOT}/${DEPLOY_ENV}" GOROOT="${GO_ROOT}" CGO_ENABLED=0 && cd "${GOPATH}/src/${GIT_HOST}/${GIT_GROUP}/${GIT_PROJECT}" && "${GOROOT}/bin/go" build -o "${APP_ID}" -a -installsuffix cgo ./app ;; *) echo "skip build for ${DEPLOY_ACTION}" ;; esac`),
			PipelineStepWithCommand("image", "docker_build", `case "${DEPLOY_ACTION}" in update-first|update-all) IMAGE="${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_ID}:${DEPLOY_ENV}_v${BUILD_NUMBER}" && docker build -t "${APP_ID}" "${BUILD_ROOT}/${DEPLOY_ENV}/src/${GIT_HOST}/${GIT_GROUP}/${GIT_PROJECT}" && docker tag "${APP_ID}" "${IMAGE}" && docker push "${IMAGE}" && docker images | grep "${HARBOR_REGISTRY}/${HARBOR_PROJECT}/${APP_ID}" | awk '{print $3}' | xargs -r docker rmi -f ;; *) echo "skip image for ${DEPLOY_ACTION}" ;; esac`),
			PipelineStepWithCommand("manifest", "k8s_manifest", `case "${DEPLOY_ACTION}" in update-first|update-all) sed -i -r "/image: ${HARBOR_REGISTRY}/ s/(_v[0-9]+)/_v${BUILD_NUMBER}/g" "${BUILD_ROOT}/${DEPLOY_ENV}/${DEPARTMENT}/${APP_ID}-ack.yml" ;; rollback) sed -i -r "/image: ${HARBOR_REGISTRY}/ s/(v[0-9]+)/v${ROLLBACK_VERSION}/g" "${BUILD_ROOT}/${DEPLOY_ENV}/${DEPARTMENT}/${APP_ID}-ack.yml" ;; *) echo "skip manifest for ${DEPLOY_ACTION}" ;; esac`),
			PipelineStepWithCommand("update_first", "k8s_canary_first", `test "${DEPLOY_ACTION}" = "update-first" || exit 0; kubectl apply -f "${BUILD_ROOT}/${DEPLOY_ENV}/${DEPARTMENT}/${APP_ID}-ack.yml" && kubectl rollout pause deployment "${APP_ID}" -n "${DEPLOY_ENV}"`),
			PipelineStepWithCommand("update_rest", "k8s_canary_rest", `test "${DEPLOY_ACTION}" = "update-rest" || exit 0; kubectl rollout resume deployment "${APP_ID}" -n "${DEPLOY_ENV}"`),
			PipelineStepWithCommand("update_all", "k8s_deploy", `test "${DEPLOY_ACTION}" = "update-all" || exit 0; kubectl apply -f "${BUILD_ROOT}/${DEPLOY_ENV}/${DEPARTMENT}/${APP_ID}-ack.yml"`),
			PipelineStepWithCommand("restart", "k8s_restart", `test "${DEPLOY_ACTION}" = "restart" || exit 0; kubectl patch deployment "${APP_ID}" -n "${DEPLOY_ENV}" -p '{"spec":{"template":{"spec":{"containers":[{"name":"'"${APP_ID}"'","env":[{"name":"RESTART_","value":"'"$(date +%s)"'"}]}]}}}}'`),
			PipelineStepWithCommand("rollback", "k8s_rollback", `test "${DEPLOY_ACTION}" = "rollback" || exit 0; kubectl apply -f "${BUILD_ROOT}/${DEPLOY_ENV}/${DEPARTMENT}/${APP_ID}-ack.yml"`),
			PipelineStepWithCommand("pod_status", "k8s_pod_status", `for i in $(seq "${POD_CHECK_TIMES}"); do sleep "${POD_CHECK_INTERVAL}"; kubectl get pod -n "${DEPLOY_ENV}" -l "app=${APP_ID}"; done`),
			PipelineStepWithCommand("judge_canary", "k8s_canary_check", `test "${DEPLOY_ACTION}" = "update-first" || exit 0; rs_num=$(kubectl get pod -n "${DEPLOY_ENV}" | grep "${APP_ID}" | grep Running | awk -F"-" '{print $(NF-1)}' | cut -d " " -f 1 | sort -u | wc -l); test "${rs_num}" -eq 2`),
			PipelineStepWithCommand("rollout", "k8s_rollout_status", `case "${DEPLOY_ACTION}" in update-rest|update-all|restart|rollback) kubectl rollout status deployment/"${APP_ID}" -n "${DEPLOY_ENV}" | grep successful ;; *) echo "skip rollout check for ${DEPLOY_ACTION}" ;; esac`),
			PipelineStepWithCommand("notify", "notification", `curl -X POST "${WEBHOOK_URL}" -d "text=${APP_ID} ${DEPLOY_ENV}_v${BUILD_NUMBER} ${DEPLOY_ACTION} completed"`),
		},
	}
}

func JenkinsK8sModularPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("unit_test", "go_test"),
			ModulePipelineStep("build", "go_build"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("update_first", "k8s_canary_first"),
			ModulePipelineStep("update_rest", "k8s_canary_rest"),
			ModulePipelineStep("update_all", "k8s_deploy_all"),
			ModulePipelineStep("restart", "k8s_restart"),
			ModulePipelineStep("rollback", "k8s_rollback"),
			ModulePipelineStep("pod_status", "k8s_pod_status"),
			ModulePipelineStep("judge_canary", "k8s_canary_check"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func JDK8K8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("build", "jdk8_maven_build"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("deploy", "k8s_deploy_all"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func PythonK8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("install", "python_install"),
			ModulePipelineStep("unit_test", "python_test"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("deploy", "k8s_deploy_all"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func CK8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("build", "c_make_build"),
			ModulePipelineStep("unit_test", "c_test"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("deploy", "k8s_deploy_all"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func CPPK8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("build", "cpp_cmake_build"),
			ModulePipelineStep("unit_test", "cpp_test"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("deploy", "k8s_deploy_all"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func DotnetK8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("restore", "dotnet_restore"),
			ModulePipelineStep("unit_test", "dotnet_test"),
			ModulePipelineStep("publish", "dotnet_publish"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("deploy", "k8s_deploy_all"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func RustK8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("unit_test", "rust_test"),
			ModulePipelineStep("build", "rust_build"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("deploy", "k8s_deploy_all"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func PHPK8sPipelineDefinition() PipelineDef {
	return PipelineDef{
		Triggers: []string{"manual", "git_push"},
		Steps: []PipelineStep{
			ModulePipelineStep("checkout", "gitlab_checkout"),
			ModulePipelineStep("install", "php_composer_install"),
			ModulePipelineStep("unit_test", "php_test"),
			ModulePipelineStep("image", "docker_build_push"),
			ModulePipelineStep("manifest", "k8s_manifest_image"),
			ModulePipelineStep("deploy", "k8s_deploy_all"),
			ModulePipelineStep("rollout", "k8s_rollout_status"),
			ModulePipelineStep("notify", "notification"),
		},
	}
}

func PipelineStepWithCommand(name, stepType, command string) PipelineStep {
	return PipelineStep{Name: name, Type: stepType, ModuleKey: DefaultPipelineModuleKey(stepType), With: map[string]any{"command": command}}
}

func DefaultPipelineStep(name, stepType string) PipelineStep {
	step := PipelineStep{Name: name, Type: stepType, ModuleKey: DefaultPipelineModuleKey(stepType)}
	command := DefaultPipelineStepCommand(stepType)
	if command != "" {
		step.With = map[string]any{"command": command}
	}
	if stepType == "node_install" {
		step.With["packageManager"] = "pnpm"
	}
	return step
}

func DefaultPipelineStepCommand(stepType string) string {
	switch strings.TrimSpace(stepType) {
	case "git":
		return "git fetch --all --prune && git checkout ${GIT_REF}"
	case "go_test":
		return "go test ./..."
	case "go_build":
		return "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/${APP_NAME} ./cmd/${APP_NAME}"
	case "python_install":
		return "python -m pip install -r ${PYTHON_REQUIREMENTS:-requirements.txt}"
	case "python_test":
		return "python -m pytest"
	case "c_make_build":
		return "make ${MAKE_TARGET:-build}"
	case "c_test":
		return "make ${TEST_TARGET:-test}"
	case "cpp_cmake_build":
		return "cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build --parallel"
	case "cpp_test":
		return "ctest --test-dir build --output-on-failure"
	case "dotnet_restore":
		return "dotnet restore"
	case "dotnet_test":
		return "dotnet test --no-restore"
	case "dotnet_publish":
		return "dotnet publish ${DOTNET_PROJECT:-.} -c Release -o publish"
	case "rust_test":
		return "cargo test --locked"
	case "rust_build":
		return "cargo build --release --locked"
	case "php_composer_install":
		return "composer install --no-interaction --prefer-dist --optimize-autoloader"
	case "php_test":
		return "if [ -x vendor/bin/phpunit ]; then vendor/bin/phpunit; else echo skip phpunit; fi"
	case "docker_build":
		return "docker build -t ${IMAGE} . && docker push ${IMAGE}"
	case "node_install":
		return "pnpm install --frozen-lockfile"
	case "npm_lint":
		return "pnpm lint"
	case "npm_build":
		return "pnpm build"
	case "static_artifact":
		return "tar -czf dist/static-${VERSION}.tgz dist"
	case "nginx_image":
		return "docker build -f Dockerfile.nginx -t ${IMAGE} . && docker push ${IMAGE}"
	case "k8s_deploy":
		return "kubectl -n ${NAMESPACE} set image deployment/${APP_NAME} ${CONTAINER_NAME}=${IMAGE} && kubectl -n ${NAMESPACE} rollout status deployment/${APP_NAME}"
	case "notification":
		return "curl -X POST ${WEBHOOK_URL} -d \"text=${APP_NAME} ${VERSION} pipeline completed\""
	default:
		return ""
	}
}

func ClonePipelineDef(def PipelineDef) PipelineDef {
	clone := PipelineDef{
		Triggers: append([]string(nil), def.Triggers...),
		Steps:    make([]PipelineStep, 0, len(def.Steps)),
	}
	for _, step := range def.Steps {
		copied := PipelineStep{Name: step.Name, Type: step.Type, ModuleKey: step.ModuleKey}
		if step.With != nil {
			copied.With = make(map[string]any, len(step.With))
			for key, value := range step.With {
				copied.With[key] = value
			}
		}
		clone.Steps = append(clone.Steps, copied)
	}
	return clone
}

func (s *Store) DefaultPipeline(service Service) Pipeline {
	service = NormalizeRuntimeProfile(service)
	templateID := int64(0)
	definition := DefaultPipelineDefinition(service.ServiceType)
	if template, ok := s.pipelineTemplateForService(service); ok {
		templateID = template.ID
		definition = ClonePipelineDef(template.Definition)
	}
	return Pipeline{
		ID:         s.Next("pipeline"),
		ServiceID:  service.ID,
		TemplateID: templateID,
		Name:       fmt.Sprintf("%s-default", service.Name),
		Status:     "generated",
		Definition: definition,
	}
}

func (s *Store) pipelineTemplateForService(service Service) (PipelineTemplate, bool) {
	if service.PipelineTemplateID != 0 {
		for _, template := range s.PipelineTemplates {
			if template.ID == service.PipelineTemplateID && template.Status == "enabled" {
				return template, true
			}
		}
	}
	normalized := NormalizeServiceType(service.ServiceType)
	language := NormalizeRuntimeLanguage(service.RuntimeLanguage)
	buildTool := NormalizeBuildTool(language, service.BuildTool)
	for _, template := range s.PipelineTemplates {
		if template.Status == "enabled" && NormalizeServiceType(template.ServiceType) == normalized && templateRuntimeMatches(template, language, buildTool) {
			return template, true
		}
	}
	for _, template := range s.PipelineTemplates {
		if NormalizeServiceType(template.ServiceType) == normalized && template.Status == "enabled" {
			return template, true
		}
	}
	for _, template := range s.PipelineTemplates {
		if template.Status == "enabled" {
			return template, true
		}
	}
	if len(s.PipelineTemplates) == 0 {
		return PipelineTemplate{}, false
	}
	return s.PipelineTemplates[0], true
}

func templateRuntimeMatches(template PipelineTemplate, language, buildTool string) bool {
	if language == "" {
		return false
	}
	metadataLanguage, _ := template.Metadata["runtimeLanguage"].(string)
	metadataBuildTool, _ := template.Metadata["buildTool"].(string)
	if NormalizeRuntimeLanguage(metadataLanguage) != language {
		return false
	}
	if buildTool == "" || metadataBuildTool == "" {
		return true
	}
	return NormalizeBuildTool(language, metadataBuildTool) == buildTool
}

func (s *Store) PipelineTemplateByID(id int64) (PipelineTemplate, bool) {
	for _, template := range s.PipelineTemplates {
		if template.ID == id && template.Status == "enabled" {
			return template, true
		}
	}
	return PipelineTemplate{}, false
}

func (s *Store) RecommendedPipelineTemplateID(service Service) int64 {
	service = NormalizeRuntimeProfile(service)
	if template, ok := s.pipelineTemplateForService(service); ok {
		return template.ID
	}
	return 0
}

type pipelineTemplateSpec struct {
	Name        string
	ServiceType string
	Definition  PipelineDef
	Metadata    map[string]any
}

func builtinPipelineTemplateSpecs() []pipelineTemplateSpec {
	return []pipelineTemplateSpec{
		{
			Name:        "go-k8s-default",
			ServiceType: "backend",
			Definition:  DefaultPipelineDefinition("backend"),
			Metadata: map[string]any{
				"runtimeLanguage": "go",
				"runtimeVersion":  "1.22",
				"buildTool":       "go",
			},
		},
		{
			Name:        "web-k8s-default",
			ServiceType: "frontend",
			Definition:  DefaultPipelineDefinition("frontend"),
			Metadata: map[string]any{
				"runtimeLanguage": "node",
				"runtimeVersion":  "20",
				"buildTool":       "pnpm",
			},
		},
		{
			Name:        "jdk8-k8s-default",
			ServiceType: "backend",
			Definition:  JDK8K8sPipelineDefinition(),
			Metadata: map[string]any{
				"runtimeLanguage": "java",
				"runtimeVersion":  "8",
				"buildTool":       "maven",
			},
		},
		{
			Name:        "python-k8s-default",
			ServiceType: "backend",
			Definition:  PythonK8sPipelineDefinition(),
			Metadata: map[string]any{
				"runtimeLanguage": "python",
				"runtimeVersion":  "3.11",
				"buildTool":       "pip",
			},
		},
		{
			Name:        "c-k8s-default",
			ServiceType: "backend",
			Definition:  CK8sPipelineDefinition(),
			Metadata: map[string]any{
				"runtimeLanguage": "c",
				"runtimeVersion":  "c17",
				"buildTool":       "make",
			},
		},
		{
			Name:        "cpp-k8s-default",
			ServiceType: "backend",
			Definition:  CPPK8sPipelineDefinition(),
			Metadata: map[string]any{
				"runtimeLanguage": "cpp",
				"runtimeVersion":  "c++17",
				"buildTool":       "cmake",
			},
		},
		{
			Name:        "dotnet-k8s-default",
			ServiceType: "backend",
			Definition:  DotnetK8sPipelineDefinition(),
			Metadata: map[string]any{
				"runtimeLanguage": "dotnet",
				"runtimeVersion":  "8.0",
				"buildTool":       "dotnet",
			},
		},
		{
			Name:        "rust-k8s-default",
			ServiceType: "backend",
			Definition:  RustK8sPipelineDefinition(),
			Metadata: map[string]any{
				"runtimeLanguage": "rust",
				"runtimeVersion":  "1.78",
				"buildTool":       "cargo",
			},
		},
		{
			Name:        "php-k8s-default",
			ServiceType: "backend",
			Definition:  PHPK8sPipelineDefinition(),
			Metadata: map[string]any{
				"runtimeLanguage": "php",
				"runtimeVersion":  "8.2",
				"buildTool":       "composer",
			},
		},
		{
			Name:        "jenkins-k8s-shell",
			ServiceType: "backend",
			Definition:  JenkinsK8sPipelineDefinition(),
			Metadata: map[string]any{
				"source": "jenkins_shell",
				"variables": []string{
					"DEPLOY_ACTION", "DEPLOY_ENV", "KUBECONFIG_PATH", "BUILD_ROOT", "GIT_HOST", "GIT_GROUP", "GIT_PROJECT",
					"APP_ID", "HARBOR_REGISTRY", "HARBOR_PROJECT", "DEPARTMENT", "GO_ROOT", "BUILD_NUMBER", "ROLLBACK_VERSION",
					"POD_CHECK_TIMES", "POD_CHECK_INTERVAL", "WEBHOOK_URL",
				},
			},
		},
		{
			Name:        "jenkins-k8s-modular",
			ServiceType: "backend",
			Definition:  JenkinsK8sModularPipelineDefinition(),
			Metadata: map[string]any{
				"source": "module_library",
				"mode":   "module_reference",
			},
		},
	}
}

func (s *Store) ensureBuiltinPipelineTemplatesLocked() bool {
	changed := false
	existing := make(map[string]int, len(s.PipelineTemplates))
	for index, template := range s.PipelineTemplates {
		existing[strings.TrimSpace(template.Name)] = index
	}
	for _, spec := range builtinPipelineTemplateSpecs() {
		if index, ok := existing[spec.Name]; ok {
			if mergePipelineTemplateMetadata(&s.PipelineTemplates[index], spec.Metadata) {
				changed = true
			}
			continue
		}
		s.PipelineTemplates = append(s.PipelineTemplates, PipelineTemplate{
			ID:          s.Next("pipeline_template"),
			Name:        spec.Name,
			ServiceType: spec.ServiceType,
			Status:      "enabled",
			Definition:  spec.Definition,
			Metadata:    spec.Metadata,
		})
		existing[spec.Name] = len(s.PipelineTemplates) - 1
		changed = true
	}
	return changed
}

func mergePipelineTemplateMetadata(template *PipelineTemplate, metadata map[string]any) bool {
	if len(metadata) == 0 {
		return false
	}
	changed := false
	if template.Metadata == nil {
		template.Metadata = map[string]any{}
		changed = true
	}
	for key, value := range metadata {
		if _, ok := template.Metadata[key]; ok {
			continue
		}
		template.Metadata[key] = value
		changed = true
	}
	return changed
}

func (s *Store) seedCore() {
	s.Settings = DefaultPlatformSettings()
	s.loadPlatformSettings()
	s.DingTalkConfig = DefaultDingTalkOrgConfig()
	s.loadDingTalkConfig()
	s.MenuPermissions = DefaultMenuPermissions()
	s.loadMenuPermissions()

	dept := Department{ID: s.Next("department"), ExternalID: "local-dept-root", Name: "平台组织", Source: "local", Status: "enabled"}
	user := User{ID: s.Next("user"), ExternalID: "local-admin", Username: "admin", DisplayName: "Platform Admin", AuthSource: "bootstrap", DepartmentID: dept.ID, Status: "enabled"}
	dept.ManagerUserID = user.ID
	roles := []Role{
		{ID: s.Next("role"), Code: "platform_admin", Name: "平台管理员", Status: "enabled"},
		{ID: s.Next("role"), Code: "ops_owner", Name: "运维负责人", Status: "enabled"},
		{ID: s.Next("role"), Code: "dev_owner", Name: "研发负责人", Status: "enabled"},
		{ID: s.Next("role"), Code: "developer", Name: "研发成员", Status: "enabled"},
		{ID: s.Next("role"), Code: "approver", Name: "审批人", Status: "enabled"},
		{ID: s.Next("role"), Code: "viewer", Name: "只读访客", Status: "enabled"},
	}
	s.Departments = append(s.Departments, dept)
	s.Users = append(s.Users, user)
	s.Roles = append(s.Roles, roles...)
	s.PolicyBindings = append(s.PolicyBindings, PolicyBinding{ID: s.Next("binding"), UserID: user.ID, RoleID: roles[0].ID, RoleCode: roles[0].Code, ScopeType: "global", ScopeID: 0})
	s.loadGitLabMappings()
	s.ensureBuiltinPipelineModulesLocked()
	s.ensureBuiltinPipelineTemplatesLocked()
	s.ensureBuiltinCloudResourceTypesLocked()
}

func (s *Store) seed() {
	s.seedCore()

	dept := s.Departments[0]
	user := s.Users[0]
	devDept := Department{ID: s.Next("department"), ExternalID: "dt-dept-dev", ParentID: dept.ID, Name: "研发中心", Source: "dingtalk", Status: "enabled"}
	opsDept := Department{ID: s.Next("department"), ExternalID: "dt-dept-ops", ParentID: dept.ID, Name: "运维中心", Source: "dingtalk", Status: "enabled"}
	devLead := User{ID: s.Next("user"), ExternalID: "dt-user-dev-lead", Username: "dev.lead", DisplayName: "研发主管", Email: "dev.lead@example.com", DepartmentID: devDept.ID, ManagerUserID: user.ID, Status: "enabled"}
	opsLead := User{ID: s.Next("user"), ExternalID: "dt-user-ops-lead", Username: "ops.lead", DisplayName: "运维主管", Email: "ops.lead@example.com", DepartmentID: opsDept.ID, ManagerUserID: user.ID, Status: "enabled"}
	developer := User{ID: s.Next("user"), ExternalID: "dt-user-developer", Username: "developer", DisplayName: "研发同学", Email: "developer@example.com", DepartmentID: devDept.ID, ManagerUserID: devLead.ID, Status: "enabled"}
	devDept.ManagerUserID = devLead.ID
	opsDept.ManagerUserID = opsLead.ID
	s.Departments = append(s.Departments, devDept, opsDept)
	s.Users = append(s.Users, devLead, opsLead, developer)
	if _, err := os.Stat(s.gitLabMappingFile); os.IsNotExist(err) {
		s.GitLabMappings = append(s.GitLabMappings,
			GitLabGroupMapping{ID: s.Next("gitlab_mapping"), DepartmentID: devDept.ID, GitLabGroupPath: "platform/rd-center", AccessLevel: "maintainer", SyncMode: "dingtalk_manager_owner", Status: "enabled", UpdatedAt: time.Now()},
			GitLabGroupMapping{ID: s.Next("gitlab_mapping"), DepartmentID: opsDept.ID, GitLabGroupPath: "platform/ops-center", AccessLevel: "maintainer", SyncMode: "dingtalk_manager_owner", Status: "enabled", UpdatedAt: time.Now()},
		)
	}

	businessGroup := BusinessGroup{ID: s.Next("business_group"), Name: "交易业务组", OwnerUserID: user.ID, Status: "enabled"}
	businessCenter := BusinessCenter{ID: s.Next("business_center"), BusinessGroupID: businessGroup.ID, Name: "电商业务中心", OwnerUserID: user.ID, Status: "enabled"}
	businessLine := BusinessLine{ID: s.Next("business_line"), BusinessCenterID: businessCenter.ID, Name: "交易业务线", OwnerUserID: user.ID, Status: "enabled"}
	system := System{ID: s.Next("system"), BusinessLineID: businessLine.ID, Name: "订单履约系统", OwnerUserID: user.ID, Status: "enabled"}
	targetGroup := BusinessGroup{ID: s.Next("business_group"), Name: "增长业务组", OwnerUserID: user.ID, Status: "enabled"}
	targetCenter := BusinessCenter{ID: s.Next("business_center"), BusinessGroupID: targetGroup.ID, Name: "会员业务中心", OwnerUserID: user.ID, Status: "enabled"}
	targetLine := BusinessLine{ID: s.Next("business_line"), BusinessCenterID: targetCenter.ID, Name: "会员业务线", OwnerUserID: user.ID, Status: "enabled"}
	targetSystem := System{ID: s.Next("system"), BusinessLineID: targetLine.ID, Name: "会员系统", OwnerUserID: user.ID, Status: "enabled"}
	app := Application{ID: s.Next("application"), SystemID: system.ID, AppID: "order-center", Name: "order-center", RepositoryProvider: "gitlab", RepositoryFullName: "platform/rd-center/order-center", RepositoryURL: "git@code.example.internal:platform/rd-center/order-center.git", OwnerUserID: user.ID, LifecycleStatus: "running"}
	service := Service{ID: s.Next("service"), ApplicationID: app.ID, Name: "order-api", ServiceType: "backend", RepositoryProvider: app.RepositoryProvider, RepositoryFullName: app.RepositoryFullName, RepositoryURL: app.RepositoryURL, OwnerUserID: user.ID, Status: "enabled"}
	service = NormalizeRuntimeProfile(service)
	service.PipelineTemplateID = s.RecommendedPipelineTemplateID(service)
	now := time.Now()
	serviceMembers := []ServiceMember{
		{ID: s.Next("service_member"), ServiceID: service.ID, UserID: user.ID, Role: "owner", Status: "enabled", CreatedAt: now},
		{ID: s.Next("service_member"), ServiceID: service.ID, UserID: devLead.ID, Role: "maintainer", Status: "enabled", CreatedAt: now},
		{ID: s.Next("service_member"), ServiceID: service.ID, UserID: developer.ID, Role: "developer", Status: "enabled", CreatedAt: now},
	}
	cluster := K8sCluster{ID: s.Next("k8s_cluster"), Name: "prod-hz-ack-01", Provider: "aliyun", APIServer: "https://k8s.example.internal", Status: "enabled"}
	devNamespace := K8sNamespace{ID: s.Next("k8s_namespace"), ClusterID: cluster.ID, Name: "order-dev", ScopeType: "service", ScopeID: service.ID, Status: "enabled"}
	testNamespace := K8sNamespace{ID: s.Next("k8s_namespace"), ClusterID: cluster.ID, Name: "order-test", ScopeType: "service", ScopeID: service.ID, Status: "enabled"}
	prodNamespace := K8sNamespace{ID: s.Next("k8s_namespace"), ClusterID: cluster.ID, Name: "order-prod", ScopeType: "service", ScopeID: service.ID, Status: "enabled"}
	devEnv := Environment{ID: s.Next("environment"), ServiceID: service.ID, Name: "dev", ReleaseLevel: "development", K8sClusterID: cluster.ID, K8sNamespaceID: devNamespace.ID, Status: "enabled"}
	testEnv := Environment{ID: s.Next("environment"), ServiceID: service.ID, Name: "test", ReleaseLevel: "testing", K8sClusterID: cluster.ID, K8sNamespaceID: testNamespace.ID, Status: "enabled"}
	prodEnv := Environment{ID: s.Next("environment"), ServiceID: service.ID, Name: "prod", ReleaseLevel: "production", K8sClusterID: cluster.ID, K8sNamespaceID: prodNamespace.ID, Status: "enabled"}
	pipeline := s.DefaultPipeline(service)
	pipeline.Status = "enabled"
	ticket := Ticket{ID: s.Next("ticket"), TicketNo: "ITSM-000001", TicketType: "release", Title: "order-api v1.2.3 生产发布", Description: "生产灰度发布申请", ApplicantUserID: user.ID, HandlerUserID: user.ID, Status: "processing", ScopeType: "application", ScopeID: app.ID}
	run := PipelineRun{ID: s.Next("pipeline_run"), PipelineID: pipeline.ID, TriggerUserID: user.ID, RunType: "deploy", Status: "success"}
	release := Release{ID: s.Next("release"), ReleaseNo: "REL-000001", ServiceID: service.ID, EnvironmentID: prodEnv.ID, TicketID: ticket.ID, PipelineRunID: run.ID, Version: "v1.2.3", Strategy: "canary", Status: "health_checking"}
	k8sAsset := Asset{ID: s.Next("asset"), Provider: "aliyun", ResourceType: "k8s_cluster", ResourceUID: "ack-prod-hz-01", Name: cluster.Name, Region: "cn-hangzhou", Source: "sync", ScopeType: "service", ScopeID: service.ID, Status: "active", LastSyncedAt: time.Now()}
	serverAsset := Asset{
		ID:           s.Next("asset"),
		Provider:     "aliyun",
		ResourceType: "server",
		ResourceUID:  "i-prod-order-api-01",
		Name:         "prod-order-api-01",
		Region:       "cn-hangzhou",
		Source:       "sync",
		ScopeType:    "service",
		ScopeID:      service.ID,
		Status:       "running",
		LastSyncedAt: time.Now(),
		Raw: map[string]any{
			"privateIp":     "10.23.8.17",
			"publicIp":      "",
			"os":            "Alibaba Cloud Linux 3",
			"zone":          "cn-hangzhou-h",
			"loginProtocol": "ssh",
		},
	}
	notification := Notification{ID: s.Next("notification"), ReceiverUserID: user.ID, Channel: "in_app", Title: "生产发布待健康确认", Content: "order-api v1.2.3 灰度阶段正在等待健康检查结果", Status: "unread"}

	s.BusinessGroups = append(s.BusinessGroups, businessGroup)
	s.BusinessGroups = append(s.BusinessGroups, targetGroup)
	s.BusinessCenters = append(s.BusinessCenters, businessCenter)
	s.BusinessCenters = append(s.BusinessCenters, targetCenter)
	s.BusinessLines = append(s.BusinessLines, businessLine)
	s.BusinessLines = append(s.BusinessLines, targetLine)
	s.Systems = append(s.Systems, system)
	s.Systems = append(s.Systems, targetSystem)
	s.Applications = append(s.Applications, app)
	s.Services = append(s.Services, service)
	s.ServiceMembers = append(s.ServiceMembers, serviceMembers...)
	s.K8sClusters = append(s.K8sClusters, cluster)
	s.K8sNamespaces = append(s.K8sNamespaces, devNamespace, testNamespace, prodNamespace)
	s.Environments = append(s.Environments, devEnv, testEnv, prodEnv)
	s.Pipelines = append(s.Pipelines, pipeline)
	s.Tickets = append(s.Tickets, ticket)
	s.PipelineRuns = append(s.PipelineRuns, run)
	s.Releases = append(s.Releases, release)
	s.Assets = append(s.Assets, k8sAsset, serverAsset)
	s.Notifications = append(s.Notifications, notification)
	s.HealthChecks = append(s.HealthChecks, HealthCheck{ID: s.Next("health_check"), ReleaseID: release.ID, CheckType: "default", Status: "running", Result: map[string]any{"successRate": "99.92%", "latencyP95": "118ms"}})
	s.Audit(user.ID, "seed.demo", "platform", 1, "success", "demo data loaded")
}
