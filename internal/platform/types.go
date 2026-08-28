package platform

import "time"

type User struct {
	ID            int64  `json:"id"`
	ExternalID    string `json:"externalId"`
	AuthSource    string `json:"authSource,omitempty"`
	Username      string `json:"username"`
	DisplayName   string `json:"displayName"`
	Email         string `json:"email"`
	DepartmentID  int64  `json:"departmentId"`
	ManagerUserID int64  `json:"managerUserId"`
	Status        string `json:"status"`
}

type Department struct {
	ID            int64  `json:"id"`
	ExternalID    string `json:"externalId"`
	ParentID      int64  `json:"parentId"`
	Name          string `json:"name"`
	ManagerUserID int64  `json:"managerUserId"`
	Source        string `json:"source"`
	Status        string `json:"status"`
}

type Role struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type PolicyBinding struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"userId"`
	RoleID    int64  `json:"roleId"`
	RoleCode  string `json:"roleCode"`
	ScopeType string `json:"scopeType"`
	ScopeID   int64  `json:"scopeId"`
}

type MenuPermission struct {
	MenuKey   string   `json:"menuKey"`
	MenuName  string   `json:"menuName"`
	RoleCodes []string `json:"roleCodes"`
}

type BusinessGroup struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	OwnerUserID int64  `json:"ownerUserId"`
	Status      string `json:"status"`
}

type BusinessCenter struct {
	ID              int64  `json:"id"`
	BusinessGroupID int64  `json:"businessGroupId"`
	Name            string `json:"name"`
	OwnerUserID     int64  `json:"ownerUserId"`
	Status          string `json:"status"`
}

type BusinessLine struct {
	ID               int64  `json:"id"`
	BusinessCenterID int64  `json:"businessCenterId"`
	Name             string `json:"name"`
	OwnerUserID      int64  `json:"ownerUserId"`
	Status           string `json:"status"`
}

type System struct {
	ID             int64  `json:"id"`
	BusinessLineID int64  `json:"businessLineId"`
	Name           string `json:"name"`
	OwnerUserID    int64  `json:"ownerUserId"`
	Status         string `json:"status"`
}

type Application struct {
	ID                 int64  `json:"id"`
	SystemID           int64  `json:"systemId"`
	AppID              string `json:"appId"`
	Name               string `json:"name"`
	RepositoryProvider string `json:"repositoryProvider,omitempty"`
	RepositoryFullName string `json:"repositoryFullName,omitempty"`
	RepositoryURL      string `json:"repositoryUrl,omitempty"`
	OwnerUserID        int64  `json:"ownerUserId"`
	LifecycleStatus    string `json:"lifecycleStatus"`
}

type Service struct {
	ID                 int64  `json:"id"`
	ApplicationID      int64  `json:"applicationId"`
	Name               string `json:"name"`
	ServiceType        string `json:"serviceType"`
	RepositoryProvider string `json:"repositoryProvider,omitempty"`
	RepositoryFullName string `json:"repositoryFullName,omitempty"`
	RepositoryURL      string `json:"repositoryUrl,omitempty"`
	RuntimeLanguage    string `json:"runtimeLanguage,omitempty"`
	RuntimeVersion     string `json:"runtimeVersion,omitempty"`
	BuildTool          string `json:"buildTool,omitempty"`
	PipelineTemplateID int64  `json:"pipelineTemplateId,omitempty"`
	OwnerUserID        int64  `json:"ownerUserId"`
	Status             string `json:"status"`
}

type ServiceMember struct {
	ID        int64     `json:"id"`
	ServiceID int64     `json:"serviceId"`
	UserID    int64     `json:"userId"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type Environment struct {
	ID             int64  `json:"id"`
	ServiceID      int64  `json:"serviceId"`
	Name           string `json:"name"`
	ReleaseLevel   string `json:"releaseLevel"`
	K8sClusterID   int64  `json:"k8sClusterId"`
	K8sNamespaceID int64  `json:"k8sNamespaceId"`
	Status         string `json:"status"`
}

