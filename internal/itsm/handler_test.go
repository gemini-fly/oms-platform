package itsm

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func TestCreateTicketDefaultsAndAudits(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	body := bytes.NewBufferString(`{"ticketType":"change","title":"变更配置","description":"调整生产配置","scopeId":1}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"ticketNo":"ITSM-000002"`) {
		t.Fatalf("create response should contain generated ticket no: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("create response should contain draft status: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"scopeType":"application"`) {
		t.Fatalf("create response should force application scope: %s", rec.Body.String())
	}
	last := server.Store.AuditLogs[len(server.Store.AuditLogs)-1]
	if last.Action != "itsm.ticket.create" {
		t.Fatalf("audit action = %q, want itsm.ticket.create", last.Action)
	}
}

func TestCreateTicketRequiresTitle(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	server := platform.NewServer(platform.NewDemoStore())
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", bytes.NewBufferString(`{"ticketType":"change"}`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "title is required") {
		t.Fatalf("response should explain title requirement: %s", rec.Body.String())
	}
}

func TestCreateTicketUsesCurrentActorAndVisibleApplication(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	developerID := store.Users[3].ID
	setOnlyRole(t, store, developerID, "developer")
	server := platform.NewServer(store)
	Register(server)

	body := bytes.NewBufferString(`{"ticketType":"access","title":"申请权限","applicantUserId":1,"handlerUserId":1,"scopeId":1}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	for _, want := range []string{`"applicantUserId":4`, `"handlerUserId":4`, `"scopeType":"application"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("create response should contain %q: %s", want, rec.Body.String())
		}
	}
}

func TestTicketsAreFilteredByApplicationVisibility(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	store.ServiceMembers = nil
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/itsm/tickets", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "ITSM-000001") {
		t.Fatalf("developer without service access should not see application ticket: %s", rec.Body.String())
	}
}

func TestTicketApproveRequiresApproverRole(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	setOnlyRole(t, store, store.Users[3].ID, "developer")
	server := platform.NewServer(store)
	Register(server)

	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets/1/approve", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "TICKET_APPROVE_FORBIDDEN") {
		t.Fatalf("response should explain approval permission: %s", rec.Body.String())
	}
}

func TestCreateResourceRequestCreatesApprovalAndNotification(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	store.CloudAccounts = append(store.CloudAccounts, platform.CloudAccount{ID: store.Next("cloud_account"), Provider: "aliyun", Name: "阿里云生产账号", AccountRef: "aliyun-prod", Status: "enabled"})
	server := platform.NewServer(store)
	Register(server)

	body := bytes.NewBufferString(`{
		"provider":"aliyun",
		"accountId":1,
		"applicationId":1,
		"resourceType":"ecs",
		"resourceName":"order-worker",
		"region":"cn-beijing",
		"environment":"prod",
		"quantity":2,
		"approvalChannel":"dingtalk",
		"reason":"订单异步任务扩容",
		"spec":{"cpu":"4c","memory":"8g"}
	}`)
	rec := httptest.NewRecorder()
	server.Mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/itsm/resource-requests", body))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	for _, want := range []string{`"ticketType":"resource_request"`, `"status":"approving"`, `"providerResourceType":"ecs.instance"`, `"channel":"dingtalk"`, `"externalStatus":"pending_configuration"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response should contain %q: %s", want, rec.Body.String())
		}
	}
	if len(store.Approvals) == 0 || store.Approvals[len(store.Approvals)-1].Status != "pending" {
		t.Fatalf("resource request should create pending approval: %+v", store.Approvals)
	}
	if len(store.Notifications) == 0 || store.Notifications[len(store.Notifications)-1].Title != "资源申请待审批" {
		t.Fatalf("resource request should notify approver: %+v", store.Notifications)
	}
	last := store.AuditLogs[len(store.AuditLogs)-1]
	if last.Action != "itsm.ticket.transition" {
		t.Fatalf("last audit action = %q, want ticket transition", last.Action)
	}
}

func TestApproveResourceRequestQueuesProvisioning(t *testing.T) {
	t.Setenv("SY_PLATFORM_SETTINGS_FILE", filepath.Join(t.TempDir(), "settings.json"))
	store := platform.NewDemoStore()
	server := platform.NewServer(store)
	Register(server)

	createRec := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"provider":"aliyun","applicationId":1,"resourceType":"database","resourceName":"order-db","environment":"prod","quantity":1,"approvalChannel":"in_app","reason":"订单库独立实例","spec":{"engine":"mysql","storageGb":100}}`)
	server.Mux.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/api/v1/itsm/resource-requests", body))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d, body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	approveRec := httptest.NewRecorder()
	server.Mux.ServeHTTP(approveRec, httptest.NewRequest(http.MethodPost, "/api/v1/itsm/tickets/2/approve", nil))
	if approveRec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want %d, body=%s", approveRec.Code, http.StatusOK, approveRec.Body.String())
	}
	for _, want := range []string{`"status":"processing"`, `"status":"queued"`, `"nextAction":"create_cloud_resource"`} {
		if !strings.Contains(approveRec.Body.String(), want) {
			t.Fatalf("approve response should contain %q: %s", want, approveRec.Body.String())
		}
	}
	if store.Tickets[len(store.Tickets)-1].Payload["provisioning"].(map[string]any)["status"] != "queued" {
		t.Fatalf("resource request provisioning should be queued: %+v", store.Tickets[len(store.Tickets)-1].Payload)
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
