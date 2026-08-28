package apptree

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

func TestTreeIncludesBusinessGroupAndCenter(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/app-tree/tree", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"businessGroups", "businessCenters", "交易业务组", "电商业务中心", "增长业务组", "businessCenterId"} {
		if !strings.Contains(body, want) {
			t.Fatalf("tree response should contain %q: %s", want, body)
		}
	}
}

func TestMoveApplicationToBusinessGroup(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	targetGroupID := server.Store.BusinessGroups[1].ID
	targetSystemID := server.Store.Systems[1].ID
	body := bytes.NewBufferString(`{"targetType":"business_group","targetId":` + strconv.FormatInt(targetGroupID, 10) + `}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/app-tree/applications/1/move", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := server.Store.Applications[0].SystemID; got != targetSystemID {
		t.Fatalf("application system id = %d, want %d", got, targetSystemID)
	}
	if !strings.Contains(rec.Body.String(), "targetSystem") {
		t.Fatalf("move response should contain target system: %s", rec.Body.String())
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "app_tree.application.move" {
		t.Fatalf("audit action = %q, want app_tree.application.move", last.Action)
	}
}

func TestApplyApplicationThenCreateServiceGeneratesPipeline(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	appBody := bytes.NewBufferString(`{"systemId":1,"repositoryUrl":"git@code.example.internal:platform/rd-center/payment_center.git"}`)
	appRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(appRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/applications", appBody))

	if appRec.Code != http.StatusCreated {
		t.Fatalf("application status = %d, want %d, body=%s", appRec.Code, http.StatusCreated, appRec.Body.String())
	}
	for _, want := range []string{`"appId":"payment-center"`, `"name":"payment-center"`, `"repositoryProvider":"gitlab"`, `"repositoryFullName":"platform/rd-center/payment_center"`, `"lifecycleStatus":"applying"`} {
		if !strings.Contains(appRec.Body.String(), want) {
			t.Fatalf("application response should contain %q: %s", want, appRec.Body.String())
		}
	}
	if !strings.Contains(appRec.Body.String(), `"lifecycleStatus":"applying"`) {
		t.Fatalf("application should default to applying: %s", appRec.Body.String())
	}
	if got := server.Store.AuditLogs[len(server.Store.AuditLogs)-1].Action; got != "app_tree.application.create" {
		t.Fatalf("audit action = %q, want app_tree.application.create", got)
	}

	serviceBody := bytes.NewBufferString(`{"applicationId":2,"name":"payment-center","serviceType":"frontend"}`)
	serviceRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(serviceRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/services", serviceBody))

	if serviceRec.Code != http.StatusCreated {
		t.Fatalf("service status = %d, want %d, body=%s", serviceRec.Code, http.StatusCreated, serviceRec.Body.String())
	}
	for _, want := range []string{`"serviceType":"frontend"`, `"repositoryFullName":"platform/rd-center/payment_center"`, `"runtimeLanguage":"node"`, `"runtimeVersion":"20"`, `"buildTool":"pnpm"`, `"pipeline"`, "npm_build"} {
		if !strings.Contains(serviceRec.Body.String(), want) {
			t.Fatalf("service response should contain %q: %s", want, serviceRec.Body.String())
		}
	}
	if len(server.Store.ServiceMembers) == 0 {
		t.Fatal("service creation should add owner service member")
	}
}

func TestCreateJavaServiceUsesJDK8Template(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	appRec := httptest.NewRecorder()
	appBody := bytes.NewBufferString(`{"systemId":1,"repositoryUrl":"git@code.example.internal:platform/rd-center/order-java.git"}`)
	server.Mux.ServeHTTP(appRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/applications", appBody))
	if appRec.Code != http.StatusCreated {
		t.Fatalf("application status = %d, want %d, body=%s", appRec.Code, http.StatusCreated, appRec.Body.String())
	}

	serviceRec := httptest.NewRecorder()
	serviceBody := bytes.NewBufferString(`{"applicationId":2,"name":"order-java","serviceType":"backend","runtimeLanguage":"java","runtimeVersion":"8","buildTool":"maven"}`)
	server.Mux.ServeHTTP(serviceRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/services", serviceBody))
	if serviceRec.Code != http.StatusCreated {
		t.Fatalf("service status = %d, want %d, body=%s", serviceRec.Code, http.StatusCreated, serviceRec.Body.String())
	}
	for _, want := range []string{`"runtimeLanguage":"java"`, `"runtimeVersion":"8"`, `"buildTool":"maven"`, "jdk8_maven_build"} {
		if !strings.Contains(serviceRec.Body.String(), want) {
			t.Fatalf("service response should contain %q: %s", want, serviceRec.Body.String())
		}
	}
}

func TestInspectRepositoryFallsBackToPHPFromRepositoryURL(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	t.Setenv("SY_PLATFORM_GITLAB_INSPECT_DISABLED", "true")
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"repositoryUrl":"git@gitlab.example.com:example-group/php_worker.git","serviceType":"backend"}`)
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/repository/inspect", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, want := range []string{`"appId":"php-worker"`, `"repositoryFullName":"example-group/php_worker"`, `"runtimeLanguage":"php"`, `"runtimeVersion":"8.2"`, `"buildTool":"composer"`, `"pipelineTemplate":"php-k8s-default"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("inspect response should contain %q: %s", want, rec.Body.String())
		}
	}
}

