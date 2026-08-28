package cmdb

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

type cloudAccountView struct {
	ID               int64          `json:"id"`
	Provider         string         `json:"provider"`
	Name             string         `json:"name"`
	AccountRef       string         `json:"accountRef"`
	CredentialRef    string         `json:"credentialRef,omitempty"`
	AccessKeyID      string         `json:"accessKeyId,omitempty"`
	SecretConfigured bool           `json:"secretConfigured"`
	CredentialMode   string         `json:"credentialMode"`
	Regions          []string       `json:"regions,omitempty"`
	ResourceTypes    []string       `json:"resourceTypes,omitempty"`
	Status           string         `json:"status"`
	Raw              map[string]any `json:"raw,omitempty"`
}

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/cmdb/cloud-accounts", cloudAccounts(s))
	s.Mux.HandleFunc("/api/v1/cmdb/cloud-accounts/", cloudAccountAction(s))
	s.Mux.HandleFunc("/api/v1/cmdb/resource-types", cloudResourceTypes(s))
	s.Mux.HandleFunc("/api/v1/cmdb/assets", assets(s))
	s.Mux.HandleFunc("/api/v1/cmdb/assets/import", importAssets(s))
	s.Mux.HandleFunc("/api/v1/cmdb/server-assets", serverAssets(s))
	s.Mux.HandleFunc("/api/v1/cmdb/server-assets/", serverAssetAction(s))
	s.Mux.HandleFunc("/api/v1/cmdb/server-sessions/", serverSessionAction(s))
	s.Mux.HandleFunc("/api/v1/cmdb/file-snapshots", fileSnapshots(s))
	s.Mux.HandleFunc("/api/v1/cmdb/k8s/clusters", k8sClusters(s))
	s.Mux.HandleFunc("/api/v1/cmdb/k8s/namespaces", k8sNamespaces(s))
	s.Mux.HandleFunc("/api/v1/cmdb/sync-jobs", syncJobs(s))
	s.Mux.HandleFunc("/api/v1/cmdb/sync-jobs/", syncJobByID(s))
}

func cloudAccounts(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, cloudAccountViews(s.Store.CloudAccounts))
			return
		}
		var item platform.CloudAccount
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		normalized, err := normalizeCloudAccount(item)
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item = normalized
		item.ID = s.Store.Next("cloud_account")
		s.Store.CloudAccounts = append(s.Store.CloudAccounts, item)
		s.Store.Audit(actorID, "cmdb.cloud_account.create", "cloud_account", item.ID, "success", item.Provider)
		platform.JSON(w, http.StatusCreated, cloudAccountViewOf(item))
	}
}

func cloudAccountAction(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/sync") {
			platform.Error(w, http.StatusNotFound, "NOT_FOUND", "route not found")
			return
		}
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/cmdb/cloud-accounts/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		var account platform.CloudAccount
		for _, item := range s.Store.CloudAccounts {
			if item.ID == id {
				account = item
				break
			}
		}
		if account.ID == 0 {
			platform.Error(w, http.StatusNotFound, "CLOUD_ACCOUNT_NOT_FOUND", "cloud account not found")
			return
		}
		job := platform.SyncJob{
			ID:            s.Store.Next("sync_job"),
			Name:          fmt.Sprintf("同步%s", account.Name),
			Provider:      platform.NormalizeCloudProvider(account.Provider),
			AccountID:     account.ID,
			ResourceTypes: normalizeResourceTypes(account.Provider, account.ResourceTypes),
			Regions:       normalizeCloudRegions(account.Provider, account.Regions),
			Status:        "created",
		}
		s.Store.SyncJobs = append(s.Store.SyncJobs, job)
		job, err = runCloudSyncLocked(s.Store, job.ID, actorID)
		if err != nil {
			s.Store.Audit(actorID, "cmdb.sync_job.run", "sync_job", job.ID, "failed", err.Error())
			platform.Error(w, http.StatusBadRequest, "SYNC_JOB_FAILED", err.Error())
			return
		}
		platform.JSON(w, http.StatusCreated, job)
	}
}

func cloudResourceTypes(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		if !canUseCMDB(s.Store, platform.ActorID(r, s.Store)) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		platform.JSON(w, http.StatusOK, s.Store.CloudResourceTypes)
	}
}

