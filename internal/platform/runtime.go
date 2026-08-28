package platform

import "strings"

type RuntimeProfile struct {
	Language        string
	Version         string
	BuildTool       string
	DetectionSource string
}

func NormalizeRuntimeLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "python", "py", "pip", "poetry", "pipenv", "django", "flask", "fastapi":
		return "python"
	case "c":
		return "c"
	case "cpp", "c++", "cplusplus", "cxx", "cc":
		return "cpp"
	case "dotnet", ".net", "net", "csharp", "c#", "aspnet":
		return "dotnet"
	case "rust", "cargo":
		return "rust"
	case "php", "composer", "laravel", "symfony":
		return "php"
	case "node", "nodejs", "javascript", "typescript", "vue", "react", "frontend":
		return "node"
	case "java", "jdk", "spring":
		return "java"
	case "go", "golang":
		return "go"
	default:
		return ""
	}
}

func NormalizeRuntimeVersion(language, version string) string {
	version = strings.TrimSpace(version)
	if version != "" {
		return version
	}
	switch NormalizeRuntimeLanguage(language) {
	case "python":
		return "3.11"
	case "c":
		return "c17"
	case "cpp":
		return "c++17"
	case "dotnet":
		return "8.0"
	case "rust":
		return "1.78"
	case "php":
		return "8.2"
	case "node":
		return "20"
	case "java":
		return "8"
	case "go":
		return "1.22"
	default:
		return ""
	}
}

func NormalizeBuildTool(language, value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "pip", "poetry", "pipenv":
		return strings.ToLower(strings.TrimSpace(value))
	case "make", "cmake", "conan":
		return strings.ToLower(strings.TrimSpace(value))
	case "dotnet":
		return "dotnet"
	case "cargo":
		return "cargo"
	case "composer":
		return "composer"
	case "pnpm", "npm", "yarn":
		return strings.ToLower(strings.TrimSpace(value))
	case "maven", "gradle":
		return strings.ToLower(strings.TrimSpace(value))
	case "go", "go build", "golang":
		return "go"
	}
	switch NormalizeRuntimeLanguage(language) {
	case "python":
		return "pip"
	case "c":
		return "make"
	case "cpp":
		return "cmake"
	case "dotnet":
		return "dotnet"
	case "rust":
		return "cargo"
	case "php":
		return "composer"
	case "node":
		return "pnpm"
	case "java":
		return "maven"
	case "go":
		return "go"
	default:
		return ""
	}
}

func InferRuntimeProfile(repositoryFullName, repositoryURL, serviceType string) RuntimeProfile {
	text := strings.ToLower(repositoryFullName + " " + repositoryURL)
	if containsRuntimeToken(text, "python", "django", "flask", "fastapi") ||
		containsDelimitedToken(text, "py") {
		return RuntimeProfile{Language: "python", Version: "3.11", BuildTool: "pip", DetectionSource: "repository_rule"}
	}
	if containsRuntimeToken(text, "rust", "cargo") {
		return RuntimeProfile{Language: "rust", Version: "1.78", BuildTool: "cargo", DetectionSource: "repository_rule"}
	}
	if containsRuntimeToken(text, "dotnet", "csharp", "aspnet") ||
		strings.Contains(text, ".net") {
		return RuntimeProfile{Language: "dotnet", Version: "8.0", BuildTool: "dotnet", DetectionSource: "repository_rule"}
	}
	if containsRuntimeToken(text, "cpp", "cxx", "cplusplus") ||
		strings.Contains(text, "c++") ||
		strings.Contains(text, "c-plus-plus") {
		return RuntimeProfile{Language: "cpp", Version: "c++17", BuildTool: "cmake", DetectionSource: "repository_rule"}
	}
	if containsDelimitedToken(text, "c") ||
		containsRuntimeToken(text, "clang") {
		return RuntimeProfile{Language: "c", Version: "c17", BuildTool: "make", DetectionSource: "repository_rule"}
	}
	if strings.Contains(text, "php") ||
		strings.Contains(text, "composer") ||
		strings.Contains(text, "laravel") ||
		strings.Contains(text, "symfony") {
		return RuntimeProfile{Language: "php", Version: "8.2", BuildTool: "composer", DetectionSource: "repository_rule"}
	}
	if NormalizeServiceType(serviceType) == "frontend" ||
		strings.Contains(text, "frontend") ||
		strings.Contains(text, "front-end") ||
		strings.Contains(text, "web") ||
		strings.Contains(text, "h5") ||
		strings.Contains(text, "vue") ||
		strings.Contains(text, "react") ||
		strings.Contains(text, "admin") {
		return RuntimeProfile{Language: "node", Version: "20", BuildTool: "pnpm", DetectionSource: "repository_rule"}
	}
	if strings.Contains(text, "java") ||
		strings.Contains(text, "jdk") ||
		strings.Contains(text, "spring") {
		return RuntimeProfile{Language: "java", Version: "8", BuildTool: "maven", DetectionSource: "repository_rule"}
	}
	return RuntimeProfile{Language: "go", Version: "1.22", BuildTool: "go", DetectionSource: "repository_rule"}
}

func containsRuntimeToken(text string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func containsDelimitedToken(text, token string) bool {
	for _, delimiter := range []string{"/", "-", "_", ".", " "} {
		if strings.Contains(text, delimiter+token+delimiter) ||
			strings.HasPrefix(text, token+delimiter) ||
			strings.HasSuffix(text, delimiter+token) {
			return true
		}
	}
	return false
}

func NormalizeRuntimeProfile(service Service) Service {
	profile := InferRuntimeProfile(service.RepositoryFullName, service.RepositoryURL, service.ServiceType)
	service.RuntimeLanguage = NormalizeRuntimeLanguage(service.RuntimeLanguage)
	if service.RuntimeLanguage == "" {
		service.RuntimeLanguage = profile.Language
	}
	service.RuntimeVersion = NormalizeRuntimeVersion(service.RuntimeLanguage, service.RuntimeVersion)
	service.BuildTool = NormalizeBuildTool(service.RuntimeLanguage, service.BuildTool)
	return service
}