func TestRuntimeProfileFromRepositoryFilesDetectsCommonLanguages(t *testing.T) {
	tests := []struct {
		name      string
		paths     []string
		language  string
		buildTool string
	}{
		{name: "python", paths: []string{"pyproject.toml", "app/main.py"}, language: "python", buildTool: "poetry"},
		{name: "c", paths: []string{"Makefile", "src/main.c", "include/app.h"}, language: "c", buildTool: "make"},
		{name: "cpp", paths: []string{"CMakeLists.txt", "src/main.cpp", "include/app.hpp"}, language: "cpp", buildTool: "cmake"},
		{name: "dotnet", paths: []string{"src/Order.Api/Order.Api.csproj"}, language: "dotnet", buildTool: "dotnet"},
		{name: "rust", paths: []string{"Cargo.toml", "src/main.rs"}, language: "rust", buildTool: "cargo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, _, ok := runtimeProfileFromRepositoryFiles(tt.paths, "backend")
			if !ok {
				t.Fatalf("expected %s profile from paths %#v", tt.name, tt.paths)
			}
			if profile.Language != tt.language || profile.BuildTool != tt.buildTool {
				t.Fatalf("profile = %#v, want language=%s buildTool=%s", profile, tt.language, tt.buildTool)
			}
		})
	}
}

func TestApplyApplicationRequiresAppIDToMatchRepositoryProject(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"systemId":1,"appId":"payment-center","name":"支付中心","repositoryProvider":"gitlab","repositoryFullName":"platform/rd-center/order-center"}`)
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/applications", body))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "APPID_REPOSITORY_MISMATCH") {
		t.Fatalf("response should explain appid repository mismatch: %s", rec.Body.String())
	}
}

func TestApplicationTreeStructureRequiresDevOwner(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/business-groups", bytes.NewBufferString(`{"name":"未授权业务组","ownerUserId":1}`)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestApplyApplicationIgnoresSpoofedOwner(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	developerID := store.Users[3].ID
	setOnlyRole(t, store, developerID, "developer")
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"systemId":1,"repositoryUrl":"https://github.com/example-org/self-service-app.git","ownerUserId":1}`)
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/applications", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ownerUserId":4`) {
		t.Fatalf("application owner should be current actor, not client payload: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"appId":"self-service-app"`) || !strings.Contains(rec.Body.String(), `"repositoryProvider":"github"`) {
		t.Fatalf("application should infer appid and provider from repository URL: %s", rec.Body.String())
	}
}

func TestServiceCreateRequiresApplicationManager(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"applicationId":1,"name":"unsafe-api","ownerUserId":4}`)
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/services", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestMoveApplicationRequiresApplicationManager(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	targetGroupID := server.Store.BusinessGroups[1].ID
	body := bytes.NewBufferString(`{"targetType":"business_group","targetId":` + strconv.FormatInt(targetGroupID, 10) + `}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/app-tree/applications/1/move", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestTreeFiltersServicesByServiceMembers(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.PolicyBindings = []platform.PolicyBinding{{
		ID:        store.Next("binding"),
		UserID:    store.CurrentActorID(),
		RoleID:    store.Roles[3].ID,
		RoleCode:  "developer",
		ScopeType: "global",
	}}
	store.Services[0].OwnerUserID = store.Users[1].ID
	store.ServiceMembers = nil
	server := platform.NewServer(store)
	Register(server)

	hiddenRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(hiddenRec, httptest.NewRequest(http.MethodGet, "/api/v1/app-tree/tree", nil))
	if hiddenRec.Code != http.StatusOK {
		t.Fatalf("tree status = %d, want %d", hiddenRec.Code, http.StatusOK)
	}
	if strings.Contains(hiddenRec.Body.String(), "order-api") {
		t.Fatalf("tree should hide services without service member access: %s", hiddenRec.Body.String())
	}

	server.Store.ServiceMembers = append(server.Store.ServiceMembers, platform.ServiceMember{
		ID:        server.Store.Next("service_member"),
		ServiceID: server.Store.Services[0].ID,
		UserID:    server.Store.CurrentActorID(),
		Role:      "developer",
		Status:    "enabled",
	})
	visibleRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(visibleRec, httptest.NewRequest(http.MethodGet, "/api/v1/app-tree/tree", nil))
	if !strings.Contains(visibleRec.Body.String(), "order-api") {
		t.Fatalf("tree should show services for configured service members: %s", visibleRec.Body.String())
	}
}

