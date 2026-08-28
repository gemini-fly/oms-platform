package platform

import (
	"path/filepath"
	"testing"
)

func TestNewStoreStartsWithoutDemoBusinessData(t *testing.T) {
	store := NewStore()

	if len(store.Users) != 1 || store.Users[0].Username != "admin" {
		t.Fatalf("bootstrap users = %#v, want only local admin", store.Users)
	}
	if len(store.Applications) != 0 || len(store.Services) != 0 || len(store.Assets) != 0 || len(store.Tickets) != 0 {
		t.Fatalf("new store must not contain demo business data")
	}
	if len(store.PipelineTemplates) == 0 || len(store.CloudResourceTypes) == 0 {
		t.Fatalf("new store should retain built-in catalogs")
	}
}

func TestDefaultPipelineContainsK8sDeploy(t *testing.T) {
	store := NewDemoStore()
	service := Service{ID: 42, Name: "order-center", ServiceType: "backend"}

	pipeline := store.DefaultPipeline(service)

	if pipeline.ServiceID != service.ID {
		t.Fatalf("pipeline service id = %d, want %d", pipeline.ServiceID, service.ID)
	}
	found := false
	for _, step := range pipeline.Definition.Steps {
		if step.Type == "k8s_deploy" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("default pipeline should include k8s_deploy step")
	}
}

func TestDefaultPipelineUsesServiceTypeTemplate(t *testing.T) {
	store := NewDemoStore()
	service := Service{ID: 43, Name: "web-portal", ServiceType: "frontend"}

	pipeline := store.DefaultPipeline(service)

	if pipeline.TemplateID != store.PipelineTemplates[1].ID {
		t.Fatalf("pipeline template id = %d, want %d", pipeline.TemplateID, store.PipelineTemplates[1].ID)
	}
	foundFrontendBuild := false
	for _, step := range pipeline.Definition.Steps {
		if step.Type == "npm_build" {
			foundFrontendBuild = true
			break
		}
	}
	if !foundFrontendBuild {
		t.Fatalf("frontend pipeline should include npm_build step: %#v", pipeline.Definition.Steps)
	}
}

func TestDefaultPipelineUsesRuntimeTemplate(t *testing.T) {
	store := NewDemoStore()
	service := NormalizeRuntimeProfile(Service{ID: 44, Name: "order-java", ServiceType: "backend", RuntimeLanguage: "java", RuntimeVersion: "8", BuildTool: "maven"})

	pipeline := store.DefaultPipeline(service)

	template := PipelineTemplate{}
	for _, item := range store.PipelineTemplates {
		if item.ID == pipeline.TemplateID {
			template = item
			break
		}
	}
	if template.Name != "jdk8-k8s-default" {
		t.Fatalf("pipeline template = %q, want jdk8-k8s-default", template.Name)
	}
	foundJDKBuild := false
	for _, step := range pipeline.Definition.Steps {
		if step.ModuleKey == "jdk8_maven_build" {
			foundJDKBuild = true
			break
		}
	}
	if !foundJDKBuild {
		t.Fatalf("java pipeline should include jdk8_maven_build step: %#v", pipeline.Definition.Steps)
	}
}

func TestDefaultPipelineUsesPHPTemplate(t *testing.T) {
	store := NewDemoStore()
	service := NormalizeRuntimeProfile(Service{ID: 45, Name: "php-worker", ServiceType: "backend", RepositoryFullName: "example-group/php_worker"})

	pipeline := store.DefaultPipeline(service)

	template := PipelineTemplate{}
	for _, item := range store.PipelineTemplates {
		if item.ID == pipeline.TemplateID {
			template = item
			break
		}
	}
	if template.Name != "php-k8s-default" {
		t.Fatalf("pipeline template = %q, want php-k8s-default", template.Name)
	}
	foundComposer := false
	for _, step := range pipeline.Definition.Steps {
		if step.ModuleKey == "php_composer_install" {
			foundComposer = true
			break
		}
	}
	if !foundComposer {
		t.Fatalf("php pipeline should include php_composer_install step: %#v", pipeline.Definition.Steps)
	}
}

