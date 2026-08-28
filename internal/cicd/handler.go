package cicd

import (
	"net/http"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/cicd/modules", modules(s))
	s.Mux.HandleFunc("/api/v1/cicd/modules/", moduleByID(s))
	s.Mux.HandleFunc("/api/v1/cicd/templates", templates(s))
	s.Mux.HandleFunc("/api/v1/cicd/templates/", templateByID(s))
	s.Mux.HandleFunc("/api/v1/cicd/pipelines", pipelines(s))
	s.Mux.HandleFunc("/api/v1/cicd/pipelines/", pipelineByID(s))
	s.Mux.HandleFunc("/api/v1/cicd/runs/", runByID(s))
}

func DefaultDefinition() platform.PipelineDef {
	return platform.DefaultPipelineDefinition("backend")
}

func modules(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, s.Store.PipelineModules)
			return
		}
		if !canManageTemplates(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CICD_MODULE_FORBIDDEN", "ops or platform admin role required")
			return
		}
		var item platform.PipelineModule
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item.ID = s.Store.Next("pipeline_module")
		item = normalizeModule(item)
		s.Store.PipelineModules = append(s.Store.PipelineModules, item)
		s.Store.Audit(actorID, "cicd.module.create", "pipeline_module", item.ID, "success", item.Key)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func moduleByID(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r.URL.Path, "/api/v1/cicd/modules/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		for i := range s.Store.PipelineModules {
			if s.Store.PipelineModules[i].ID != id {
				continue
			}
			switch r.Method {
			case http.MethodGet:
				platform.JSON(w, http.StatusOK, s.Store.PipelineModules[i])
			case http.MethodPut:
				if !canManageTemplates(s.Store, actorID) {
					platform.Error(w, http.StatusForbidden, "CICD_MODULE_FORBIDDEN", "ops or platform admin role required")
					return
				}
				var req platform.PipelineModule
				if err := platform.Decode(r, &req); err != nil {
					platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
					return
				}
				req.ID = id
				req = normalizeModule(req)
				s.Store.PipelineModules[i] = req
				s.Store.Audit(actorID, "cicd.module.update", "pipeline_module", id, "success", req.Key)
				platform.JSON(w, http.StatusOK, req)
			default:
				platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			}
			return
		}
		platform.Error(w, http.StatusNotFound, "MODULE_NOT_FOUND", "pipeline module not found")
	}
}

func templates(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, s.Store.PipelineTemplates)
			return
		}
		if !canManageTemplates(s.Store, actorID) {
			platform.Error(w, http.StatusForbidden, "CICD_TEMPLATE_FORBIDDEN", "ops or platform admin role required")
			return
		}
		var item platform.PipelineTemplate
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item.ID = s.Store.Next("pipeline_template")
		item = normalizeTemplate(item)
		s.Store.PipelineTemplates = append(s.Store.PipelineTemplates, item)
		s.Store.Audit(actorID, "cicd.template.create", "pipeline_template", item.ID, "success", "create pipeline template")
		platform.JSON(w, http.StatusCreated, item)
	}
}

func templateByID(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r.URL.Path, "/api/v1/cicd/templates/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		tail := ""
		if len(segments) > 5 {
			tail = segments[5]
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		for i := range s.Store.PipelineTemplates {
			if s.Store.PipelineTemplates[i].ID != id {
				continue
			}
			switch {
			case r.Method == http.MethodGet && tail == "":
				platform.JSON(w, http.StatusOK, s.Store.PipelineTemplates[i])
			case r.Method == http.MethodPut && tail == "":
				if !canManageTemplates(s.Store, actorID) {
					platform.Error(w, http.StatusForbidden, "CICD_TEMPLATE_FORBIDDEN", "ops or platform admin role required")
					return
				}
				var req platform.PipelineTemplate
				if err := platform.Decode(r, &req); err != nil {
					platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
					return
				}
				req.ID = id
				req = normalizeTemplate(req)
				s.Store.PipelineTemplates[i] = req
				s.Store.Audit(actorID, "cicd.template.update", "pipeline_template", id, "success", "update pipeline template")
				platform.JSON(w, http.StatusOK, req)
			default:
				platform.Error(w, http.StatusNotFound, "ACTION_NOT_FOUND", "template action not found")
			}
			return
		}
		platform.Error(w, http.StatusNotFound, "TEMPLATE_NOT_FOUND", "pipeline template not found")
	}
}

