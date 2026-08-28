package apptree

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

type repositoryInspectRequest struct {
	RepositoryURL      string `json:"repositoryUrl"`
	RepositoryFullName string `json:"repositoryFullName"`
	ServiceType        string `json:"serviceType"`
}

type repositoryInspectResponse struct {
	RepositoryProvider string   `json:"repositoryProvider"`
	RepositoryFullName string   `json:"repositoryFullName"`
	AppID              string   `json:"appId"`
	RuntimeLanguage    string   `json:"runtimeLanguage"`
	RuntimeVersion     string   `json:"runtimeVersion"`
	BuildTool          string   `json:"buildTool"`
	PipelineTemplateID int64    `json:"pipelineTemplateId"`
	PipelineTemplate   string   `json:"pipelineTemplate"`
	DetectionSource    string   `json:"detectionSource"`
	Inspected          bool     `json:"inspected"`
	Evidence           []string `json:"evidence,omitempty"`
	Message            string   `json:"message,omitempty"`
}

func inspectRepository(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var req repositoryInspectRequest
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		req.RepositoryURL = strings.TrimSpace(req.RepositoryURL)
		req.RepositoryFullName = platform.NormalizeRepositoryFullName(req.RepositoryFullName)
		if req.RepositoryFullName == "" {
			req.RepositoryFullName = platform.RepositoryFullNameFromURL(req.RepositoryURL)
		}
		if req.RepositoryURL == "" && req.RepositoryFullName == "" {
			platform.Error(w, http.StatusBadRequest, "REPOSITORY_REQUIRED", "repository url is required")
			return
		}

		serviceType := platform.NormalizeServiceType(req.ServiceType)
		provider := platform.RepositoryProviderFromURL(req.RepositoryURL)
		appID := platform.NormalizeAppID(platform.RepositoryProjectName(req.RepositoryFullName, req.RepositoryURL))
		profile := platform.InferRuntimeProfile(req.RepositoryFullName, req.RepositoryURL, serviceType)
		inspected := false
		evidence := []string(nil)
		message := ""

		if provider == "gitlab" {
			remoteProfile, remoteEvidence, err := inspectGitLabRuntime(req.RepositoryURL, req.RepositoryFullName, serviceType)
			if err == nil {
				profile = remoteProfile
				evidence = remoteEvidence
				inspected = true
			} else {
				message = err.Error()
			}
		} else {
			message = "GitHub remote inspection is not enabled yet; used repository URL rules"
		}

		service := platform.NormalizeRuntimeProfile(platform.Service{
			Name:               appID,
			ServiceType:        serviceType,
			RepositoryProvider: provider,
			RepositoryFullName: req.RepositoryFullName,
			RepositoryURL:      req.RepositoryURL,
			RuntimeLanguage:    profile.Language,
			RuntimeVersion:     profile.Version,
			BuildTool:          profile.BuildTool,
		})
		templateID, templateName := recommendedTemplate(s, service)
		platform.JSON(w, http.StatusOK, repositoryInspectResponse{
			RepositoryProvider: provider,
			RepositoryFullName: req.RepositoryFullName,
			AppID:              appID,
			RuntimeLanguage:    service.RuntimeLanguage,
			RuntimeVersion:     service.RuntimeVersion,
			BuildTool:          service.BuildTool,
			PipelineTemplateID: templateID,
			PipelineTemplate:   templateName,
			DetectionSource:    fallback(profile.DetectionSource, "repository_rule"),
			Inspected:          inspected,
			Evidence:           evidence,
			Message:            message,
		})
	}
}

func recommendedTemplate(s *platform.Server, service platform.Service) (int64, string) {
	s.Store.Lock()
	defer s.Store.Unlock()
	templateID := s.Store.RecommendedPipelineTemplateID(service)
	if template, ok := s.Store.PipelineTemplateByID(templateID); ok {
		return templateID, template.Name
	}
	return templateID, ""
}

