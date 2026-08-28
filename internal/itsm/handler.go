package itsm

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func Register(s *platform.Server) {
	s.Mux.HandleFunc("/api/v1/itsm/tickets", tickets(s))
	s.Mux.HandleFunc("/api/v1/itsm/tickets/", ticketAction(s))
	s.Mux.HandleFunc("/api/v1/itsm/resource-requests", resourceRequests(s))
	s.Mux.HandleFunc("/api/v1/itsm/knowledge", knowledge(s))
}

func tickets(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if r.Method == http.MethodGet {
			items := visibleTickets(s.Store, actorID)
			platform.JSON(w, http.StatusOK, platform.Page[platform.Ticket]{Items: items, Page: 1, PageSize: len(items), Total: int64(len(items))})
			return
		}
		var item platform.Ticket
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		item.Title = strings.TrimSpace(item.Title)
		item.TicketType = strings.TrimSpace(item.TicketType)
		if item.Title == "" {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", "title is required")
			return
		}
		item.ID = s.Store.Next("ticket")
		item.TicketNo = fmt.Sprintf("ITSM-%06d", item.ID)
		item.TicketType = fallback(item.TicketType, "general")
		item.ApplicantUserID = actorID
		item.HandlerUserID = actorID
		item.ScopeType = "application"
		item.ScopeID = fallbackID(item.ScopeID, defaultVisibleApplicationID(s.Store, actorID))
		if item.ScopeID == 0 || !s.Store.HasApplicationAccess(actorID, item.ScopeID) {
			platform.Error(w, http.StatusForbidden, "TICKET_SCOPE_FORBIDDEN", "current user cannot create ticket for this application")
			return
		}
		item.Status = fallback(item.Status, TicketDraft)
		s.Store.Tickets = append(s.Store.Tickets, item)
		s.Store.Audit(actorID, "itsm.ticket.create", "ticket", item.ID, "success", item.Title)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func ticketAction(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := platform.PathID(r.URL.Path, "/api/v1/itsm/tickets/")
		if err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		action := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, fmt.Sprintf("/api/v1/itsm/tickets/%d", id)), "/")
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		for i := range s.Store.Tickets {
			if s.Store.Tickets[i].ID != id {
				continue
			}
			if !canViewTicket(s.Store, actorID, s.Store.Tickets[i]) {
				platform.Error(w, http.StatusForbidden, "TICKET_FORBIDDEN", "current user cannot access this ticket")
				return
			}
			switch action {
			case "":
				if !platform.Method(w, r, http.MethodGet) {
					return
				}
				platform.JSON(w, http.StatusOK, s.Store.Tickets[i])
			case "submit":
				if !platform.Method(w, r, http.MethodPost) {
					return
				}
				if !canOperateTicket(s.Store, actorID, s.Store.Tickets[i]) {
					platform.Error(w, http.StatusForbidden, "TICKET_FORBIDDEN", "current user cannot submit this ticket")
					return
				}
				moveTicket(w, s, actorID, &s.Store.Tickets[i], TicketSubmitted)
			case "approve":
				if !platform.Method(w, r, http.MethodPost) {
					return
				}
				if !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner", "approver") {
					platform.Error(w, http.StatusForbidden, "TICKET_APPROVE_FORBIDDEN", "approver, ops, or platform admin role required")
					return
				}
				from, err := transitionTicket(&s.Store.Tickets[i], TicketProcessing)
				if err != nil {
					platform.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", err.Error())
					return
				}
				now := time.Now()
				s.Store.Approvals = append(s.Store.Approvals, platform.Approval{ID: s.Store.Next("approval"), TicketID: id, StepNo: nextApprovalStep(s.Store, id), ApproverUserID: actorID, Status: "approved", Comment: "approved", ApprovedAt: &now})
				afterTicketApproved(s.Store, actorID, &s.Store.Tickets[i])
				s.Store.Audit(actorID, "itsm.ticket.transition", "ticket", s.Store.Tickets[i].ID, "success", from+"->"+TicketProcessing)
				platform.JSON(w, http.StatusOK, s.Store.Tickets[i])
			case "reject":
				if !platform.Method(w, r, http.MethodPost) {
					return
				}
				if !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner", "approver") {
					platform.Error(w, http.StatusForbidden, "TICKET_APPROVE_FORBIDDEN", "approver, ops, or platform admin role required")
					return
				}
				from, err := transitionTicket(&s.Store.Tickets[i], TicketRejected)
				if err != nil {
					platform.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", err.Error())
					return
				}
				afterTicketRejected(s.Store, actorID, &s.Store.Tickets[i])
				s.Store.Audit(actorID, "itsm.ticket.transition", "ticket", s.Store.Tickets[i].ID, "success", from+"->"+TicketRejected)
				platform.JSON(w, http.StatusOK, s.Store.Tickets[i])
			default:
				platform.Error(w, http.StatusNotFound, "ACTION_NOT_FOUND", "ticket action not found")
			}
			return
		}
		platform.Error(w, http.StatusNotFound, "TICKET_NOT_FOUND", "ticket not found")
	}
}

