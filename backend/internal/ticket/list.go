package ticket

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/canove/whaticket-community/backend/internal/auth"
	"github.com/canove/whaticket-community/backend/internal/platform/errors"
)

const ticketListPageSize = 40

type ListParams struct {
	SearchParam        string
	PageNumber         int
	Status             []string
	QueueIDs           []*uint
	UserID             *uint
	WithUnreadMessages *bool
	ShowAll            bool
	Date               *time.Time
}

func (d *Deps) List(ctx context.Context, params ListParams, actor *auth.UserClaims) ([]Ticket, int64, bool, *errors.AppError) {
	if actor == nil {
		return nil, 0, false, errors.New(errNoPermission, http.StatusForbidden)
	}
	if params.PageNumber < 1 {
		params.PageNumber = 1
	}

	queues, qErr := d.userQueueIDs(ctx, actor)
	if qErr != nil {
		return nil, 0, false, qErr
	}

	q := d.DB.WithContext(ctx).Model(&Ticket{}).
		Scopes(
			scopeStatus(params.Status),
			scopeQueueIDs(params.QueueIDs),
			scopeUserID(params.UserID, actor),
			scopeWithUnread(params.WithUnreadMessages),
			scopeDate(params.Date),
			scopeSearchParam(params.SearchParam),
			scopeVisibility(actor, queues, params.ShowAll),
		)

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}

	offset := (params.PageNumber - 1) * ticketListPageSize
	var items []Ticket
	if err := q.
		Preload("Contact").
		Preload("Contact.ExtraInfo").
		Preload("User").
		Preload("Queue").
		Preload("Whatsapp").
		Order("updated_at DESC").
		Offset(offset).
		Limit(ticketListPageSize).
		Find(&items).Error; err != nil {
		return nil, 0, false, errors.Wrap(err, "ERR_DB_QUERY", http.StatusInternalServerError)
	}

	hasMore := count > int64(params.PageNumber*ticketListPageSize)
	return items, count, hasMore, nil
}

func scopeStatus(statuses []string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(statuses) == 0 {
			return db
		}
		return db.Where("tickets.status IN ?", statuses)
	}
}

func scopeQueueIDs(ids []*uint) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(ids) == 0 {
			return db
		}
		var concrete []uint
		hasNull := false
		for _, p := range ids {
			if p == nil {
				hasNull = true
				continue
			}
			concrete = append(concrete, *p)
		}
		switch {
		case hasNull && len(concrete) > 0:
			return db.Where("tickets.queue_id IN ? OR tickets.queue_id IS NULL", concrete)
		case hasNull:
			return db.Where("tickets.queue_id IS NULL")
		default:
			return db.Where("tickets.queue_id IN ?", concrete)
		}
	}
}

func scopeUserID(target *uint, actor *auth.UserClaims) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if target == nil {
			return db
		}
		if actor.Profile != "admin" && *target != actor.ID {
			return db.Where("1 = 0")
		}
		return db.Where("tickets.user_id = ?", *target)
	}
}

func scopeWithUnread(flag *bool) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if flag == nil || !*flag {
			return db
		}
		return db.Where("tickets.unread_messages > 0")
	}
}

func scopeDate(date *time.Time) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if date == nil {
			return db
		}
		day := date.Format("2006-01-02")
		return db.Where("tickets.created_at::date = ?::date", day)
	}
}

func scopeSearchParam(raw string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return db
		}
		pattern := "%" + escapeLike(strings.ToLower(trimmed)) + "%"
		numberPattern := "%" + escapeLike(trimmed) + "%"
		return db.Where(
			"EXISTS (SELECT 1 FROM contacts c WHERE c.id = tickets.contact_id AND (LOWER(c.name) ILIKE ? OR c.number LIKE ?)) "+
				"OR EXISTS (SELECT 1 FROM messages m WHERE m.ticket_id = tickets.id AND LOWER(m.body) ILIKE ?)",
			pattern, numberPattern, pattern,
		)
	}
}

func scopeVisibility(actor *auth.UserClaims, queues []uint, showAll bool) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if actor.Profile == "admin" {
			return db
		}
		if showAll {
			return db
		}
		if len(queues) > 0 {
			return db.Where(
				"tickets.user_id = ? OR tickets.queue_id IN ? OR tickets.queue_id IS NULL",
				actor.ID, queues,
			)
		}
		return db.Where(
			"tickets.user_id = ? OR tickets.queue_id IS NULL",
			actor.ID,
		)
	}
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
