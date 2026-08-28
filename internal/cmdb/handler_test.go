package cmdb

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestServerAssetSessionFlow(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	listRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/server-assets", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("server asset list status = %d, want %d", listRec.Code, http.StatusOK)
	}
	if !strings.Contains(listRec.Body.String(), "prod-order-api-01") {
		t.Fatalf("server asset list should contain demo server: %s", listRec.Body.String())
	}

	loginRec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"account":"ops","protocol":"ssh"}`)
	server.Mux.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/server-assets/2/sessions", body))
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d, body=%s", loginRec.Code, http.StatusCreated, loginRec.Body.String())
	}
	if !strings.Contains(loginRec.Body.String(), "connected") {
		t.Fatalf("login response should contain connected session: %s", loginRec.Body.String())
	}

	commandRec := httptest.NewRecorder()
	commandBody := bytes.NewBufferString(`{"command":"hostname"}`)
	server.Mux.ServeHTTP(commandRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/server-sessions/1/commands", commandBody))
	if commandRec.Code != http.StatusOK {
		t.Fatalf("command status = %d, want %d, body=%s", commandRec.Code, http.StatusOK, commandRec.Body.String())
	}
	if !strings.Contains(commandRec.Body.String(), "prod-order-api-01") {
		t.Fatalf("command response should contain hostname: %s", commandRec.Body.String())
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "cmdb.server.command" {
		t.Fatalf("audit action = %q, want cmdb.server.command", last.Action)
	}
	if !strings.Contains(last.Reason, "prod-order-api-01 ops$ hostname") {
		t.Fatalf("audit reason should include asset, account and command: %q", last.Reason)
	}

	snapshotRec := httptest.NewRecorder()
	snapshotBody := bytes.NewBufferString(`{"command":"vim /etc/nginx/nginx.conf"}`)
	server.Mux.ServeHTTP(snapshotRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/server-sessions/1/commands", snapshotBody))
	if snapshotRec.Code != http.StatusOK {
		t.Fatalf("snapshot command status = %d, want %d, body=%s", snapshotRec.Code, http.StatusOK, snapshotRec.Body.String())
	}
	if !strings.Contains(snapshotRec.Body.String(), `"snapshots"`) {
		t.Fatalf("snapshot command response should contain snapshots: %s", snapshotRec.Body.String())
	}
	if len(server.Store.FileSnapshots) != 2 {
		t.Fatalf("file snapshot count = %d, want 2", len(server.Store.FileSnapshots))
	}
	last = server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if !strings.Contains(last.Reason, "snapshots: before#") {
		t.Fatalf("snapshot audit reason should contain snapshot summary: %q", last.Reason)
	}

	listSnapshotsRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(listSnapshotsRec, httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/file-snapshots", nil))
	if listSnapshotsRec.Code != http.StatusOK {
		t.Fatalf("file snapshot list status = %d, want %d, body=%s", listSnapshotsRec.Code, http.StatusOK, listSnapshotsRec.Body.String())
	}
	if !strings.Contains(listSnapshotsRec.Body.String(), "/etc/nginx/nginx.conf") {
		t.Fatalf("file snapshot list should contain edited path: %s", listSnapshotsRec.Body.String())
	}
}

func TestCMDBRequiresOpsRole(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/server-assets", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCloudSyncSupportsMultipleAccountsPerProvider(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.CloudAccounts = nil
	store.CloudResourceTypes = nil
	store.Assets = nil
	store.SyncJobs = nil
	server := platform.NewServer(store)
	Register(server)

	originalListInstances := aliyunECSListInstances
	aliyunECSListInstances = func(account platform.CloudAccount, region string) ([]aliyunECSInstance, error) {
		return []aliyunECSInstance{{
			InstanceID:   "i-" + account.AccountRef,
			InstanceName: account.AccountRef + "-prod-01",
			InstanceType: "ecs.g6.large",
			RegionID:     region,
			ZoneID:       region + "-h",
			Status:       "Running",
			OSName:       "Alibaba Cloud Linux 3",
			VpcAttributes: aliyunECSVPCAttributes{
				VpcID:     "vpc-" + account.AccountRef,
				VSwitchID: "vsw-" + account.AccountRef,
				PrivateIPAddress: aliyunECSIPAddress{
					IPAddress: []string{"10.0.0.1"},
				},
			},
			PublicIPAddress: aliyunECSIPAddress{IPAddress: []string{"47.100.0.1"}},
			EIPAddress:      aliyunECSEIPAddress{IPAddress: "8.130.0.1"},
		}}, nil
	}
	defer func() { aliyunECSListInstances = originalListInstances }()

	for _, body := range []string{
		`{"provider":"aliyun","name":"阿里云账号A","accountRef":"aliyun-a","accessKeyId":"LTAIaliyuna","accessKeySecret":"secret-a","regions":["cn-hangzhou"],"resourceTypes":["ecs.instance"]}`,
		`{"provider":"aliyun","name":"阿里云账号B","accountRef":"aliyun-b","accessKeyId":"LTAIaliyunb","accessKeySecret":"secret-b","regions":["cn-hangzhou"],"resourceTypes":["ecs.instance"]}`,
	} {
		rec := httptest.NewRecorder()
		server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/cloud-accounts", bytes.NewBufferString(body)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("cloud account create status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
		}
	}

	createJobRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(createJobRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/sync-jobs", bytes.NewBufferString(`{"name":"同步阿里云全部账号","provider":"aliyun","accountId":0,"resourceTypes":["ecs.instance"],"regions":["cn-hangzhou"]}`)))
	if createJobRec.Code != http.StatusCreated {
		t.Fatalf("sync job create status = %d, want %d, body=%s", createJobRec.Code, http.StatusCreated, createJobRec.Body.String())
	}
	if len(store.SyncJobs) != 1 {
		t.Fatalf("sync job count = %d, want 1", len(store.SyncJobs))
	}

	runRec := httptest.NewRecorder()
	runPath := "/api/v1/cmdb/sync-jobs/" + strconv.FormatInt(store.SyncJobs[0].ID, 10) + "/run"
	server.Mux.ServeHTTP(runRec, httptest.NewRequest(http.MethodPost, runPath, nil))
	if runRec.Code != http.StatusOK {
		t.Fatalf("sync job run status = %d, want %d, body=%s", runRec.Code, http.StatusOK, runRec.Body.String())
	}
	if !strings.Contains(runRec.Body.String(), "真实同步账号:2") {
		t.Fatalf("sync result should include two accounts: %s", runRec.Body.String())
	}
	if len(store.Assets) != 2 {
		t.Fatalf("asset count = %d, want 2", len(store.Assets))
	}
	accountIDs := map[int64]bool{}
	for _, asset := range store.Assets {
		if asset.Provider != "aliyun" || asset.ResourceType != "ecs.instance" || asset.Region != "cn-hangzhou" {
			t.Fatalf("unexpected synced asset: %+v", asset)
		}
		if asset.Raw["syncMode"] == "generic" {
			t.Fatalf("synced asset should not use generic mode: %+v", asset.Raw)
		}
		if asset.Raw["publicIp"] != "47.100.0.1" {
			t.Fatalf("synced asset should expose first public ip, got %+v", asset.Raw)
		}
		publicIPs, ok := asset.Raw["publicIps"].([]string)
		if !ok || strings.Join(publicIPs, ",") != "47.100.0.1,8.130.0.1" {
			t.Fatalf("synced asset should keep public ip list, got %+v", asset.Raw["publicIps"])
		}
		accountIDs[asset.AccountID] = true
	}
	if len(accountIDs) != 2 {
		t.Fatalf("assets should keep separate account ids, got %+v", accountIDs)
	}
	if len(store.CloudResourceTypes) != 1 || store.CloudResourceTypes[0].ResourceType != "ecs.instance" || store.CloudResourceTypes[0].SyncMode != "cloud_api" {
		t.Fatalf("resource type catalog not updated correctly: %+v", store.CloudResourceTypes)
	}
}

func TestCloudSyncDefaultsToAllAliyunRegions(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.CloudAccounts = nil
	store.CloudResourceTypes = nil
	store.Assets = nil
	store.SyncJobs = nil
	server := platform.NewServer(store)
	Register(server)

	originalListRegions := aliyunECSListRegions
	originalListInstances := aliyunECSListInstances
	aliyunECSListRegions = func(account platform.CloudAccount) ([]string, error) {
		return []string{"cn-shenzhen", "cn-hangzhou"}, nil
	}
	calledRegions := []string{}
	aliyunECSListInstances = func(account platform.CloudAccount, region string) ([]aliyunECSInstance, error) {
		calledRegions = append(calledRegions, region)
		return []aliyunECSInstance{{
			InstanceID:   "i-" + region,
			InstanceName: "prod-" + region,
			RegionID:     region,
			Status:       "Running",
		}}, nil
	}
	defer func() {
		aliyunECSListRegions = originalListRegions
		aliyunECSListInstances = originalListInstances
	}()

	createRec := httptest.NewRecorder()
	accountBody := `{"provider":"aliyun","name":"阿里云账号A","accountRef":"aliyun-a","accessKeyId":"LTAIaliyuna","accessKeySecret":"secret-a","resourceTypes":["ecs.instance"]}`
	server.Mux.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/cloud-accounts", bytes.NewBufferString(accountBody)))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("cloud account create status = %d, want %d, body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if len(store.CloudAccounts[0].Regions) != 0 {
		t.Fatalf("empty account regions should mean all regions, got %+v", store.CloudAccounts[0].Regions)
	}

	runRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(runRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/cloud-accounts/"+strconv.FormatInt(store.CloudAccounts[0].ID, 10)+"/sync", nil))
	if runRec.Code != http.StatusCreated {
		t.Fatalf("cloud account sync status = %d, want %d, body=%s", runRec.Code, http.StatusCreated, runRec.Body.String())
	}
	if strings.Join(calledRegions, ",") != "cn-hangzhou,cn-shenzhen" {
		t.Fatalf("sync should discover and scan all regions in stable order, got %+v", calledRegions)
	}
	if len(store.Assets) != 2 {
		t.Fatalf("asset count = %d, want 2", len(store.Assets))
	}
}

func TestAliyunECSEndpointUsesRegionEndpoint(t *testing.T) {
	if got := aliyunECSEndpointForRegion("cn-shenzhen"); got != "https://ecs.cn-shenzhen.aliyuncs.com/" {
		t.Fatalf("endpoint = %q, want regional endpoint", got)
	}
	if got := aliyunECSEndpointForRegion(""); got != aliyunECSEndpoint {
		t.Fatalf("empty region endpoint = %q, want global endpoint", got)
	}
}

func TestCloudSyncRejectsUnsupportedResourceWithoutGenericAssets(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.CloudAccounts = nil
	store.CloudResourceTypes = nil
	store.Assets = nil
	store.SyncJobs = nil
	server := platform.NewServer(store)
	Register(server)

	createAccountRec := httptest.NewRecorder()
	accountBody := `{"provider":"aliyun","name":"阿里云账号A","accountRef":"aliyun-a","accessKeyId":"LTAIaliyuna","accessKeySecret":"secret-a","regions":["cn-hangzhou"],"resourceTypes":["rds.instance"]}`
	server.Mux.ServeHTTP(createAccountRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/cloud-accounts", bytes.NewBufferString(accountBody)))
	if createAccountRec.Code != http.StatusCreated {
		t.Fatalf("cloud account create status = %d, want %d, body=%s", createAccountRec.Code, http.StatusCreated, createAccountRec.Body.String())
	}

	createJobRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(createJobRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/sync-jobs", bytes.NewBufferString(`{"name":"同步阿里云RDS","provider":"aliyun","accountId":0,"resourceTypes":["rds.instance"],"regions":["cn-hangzhou"]}`)))
	if createJobRec.Code != http.StatusCreated {
		t.Fatalf("sync job create status = %d, want %d, body=%s", createJobRec.Code, http.StatusCreated, createJobRec.Body.String())
	}

	runRec := httptest.NewRecorder()
	runPath := "/api/v1/cmdb/sync-jobs/" + strconv.FormatInt(store.SyncJobs[0].ID, 10) + "/run"
	server.Mux.ServeHTTP(runRec, httptest.NewRequest(http.MethodPost, runPath, nil))
	if runRec.Code != http.StatusBadRequest {
		t.Fatalf("sync job run status = %d, want %d, body=%s", runRec.Code, http.StatusBadRequest, runRec.Body.String())
	}
	if !strings.Contains(runRec.Body.String(), "当前仅支持真实同步 ecs.instance") {
		t.Fatalf("sync failure should explain unsupported real adapter: %s", runRec.Body.String())
	}
	if len(store.Assets) != 0 {
		t.Fatalf("unsupported sync should not create generic assets: %+v", store.Assets)
	}
	if len(store.CloudResourceTypes) != 1 || store.CloudResourceTypes[0].SyncMode != "api_pending" {
		t.Fatalf("unsupported resource type should be marked api_pending: %+v", store.CloudResourceTypes)
	}
}

func TestCloudAccountAccessKeySecretIsNotReturned(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.CloudAccounts = nil
	server := platform.NewServer(store)
	Register(server)

	body := `{"provider":"aliyun","name":"阿里云生产账号","accountRef":"aliyun-prod","accessKeyId":"TESTACCESSKEY1234","accessKeySecret":"secret-value-should-not-leak","raw":{"token":"raw-token-should-not-leak"}}`
	createRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/cloud-accounts", bytes.NewBufferString(body)))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("cloud account create status = %d, want %d, body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}
	if strings.Contains(createRec.Body.String(), "secret-value-should-not-leak") || strings.Contains(createRec.Body.String(), "raw-token-should-not-leak") {
		t.Fatalf("create response leaked secret: %s", createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), `"secretConfigured":true`) {
		t.Fatalf("create response should indicate configured secret: %s", createRec.Body.String())
	}
	if !strings.Contains(createRec.Body.String(), "TEST*********1234") {
		t.Fatalf("create response should mask access key id: %s", createRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/v1/cmdb/cloud-accounts", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("cloud account list status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if strings.Contains(listRec.Body.String(), "secret-value-should-not-leak") || strings.Contains(listRec.Body.String(), "raw-token-should-not-leak") {
		t.Fatalf("list response leaked secret: %s", listRec.Body.String())
	}
}

func TestServerSessionRejectsSpoofedAccountOtherUserAndClosedSession(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	adminID := store.Users[0].ID
	opsID := store.Users[2].ID
	store.PolicyBindings = append(store.PolicyBindings, roleBinding(t, store, opsID, "ops_owner"))
	server := platform.NewServer(store)
	Register(server)

	loginRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/server-assets/2/sessions", bytes.NewBufferString(`{"account":"root","protocol":"telnet"}`)))
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("login status = %d, want %d, body=%s", loginRec.Code, http.StatusCreated, loginRec.Body.String())
	}
	if strings.Contains(loginRec.Body.String(), `"account":"root"`) || strings.Contains(loginRec.Body.String(), `"protocol":"telnet"`) {
		t.Fatalf("login should normalize unsafe account/protocol: %s", loginRec.Body.String())
	}
	if !strings.Contains(loginRec.Body.String(), `"actorUserId":`+strings.TrimSpace("1")) {
		t.Fatalf("login should record actor user id: %s", loginRec.Body.String())
	}

	server.Store.CurrentUserID = opsID
	otherUserRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(otherUserRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/server-sessions/1/commands", bytes.NewBufferString(`{"command":"hostname"}`)))
	if otherUserRec.Code != http.StatusForbidden {
		t.Fatalf("other user command status = %d, want %d, body=%s", otherUserRec.Code, http.StatusForbidden, otherUserRec.Body.String())
	}

	server.Store.CurrentUserID = adminID
	closeRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(closeRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/server-sessions/1/commands", bytes.NewBufferString(`{"command":"exit"}`)))
	if closeRec.Code != http.StatusOK {
		t.Fatalf("close command status = %d, want %d, body=%s", closeRec.Code, http.StatusOK, closeRec.Body.String())
	}
	closedRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(closedRec, httptest.NewRequest(http.MethodPost, "/api/v1/cmdb/server-sessions/1/commands", bytes.NewBufferString(`{"command":"hostname"}`)))
	if closedRec.Code != http.StatusConflict {
		t.Fatalf("closed command status = %d, want %d, body=%s", closedRec.Code, http.StatusConflict, closedRec.Body.String())
	}
}

func setOnlyRole(t *testing.T, store *platform.Store, userID int64, roleCode string) {
	t.Helper()
	store.CurrentUserID = userID
	store.PolicyBindings = []platform.PolicyBinding{roleBinding(t, store, userID, roleCode)}
}

func roleBinding(t *testing.T, store *platform.Store, userID int64, roleCode string) platform.PolicyBinding {
	t.Helper()
	for _, role := range store.Roles {
		if role.Code == roleCode {
			return platform.PolicyBinding{ID: store.Next("binding"), UserID: userID, RoleID: role.ID, RoleCode: role.Code, ScopeType: "global"}
		}
	}
	t.Fatalf("role %q not found", roleCode)
	return platform.PolicyBinding{}
}
