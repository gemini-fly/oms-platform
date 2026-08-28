package itsm

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

type resourceRequestPayload struct {
	Provider        string         `json:"provider"`
	AccountID       int64          `json:"accountId"`
	ApplicationID   int64          `json:"applicationId"`
	ResourceType    string         `json:"resourceType"`
	ResourceName    string         `json:"resourceName"`
	Region          string         `json:"region"`
	Environment     string         `json:"environment"`
	Quantity        int            `json:"quantity"`
	Spec            map[string]any `json:"spec,omitempty"`
	Reason          string         `json:"reason"`
	ApprovalChannel string         `json:"approvalChannel"`
}

type normalizedResourceRequest struct {
	resourceRequestPayload
	ProviderResourceType string
	ResourceLabel        string
	AccountRef           string
}

type approvalDispatch struct {
	Channel        string `json:"channel"`
	ExternalStatus string `json:"externalStatus"`
	ExternalID     string `json:"externalId,omitempty"`
	Message        string `json:"message"`
}

func resourceRequests(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platform.Method(w, r, http.MethodPost) {
			return
		}
		var req resourceRequestPayload
		if err := platform.Decode(r, &req); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}

		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)

		normalized, err := normalizeResourceRequest(s.Store, actorID, req)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "cannot create resource request for this application") {
				status = http.StatusForbidden
			}
			platform.Error(w, status, "RESOURCE_REQUEST_INVALID", err.Error())
			return
		}

		approver := firstUserWithRole(s.Store, "ops_owner", "platform_admin", "approver")
		if approver.ID == 0 {
			approver = s.Store.UserByID(actorID)
		}
		ticket := platform.Ticket{
			ID:              s.Store.Next("ticket"),
			TicketType:      "resource_request",
			Title:           resourceRequestTitle(normalized),
			Description:     normalized.Reason,
			ApplicantUserID: actorID,
			HandlerUserID:   approver.ID,
			Status:          TicketApproving,
			ScopeType:       "application",
			ScopeID:         normalized.ApplicationID,
			Payload: map[string]any{
				"category": "cloud_resource_request",
				"resourceRequest": map[string]any{
					"provider":             normalized.Provider,
					"accountId":            normalized.AccountID,
					"accountRef":           normalized.AccountRef,
					"applicationId":        normalized.ApplicationID,
					"resourceType":         normalized.ResourceType,
					"providerResourceType": normalized.ProviderResourceType,
					"resourceLabel":        normalized.ResourceLabel,
					"resourceName":         normalized.ResourceName,
					"region":               normalized.Region,
					"environment":          normalized.Environment,
					"quantity":             normalized.Quantity,
					"spec":                 normalized.Spec,
					"reason":               normalized.Reason,
				},
				"approval": map[string]any{
					"required":            true,
					"status":              "pending",
					"channel":             normalized.ApprovalChannel,
					"approverUserId":      approver.ID,
					"approverDisplayName": approver.DisplayName,
					"dispatch":            approvalDispatchFor(s.Store, normalized.ApprovalChannel),
				},
				"provisioning": map[string]any{
					"status":     "waiting_approval",
					"nextAction": "approve_then_create_resource",
				},
			},
		}
		ticket.TicketNo = fmt.Sprintf("ITSM-%06d", ticket.ID)
		s.Store.Tickets = append(s.Store.Tickets, ticket)
		s.Store.Approvals = append(s.Store.Approvals, platform.Approval{
			ID:             s.Store.Next("approval"),
			TicketID:       ticket.ID,
			StepNo:         1,
			ApproverUserID: approver.ID,
			Status:         "pending",
			Comment:        "cloud resource request approval",
		})
		s.Store.Notifications = append(s.Store.Notifications, platform.Notification{
			ID:             s.Store.Next("notification"),
			ReceiverUserID: approver.ID,
			Channel:        "in_app",
			Title:          "资源申请待审批",
			Content:        fmt.Sprintf("%s 申请 %s %d 个，工单 %s", s.Store.UserByID(actorID).DisplayName, normalized.ResourceLabel, normalized.Quantity, ticket.TicketNo),
			Status:         "unread",
		})
		s.Store.Audit(actorID, "itsm.resource_request.create", "ticket", ticket.ID, "success", ticket.Title)
		s.Store.Audit(actorID, "itsm.ticket.transition", "ticket", ticket.ID, "success", "created->approving")
		platform.JSON(w, http.StatusCreated, ticket)
	}
}

