package deploy

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/deploy/releases", releases(s))
	s.Mux.HandleFunc("/api/v1/deploy/releases/", releaseAction(s))
	s.Mux.HandleFunc("/api/v1/deploy/windows", windows(s))
}

func releases(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if r.Method == http.MethodGet {
			items := visibleReleases(s.Store, actorID)
			platform.JSON(w, http.StatusOK, platform.Page[platform.Release]{Items: items, Page: 1, PageSize: len(items), Total: int64(len(items))})
			return
		}
		var item platform.Release
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if item.ServiceID == 0 || !s.Store.CanManageService(actorID, item.ServiceID) {
			platform.Error(w, http.StatusForbidden, "RELEASE_FORBIDDEN", "current user cannot create release for this service")
			return
		}
		item.ID = s.Store.Next("release")
		item.ReleaseNo = fmt.Sprintf("REL-%06d", item.ID)
		item.Status = ReleaseCreated
		if item.Strategy == "" {
			item.Strategy = "canary"
		}
		s.Store.Releases = append(s.Store.Releases, item)
		s.Store.Audit(actorID, "deploy.release.create", "release", item.ID, "success", item.Version)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func releaseAction(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r.URL.Path, "/api/v1/deploy/releases/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		action := ""
		if len(segments) > 5 {
			action = segments[5]
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		for i := range s.Store.Releases {
			if s.Store.Releases[i].ID != id {
				continue
			}
			if !canViewRelease(s.Store, actorID, s.Store.Releases[i]) {
				platform.Error(w, http.StatusForbidden, "RELEASE_FORBIDDEN", "current user cannot access this release")
				return
			}
			switch {
			case r.Method == http.MethodGet && action == "":
				platform.JSON(w, http.StatusOK, s.Store.Releases[i])
			case r.Method == http.MethodPost && action == "approve":
				if !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner", "approver") {
					platform.Error(w, http.StatusForbidden, "RELEASE_APPROVE_FORBIDDEN", "approver, ops, or platform admin role required")
					return
				}
				moveRelease(w, s, actorID, &s.Store.Releases[i], ReleaseWaitingWindow)
			case r.Method == http.MethodPost && action == "start":
				if !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner") {
					platform.Error(w, http.StatusForbidden, "RELEASE_OPERATE_FORBIDDEN", "ops or platform admin role required")
					return
				}
				if s.Store.Releases[i].Status == ReleaseCreated {
					s.Store.Releases[i].Status = ReleaseWaitingWindow
				}
				moveRelease(w, s, actorID, &s.Store.Releases[i], ReleaseRunning)
			case r.Method == http.MethodPost && action == "pause":
				if !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner") {
					platform.Error(w, http.StatusForbidden, "RELEASE_OPERATE_FORBIDDEN", "ops or platform admin role required")
					return
				}
				moveRelease(w, s, actorID, &s.Store.Releases[i], ReleaseFailed)
			case r.Method == http.MethodPost && action == "rollback":
				if !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner") {
					platform.Error(w, http.StatusForbidden, "RELEASE_OPERATE_FORBIDDEN", "ops or platform admin role required")
					return
				}
				moveRelease(w, s, actorID, &s.Store.Releases[i], ReleaseRolledBack)
			case r.Method == http.MethodGet && action == "health-checks":
				platform.JSON(w, http.StatusOK, healthChecks(s.Store.HealthChecks, id))
			case r.Method == http.MethodGet && action == "logs":
				platform.JSON(w, http.StatusOK, []string{"local scaffold release log"})
			default:
				platform.Error(w, http.StatusNotFound, "ACTION_NOT_FOUND", "release action not found")
			}
			return
		}
		platform.Error(w, http.StatusNotFound, "RELEASE_NOT_FOUND", "release not found")
	}
}

func moveRelease(w http.ResponseWriter, s *platform.Server, actorID int64, release *platform.Release, to string) {
	from := release.Status
	if err := Transition(from, to); err != nil {
		platform.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", err.Error())
		return
	}
	release.Status = to
	s.Store.Audit(actorID, "deploy.release.transition", "release", release.ID, "success", from+"->"+to)
	platform.JSON(w, http.StatusOK, release)
}

func healthChecks(items []platform.HealthCheck, releaseID int64) []platform.HealthCheck {
	out := make([]platform.HealthCheck, 0)
	for _, item := range items {
		if item.ReleaseID == releaseID {
			out = append(out, item)
		}
	}
	return out
}

func windows(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, []map[string]any{{"name": "default-production-window", "enabled": true}})
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		if !s.Store.HasAnyRole(platform.ActorID(r, s.Store), "platform_admin", "ops_owner") {
			platform.Error(w, http.StatusForbidden, "RELEASE_WINDOW_FORBIDDEN", "ops or platform admin role required")
			return
		}
		platform.JSON(w, http.StatusCreated, map[string]string{"status": "created"})
	}
}

func visibleReleases(store *platform.Store, actorID int64) []platform.Release {
	out := make([]platform.Release, 0)
	for _, item := range store.Releases {
		if canViewRelease(store, actorID, item) {
			out = append(out, item)
		}
	}
	return out
}

func canViewRelease(store *platform.Store, actorID int64, item platform.Release) bool {
	return item.ServiceID != 0 && store.HasServiceAccess(actorID, item.ServiceID)
}
