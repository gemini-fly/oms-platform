package itsm

import "testing"

func TestTicketTransitions(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		ok   bool
	}{
		{name: "draft submit", from: TicketDraft, to: TicketSubmitted, ok: true},
		{name: "approving processing", from: TicketApproving, to: TicketProcessing, ok: true},
		{name: "approving reject", from: TicketApproving, to: TicketRejected, ok: true},
		{name: "closed cannot process", from: TicketClosed, to: TicketProcessing, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransition(tt.from, tt.to); got != tt.ok {
				t.Fatalf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.ok)
			}
		})
	}
}