type Ticket struct {
	ID              int64          `json:"id"`
	TicketNo        string         `json:"ticketNo"`
	TicketType      string         `json:"ticketType"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	ApplicantUserID int64          `json:"applicantUserId"`
	HandlerUserID   int64          `json:"handlerUserId"`
	Status          string         `json:"status"`
	ScopeType       string         `json:"scopeType"`
	ScopeID         int64          `json:"scopeId"`
	Payload         map[string]any `json:"payload,omitempty"`
}

type Approval struct {
	ID             int64      `json:"id"`
	TicketID       int64      `json:"ticketId"`
	StepNo         int        `json:"stepNo"`
	ApproverUserID int64      `json:"approverUserId"`
	Status         string     `json:"status"`
	Comment        string     `json:"comment"`
	ApprovedAt     *time.Time `json:"approvedAt,omitempty"`
}

type Knowledge struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	SourceTicketID int64  `json:"sourceTicketId"`
	ScopeType      string `json:"scopeType"`
	ScopeID        int64  `json:"scopeId"`
	Status         string `json:"status"`
}

type CloudAccount struct {
	ID              int64          `json:"id"`
	Provider        string         `json:"provider"`
	Name            string         `json:"name"`
	AccountRef      string         `json:"accountRef"`
	CredentialRef   string         `json:"credentialRef,omitempty"`
	AccessKeyID     string         `json:"accessKeyId,omitempty"`
	AccessKeySecret string         `json:"accessKeySecret,omitempty"`
	Regions         []string       `json:"regions,omitempty"`
	ResourceTypes   []string       `json:"resourceTypes,omitempty"`
	Status          string         `json:"status"`
	Raw             map[string]any `json:"raw,omitempty"`
}

type Asset struct {
	ID           int64             `json:"id"`
	Provider     string            `json:"provider"`
	AccountID    int64             `json:"accountId"`
	ResourceType string            `json:"resourceType"`
	ResourceUID  string            `json:"resourceUid"`
	Name         string            `json:"name"`
	Region       string            `json:"region"`
	Source       string            `json:"source"`
	ScopeType    string            `json:"scopeType"`
	ScopeID      int64             `json:"scopeId"`
	Status       string            `json:"status"`
	Tags         map[string]string `json:"tags,omitempty"`
	LastSyncedAt time.Time         `json:"lastSyncedAt,omitempty"`
	Raw          map[string]any    `json:"raw,omitempty"`
}

type CloudResourceType struct {
	ID           int64     `json:"id"`
	Provider     string    `json:"provider"`
	ResourceType string    `json:"resourceType"`
	DisplayName  string    `json:"displayName"`
	Category     string    `json:"category"`
	SyncMode     string    `json:"syncMode"`
	Status       string    `json:"status"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type K8sCluster struct {
	ID        int64  `json:"id"`
	AssetID   int64  `json:"assetId"`
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	APIServer string `json:"apiServer"`
	Status    string `json:"status"`
}

type K8sNamespace struct {
	ID        int64  `json:"id"`
	ClusterID int64  `json:"clusterId"`
	Name      string `json:"name"`
	ScopeType string `json:"scopeType"`
	ScopeID   int64  `json:"scopeId"`
	Status    string `json:"status"`
}

type SyncJob struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Provider      string     `json:"provider"`
	AccountID     int64      `json:"accountId"`
	ResourceTypes []string   `json:"resourceTypes,omitempty"`
	Regions       []string   `json:"regions,omitempty"`
	Status        string     `json:"status"`
	Summary       string     `json:"summary"`
	LastRunAt     *time.Time `json:"lastRunAt,omitempty"`
}

type ServerSession struct {
	ID           int64          `json:"id"`
	ActorUserID  int64          `json:"actorUserId"`
	AssetID      int64          `json:"assetId"`
	AssetName    string         `json:"assetName"`
	Account      string         `json:"account"`
	Protocol     string         `json:"protocol"`
	Status       string         `json:"status"`
	StartedAt    time.Time      `json:"startedAt"`
	LastActiveAt time.Time      `json:"lastActiveAt"`
	Lines        []TerminalLine `json:"lines"`
}

type PodSession struct {
	ID              int64          `json:"id"`
	ActorUserID     int64          `json:"actorUserId"`
	EnvironmentID   int64          `json:"environmentId"`
	EnvironmentName string         `json:"environmentName"`
	ServiceID       int64          `json:"serviceId"`
	ServiceName     string         `json:"serviceName"`
	ClusterName     string         `json:"clusterName"`
	Namespace       string         `json:"namespace"`
	PodName         string         `json:"podName"`
	Account         string         `json:"account"`
	Status          string         `json:"status"`
	StartedAt       time.Time      `json:"startedAt"`
	LastActiveAt    time.Time      `json:"lastActiveAt"`
	Lines           []TerminalLine `json:"lines"`
}