func normalizeResourceRequest(store *platform.Store, actorID int64, req resourceRequestPayload) (normalizedResourceRequest, error) {
	provider := platform.NormalizeCloudProvider(req.Provider)
	if provider == "" {
		provider = "aliyun"
	}
	if !platform.SupportedCloudProvider(provider) {
		return normalizedResourceRequest{}, fmt.Errorf("unsupported cloud provider: %s", req.Provider)
	}
	resourceType, label, providerResourceType, err := normalizeRequestedResourceType(provider, req.ResourceType)
	if err != nil {
		return normalizedResourceRequest{}, err
	}
	applicationID := fallbackID(req.ApplicationID, defaultVisibleApplicationID(store, actorID))
	if applicationID == 0 || !store.HasApplicationAccess(actorID, applicationID) {
		return normalizedResourceRequest{}, fmt.Errorf("cannot create resource request for this application")
	}
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	if quantity > 50 {
		return normalizedResourceRequest{}, fmt.Errorf("quantity must be <= 50")
	}
	accountRef := ""
	if req.AccountID != 0 {
		account, ok := findCloudAccount(store.CloudAccounts, req.AccountID)
		if !ok {
			return normalizedResourceRequest{}, fmt.Errorf("cloud account not found")
		}
		if platform.NormalizeCloudProvider(account.Provider) != provider {
			return normalizedResourceRequest{}, fmt.Errorf("cloud account provider does not match request provider")
		}
		accountRef = account.AccountRef
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return normalizedResourceRequest{}, fmt.Errorf("reason is required")
	}
	name := strings.TrimSpace(req.ResourceName)
	if name == "" {
		name = resourceType + "-" + strings.ToLower(strings.TrimSpace(req.Environment))
	}
	return normalizedResourceRequest{
		resourceRequestPayload: resourceRequestPayload{
			Provider:        provider,
			AccountID:       req.AccountID,
			ApplicationID:   applicationID,
			ResourceType:    resourceType,
			ResourceName:    name,
			Region:          fallback(strings.TrimSpace(req.Region), "auto"),
			Environment:     fallback(strings.ToLower(strings.TrimSpace(req.Environment)), "test"),
			Quantity:        quantity,
			Spec:            sanitizeResourceSpec(req.Spec),
			Reason:          reason,
			ApprovalChannel: normalizeApprovalChannel(req.ApprovalChannel),
		},
		ProviderResourceType: providerResourceType,
		ResourceLabel:        label,
		AccountRef:           accountRef,
	}, nil
}

func normalizeRequestedResourceType(provider, value string) (string, string, string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	switch normalized {
	case "ecs", "server", "vm", "ecs.instance", "cvm.instance", "ecs.cloudservers":
		return "ecs", "ECS 云服务器", providerResourceType(provider, "ecs"), nil
	case "slb", "lb", "loadbalancer", "load-balancer", "slb.loadbalancer", "clb.loadbalancer", "elb.loadbalancers":
		return "slb", "负载均衡", providerResourceType(provider, "slb"), nil
	case "database", "db", "rds", "mysql", "postgres", "rds.instance", "rds.instances", "cdb.instance":
		return "database", "数据库", providerResourceType(provider, "database"), nil
	default:
		return "", "", "", fmt.Errorf("unsupported resource type: %s", value)
	}
}

func providerResourceType(provider, resourceType string) string {
	switch platform.NormalizeCloudProvider(provider) + "/" + resourceType {
	case "aliyun/ecs":
		return "ecs.instance"
	case "aliyun/slb":
		return "slb.loadbalancer"
	case "aliyun/database":
		return "rds.instance"
	case "tencent/ecs":
		return "cvm.instance"
	case "tencent/slb":
		return "clb.loadbalancer"
	case "tencent/database":
		return "cdb.instance"
	case "huawei/ecs":
		return "ecs.cloudservers"
	case "huawei/slb":
		return "elb.loadbalancers"
	case "huawei/database":
		return "rds.instances"
	default:
		return resourceType
	}
}

func normalizeApprovalChannel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "dingtalk", "dingding", "钉钉":
		return "dingtalk"
	case "feishu", "lark", "飞书":
		return "feishu"
	default:
		return "in_app"
	}
}

