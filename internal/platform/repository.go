package platform

import (
	"errors"
	"net/url"
	"strings"
	"unicode"
)

var (
	ErrAppIDRequired      = errors.New("appId is required")
	ErrAppIDInvalid       = errors.New("appId can only contain letters, numbers, dot and hyphen")
	ErrAppIDTooLong       = errors.New("appId is too long")
	ErrRepositoryRequired = errors.New("repositoryFullName or repositoryUrl is required")
)

func NormalizeAppID(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "_", "-")
}

func ValidateAppID(value string) error {
	value = NormalizeAppID(value)
	if value == "" {
		return ErrAppIDRequired
	}
	if len(value) > 80 {
		return ErrAppIDTooLong
	}
	for _, ch := range value {
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '.' || ch == '-' {
			continue
		}
		return ErrAppIDInvalid
	}
	return nil
}

func NormalizeRepositoryProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "github":
		return "github"
	default:
		return "gitlab"
	}
}

func RepositoryProviderFromURL(repoURL string) string {
	if strings.Contains(strings.ToLower(repoURL), "github.com") {
		return "github"
	}
	return "gitlab"
}

func NormalizeRepositoryFullName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "/")
	value = strings.TrimSuffix(value, "/")
	value = strings.TrimSuffix(value, ".git")
	return value
}

func RepositoryFullNameFromURL(repoURL string) string {
	value := strings.TrimSpace(repoURL)
	if value == "" {
		return ""
	}
	value = strings.TrimSuffix(value, ".git")
	if at := strings.Index(value, "@"); at >= 0 {
		if colon := strings.Index(value[at:], ":"); colon >= 0 {
			value = value[at+colon+1:]
		}
	} else if strings.Contains(value, "://") {
		if parsed, err := url.Parse(value); err == nil {
			value = parsed.Path
		}
	}
	return NormalizeRepositoryFullName(value)
}

func RepositoryHostFromURL(repoURL string) string {
	value := strings.TrimSpace(repoURL)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(parsed.Hostname())
	}
	if at := strings.Index(value, "@"); at >= 0 {
		rest := value[at+1:]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			return strings.TrimSpace(rest[:colon])
		}
	}
	return ""
}

func RepositoryProjectName(fullName, repoURL string) string {
	candidate := NormalizeRepositoryFullName(fullName)
	if candidate == "" {
		candidate = RepositoryFullNameFromURL(repoURL)
	}
	candidate = strings.TrimSuffix(candidate, ".git")
	if at := strings.Index(candidate, "@"); at >= 0 {
		if colon := strings.Index(candidate[at:], ":"); colon >= 0 {
			candidate = candidate[at+colon+1:]
		}
	}
	candidate = strings.Trim(candidate, "/")
	if candidate == "" {
		return ""
	}
	parts := strings.Split(candidate, "/")
	return strings.TrimSuffix(parts[len(parts)-1], ".git")
}

func ValidateRepositoryForAppID(appID, fullName, repoURL string) error {
	if NormalizeRepositoryFullName(fullName) == "" && strings.TrimSpace(repoURL) == "" {
		return ErrRepositoryRequired
	}
	projectName := RepositoryProjectName(fullName, repoURL)
	if projectName == "" {
		return ErrRepositoryRequired
	}
	if NormalizeAppID(projectName) != NormalizeAppID(appID) {
		return errors.New("repository project name must equal appId")
	}
	return nil
}
