package cicd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestTemplatesIncludeBuiltinDefaults(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cicd/templates", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"go-k8s-default", "web-k8s-default", "jenkins-k8s-shell", "jenkins-k8s-modular", "backend", "frontend", "npm_build", "go_build", "go test ./...", "kubectl -n ${NAMESPACE}", "${APP_ID}", "${DEPLOY_ACTION}", "k8s_canary_first", "${ROLLBACK_VERSION}", "gitlab_checkout", "docker_build_push"} {
		if !strings.Contains(body, want) {
			t.Fatalf("templates response should contain %q: %s", want, body)
		}
	}
}

func TestModulesIncludeBuildRuntimes(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/cicd/modules", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"gitlab_checkout", "go_build", "jdk8_maven_build", "docker_build_push", "k8s_deploy_all", "k8s_rollback"} {
		if !strings.Contains(body, want) {
			t.Fatalf("modules response should contain %q: %s", want, body)
		}
	}
}

func TestJenkinsK8sTemplateUsesVariables(t *testing.T) {
	definition := platform.JenkinsK8sPipelineDefinition()
	commands := make([]string, 0, len(definition.Steps))
	for _, step := range definition.Steps {
		command, _ := step.With["command"].(string)
		commands = append(commands, command)
	}
	body := strings.Join(commands, "\n")

	for _, want := range []string{"${APP_ID}", "${DEPLOY_ENV}", "${GIT_GROUP}", "${GIT_PROJECT}", "${HARBOR_PROJECT}", "${DEPARTMENT}", "${DEPLOY_ACTION}", "${ROLLBACK_VERSION}"} {
		if !strings.Contains(body, want) {
			t.Fatalf("jenkins template should contain variable %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"go_passport", "project=go_passport", "version=legacy-group", "department=legacy-department", "env=test"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("jenkins template should not contain hardcoded value %q: %s", forbidden, body)
		}
	}
}

func TestUpdatePipelineTemplate(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := bytes.NewBufferString(`{
		"name": "vue-k8s-custom",
		"serviceType": "frontend",
		"status": "enabled",
		"definition": {
			"triggers": ["manual", "git_push"],
			"steps": [
				{"name": "checkout", "type": "git"},
				{"name": "build", "type": "npm_build"},
				{"name": "deploy", "type": "k8s_deploy"}
			]
		}
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/cicd/templates/2", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := server.Store.PipelineTemplates[1].Name; got != "vue-k8s-custom" {
		t.Fatalf("template name = %q, want vue-k8s-custom", got)
	}
	if got := server.Store.PipelineTemplates[1].ServiceType; got != "frontend" {
		t.Fatalf("service type = %q, want frontend", got)
	}
	if got := server.Store.PipelineTemplates[1].Definition.Steps[1].With["command"]; got != "pnpm build" {
		t.Fatalf("build command = %q, want pnpm build", got)
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "cicd.template.update" {
		t.Fatalf("audit action = %q, want cicd.template.update", last.Action)
	}
}

func TestUpdatePipelineModule(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := bytes.NewBufferString(`{
		"key": "jdk8_maven_build",
		"name": "JDK8 Maven Build",
		"category": "build",
		"runtime": "jdk8",
		"variables": ["WORKSPACE", "JDK8_HOME"],
		"command": "cd ${WORKSPACE} && ${JDK8_HOME}/bin/java -version && mvn package",
		"status": "enabled"
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/cicd/modules/4", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := server.Store.PipelineModules[3].Runtime; got != "jdk8" {
		t.Fatalf("runtime = %q, want jdk8", got)
	}
	if got := server.Store.PipelineModules[3].Command; !strings.Contains(got, "mvn package") {
		t.Fatalf("command = %q, want mvn package", got)
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "cicd.module.update" {
		t.Fatalf("audit action = %q, want cicd.module.update", last.Action)
	}
}

func TestPipelineTemplateMutationRequiresOps(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"name":"unsafe","serviceType":"backend","status":"enabled"}`)
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/v1/cicd/templates/1", body))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestPipelinesAreFilteredAndRequireServiceManageForRun(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	developerID := store.Users[3].ID
	setOnlyRole(t, store, developerID, "developer")
	server := platform.NewServer(store)
	Register(server)

	listRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/v1/cicd/pipelines", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), "order-api-default") {
		t.Fatalf("developer service member should see own service pipeline: %s", listRec.Body.String())
	}

	runRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(runRec, httptest.NewRequest(http.MethodPost, "/api/v1/cicd/pipelines/1/runs", nil))
	if runRec.Code != http.StatusForbidden {
		t.Fatalf("run status = %d, want %d, body=%s", runRec.Code, http.StatusForbidden, runRec.Body.String())
	}

	server.Store.ServiceMembers = nil
	hiddenRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(hiddenRec, httptest.NewRequest(http.MethodGet, "/api/v1/cicd/pipelines", nil))
	if hiddenRec.Code != http.StatusOK {
		t.Fatalf("hidden list status = %d, want %d, body=%s", hiddenRec.Code, http.StatusOK, hiddenRec.Body.String())
	}
	if strings.Contains(hiddenRec.Body.String(), "order-api-default") {
		t.Fatalf("non member should not see pipeline: %s", hiddenRec.Body.String())
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