func approvalDispatchFor(store *platform.Store, channel string) approvalDispatch {
	channel = normalizeApprovalChannel(channel)
	switch channel {
	case "dingtalk":
		if dingtalkApprovalConfigReady(store.DingTalkConfig) {
			return approvalDispatch{Channel: channel, ExternalStatus: "pending_process_code", Message: "钉钉应用凭据已配置；还需要审批模板 processCode 后才能推送外部审批单"}
		}
		return approvalDispatch{Channel: channel, ExternalStatus: "pending_configuration", Message: "钉钉审批未配置完整，当前仅创建站内审批"}
	case "feishu":
		return approvalDispatch{Channel: channel, ExternalStatus: "pending_configuration", Message: "飞书审批应用未配置，当前仅创建站内审批"}
	default:
		return approvalDispatch{Channel: "in_app", ExternalStatus: "local_pending", Message: "站内审批已创建"}
	}
}

func dingtalkApprovalConfigReady(config platform.DingTalkOrgConfig) bool {
	return strings.EqualFold(config.Status, "enabled") &&
		strings.TrimSpace(config.CorpID) != "" &&
		strings.TrimSpace(config.AppKey) != "" &&
		strings.TrimSpace(config.AppSecret) != "" &&
		strings.TrimSpace(config.AgentID) != ""
}

func sanitizeResourceSpec(spec map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range spec {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func resourceRequestTitle(req normalizedResourceRequest) string {
	return fmt.Sprintf("申请%s：%s x%d / %s / %s", req.ResourceLabel, req.ResourceName, req.Quantity, platform.CloudProviderDisplayName(req.Provider), req.Environment)
}

func findCloudAccount(items []platform.CloudAccount, id int64) (platform.CloudAccount, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return platform.CloudAccount{}, false
}

func firstUserWithRole(store *platform.Store, roleCodes ...string) platform.User {
	wanted := map[string]bool{}
	for _, roleCode := range roleCodes {
		wanted[roleCode] = true
	}
	for _, binding := range store.PolicyBindings {
		if !wanted[binding.RoleCode] {
			continue
		}
		for _, user := range store.Users {
			if user.ID == binding.UserID && user.Status != "disabled" {
				return user
			}
		}
	}
	return platform.User{}
}

func afterTicketApproved(store *platform.Store, actorID int64, ticket *platform.Ticket) {
	if ticket.TicketType != "resource_request" {
		return
	}
	mergeTicketPayloadMap(ticket, "approval", map[string]any{"status": "approved", "approvedAt": time.Now().Format(time.RFC3339)})
	mergeTicketPayloadMap(ticket, "provisioning", map[string]any{"status": "queued", "nextAction": "create_cloud_resource"})
	store.Notifications = append(store.Notifications, platform.Notification{
		ID:             store.Next("notification"),
		ReceiverUserID: ticket.ApplicantUserID,
		Channel:        "in_app",
		Title:          "资源申请已审批通过",
		Content:        fmt.Sprintf("%s 已通过，等待自动开通资源", ticket.TicketNo),
		Status:         "unread",
	})
	store.Audit(actorID, "itsm.resource_request.approve", "ticket", ticket.ID, "success", ticket.TicketNo)
}

func afterTicketRejected(store *platform.Store, actorID int64, ticket *platform.Ticket) {
	if ticket.TicketType != "resource_request" {
		return
	}
	mergeTicketPayloadMap(ticket, "approval", map[string]any{"status": "rejected", "rejectedAt": time.Now().Format(time.RFC3339)})
	mergeTicketPayloadMap(ticket, "provisioning", map[string]any{"status": "rejected", "nextAction": "none"})
	store.Notifications = append(store.Notifications, platform.Notification{
		ID:             store.Next("notification"),
		ReceiverUserID: ticket.ApplicantUserID,
		Channel:        "in_app",
		Title:          "资源申请已驳回",
		Content:        fmt.Sprintf("%s 已驳回", ticket.TicketNo),
		Status:         "unread",
	})
	store.Audit(actorID, "itsm.resource_request.reject", "ticket", ticket.ID, "success", ticket.TicketNo)
}

func mergeTicketPayloadMap(ticket *platform.Ticket, key string, fields map[string]any) {
	if ticket.Payload == nil {
		ticket.Payload = map[string]any{}
	}
	current, _ := ticket.Payload[key].(map[string]any)
	if current == nil {
		current = map[string]any{}
	}
	for field, value := range fields {
		current[field] = value
	}
	ticket.Payload[key] = current
}