func assets(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, platform.Page[platform.Asset]{Items: s.Store.Assets, Page: 1, PageSize: len(s.Store.Assets), Total: int64(len(s.Store.Assets))})
			return
		}
		var item platform.Asset
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item.ID = s.Store.Next("asset")
		item.Source = fallback(item.Source, "manual")
		item.Status = fallback(item.Status, "active")
		s.Store.Assets = append(s.Store.Assets, item)
		s.Store.Audit(actorID, "cmdb.asset.create", "asset", item.ID, "success", item.Name)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func importAssets(s *platform.Server) http.HandlerFunc {
	type request struct {
		Items []platform.Asset `json:"items"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var req request
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		for i := range req.Items {
			req.Items[i].ID = s.Store.Next("asset")
			req.Items[i].Source = fallback(req.Items[i].Source, "import")
			req.Items[i].Status = fallback(req.Items[i].Status, "active")
			s.Store.Assets = append(s.Store.Assets, req.Items[i])
		}
		s.Store.Audit(actorID, "cmdb.asset.import", "asset", 0, "success", fmt.Sprintf("imported:%d", len(req.Items)))
		platform.JSON(w, http.StatusCreated, map[string]int{"imported": len(req.Items)})
	}
}

func serverAssets(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		if !canUseCMDB(s.Store, platform.ActorID(r, s.Store)) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		items := make([]platform.Asset, 0)
		for _, item := range s.Store.Assets {
			if isServerAsset(item) {
				items = append(items, item)
			}
		}
		platform.JSON(w, http.StatusOK, platform.Page[platform.Asset]{Items: items, Page: 1, PageSize: len(items), Total: int64(len(items))})
	}
}

func serverAssetAction(s *platform.Server) http.HandlerFunc {
	type request struct {
		Account  string `json:"account"`
		Protocol string `json:"protocol"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/sessions") {
			platform.Error(w, http.StatusNotFound, "NOT_FOUND", "route not found")
			return
		}
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/cmdb/server-assets/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		var req request
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		req.Account = normalizeServerAccount(req.Account)
		req.Protocol = normalizeServerProtocol(req.Protocol)

		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		asset, ok := findAsset(s.Store.Assets, id)
		if !ok || !isServerAsset(asset) {
			platform.Error(w, http.StatusNotFound, "SERVER_ASSET_NOT_FOUND", "server asset not found")
			return
		}
		now := time.Now()
		session := platform.ServerSession{
			ID:           s.Store.Next("server_session"),
			ActorUserID:  actorID,
			AssetID:      asset.ID,
			AssetName:    asset.Name,
			Account:      req.Account,
			Protocol:     req.Protocol,
			Status:       "connected",
			StartedAt:    now,
			LastActiveAt: now,
			Lines: []platform.TerminalLine{
				terminalLine("system", fmt.Sprintf("connected to %s by %s as %s", asset.Name, req.Protocol, req.Account)),
				terminalLine("output", fmt.Sprintf("Last login: %s from sy-platform", now.Format("2006-01-02 15:04:05"))),
			},
		}
		s.Store.ServerSessions = append(s.Store.ServerSessions, session)
		s.Store.Audit(actorID, "cmdb.server.login", "asset", asset.ID, "success", fmt.Sprintf("%s %s as %s", asset.Name, req.Protocol, req.Account))
		platform.JSON(w, http.StatusCreated, session)
	}
}