func inspectGitLabRuntime(repoURL, fullName, serviceType string) (platform.RuntimeProfile, []string, error) {
	if envTruthy(os.Getenv("SY_PLATFORM_GITLAB_INSPECT_DISABLED")) {
		return platform.RuntimeProfile{}, nil, errors.New("GitLab remote inspection disabled; used repository URL rules")
	}
	fullName = platform.NormalizeRepositoryFullName(fullName)
	if fullName == "" {
		return platform.RuntimeProfile{}, nil, errors.New("cannot inspect GitLab repository without project path; used repository URL rules")
	}
	apiBase, err := gitLabAPIBase(repoURL)
	if err != nil {
		return platform.RuntimeProfile{}, nil, fmt.Errorf("%v; used repository URL rules", err)
	}
	paths, err := fetchGitLabRepositoryTree(apiBase, fullName)
	if err != nil {
		return platform.RuntimeProfile{}, nil, fmt.Errorf("%v; used repository URL rules", err)
	}
	profile, evidence, ok := runtimeProfileFromRepositoryFiles(paths, serviceType)
	if !ok {
		return platform.RuntimeProfile{}, nil, errors.New("GitLab repository inspected but no supported language marker found; used repository URL rules")
	}
	profile.DetectionSource = "gitlab_repository"
	return profile, evidence, nil
}

func gitLabAPIBase(repoURL string) (string, error) {
	if configured := strings.TrimRight(strings.TrimSpace(os.Getenv("SY_PLATFORM_GITLAB_API_BASE")), "/"); configured != "" {
		return configured, nil
	}
	host := platform.RepositoryHostFromURL(repoURL)
	if host == "" {
		return "", errors.New("cannot parse GitLab host")
	}
	scheme := "https"
	if parsed, err := url.Parse(repoURL); err == nil && parsed.Scheme == "http" {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/api/v4", scheme, host), nil
}

func fetchGitLabRepositoryTree(apiBase, fullName string) ([]string, error) {
	client := http.Client{Timeout: 4 * time.Second}
	token := strings.TrimSpace(os.Getenv("SY_PLATFORM_GITLAB_TOKEN"))
	paths := make([]string, 0, 128)
	for page := 1; page <= 5; page++ {
		endpoint := fmt.Sprintf("%s/projects/%s/repository/tree?recursive=true&per_page=100&page=%d", apiBase, url.PathEscape(fullName), page)
		request, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			request.Header.Set("PRIVATE-TOKEN", token)
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("GitLab repository inspection failed: %w", err)
		}
		var entries []struct {
			Name string `json:"name"`
			Path string `json:"path"`
			Type string `json:"type"`
		}
		if response.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 512))
			_ = response.Body.Close()
			switch response.StatusCode {
			case http.StatusUnauthorized, http.StatusForbidden:
				return nil, errors.New("GitLab token is missing or has no repository read permission")
			case http.StatusNotFound:
				return nil, errors.New("GitLab project not found or token has no access")
			default:
				return nil, fmt.Errorf("GitLab repository inspection failed with status %d", response.StatusCode)
			}
		}
		if err := json.NewDecoder(response.Body).Decode(&entries); err != nil {
			_ = response.Body.Close()
			return nil, fmt.Errorf("GitLab repository inspection response invalid: %w", err)
		}
		_ = response.Body.Close()
		for _, entry := range entries {
			if entry.Path != "" {
				paths = append(paths, entry.Path)
			} else if entry.Name != "" {
				paths = append(paths, entry.Name)
			}
		}
		if len(entries) < 100 || response.Header.Get("X-Next-Page") == "" {
			break
		}
	}
	if len(paths) == 0 {
		return nil, errors.New("GitLab repository inspection returned no files")
	}
	return paths, nil
}

