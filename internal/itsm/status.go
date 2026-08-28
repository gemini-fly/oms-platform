package itsm

import "fmt"

const (
	TicketDraft      = "draft"
	TicketSubmitted  = "submitted"
	TicketApproving  = "approving"
	TicketProcessing = "processing"
	TicketVerifying  = "verifying"
	TicketRejected   = "rejected"
	TicketClosed     = "closed"
	TicketCanceled   = "canceled"
)

func CanTransition(from, to string) bool {
	allowed := map[string][]string{
		TicketDraft:      {TicketSubmitted, TicketCanceled},
		TicketSubmitted:  {TicketApproving, TicketCanceled},
		TicketApproving:  {TicketProcessing, TicketRejected, TicketCanceled},
		TicketProcessing: {TicketVerifying, TicketClosed, TicketCanceled},
		TicketVerifying:  {TicketClosed, TicketProcessing, TicketCanceled},
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
	return fmt.Errorf("ticket status transition %s -> %s is not allowed", from, to)
}
