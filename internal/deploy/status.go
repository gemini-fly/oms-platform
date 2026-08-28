package deploy

import "fmt"

const (
	ReleaseCreated        = "created"
	ReleaseApproving      = "approving"
	ReleaseWaitingWindow  = "waiting_window"
	ReleaseRunning        = "running"
	ReleaseCanary         = "canary"
	ReleaseHealthChecking = "health_checking"
	ReleaseSuccess        = "success"
	ReleaseFailed         = "failed"
	ReleaseRolledBack     = "rolled_back"
)

func CanTransition(from, to string) bool {
	allowed := map[string][]string{
		ReleaseCreated:        {ReleaseApproving, ReleaseWaitingWindow},
		ReleaseApproving:      {ReleaseWaitingWindow, ReleaseFailed},
		ReleaseWaitingWindow:  {ReleaseRunning},
		ReleaseRunning:        {ReleaseCanary, ReleaseFailed},
		ReleaseCanary:         {ReleaseHealthChecking, ReleaseRolledBack, ReleaseFailed},
		ReleaseHealthChecking: {ReleaseSuccess, ReleaseRolledBack, ReleaseFailed},
		ReleaseSuccess:        {ReleaseRolledBack},
		ReleaseFailed:         {ReleaseRolledBack},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return true
		}
	}
	return false
}

func Transition(from, to string) error {
	if CanTransition(from, to) {
		return nil
	}
	return fmt.Errorf("release status transition %s -> %s is not allowed", from, to)
}
