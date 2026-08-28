package notify

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestNotificationsAreScopedToCurrentUser(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	developerID := store.Users[3].ID
	store.CurrentUserID = developerID
	store.Notifications = append(store.Notifications, platform.Notification{
		ID:             store.Next("notification"),
		ReceiverUserID: developerID,
		Channel:        "in_app",
		Title:          "开发通知",
		Content:        "only developer can see this",
		Status:         "unread",
	})
	server := platform.NewServer(store)
	Register(server)

	listRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil))

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d, body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	if strings.Contains(listRec.Body.String(), "生产发布待健康确认") {
		t.Fatalf("list should not include another user's notification: %s", listRec.Body.String())
	}
	if !strings.Contains(listRec.Body.String(), "开发通知") {
		t.Fatalf("list should include current user's notification: %s", listRec.Body.String())
	}

	readOtherRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(readOtherRec, httptest.NewRequest(http.MethodGet, "/api/v1/notifications/1", nil))
	if readOtherRec.Code != http.StatusForbidden {
		t.Fatalf("read other status = %d, want %d, body=%s", readOtherRec.Code, http.StatusForbidden, readOtherRec.Body.String())
	}
}