type TerminalLine struct {
	Kind      string    `json:"kind"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type FileSnapshot struct {
	ID           int64     `json:"id"`
	SessionID    int64     `json:"sessionId"`
	AssetID      int64     `json:"assetId"`
	AssetName    string    `json:"assetName"`
	Account      string    `json:"account"`
	Command      string    `json:"command"`
	Path         string    `json:"path"`
	SnapshotType string    `json:"snapshotType"`
	Content      string    `json:"content"`
	Diff         string    `json:"diff,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type PipelineTemplate struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	ServiceType string         `json:"serviceType"`
	Definition  PipelineDef    `json:"definition"`
	Status      string         `json:"status"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type PipelineModule struct {
	ID          int64          `json:"id"`
	Key         string         `json:"key"`
	Name        string         `json:"name"`
	Category    string         `json:"category"`
	Runtime     string         `json:"runtime,omitempty"`
	Description string         `json:"description,omitempty"`
	Command     string         `json:"command"`
	Variables   []string       `json:"variables,omitempty"`
	Status      string         `json:"status"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type Pipeline struct {
	ID         int64       `json:"id"`
	ServiceID  int64       `json:"serviceId"`
	TemplateID int64       `json:"templateId"`
	Name       string      `json:"name"`
	Definition PipelineDef `json:"definition"`
	Status     string      `json:"status"`
}

type PipelineDef struct {
	Steps    []PipelineStep `json:"steps"`
	Triggers []string       `json:"triggers"`
}

type PipelineStep struct {
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	ModuleKey string         `json:"moduleKey,omitempty"`
	With      map[string]any `json:"with,omitempty"`
}

type PipelineRun struct {
	ID            int64      `json:"id"`
	PipelineID    int64      `json:"pipelineId"`
	TriggerUserID int64      `json:"triggerUserId"`
	RunType       string     `json:"runType"`
	Status        string     `json:"status"`
	StartedAt     *time.Time `json:"startedAt,omitempty"`
	FinishedAt    *time.Time `json:"finishedAt,omitempty"`
}

type PipelineLog struct {
	ID         int64  `json:"id"`
	RunID      int64  `json:"runId"`
	StepName   string `json:"stepName"`
	LogContent string `json:"logContent"`
}

type Release struct {
	ID            int64  `json:"id"`
	ReleaseNo     string `json:"releaseNo"`
	ServiceID     int64  `json:"serviceId"`
	EnvironmentID int64  `json:"environmentId"`
	TicketID      int64  `json:"ticketId"`
	PipelineRunID int64  `json:"pipelineRunId"`
	Version       string `json:"version"`
	Strategy      string `json:"strategy"`
	Status        string `json:"status"`
}

type HealthCheck struct {
	ID        int64          `json:"id"`
	ReleaseID int64          `json:"releaseId"`
	CheckType string         `json:"checkType"`
	Status    string         `json:"status"`
	Result    map[string]any `json:"result,omitempty"`
}

type AuditLog struct {
	ID           int64     `json:"id"`
	ActorUserID  int64     `json:"actorUserId"`
	Action       string    `json:"action"`
	ResourceType string    `json:"resourceType"`
	ResourceID   int64     `json:"resourceId"`
	Result       string    `json:"result"`
	Reason       string    `json:"reason"`
	CreatedAt    time.Time `json:"createdAt"`
}

type AuditLogView struct {
	ID               int64     `json:"id"`
	ActorUserID      int64     `json:"actorUserId"`
	ActorUsername    string    `json:"actorUsername"`
	ActorDisplayName string    `json:"actorDisplayName"`
	Action           string    `json:"action"`
	ResourceType     string    `json:"resourceType"`
	ResourceID       int64     `json:"resourceId"`
	Result           string    `json:"result"`
	Reason           string    `json:"reason"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Notification struct {
	ID             int64  `json:"id"`
	ReceiverUserID int64  `json:"receiverUserId"`
	Channel        string `json:"channel"`
	Title          string `json:"title"`
	Content        string `json:"content"`
	Status         string `json:"status"`
}

type PlatformSettings struct {
	PlatformName string           `json:"platformName"`
	LogoText     string           `json:"logoText"`
	LogoURL      string           `json:"logoUrl"`
	FaviconURL   string           `json:"faviconUrl"`
	ThemeColor   string           `json:"themeColor"`
	LDAPAuth     LDAPAuthSettings `json:"ldapAuth"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type LDAPAuthSettings struct {
	Enabled              bool   `json:"enabled"`
	URL                  string `json:"url"`
	StartTLS             bool   `json:"startTls"`
	BaseDN               string `json:"baseDn"`
	BindDN               string `json:"bindDn"`
	BindPassword         string `json:"bindPassword,omitempty"`
	UserFilter           string `json:"userFilter"`
	UserAttribute        string `json:"userAttribute"`
	DisplayNameAttribute string `json:"displayNameAttribute"`
	EmailAttribute       string `json:"emailAttribute"`
	DefaultRoleCode      string `json:"defaultRoleCode"`
	AdminUsername        string `json:"adminUsername"`
}

type DingTalkOrgConfig struct {
	CorpID     string    `json:"corpId"`
	AppKey     string    `json:"appKey"`
	AppSecret  string    `json:"appSecret,omitempty"`
	AgentID    string    `json:"agentId"`
	RootDeptID string    `json:"rootDeptId"`
	SyncMode   string    `json:"syncMode"`
	Status     string    `json:"status"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type GitLabGroupMapping struct {
	ID              int64     `json:"id"`
	DepartmentID    int64     `json:"departmentId"`
	GitLabGroupPath string    `json:"gitlabGroupPath"`
	AccessLevel     string    `json:"accessLevel"`
	SyncMode        string    `json:"syncMode"`
	Status          string    `json:"status"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
