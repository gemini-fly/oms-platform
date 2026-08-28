package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultStoreSnapshotID = "default"

type DatabaseStatus struct {
	Enabled       bool       `json:"enabled"`
	Provider      string     `json:"provider"`
	Status        string     `json:"status"`
	DSN           string     `json:"dsn"`
	SnapshotID    string     `json:"snapshotId"`
	TableName     string     `json:"tableName"`
	SnapshotBytes int64      `json:"snapshotBytes"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
	Error         string     `json:"error,omitempty"`
}

type storeSnapshot struct {
	Version   int              `json:"version"`
	UpdatedAt time.Time        `json:"updatedAt"`
	Next      map[string]int64 `json:"next"`

	Settings       PlatformSettings  `json:"settings"`
	DingTalkConfig DingTalkOrgConfig `json:"dingtalkConfig"`

	Users           []User               `json:"users"`
	Departments     []Department         `json:"departments"`
	Roles           []Role               `json:"roles"`
	PolicyBindings  []PolicyBinding      `json:"policyBindings"`
	MenuPermissions []MenuPermission     `json:"menuPermissions"`
	GitLabMappings  []GitLabGroupMapping `json:"gitlabMappings"`

	BusinessGroups  []BusinessGroup  `json:"businessGroups"`
	BusinessCenters []BusinessCenter `json:"businessCenters"`
	BusinessLines   []BusinessLine   `json:"businessLines"`
	Systems         []System         `json:"systems"`
	Applications    []Application    `json:"applications"`
	Services        []Service        `json:"services"`
	ServiceMembers  []ServiceMember  `json:"serviceMembers"`
	Environments    []Environment    `json:"environments"`

	Tickets   []Ticket    `json:"tickets"`
	Approvals []Approval  `json:"approvals"`
	Knowledge []Knowledge `json:"knowledge"`

	CloudAccounts      []CloudAccount      `json:"cloudAccounts"`
	CloudResourceTypes []CloudResourceType `json:"cloudResourceTypes"`
	Assets             []Asset             `json:"assets"`
	K8sClusters        []K8sCluster        `json:"k8sClusters"`
	K8sNamespaces      []K8sNamespace      `json:"k8sNamespaces"`
	SyncJobs           []SyncJob           `json:"syncJobs"`
	ServerSessions     []ServerSession     `json:"serverSessions"`
	PodSessions        []PodSession        `json:"podSessions"`
	FileSnapshots      []FileSnapshot      `json:"fileSnapshots"`

	PipelineTemplates []PipelineTemplate `json:"pipelineTemplates"`
	PipelineModules   []PipelineModule   `json:"pipelineModules"`
	Pipelines         []Pipeline         `json:"pipelines"`
	PipelineRuns      []PipelineRun      `json:"pipelineRuns"`
	PipelineLogs      []PipelineLog      `json:"pipelineLogs"`

	Releases     []Release     `json:"releases"`
	HealthChecks []HealthCheck `json:"healthChecks"`

	AuditLogs     []AuditLog     `json:"auditLogs"`
	Notifications []Notification `json:"notifications"`
}

func DBDSNFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("SY_PLATFORM_DB_DSN")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("DATABASE_URL"))
}

func (s *Store) DatabaseStatus(ctx context.Context) DatabaseStatus {
	dsn := DBDSNFromEnv()
	status := DatabaseStatus{
		Enabled:    false,
		Provider:   "memory",
		Status:     "memory_only",
		DSN:        redactDatabaseDSN(dsn),
		SnapshotID: defaultStoreSnapshotID,
		TableName:  "platform_store_snapshots",
	}

	s.Lock()
	db := s.db
	snapshotID := s.dbSnapshotID
	s.Unlock()

	if db == nil {
		return status
	}
	if snapshotID == "" {
		snapshotID = defaultStoreSnapshotID
	}
	status.Enabled = true
	status.Provider = "postgres"
	status.Status = "connected"
	status.SnapshotID = snapshotID

	var updatedAt time.Time
	var snapshotBytes int64
	err := db.QueryRowContext(ctx, `
SELECT updated_at, pg_column_size(payload)::bigint
FROM platform_store_snapshots
WHERE id = $1`, snapshotID).Scan(&updatedAt, &snapshotBytes)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	status.UpdatedAt = &updatedAt
	status.SnapshotBytes = snapshotBytes
	return status
}

func redactDatabaseDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.User == nil {
		return dsn
	}
	username := parsed.User.Username()
	if _, ok := parsed.User.Password(); ok {
		parsed.User = url.UserPassword(username, "******")
	} else {
		parsed.User = url.User(username)
	}
	return strings.ReplaceAll(parsed.String(), "%2A%2A%2A%2A%2A%2A", "******")
}

func (s *Store) ConnectPostgres(ctx context.Context, dsn string) error {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("ping postgres: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS platform_store_snapshots (
	id TEXT PRIMARY KEY,
	payload JSONB NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		_ = db.Close()
		return fmt.Errorf("migrate platform store snapshot table: %w", err)
	}

	var payload []byte
	err = db.QueryRowContext(ctx, `SELECT payload FROM platform_store_snapshots WHERE id = $1`, defaultStoreSnapshotID).Scan(&payload)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		_ = db.Close()
		return fmt.Errorf("load platform store snapshot: %w", err)
	}

	s.Lock()
	defer s.Unlock()
	s.db = db
	s.dbSnapshotID = defaultStoreSnapshotID

	if errors.Is(err, sql.ErrNoRows) {
		return s.persistSnapshotLocked(ctx)
	}

	var snapshot storeSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return fmt.Errorf("decode platform store snapshot: %w", err)
	}
	if s.applySnapshotLocked(snapshot) {
		return s.persistSnapshotLocked(ctx)
	}
	return nil
}

func (s *Store) Close() error {
	s.Lock()
	db := s.db
	s.db = nil
	s.Unlock()
	if db == nil {
		return nil
	}
	return db.Close()
}

func (s *Store) PersistSnapshot() error {
	s.Lock()
	defer s.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.persistSnapshotLocked(ctx)
}

func (s *Store) persistSnapshotLocked(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	snapshot := s.snapshotLocked()
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode platform store snapshot: %w", err)
	}
	id := s.dbSnapshotID
	if id == "" {
		id = defaultStoreSnapshotID
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO platform_store_snapshots (id, payload, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (id) DO UPDATE
SET payload = EXCLUDED.payload,
	updated_at = now()`, id, payload)
	if err != nil {
		return fmt.Errorf("persist platform store snapshot: %w", err)
	}
	return nil
}