func TestDefaultPipelineUsesCommonLanguageTemplates(t *testing.T) {
	store := NewDemoStore()
	tests := []struct {
		name         string
		service      Service
		wantTemplate string
		wantModule   string
	}{
		{
			name:         "python",
			service:      Service{ID: 46, Name: "risk-python", ServiceType: "backend", RepositoryFullName: "example-group/risk_python"},
			wantTemplate: "python-k8s-default",
			wantModule:   "python_install",
		},
		{
			name:         "c",
			service:      Service{ID: 47, Name: "edge-c", ServiceType: "backend", RuntimeLanguage: "c", BuildTool: "make"},
			wantTemplate: "c-k8s-default",
			wantModule:   "c_make_build",
		},
		{
			name:         "cpp",
			service:      Service{ID: 48, Name: "media-cpp", ServiceType: "backend", RepositoryFullName: "example-group/media_cpp"},
			wantTemplate: "cpp-k8s-default",
			wantModule:   "cpp_cmake_build",
		},
		{
			name:         "dotnet",
			service:      Service{ID: 49, Name: "account-dotnet", ServiceType: "backend", RepositoryFullName: "example-group/account_dotnet"},
			wantTemplate: "dotnet-k8s-default",
			wantModule:   "dotnet_publish",
		},
		{
			name:         "rust",
			service:      Service{ID: 50, Name: "gateway-rust", ServiceType: "backend", RepositoryFullName: "example-group/gateway_rust"},
			wantTemplate: "rust-k8s-default",
			wantModule:   "rust_build",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pipeline := store.DefaultPipeline(NormalizeRuntimeProfile(tt.service))
			template := PipelineTemplate{}
			for _, item := range store.PipelineTemplates {
				if item.ID == pipeline.TemplateID {
					template = item
					break
				}
			}
			if template.Name != tt.wantTemplate {
				t.Fatalf("pipeline template = %q, want %q", template.Name, tt.wantTemplate)
			}
			foundModule := false
			for _, step := range pipeline.Definition.Steps {
				if step.ModuleKey == tt.wantModule {
					foundModule = true
					break
				}
			}
			if !foundModule {
				t.Fatalf("pipeline should include %s step: %#v", tt.wantModule, pipeline.Definition.Steps)
			}
		})
	}
}

func TestGitLabMappingsPersistEmptyList(t *testing.T) {
	t.Setenv("SY_PLATFORM_GITLAB_MAPPING_FILE", filepath.Join(t.TempDir(), "gitlab-mappings.json"))
	store := NewDemoStore()

	if err := store.PersistGitLabMappings([]GitLabGroupMapping{}); err != nil {
		t.Fatalf("persist empty gitlab mappings: %v", err)
	}
	reloaded := NewDemoStore()
	if len(reloaded.GitLabMappings) != 0 {
		t.Fatalf("reloaded gitlab mappings = %d, want 0", len(reloaded.GitLabMappings))
	}
}

func TestStoreSnapshotRoundTripPreservesStateAndCounters(t *testing.T) {
	store := NewDemoStore()
	store.Settings.PlatformName = "Acme 运维平台"
	store.Tickets = append(store.Tickets, Ticket{ID: 99, TicketNo: "ITSM-000099", Title: "db snapshot"})

	snapshot := store.snapshotLocked()
	reloaded := NewDemoStore()
	reloaded.applySnapshotLocked(snapshot)

	if got := reloaded.Settings.PlatformName; got != "Acme 运维平台" {
		t.Fatalf("platform name = %q, want %q", got, "Acme 运维平台")
	}
	if got := reloaded.Tickets[len(reloaded.Tickets)-1].TicketNo; got != "ITSM-000099" {
		t.Fatalf("last ticket = %q, want ITSM-000099", got)
	}
	if next := reloaded.Next("ticket"); next != 100 {
		t.Fatalf("next ticket id = %d, want 100", next)
	}
}

func TestStoreSnapshotBackfillsBuiltinPipelineTemplates(t *testing.T) {
	store := NewDemoStore()
	snapshot := store.snapshotLocked()
	snapshot.PipelineTemplates = snapshot.PipelineTemplates[:2]
	snapshot.PipelineModules = nil
	snapshot.Next["pipeline_template"] = 2
	snapshot.Next["pipeline_module"] = 0

	reloaded := NewDemoStore()
	if !reloaded.applySnapshotLocked(snapshot) {
		t.Fatal("expected snapshot apply to report a built-in module/template backfill")
	}

	foundTemplate := false
	for _, template := range reloaded.PipelineTemplates {
		if template.Name == "jenkins-k8s-shell" {
			foundTemplate = true
			break
		}
	}
	if !foundTemplate {
		t.Fatalf("expected jenkins-k8s-shell template to be backfilled: %#v", reloaded.PipelineTemplates)
	}
	foundModule := false
	for _, module := range reloaded.PipelineModules {
		if module.Key == "jdk8_maven_build" {
			foundModule = true
			break
		}
	}
	if !foundModule {
		t.Fatalf("expected jdk8_maven_build module to be backfilled: %#v", reloaded.PipelineModules)
	}
	foundPHPTemplate := false
	for _, template := range reloaded.PipelineTemplates {
		if template.Name == "php-k8s-default" {
			foundPHPTemplate = true
			break
		}
	}
	if !foundPHPTemplate {
		t.Fatalf("expected php-k8s-default template to be backfilled: %#v", reloaded.PipelineTemplates)
	}
}
