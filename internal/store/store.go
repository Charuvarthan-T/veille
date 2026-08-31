package store

import (
	"context"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

type ContestStore interface {
	UpsertContest(ctx context.Context, contest domain.Contest) (domain.Contest, bool, error)
	MarkMissingUpcoming(ctx context.Context, platform domain.Platform, seenExternalIDs []string, observedAt time.Time) error
	GetContest(ctx context.Context, id int64) (domain.Contest, error)
	ListUpcomingBefore(ctx context.Context, before time.Time) ([]domain.Contest, error)
}

type NotificationStore interface {
	EnsureReminder(ctx context.Context, contestID int64, channel domain.Channel, dueAt time.Time) error
	ClaimDue(ctx context.Context, now time.Time, limit int, maxAttempts int) ([]domain.Notification, error)
	MarkSent(ctx context.Context, id int64, sentAt time.Time) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
	ReleaseStaleSending(ctx context.Context, olderThan time.Time) (int64, error)
}

type Store interface {
	ContestStore
	NotificationStore
	Ping(ctx context.Context) error
	Close() error
}