func (s *Store) snapshotLocked() storeSnapshot {
	next := make(map[string]int64, len(s.next))
	for key, value := range s.next {
		next[key] = value
	}
	return storeSnapshot{
		Version:   1,
		UpdatedAt: time.Now(),
		Next:      next,

		Settings:       s.Settings,
		DingTalkConfig: s.DingTalkConfig,

		Users:           s.Users,
		Departments:     s.Departments,
		Roles:           s.Roles,
		PolicyBindings:  s.PolicyBindings,
		MenuPermissions: s.MenuPermissions,
		GitLabMappings:  s.GitLabMappings,

		BusinessGroups:  s.BusinessGroups,
		BusinessCenters: s.BusinessCenters,
		BusinessLines:   s.BusinessLines,
		Systems:         s.Systems,
		Applications:    s.Applications,
		Services:        s.Services,
		ServiceMembers:  s.ServiceMembers,
		Environments:    s.Environments,

		Tickets:   s.Tickets,
		Approvals: s.Approvals,
		Knowledge: s.Knowledge,

		CloudAccounts:      s.CloudAccounts,
		CloudResourceTypes: s.CloudResourceTypes,
		Assets:             s.Assets,
		K8sClusters:        s.K8sClusters,
		K8sNamespaces:      s.K8sNamespaces,
		SyncJobs:           s.SyncJobs,
		ServerSessions:     s.ServerSessions,
		PodSessions:        s.PodSessions,
		FileSnapshots:      s.FileSnapshots,

		PipelineTemplates: s.PipelineTemplates,
		PipelineModules:   s.PipelineModules,
		Pipelines:         s.Pipelines,
		PipelineRuns:      s.PipelineRuns,
		PipelineLogs:      s.PipelineLogs,

		Releases:     s.Releases,
		HealthChecks: s.HealthChecks,

		AuditLogs:     s.AuditLogs,
		Notifications: s.Notifications,
	}
}

