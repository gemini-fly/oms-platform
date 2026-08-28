package notify

import (
	"net/http"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/notifications", notifications(s))
	s.Mux.HandleFunc("/api/v1/notifications/", notificationByID(s))
}

func notifications(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodGet) {
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		items := notificationsForUser(s.Store.Notifications, platform.ActorID(r, s.Store))
		platform.JSON(w, http.StatusOK, platform.Page[platform.Notification]{Items: items, Page: 1, PageSize: len(items), Total: int64(len(items))})
	}
}

func notificationByID(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r.URL.Path, "/api/v1/notifications/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		for i := range s.Store.Notifications {
			if s.Store.Notifications[i].ID == id {
				if s.Store.Notifications[i].ReceiverUserID != platform.ActorID(r, s.Store) {
					platform.Error(w, http.StatusForbidden, "NOTIFICATION_FORBIDDEN", "current user cannot read this notification")
					return
				}
				s.Store.Notifications[i].Status = "read"
				platform.JSON(w, http.StatusOK, s.Store.Notifications[i])
				return
			}
		}
		platform.Error(w, http.StatusNotFound, "NOTIFICATION_NOT_FOUND", "notification not found")
	}
}

func notificationsForUser(items []platform.Notification, userID int64) []platform.Notification {
	out := make([]platform.Notification, 0)
	for _, item := range items {
		if item.ReceiverUserID == userID {
			out = append(out, item)
		}
	}
	return out
}