func runtimeProfileFromRepositoryFiles(paths []string, serviceType string) (platform.RuntimeProfile, []string, bool) {
	hasPackageJSON := false
	hasPHPFile := false
	hasPythonFile := false
	hasCFile := false
	hasCPPFile := false
	hasMakefile := false
	hasCMake := false
	for _, path := range paths {
		lowerPath := strings.ToLower(strings.TrimSpace(path))
		name := lowerPath
		if slash := strings.LastIndex(name, "/"); slash >= 0 {
			name = name[slash+1:]
		}
		switch name {
		case "composer.json":
			return platform.RuntimeProfile{Language: "php", Version: "8.2", BuildTool: "composer"}, []string{path}, true
		case "pyproject.toml":
			return platform.RuntimeProfile{Language: "python", Version: "3.11", BuildTool: "poetry"}, []string{path}, true
		case "requirements.txt", "setup.py":
			return platform.RuntimeProfile{Language: "python", Version: "3.11", BuildTool: "pip"}, []string{path}, true
		case "pipfile":
			return platform.RuntimeProfile{Language: "python", Version: "3.11", BuildTool: "pipenv"}, []string{path}, true
		case "cargo.toml":
			return platform.RuntimeProfile{Language: "rust", Version: "1.78", BuildTool: "cargo"}, []string{path}, true
		case "makefile":
			hasMakefile = true
		case "cmakelists.txt":
			hasCMake = true
		case "go.mod":
			return platform.RuntimeProfile{Language: "go", Version: "1.22", BuildTool: "go"}, []string{path}, true
		case "pom.xml":
			return platform.RuntimeProfile{Language: "java", Version: "8", BuildTool: "maven"}, []string{path}, true
		case "build.gradle", "build.gradle.kts":
			return platform.RuntimeProfile{Language: "java", Version: "8", BuildTool: "gradle"}, []string{path}, true
		case "package.json":
			hasPackageJSON = true
		}
		if strings.HasSuffix(lowerPath, ".csproj") || strings.HasSuffix(lowerPath, ".sln") {
			return platform.RuntimeProfile{Language: "dotnet", Version: "8.0", BuildTool: "dotnet"}, []string{path}, true
		}
		if strings.HasSuffix(lowerPath, ".py") {
			hasPythonFile = true
		}
		if strings.HasSuffix(lowerPath, ".php") {
			hasPHPFile = true
		}
		if strings.HasSuffix(lowerPath, ".c") || strings.HasSuffix(lowerPath, ".h") {
			hasCFile = true
		}
		if strings.HasSuffix(lowerPath, ".cpp") || strings.HasSuffix(lowerPath, ".cc") ||
			strings.HasSuffix(lowerPath, ".cxx") || strings.HasSuffix(lowerPath, ".hpp") ||
			strings.HasSuffix(lowerPath, ".hh") || strings.HasSuffix(lowerPath, ".hxx") {
			hasCPPFile = true
		}
	}
	if hasCMake && (hasCPPFile || !hasCFile) {
		return platform.RuntimeProfile{Language: "cpp", Version: "c++17", BuildTool: "cmake"}, []string{"CMakeLists.txt"}, true
	}
	if hasCMake && hasCFile {
		return platform.RuntimeProfile{Language: "c", Version: "c17", BuildTool: "cmake"}, []string{"CMakeLists.txt"}, true
	}
	if hasMakefile && hasCPPFile {
		return platform.RuntimeProfile{Language: "cpp", Version: "c++17", BuildTool: "make"}, []string{"Makefile"}, true
	}
	if hasMakefile && hasCFile {
		return platform.RuntimeProfile{Language: "c", Version: "c17", BuildTool: "make"}, []string{"Makefile"}, true
	}
	if hasPythonFile {
		return platform.RuntimeProfile{Language: "python", Version: "3.11", BuildTool: "pip"}, []string{"*.py"}, true
	}
	if hasPackageJSON {
		return platform.RuntimeProfile{Language: "node", Version: "20", BuildTool: "pnpm"}, []string{"package.json"}, true
	}
	if hasPHPFile {
		return platform.RuntimeProfile{Language: "php", Version: "8.2", BuildTool: "composer"}, []string{"*.php"}, true
	}
	if hasCPPFile {
		return platform.RuntimeProfile{Language: "cpp", Version: "c++17", BuildTool: "cmake"}, []string{"*.cpp"}, true
	}
	if hasCFile {
		return platform.RuntimeProfile{Language: "c", Version: "c17", BuildTool: "make"}, []string{"*.c"}, true
	}
	if platform.NormalizeServiceType(serviceType) == "frontend" {
		return platform.RuntimeProfile{Language: "node", Version: "20", BuildTool: "pnpm"}, []string{"serviceType=frontend"}, true
	}
	return platform.RuntimeProfile{}, nil, false
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