func serverSessionAction(s *platform.Server) http.HandlerFunc {
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
		id, err := platform.PathID(r.URL.Path, "/api/v1/cmdb/server-sessions/")
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
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		session := findSession(s.Store.ServerSessions, id)
		if session == nil {
			platform.Error(w, http.StatusNotFound, "SESSION_NOT_FOUND", "server session not found")
			return
		}
		if session.Status == "closed" {
			platform.Error(w, http.StatusConflict, "SESSION_CLOSED", "server session is closed")
			return
		}
		if !canControlServerSession(s.Store, actorID, *session) {
			s.Store.Audit(actorID, "cmdb.server.command", "server_session", session.ID, "denied", "server session owner or platform admin required")
			platform.Error(w, http.StatusForbidden, "SERVER_SESSION_FORBIDDEN", "current user cannot control this server session")
			return
		}
		now := time.Now()
		session.Lines = append(session.Lines, platform.TerminalLine{Kind: "input", Content: command, CreatedAt: now})
		snapshots := captureFileSnapshots(s.Store, *session, command, now)
		output := simulateCommand(*session, command)
		if len(snapshots) > 0 {
			output = append(output, platform.TerminalLine{Kind: "system", Content: "已生成文件快照：" + snapshotSummary(snapshots), CreatedAt: now})
		}
		for _, line := range output {
			session.Lines = append(session.Lines, line)
		}
		session.LastActiveAt = now
		if command == "exit" || command == "logout" {
			session.Status = "closed"
		}
		reason := fmt.Sprintf("%s %s$ %s", session.AssetName, session.Account, command)
		if len(snapshots) > 0 {
			reason += "; " + snapshotSummary(snapshots)
		}
		s.Store.Audit(actorID, "cmdb.server.command", "server_session", session.ID, "success", reason)
		platform.JSON(w, http.StatusOK, map[string]any{"session": session, "output": output, "snapshots": snapshots})
	}
}

func fileSnapshots(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		if !canUseCMDB(s.Store, platform.ActorID(r, s.Store)) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		items := make([]platform.FileSnapshot, 0, len(s.Store.FileSnapshots))
		for i := len(s.Store.FileSnapshots) - 1; i >= 0; i-- {
			items = append(items, s.Store.FileSnapshots[i])
		}
		platform.JSON(w, http.StatusOK, platform.Page[platform.FileSnapshot]{
			Items:    items,
			Page:     1,
			PageSize: len(items),
			Total:    int64(len(items)),
		})
	}
}

func k8sClusters(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, s.Store.K8sClusters)
			return
		}
		var item platform.K8sCluster
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item.ID = s.Store.Next("k8s_cluster")
		item.Status = fallback(item.Status, "enabled")
		s.Store.K8sClusters = append(s.Store.K8sClusters, item)
		s.Store.Audit(actorID, "cmdb.k8s_cluster.create", "k8s_cluster", item.ID, "success", item.Name)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func k8sNamespaces(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, s.Store.K8sNamespaces)
			return
		}
		var item platform.K8sNamespace
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item.ID = s.Store.Next("k8s_namespace")
		item.Status = fallback(item.Status, "enabled")
		s.Store.K8sNamespaces = append(s.Store.K8sNamespaces, item)
		s.Store.Audit(actorID, "cmdb.k8s_namespace.create", "k8s_namespace", item.ID, "success", item.Name)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func syncJobs(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, s.Store.SyncJobs)
			return
		}
		var item platform.SyncJob
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item = normalizeSyncJob(item)
		if item.Provider == "" && item.AccountID == 0 {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "provider or accountId is required")
			return
		}
		if item.Provider != "" && !platform.SupportedCloudProvider(item.Provider) {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "unsupported cloud provider: "+item.Provider)
			return
		}
		item.ID = s.Store.Next("sync_job")
		item.Status = fallback(item.Status, "created")
		if item.Name == "" {
			if item.AccountID == 0 {
				item.Name = platform.CloudProviderDisplayName(item.Provider) + "全账号同步"
			} else {
				item.Name = fmt.Sprintf("云账号 #%d 同步", item.AccountID)
			}
		}
		s.Store.SyncJobs = append(s.Store.SyncJobs, item)
		s.Store.Audit(actorID, "cmdb.sync_job.create", "sync_job", item.ID, "success", item.Name)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func syncJobByID(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isRun := strings.HasSuffix(strings.TrimRight(r.URL.Path, "/"), "/run")
		if isRun {
			if !platform.Method(w, r, http.MethodPost) {
				return
			}
		} else if !platform.Method(w, r, http.MethodGet) {
			return
		}
		id, err := platform.PathID(r.URL.Path, "/api/v1/cmdb/sync-jobs/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if !canUseCMDB(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CMDB_FORBIDDEN", "ops or platform admin role required")
			return
		}
		if isRun {
			item, err := runCloudSyncLocked(s.Store, id, actorID)
			if err != nil {
				s.Store.Audit(actorID, "cmdb.sync_job.run", "sync_job", id, "failed", err.Error())
				platform.Error(w, http.StatusBadRequest, "SYNC_JOB_FAILED", err.Error())
				return
			}
			platform.JSON(w, http.StatusOK, item)
			return
		}
		for _, item := range s.Store.SyncJobs {
			if item.ID == id {
				platform.JSON(w, http.StatusOK, item)
				return
			}
		}
		platform.Error(w, http.StatusNotFound, "SYNC_JOB_NOT_FOUND", "sync job not found")
	}
}