func pipelines(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		items := visiblePipelines(s.Store, platform.ActorID(r, s.Store))
		platform.JSON(w, http.StatusOK, platform.Page[platform.Pipeline]{Items: items, Page: 1, PageSize: len(items), Total: int64(len(items))})
	}
}

func pipelineByID(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r.URL.Path, "/api/v1/cicd/pipelines/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		tail := ""
		if len(segments) > 5 {
			tail = segments[5]
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		for i := range s.Store.Pipelines {
			if s.Store.Pipelines[i].ID != id {
				continue
			}
			switch {
			case r.Method == http.MethodGet && tail == "":
				if !s.Store.HasServiceAccess(actorID, s.Store.Pipelines[i].ServiceID) {
					platform.Error(w, http.StatusForbidden, "PIPELINE_FORBIDDEN", "current user cannot access this pipeline")
					return
				}
				platform.JSON(w, http.StatusOK, s.Store.Pipelines[i])
			case r.Method == http.MethodPut && tail == "":
				if !s.Store.CanManageService(actorID, s.Store.Pipelines[i].ServiceID) {
					platform.Error(w, http.StatusForbidden, "PIPELINE_FORBIDDEN", "current user cannot edit this pipeline")
					return
				}
				var req platform.Pipeline
				if err := platform.Decode(r, &req); err != nil {
					platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
					return
				}
				req.ID = id
				req.ServiceID = s.Store.Pipelines[i].ServiceID
				req.TemplateID = s.Store.Pipelines[i].TemplateID
				req.Status = "edited"
				s.Store.Pipelines[i] = req
				s.Store.Audit(actorID, "cicd.pipeline.update", "pipeline", id, "success", "update pipeline")
				platform.JSON(w, http.StatusOK, req)
			case r.Method == http.MethodPost && tail == "enable":
				if !s.Store.CanManageService(actorID, s.Store.Pipelines[i].ServiceID) {
					platform.Error(w, http.StatusForbidden, "PIPELINE_FORBIDDEN", "current user cannot enable this pipeline")
					return
				}
				s.Store.Pipelines[i].Status = "enabled"
				s.Store.Audit(actorID, "cicd.pipeline.enable", "pipeline", id, "success", "enable pipeline")
				platform.JSON(w, http.StatusOK, s.Store.Pipelines[i])
			case r.Method == http.MethodPost && tail == "runs":
				if !s.Store.CanManageService(actorID, s.Store.Pipelines[i].ServiceID) {
					platform.Error(w, http.StatusForbidden, "PIPELINE_FORBIDDEN", "current user cannot run this pipeline")
					return
				}
				now := time.Now()
				run := platform.PipelineRun{ID: s.Store.Next("pipeline_run"), PipelineID: id, TriggerUserID: actorID, RunType: "deploy", Status: "success", StartedAt: &now, FinishedAt: &now}
				s.Store.PipelineRuns = append(s.Store.PipelineRuns, run)
				s.Store.PipelineLogs = append(s.Store.PipelineLogs, platform.PipelineLog{ID: s.Store.Next("pipeline_log"), RunID: run.ID, StepName: "deploy", LogContent: "local scaffold pipeline run completed"})
				s.Store.Audit(actorID, "cicd.pipeline.run", "pipeline", id, "success", "trigger pipeline run")
				platform.JSON(w, http.StatusCreated, run)
			default:
				platform.Error(w, http.StatusNotFound, "ACTION_NOT_FOUND", "pipeline action not found")
			}
			return
		}
		platform.Error(w, http.StatusNotFound, "PIPELINE_NOT_FOUND", "pipeline not found")
	}
}

