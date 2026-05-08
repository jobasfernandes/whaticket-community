package ticket

import (
	"github.com/canove/whaticket-community/backend/internal/auth"
)

func canSee(actor *auth.UserClaims, t *Ticket, userQueueIDs []uint) bool {
	if actor == nil {
		return false
	}
	if actor.Profile == "admin" {
		return true
	}
	if t.UserID != nil && *t.UserID == actor.ID {
		return true
	}
	if t.QueueID != nil && containsUint(userQueueIDs, *t.QueueID) {
		return true
	}
	if t.Status == "pending" && t.QueueID == nil {
		return true
	}
	return false
}

func canModify(actor *auth.UserClaims, t *Ticket, userQueueIDs []uint) bool {
	if actor == nil {
		return false
	}
	if actor.Profile == "admin" {
		return true
	}
	if !canSee(actor, t, userQueueIDs) {
		return false
	}
	if t.Status == "pending" {
		return true
	}
	if t.UserID != nil && *t.UserID == actor.ID {
		return true
	}
	return false
}

func containsUint(haystack []uint, needle uint) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}