func TestServiceMemberCanBeConfiguredAndAudited(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := bytes.NewBufferString(`{"userId":4,"role":"maintainer"}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/services/1/members", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("service member status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"role":"maintainer"`) {
		t.Fatalf("service member response should contain role: %s", rec.Body.String())
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "app_tree.service_member.update" {
		t.Fatalf("audit action = %q, want app_tree.service_member.update", last.Action)
	}
}

func TestPodSessionAllowsOpsOnProduction(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	prodEnvID := environmentIDByName(t, server.Store, "prod")
	loginRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(prodEnvID, 10)+"/pod-sessions", bytes.NewBufferString(`{}`)))

	if loginRec.Code != http.StatusCreated {
		t.Fatalf("pod login status = %d, want %d, body=%s", loginRec.Code, http.StatusCreated, loginRec.Body.String())
	}
	for _, want := range []string{"order-api-prod-7f9c8d", "order-prod", "connected"} {
		if !strings.Contains(loginRec.Body.String(), want) {
			t.Fatalf("pod login response should contain %q: %s", want, loginRec.Body.String())
		}
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "app_tree.pod.login" || last.Result != "success" {
		t.Fatalf("audit = %s/%s, want app_tree.pod.login/success", last.Action, last.Result)
	}
}

func TestPodSessionRejectsDeveloperOnProduction(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.PolicyBindings = []platform.PolicyBinding{{
		ID:        store.Next("binding"),
		UserID:    store.CurrentActorID(),
		RoleID:    store.Roles[3].ID,
		RoleCode:  "developer",
		ScopeType: "global",
	}}
	server := platform.NewServer(store)
	Register(server)

	prodEnvID := environmentIDByName(t, server.Store, "prod")
	prodRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(prodRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(prodEnvID, 10)+"/pod-sessions", bytes.NewBufferString(`{}`)))
	if prodRec.Code != http.StatusForbidden {
		t.Fatalf("prod pod login status = %d, want %d, body=%s", prodRec.Code, http.StatusForbidden, prodRec.Body.String())
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "app_tree.pod.login" || last.Result != "denied" {
		t.Fatalf("audit = %s/%s, want app_tree.pod.login/denied", last.Action, last.Result)
	}

	devEnvID := environmentIDByName(t, server.Store, "dev")
	devRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(devRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(devEnvID, 10)+"/pod-sessions", bytes.NewBufferString(`{}`)))
	if devRec.Code != http.StatusCreated {
		t.Fatalf("dev pod login status = %d, want %d, body=%s", devRec.Code, http.StatusCreated, devRec.Body.String())
	}
	if !strings.Contains(devRec.Body.String(), `"account":"developer"`) {
		t.Fatalf("developer pod login should use developer account: %s", devRec.Body.String())
	}
}

func TestPodSessionCommandAudited(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	devEnvID := environmentIDByName(t, server.Store, "dev")
	loginRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(devEnvID, 10)+"/pod-sessions", bytes.NewBufferString(`{}`)))
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("pod login status = %d, want %d, body=%s", loginRec.Code, http.StatusCreated, loginRec.Body.String())
	}

	commandRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(commandRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/pod-sessions/1/commands", bytes.NewBufferString(`{"command":"hostname"}`)))
	if commandRec.Code != http.StatusOK {
		t.Fatalf("pod command status = %d, want %d, body=%s", commandRec.Code, http.StatusOK, commandRec.Body.String())
	}
	if !strings.Contains(commandRec.Body.String(), "order-api-dev-7f9c8d") {
		t.Fatalf("pod command response should contain hostname: %s", commandRec.Body.String())
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "app_tree.pod.command" {
		t.Fatalf("audit action = %q, want app_tree.pod.command", last.Action)
	}
	if !strings.Contains(last.Reason, "order-dev/order-api-dev-7f9c8d ops$ hostname") {
		t.Fatalf("audit reason should include namespace, pod, account and command: %q", last.Reason)
	}
}

func TestPodSessionRejectsDeveloperWithoutServiceMember(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	developerID := store.Users[3].ID
	setOnlyRole(t, store, developerID, "developer")
	store.ServiceMembers = nil
	server := platform.NewServer(store)
	Register(server)

	devEnvID := environmentIDByName(t, server.Store, "dev")
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(devEnvID, 10)+"/pod-sessions", bytes.NewBufferString(`{}`)))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SERVICE_FORBIDDEN") {
		t.Fatalf("response should explain service access denial: %s", rec.Body.String())
	}
}