func canUseCMDB(store *platform.Store, actorID int64) bool {
	return store.HasAnyRole(actorID, "platform_admin", "ops_owner")
}

func canControlServerSession(store *platform.Store, actorID int64, session platform.ServerSession) bool {
	return store.HasAnyRole(actorID, "platform_admin") || session.ActorUserID == actorID
}

func normalizeServerAccount(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ops":
		return "ops"
	case "readonly", "read_only", "read-only":
		return "readonly"
	default:
		return "ops"
	}
}

func normalizeServerProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ssh":
		return "ssh"
	default:
		return "ssh"
	}
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func isServerAsset(item platform.Asset) bool {
	switch strings.ToLower(item.ResourceType) {
	case "server", "ecs", "ecs.instance", "ecs_instance", "ecs.cloudservers", "cvm", "cvm.instance", "vm", "host", "compute_instance":
		return true
	default:
		return false
	}
}

func findAsset(items []platform.Asset, id int64) (platform.Asset, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return platform.Asset{}, false
}

func findSession(items []platform.ServerSession, id int64) *platform.ServerSession {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func terminalLine(kind, content string) platform.TerminalLine {
	return platform.TerminalLine{Kind: kind, Content: content, CreatedAt: time.Now()}
}

func captureFileSnapshots(store *platform.Store, session platform.ServerSession, command string, now time.Time) []platform.FileSnapshot {
	path, mode := fileSnapshotPlan(command)
	if path == "" {
		return nil
	}
	switch mode {
	case "edit":
		before := simulatedFileContent(path, "before")
		after := simulatedFileContent(path, "after")
		snapshots := []platform.FileSnapshot{
			newFileSnapshot(store, session, command, path, "before", before, "", now),
			newFileSnapshot(store, session, command, path, "after", after, simpleDiff(before, after), now),
		}
		store.FileSnapshots = append(store.FileSnapshots, snapshots...)
		return snapshots
	case "delete":
		content := simulatedDeletedContent(path)
		snapshot := newFileSnapshot(store, session, command, path, "deleted", content, "", now)
		store.FileSnapshots = append(store.FileSnapshots, snapshot)
		return []platform.FileSnapshot{snapshot}
	default:
		return nil
	}
}

func newFileSnapshot(store *platform.Store, session platform.ServerSession, command, path, snapshotType, content, diff string, now time.Time) platform.FileSnapshot {
	return platform.FileSnapshot{
		ID:           store.Next("file_snapshot"),
		SessionID:    session.ID,
		AssetID:      session.AssetID,
		AssetName:    session.AssetName,
		Account:      session.Account,
		Command:      command,
		Path:         path,
		SnapshotType: snapshotType,
		Content:      content,
		Diff:         diff,
		CreatedAt:    now,
	}
}

func fileSnapshotPlan(command string) (string, string) {
	fields := strings.Fields(command)
	fields = stripSudo(fields)
	if len(fields) == 0 {
		return "", ""
	}
	bin := fields[0]
	switch bin {
	case "vim", "vi", "nano":
		if len(fields) > 1 {
			return fields[1], "edit"
		}
	case "sed":
		if strings.Contains(command, "-i") && len(fields) > 1 {
			return fields[len(fields)-1], "edit"
		}
	case "tee":
		if len(fields) > 1 {
			return fields[len(fields)-1], "edit"
		}
	case "truncate":
		if len(fields) > 1 {
			return fields[len(fields)-1], "edit"
		}
	case "rm":
		if len(fields) > 1 {
			return fields[len(fields)-1], "delete"
		}
	case "mv":
		if len(fields) > 2 {
			return fields[1], "delete"
		}
	}
	if strings.Contains(command, ">") {
		parts := strings.Split(command, ">")
		target := strings.TrimSpace(parts[len(parts)-1])
		if target == "" {
			return "", ""
		}
		targetFields := strings.Fields(target)
		if len(targetFields) == 0 {
			return "", ""
		}
		return targetFields[0], "edit"
	}
	return "", ""
}

func stripSudo(fields []string) []string {
	if len(fields) == 0 || fields[0] != "sudo" {
		return fields
	}
	fields = fields[1:]
	for len(fields) > 0 && strings.HasPrefix(fields[0], "-") {
		if (fields[0] == "-u" || fields[0] == "-g" || fields[0] == "-h") && len(fields) > 1 {
			fields = fields[2:]
			continue
		}
		fields = fields[1:]
	}
	return fields
}

func snapshotSummary(snapshots []platform.FileSnapshot) string {
	parts := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		parts = append(parts, fmt.Sprintf("%s#%d %s", snapshot.SnapshotType, snapshot.ID, snapshot.Path))
	}
	return "snapshots: " + strings.Join(parts, ", ")
}