func (s *Store) applySnapshotLocked(snapshot storeSnapshot) bool {
	dirty := false
	if snapshot.Next == nil {
		snapshot.Next = map[string]int64{}
	}
	if snapshot.Settings.PlatformName == "" {
		snapshot.Settings = DefaultPlatformSettings()
	} else if normalized, err := NormalizePlatformSettings(snapshot.Settings); err == nil {
		if !snapshot.Settings.UpdatedAt.IsZero() {
			normalized.UpdatedAt = snapshot.Settings.UpdatedAt
		}
		snapshot.Settings = normalized
	}
	if snapshot.DingTalkConfig.RootDeptID == "" {
		snapshot.DingTalkConfig = DefaultDingTalkOrgConfig()
	} else {
		snapshot.DingTalkConfig = NormalizeDingTalkConfig(snapshot.DingTalkConfig)
	}
	if snapshot.MenuPermissions == nil {
		snapshot.MenuPermissions = DefaultMenuPermissions()
	}

	s.next = snapshot.Next
	s.Settings = snapshot.Settings
	s.DingTalkConfig = snapshot.DingTalkConfig

	s.Users = snapshot.Users
	s.Departments = snapshot.Departments
	s.Roles = snapshot.Roles
	s.PolicyBindings = snapshot.PolicyBindings
	s.MenuPermissions = snapshot.MenuPermissions
	s.GitLabMappings = snapshot.GitLabMappings

	s.BusinessGroups = snapshot.BusinessGroups
	s.BusinessCenters = snapshot.BusinessCenters
	s.BusinessLines = snapshot.BusinessLines
	s.Systems = snapshot.Systems
	s.Applications = snapshot.Applications
	s.Services = snapshot.Services
	s.ServiceMembers = snapshot.ServiceMembers
	s.Environments = snapshot.Environments

	s.Tickets = snapshot.Tickets
	s.Approvals = snapshot.Approvals
	s.Knowledge = snapshot.Knowledge

	s.CloudAccounts = snapshot.CloudAccounts
	s.CloudResourceTypes = snapshot.CloudResourceTypes
	s.Assets = snapshot.Assets
	s.K8sClusters = snapshot.K8sClusters
	s.K8sNamespaces = snapshot.K8sNamespaces
	s.SyncJobs = snapshot.SyncJobs
	s.ServerSessions = snapshot.ServerSessions
	s.PodSessions = snapshot.PodSessions
	s.FileSnapshots = snapshot.FileSnapshots

	s.PipelineTemplates = snapshot.PipelineTemplates
	s.PipelineModules = snapshot.PipelineModules
	s.Pipelines = snapshot.Pipelines
	s.PipelineRuns = snapshot.PipelineRuns
	s.PipelineLogs = snapshot.PipelineLogs

	s.Releases = snapshot.Releases
	s.HealthChecks = snapshot.HealthChecks

	s.AuditLogs = snapshot.AuditLogs
	s.Notifications = snapshot.Notifications
	s.ensureNextCountersLocked()
	if s.ensureApplicationRepositoryIdentityLocked() {
		dirty = true
	}
	s.ensureServiceOwnerMembersLocked()
	s.ensureDemoEnvironmentTargetsLocked()
	if s.ensureBuiltinPipelineModulesLocked() {
		dirty = true
	}
	if s.ensureBuiltinPipelineTemplatesLocked() {
		dirty = true
	}
	if s.ensureBuiltinCloudResourceTypesLocked() {
		dirty = true
	}
	s.ensureNextCountersLocked()
	return dirty
}