func TestPodSessionLoginIgnoresSpoofedPodAndAccount(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	developerID := store.Users[3].ID
	setOnlyRole(t, store, developerID, "developer")
	server := platform.NewServer(store)
	Register(server)

	devEnvID := environmentIDByName(t, server.Store, "dev")
	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"podName":"prod-root-shell","account":"root"}`)
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(devEnvID, 10)+"/pod-sessions", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	for _, denied := range []string{`"podName":"prod-root-shell"`, `"account":"root"`} {
		if strings.Contains(rec.Body.String(), denied) {
			t.Fatalf("pod login should not trust client supplied %q: %s", denied, rec.Body.String())
		}
	}
	for _, want := range []string{`"podName":"order-api-dev-7f9c8d"`, `"account":"developer"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("pod login should contain %q: %s", want, rec.Body.String())
		}
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if strings.Contains(last.Reason, "root") || strings.Contains(last.Reason, "prod-root-shell") {
		t.Fatalf("audit reason should not contain spoofed pod/account: %q", last.Reason)
	}
}

func TestPodSessionCommandRejectsDifferentDeveloperSession(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	devLeadID := store.Users[1].ID
	developerID := store.Users[3].ID
	store.CurrentUserID = developerID
	store.PolicyBindings = []platform.PolicyBinding{
		roleBinding(t, store, developerID, "developer"),
		roleBinding(t, store, devLeadID, "developer"),
	}
	server := platform.NewServer(store)
	Register(server)

	devEnvID := environmentIDByName(t, server.Store, "dev")
	loginRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(devEnvID, 10)+"/pod-sessions", bytes.NewBufferString(`{}`)))
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("pod login status = %d, want %d, body=%s", loginRec.Code, http.StatusCreated, loginRec.Body.String())
	}

	server.Store.CurrentUserID = devLeadID
	commandRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(commandRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/pod-sessions/1/commands", bytes.NewBufferString(`{"command":"hostname"}`)))

	if commandRec.Code != http.StatusForbidden {
		t.Fatalf("pod command status = %d, want %d, body=%s", commandRec.Code, http.StatusForbidden, commandRec.Body.String())
	}
	if !strings.Contains(commandRec.Body.String(), "POD_SESSION_FORBIDDEN") {
		t.Fatalf("response should explain session ownership denial: %s", commandRec.Body.String())
	}
}

