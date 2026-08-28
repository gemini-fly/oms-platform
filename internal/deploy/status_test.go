package deploy

import "testing"

func TestReleaseTransitions(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		ok   bool
	}{
		{name: "created approving", from: ReleaseCreated, to: ReleaseApproving, ok: true},
		{name: "waiting window running", from: ReleaseWaitingWindow, to: ReleaseRunning, ok: true},
		{name: "health failed rollback", from: ReleaseHealthChecking, to: ReleaseRolledBack, ok: true},
		{name: "created cannot success", from: ReleaseCreated, to: ReleaseSuccess, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransition(tt.from, tt.to); got != tt.ok {
				t.Fatalf("CanTransition(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.ok)
			}
		})
	}
}