func runByID(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r.URL.Path, "/api/v1/cicd/runs/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		wantLogs := len(segments) > 5 && segments[5] == "logs"
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if wantLogs {
			if !canViewRun(s.Store, actorID, id) {
				platform.Error(w, http.StatusForbidden, "PIPELINE_RUN_FORBIDDEN", "current user cannot access this pipeline run")
				return
			}
			logs := make([]platform.PipelineLog, 0)
			for _, item := range s.Store.PipelineLogs {
				if item.RunID == id {
					logs = append(logs, item)
				}
			}
			platform.JSON(w, http.StatusOK, logs)
			return
		}
		for _, item := range s.Store.PipelineRuns {
			if item.ID == id {
				if !canViewPipeline(s.Store, actorID, item.PipelineID) {
					platform.Error(w, http.StatusForbidden, "PIPELINE_RUN_FORBIDDEN", "current user cannot access this pipeline run")
					return
				}
				platform.JSON(w, http.StatusOK, item)
				return
			}
		}
		platform.Error(w, http.StatusNotFound, "RUN_NOT_FOUND", "pipeline run not found")
	}
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func normalizeModule(item platform.PipelineModule) platform.PipelineModule {
	item.Key = strings.TrimSpace(item.Key)
	item.Name = strings.TrimSpace(item.Name)
	item.Category = strings.TrimSpace(item.Category)
	item.Runtime = strings.TrimSpace(item.Runtime)
	item.Description = strings.TrimSpace(item.Description)
	item.Command = strings.TrimSpace(item.Command)
	item.Status = fallback(strings.TrimSpace(item.Status), "enabled")
	if item.Key == "" {
		item.Key = strings.ToLower(strings.ReplaceAll(item.Name, " ", "_"))
	}
	if item.Name == "" {
		item.Name = item.Key
	}
	if item.Category == "" {
		item.Category = "custom"
	}
	for i := range item.Variables {
		item.Variables[i] = strings.TrimSpace(item.Variables[i])
	}
	return item
}

func normalizeTemplate(item platform.PipelineTemplate) platform.PipelineTemplate {
	item.Name = strings.TrimSpace(item.Name)
	item.ServiceType = platform.NormalizeServiceType(item.ServiceType)
	item.Status = fallback(strings.TrimSpace(item.Status), "enabled")
	if item.Name == "" {
		item.Name = item.ServiceType + "-k8s-template"
	}
	if len(item.Definition.Steps) == 0 {
		item.Definition = platform.DefaultPipelineDefinition(item.ServiceType)
	}
	if len(item.Definition.Triggers) == 0 {
		item.Definition.Triggers = []string{"manual"}
	}
	if item.Metadata != nil {
		language, _ := item.Metadata["runtimeLanguage"].(string)
		version, _ := item.Metadata["runtimeVersion"].(string)
		buildTool, _ := item.Metadata["buildTool"].(string)
		language = platform.NormalizeRuntimeLanguage(language)
		if language != "" {
			item.Metadata["runtimeLanguage"] = language
			item.Metadata["runtimeVersion"] = platform.NormalizeRuntimeVersion(language, version)
			item.Metadata["buildTool"] = platform.NormalizeBuildTool(language, buildTool)
		}
	}
	for i := range item.Definition.Steps {
		item.Definition.Steps[i].Name = strings.TrimSpace(item.Definition.Steps[i].Name)
		item.Definition.Steps[i].Type = strings.TrimSpace(item.Definition.Steps[i].Type)
		item.Definition.Steps[i].ModuleKey = strings.TrimSpace(item.Definition.Steps[i].ModuleKey)
		explicitModuleKey := item.Definition.Steps[i].ModuleKey != ""
		if item.Definition.Steps[i].ModuleKey == "" {
			item.Definition.Steps[i].ModuleKey = platform.DefaultPipelineModuleKey(item.Definition.Steps[i].Type)
		}
		if item.Definition.Steps[i].With == nil {
			item.Definition.Steps[i].With = map[string]any{}
		}
		command, _ := item.Definition.Steps[i].With["command"].(string)
		command = strings.TrimSpace(command)
		if command == "" && !explicitModuleKey {
			command = platform.DefaultPipelineStepCommand(item.Definition.Steps[i].Type)
		}
		if command != "" {
			item.Definition.Steps[i].With["command"] = command
		}
	}
	return item
}

func canManageTemplates(store *platform.Store, actorID int64) bool {
	return store.HasAnyRole(actorID, "platform_admin", "ops_owner")
}

func visiblePipelines(store *platform.Store, actorID int64) []platform.Pipeline {
	items := make([]platform.Pipeline, 0)
	for _, item := range store.Pipelines {
		if store.HasServiceAccess(actorID, item.ServiceID) {
			items = append(items, item)
		}
	}
	return items
}

func canViewPipeline(store *platform.Store, actorID, pipelineID int64) bool {
	for _, item := range store.Pipelines {
		if item.ID == pipelineID {
			return store.HasServiceAccess(actorID, item.ServiceID)
		}
	}
	return false
}

func canViewRun(store *platform.Store, actorID, runID int64) bool {
	for _, item := range store.PipelineRuns {
		if item.ID == runID {
			return canViewPipeline(store, actorID, item.PipelineID)
		}
	}
	return false
}