func simulatedFileContent(path, state string) string {
	if state == "after" {
		return fmt.Sprintf("# %s\nmanaged_by=sy-platform\nversion=after\nlast_change=terminal-command\n", path)
	}
	return fmt.Sprintf("# %s\nmanaged_by=unknown\nversion=before\nlast_change=manual\n", path)
}

func simulatedDeletedContent(path string) string {
	return fmt.Sprintf("# archived before deletion: %s\napps/\ndeploy/\nlogs/\ntmp/\n", path)
}

func simpleDiff(before, after string) string {
	beforeLines := strings.Split(strings.TrimSuffix(before, "\n"), "\n")
	afterLines := strings.Split(strings.TrimSuffix(after, "\n"), "\n")
	var lines []string
	lines = append(lines, "--- before", "+++ after")
	max := len(beforeLines)
	if len(afterLines) > max {
		max = len(afterLines)
	}
	for i := 0; i < max; i++ {
		var beforeLine, afterLine string
		if i < len(beforeLines) {
			beforeLine = beforeLines[i]
		}
		if i < len(afterLines) {
			afterLine = afterLines[i]
		}
		if beforeLine == afterLine {
			lines = append(lines, " "+beforeLine)
			continue
		}
		if beforeLine != "" {
			lines = append(lines, "-"+beforeLine)
		}
		if afterLine != "" {
			lines = append(lines, "+"+afterLine)
		}
	}
	return strings.Join(lines, "\n")
}

func simulateCommand(session platform.ServerSession, command string) []platform.TerminalLine {
	switch command {
	case "help":
		return []platform.TerminalLine{terminalLine("output", "可用命令：hostname, whoami, pwd, uptime, df -h, ls, cat /etc/os-release, vim /etc/nginx/nginx.conf, rm -rf /data/tmp, exit")}
	case "hostname":
		return []platform.TerminalLine{terminalLine("output", session.AssetName)}
	case "whoami":
		return []platform.TerminalLine{terminalLine("output", session.Account)}
	case "pwd":
		return []platform.TerminalLine{terminalLine("output", "/home/"+session.Account)}
	case "uptime":
		return []platform.TerminalLine{terminalLine("output", "11:58:21 up 42 days, 3 users, load average: 0.18, 0.12, 0.08")}
	case "df -h":
		return []platform.TerminalLine{terminalLine("output", "Filesystem      Size  Used Avail Use% Mounted on\n/dev/vda1        80G   31G   49G  39% /\ntmpfs           7.8G     0  7.8G   0% /dev/shm")}
	case "ls":
		return []platform.TerminalLine{terminalLine("output", "apps  deploy  logs  tmp")}
	case "cat /etc/os-release":
		return []platform.TerminalLine{terminalLine("output", "NAME=\"Alibaba Cloud Linux\"\nVERSION=\"3\"")}
	case "exit", "logout":
		return []platform.TerminalLine{terminalLine("system", "session closed")}
	default:
		if path, mode := fileSnapshotPlan(command); path != "" {
			action := "文件变更"
			if mode == "delete" {
				action = "文件删除"
			}
			return []platform.TerminalLine{terminalLine("output", fmt.Sprintf("%s命令已记录：%s。当前为本地演示终端；真实输出需接入 SSH Gateway。", action, path))}
		}
		return []platform.TerminalLine{terminalLine("output", "命令已记录。当前为本地演示终端；接入真实 SSH Gateway 后会返回目标服务器输出。")}
	}
}