func (s *Store) ensureNextCountersLocked() {
	if s.next == nil {
		s.next = map[string]int64{}
	}
	ensureNextFrom(s, "user", s.Users, func(item User) int64 { return item.ID })
	ensureNextFrom(s, "department", s.Departments, func(item Department) int64 { return item.ID })
	ensureNextFrom(s, "role", s.Roles, func(item Role) int64 { return item.ID })
	ensureNextFrom(s, "binding", s.PolicyBindings, func(item PolicyBinding) int64 { return item.ID })
	ensureNextFrom(s, "gitlab_mapping", s.GitLabMappings, func(item GitLabGroupMapping) int64 { return item.ID })
	ensureNextFrom(s, "business_group", s.BusinessGroups, func(item BusinessGroup) int64 { return item.ID })
	ensureNextFrom(s, "business_center", s.BusinessCenters, func(item BusinessCenter) int64 { return item.ID })
	ensureNextFrom(s, "business_line", s.BusinessLines, func(item BusinessLine) int64 { return item.ID })
	ensureNextFrom(s, "system", s.Systems, func(item System) int64 { return item.ID })
	ensureNextFrom(s, "application", s.Applications, func(item Application) int64 { return item.ID })
	ensureNextFrom(s, "service", s.Services, func(item Service) int64 { return item.ID })
	ensureNextFrom(s, "service_member", s.ServiceMembers, func(item ServiceMember) int64 { return item.ID })
	ensureNextFrom(s, "environment", s.Environments, func(item Environment) int64 { return item.ID })
	ensureNextFrom(s, "ticket", s.Tickets, func(item Ticket) int64 { return item.ID })
	ensureNextFrom(s, "approval", s.Approvals, func(item Approval) int64 { return item.ID })
	ensureNextFrom(s, "knowledge", s.Knowledge, func(item Knowledge) int64 { return item.ID })
	ensureNextFrom(s, "cloud_account", s.CloudAccounts, func(item CloudAccount) int64 { return item.ID })
	ensureNextFrom(s, "cloud_resource_type", s.CloudResourceTypes, func(item CloudResourceType) int64 { return item.ID })
	ensureNextFrom(s, "asset", s.Assets, func(item Asset) int64 { return item.ID })
	ensureNextFrom(s, "k8s_cluster", s.K8sClusters, func(item K8sCluster) int64 { return item.ID })
	ensureNextFrom(s, "k8s_namespace", s.K8sNamespaces, func(item K8sNamespace) int64 { return item.ID })
	ensureNextFrom(s, "sync_job", s.SyncJobs, func(item SyncJob) int64 { return item.ID })
	ensureNextFrom(s, "server_session", s.ServerSessions, func(item ServerSession) int64 { return item.ID })
	ensureNextFrom(s, "pod_session", s.PodSessions, func(item PodSession) int64 { return item.ID })
	ensureNextFrom(s, "file_snapshot", s.FileSnapshots, func(item FileSnapshot) int64 { return item.ID })
	ensureNextFrom(s, "pipeline_template", s.PipelineTemplates, func(item PipelineTemplate) int64 { return item.ID })
	ensureNextFrom(s, "pipeline_module", s.PipelineModules, func(item PipelineModule) int64 { return item.ID })
	ensureNextFrom(s, "pipeline", s.Pipelines, func(item Pipeline) int64 { return item.ID })
	ensureNextFrom(s, "pipeline_run", s.PipelineRuns, func(item PipelineRun) int64 { return item.ID })
	ensureNextFrom(s, "pipeline_log", s.PipelineLogs, func(item PipelineLog) int64 { return item.ID })
	ensureNextFrom(s, "release", s.Releases, func(item Release) int64 { return item.ID })
	ensureNextFrom(s, "health_check", s.HealthChecks, func(item HealthCheck) int64 { return item.ID })
	ensureNextFrom(s, "audit", s.AuditLogs, func(item AuditLog) int64 { return item.ID })
	ensureNextFrom(s, "notification", s.Notifications, func(item Notification) int64 { return item.ID })
}

func ensureNextFrom[T any](s *Store, kind string, items []T, idOf func(T) int64) {
	for _, item := range items {
		s.ensureNextAtLeast(kind, idOf(item))
	}
}

func (s *Store) ensureNextAtLeast(kind string, id int64) {
	if id > s.next[kind] {
		s.next[kind] = id
	}
}