func TestPodSessionCommandRejectsClosedSession(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	devEnvID := environmentIDByName(t, server.Store, "dev")
	loginRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments/"+strconv.FormatInt(devEnvID, 10)+"/pod-sessions", bytes.NewBufferString(`{}`)))
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("pod login status = %d, want %d, body=%s", loginRec.Code, http.StatusCreated, loginRec.Body.String())
	}
	closeRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(closeRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/pod-sessions/1/commands", bytes.NewBufferString(`{"command":"exit"}`)))
	if closeRec.Code != http.StatusOK {
		t.Fatalf("close command status = %d, want %d, body=%s", closeRec.Code, http.StatusOK, closeRec.Body.String())
	}

	commandRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(commandRec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/pod-sessions/1/commands", bytes.NewBufferString(`{"command":"hostname"}`)))
	if commandRec.Code != http.StatusConflict {
		t.Fatalf("closed session command status = %d, want %d, body=%s", commandRec.Code, http.StatusConflict, commandRec.Body.String())
	}
}

func TestCreateEnvironmentEnforcesRoleAndNamespaceScope(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	t.Run("developer_member_cannot_create_environment", func(t *testing.T) {
		store := platform.NewDemoStore()
		setOnlyRole(t, store, store.Users[3].ID, "developer")
		server := platform.NewServer(store)
		Register(server)

		rec := httptest.NewRecorder()
		body := bytes.NewBufferString(`{"serviceId":1,"name":"sandbox","releaseLevel":"development","k8sClusterId":1,"k8sNamespaceId":1}`)
		server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments", body))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
	})
	t.Run("developer_maintainer_cannot_create_production_environment", func(t *testing.T) {
		store := platform.NewDemoStore()
		setOnlyRole(t, store, store.Users[1].ID, "developer")
		server := platform.NewServer(store)
		Register(server)

		rec := httptest.NewRecorder()
		body := bytes.NewBufferString(`{"serviceId":1,"name":"prod-copy","releaseLevel":"production","k8sClusterId":1,"k8sNamespaceId":3}`)
		server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments", body))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "PRODUCTION_ENVIRONMENT_FORBIDDEN") {
			t.Fatalf("response should explain production environment denial: %s", rec.Body.String())
		}
	})
	t.Run("namespace_must_belong_to_service", func(t *testing.T) {
		store := platform.NewDemoStore()
		otherService := platform.Service{ID: store.Next("service"), ApplicationID: store.Applications[0].ID, Name: "inventory-api", ServiceType: "backend", OwnerUserID: store.Users[0].ID, Status: "enabled"}
		store.Services = append(store.Services, otherService)
		otherNamespace := platform.K8sNamespace{ID: store.Next("k8s_namespace"), ClusterID: store.K8sClusters[0].ID, Name: "inventory-dev", ScopeType: "service", ScopeID: otherService.ID, Status: "enabled"}
		store.K8sNamespaces = append(store.K8sNamespaces, otherNamespace)
		server := platform.NewServer(store)
		Register(server)

		rec := httptest.NewRecorder()
		body := bytes.NewBufferString(`{"serviceId":1,"name":"dev-copy","releaseLevel":"development","k8sClusterId":1,"k8sNamespaceId":` + strconv.FormatInt(otherNamespace.ID, 10) + `}`)
		server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/app-tree/environments", body))

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "K8S_NAMESPACE_FORBIDDEN") {
			t.Fatalf("response should explain namespace scope denial: %s", rec.Body.String())
		}
	})
}

func TestApplicationOverviewFiltersHiddenServiceData(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	hiddenService := platform.Service{ID: store.Next("service"), ApplicationID: store.Applications[0].ID, Name: "hidden-api", ServiceType: "backend", OwnerUserID: store.Users[0].ID, Status: "enabled"}
	store.Services = append(store.Services, hiddenService)
	store.Assets = append(store.Assets, platform.Asset{ID: store.Next("asset"), ResourceType: "server", Name: "hidden-server", ScopeType: "service", ScopeID: hiddenService.ID, Status: "running"})
	store.Tickets = append(store.Tickets, platform.Ticket{ID: store.Next("ticket"), TicketNo: "ITSM-HIDDEN", TicketType: "access", Title: "hidden access", Status: "draft", ScopeType: "service", ScopeID: hiddenService.ID})
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/app-tree/applications/1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, denied := range []string{"hidden-api", "hidden-server", "ITSM-HIDDEN"} {
		if strings.Contains(rec.Body.String(), denied) {
			t.Fatalf("application overview should hide %q: %s", denied, rec.Body.String())
		}
	}
	if !strings.Contains(rec.Body.String(), "order-api") {
		t.Fatalf("application overview should still show visible service: %s", rec.Body.String())
	}
}

func environmentIDByName(t *testing.T, store *platform.Store, name string) int64 {
	t.Helper()
	for _, env := range store.Environments {
		if env.Name == name {
			return env.ID
		}
	}
	t.Fatalf("environment %q not found", name)
	return 0
}

func setOnlyRole(t *testing.T, store *platform.Store, userID int64, roleCode string) {
	t.Helper()
	store.CurrentUserID = userID
	store.PolicyBindings = []platform.PolicyBinding{roleBinding(t, store, userID, roleCode)}
}

func roleBinding(t *testing.T, store *platform.Store, userID int64, roleCode string) platform.PolicyBinding {
	t.Helper()
	role := roleByCode(t, store, roleCode)
	return platform.PolicyBinding{
		ID:        store.Next("binding"),
		UserID:    userID,
		RoleID:    role.ID,
		RoleCode:  role.Code,
		ScopeType: "global",
	}
}

func roleByCode(t *testing.T, store *platform.Store, code string) platform.Role {
	t.Helper()
	for _, role := range store.Roles {
		if role.Code == code {
			return role
		}
	}
	t.Fatalf("role %q not found", code)
	return platform.Role{}
}
