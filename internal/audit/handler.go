package audit

import (
	"net/http"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/audit/logs", logs(s))
}

func logs(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		if !s.Store.HasAnyRole(platform.ActorID(r, s.Store), "platform_admin", "ops_owner") {
			platform.Error(w, http.StatusForbidden, "AUDIT_FORBIDDEN", "ops or platform admin role required")
			return
		}
		items := make([]platform.AuditLogView, 0, len(s.Store.AuditLogs))
		for i := len(s.Store.AuditLogs) - 1; i >= 0; i-- {
			items = append(items, toView(s.Store.AuditLogs[i], s.Store.Users))
		}
		platform.JSON(w, http.StatusOK, platform.Page[platform.AuditLogView]{Items: items, Page: 1, PageSize: len(items), Total: int64(len(items))})
	}
}

func toView(log platform.AuditLog, users []platform.User) platform.AuditLogView {
	username := "system"
	displayName := "System"
	for _, user := range users {
		if user.ID == log.ActorUserID {
			username = user.Username
			displayName = user.DisplayName
			break
		}
	}
	return platform.AuditLogView{
		ID:               log.ID,
		ActorUserID:      log.ActorUserID,
		ActorUsername:    username,
		ActorDisplayName: displayName,
		Action:           log.Action,
		ResourceType:     log.ResourceType,
		ResourceID:       log.ResourceID,
		Result:           log.Result,
		Reason:           log.Reason,
		CreatedAt:        log.CreatedAt,
	}
}