func moveTicket(w http.ResponseWriter, s *platform.Server, actorID int64, ticket *platform.Ticket, to string) {
	from, err := transitionTicket(ticket, to)
	if err != nil {
		platform.Error(w, http.StatusConflict, "INVALID_STATUS_TRANSITION", err.Error())
		return
	}
	s.Store.Audit(actorID, "itsm.ticket.transition", "ticket", ticket.ID, "success", from+"->"+to)
	platform.JSON(w, http.StatusOK, ticket)
}

func transitionTicket(ticket *platform.Ticket, to string) (string, error) {
	from := ticket.Status
	if from == TicketDraft && to == TicketProcessing {
		from = TicketApproving
		ticket.Status = TicketApproving
	}
	if from == TicketSubmitted && to == TicketProcessing {
		ticket.Status = TicketApproving
		from = TicketApproving
	}
	if err := Transition(from, to); err != nil {
		return "", err
	}
	ticket.Status = to
	return from, nil
}

func nextApprovalStep(store *platform.Store, ticketID int64) int {
	step := 1
	for _, approval := range store.Approvals {
		if approval.TicketID == ticketID && approval.StepNo >= step {
			step = approval.StepNo + 1
		}
	}
	return step
}

func knowledge(s *platform.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			platform.Error(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
			return
		}
		s.Store.Lock()
		defer s.Store.Unlock()
		actorID := platform.ActorID(r, s.Store)
		if r.Method == http.MethodGet {
			platform.JSON(w, http.StatusOK, visibleKnowledge(s.Store, actorID))
			return
		}
		if !s.Store.HasAnyRole(actorID, "platform_admin", "ops_owner") {
			platform.Error(w, http.StatusForbidden, "KNOWLEDGE_FORBIDDEN", "ops or platform admin role required")
			return
		}
		var item platform.Knowledge
		if err := platform.Decode(r, &item); err != nil {
			platform.Error(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
			return
		}
		if !canAccessScope(s.Store, actorID, item.ScopeType, item.ScopeID) {
			platform.Error(w, http.StatusForbidden, "KNOWLEDGE_SCOPE_FORBIDDEN", "current user cannot create knowledge for this scope")
			return
		}
		item.ID = s.Store.Next("knowledge")
		item.Status = "enabled"
		s.Store.Knowledge = append(s.Store.Knowledge, item)
		s.Store.Audit(actorID, "itsm.knowledge.create", "knowledge", item.ID, "success", item.Title)
		platform.JSON(w, http.StatusCreated, item)
	}
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func fallbackID(value, defaultValue int64) int64 {
	if value == 0 {
		return defaultValue
	}
	return value
}

func defaultVisibleApplicationID(store *platform.Store, actorID int64) int64 {
	visible := store.VisibleApplicationIDsForUser(actorID)
	for _, app := range store.Applications {
		if visible[app.ID] {
			return app.ID
		}
	}
	return 0
}

func visibleTickets(store *platform.Store, actorID int64) []platform.Ticket {
	out := make([]platform.Ticket, 0)
	for _, item := range store.Tickets {
		if canViewTicket(store, actorID, item) {
			out = append(out, item)
		}
	}
	return out
}

func canViewTicket(store *platform.Store, actorID int64, ticket platform.Ticket) bool {
	if store.HasAnyRole(actorID, "platform_admin", "ops_owner", "approver") {
		return true
	}
	if ticket.ApplicantUserID == actorID || ticket.HandlerUserID == actorID {
		return true
	}
	return canAccessScope(store, actorID, ticket.ScopeType, ticket.ScopeID)
}

func canOperateTicket(store *platform.Store, actorID int64, ticket platform.Ticket) bool {
	if store.HasAnyRole(actorID, "platform_admin", "ops_owner") {
		return true
	}
	return ticket.ApplicantUserID == actorID || ticket.HandlerUserID == actorID
}

func canAccessScope(store *platform.Store, actorID int64, scopeType string, scopeID int64) bool {
	switch scopeType {
	case "application":
		return store.HasApplicationAccess(actorID, scopeID)
	case "service":
		return store.HasServiceAccess(actorID, scopeID)
	default:
		return false
	}
}

func visibleKnowledge(store *platform.Store, actorID int64) []platform.Knowledge {
	if store.HasAnyRole(actorID, "platform_admin", "ops_owner") {
		return store.Knowledge
	}
	out := make([]platform.Knowledge, 0)
	for _, item := range store.Knowledge {
		if canAccessScope(store, actorID, item.ScopeType, item.ScopeID) {
			out = append(out, item)
		}
	}
	return out
}
