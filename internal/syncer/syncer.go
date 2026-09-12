package syncer

import (
	"context"
	"log/slog"
	"time"

	"github.com/Charuvarthan-T/veille/internal/clock"
	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
	"github.com/Charuvarthan-T/veille/internal/source"
	"github.com/Charuvarthan-T/veille/internal/store"
)

type ActiveNotificationStore interface {
	store.ContestStore
	EnsureActiveNotification(ctx context.Context, contestID int64, dueAt time.Time) error
	RefreshContestStatuses(ctx context.Context, now time.Time) (int64, error)
}

type Syncer struct {
	sources []source.ContestSource
	store   ActiveNotificationStore
	clock   clock.Clock
	log     *slog.Logger
}

func New(
	sources []source.ContestSource,
	contestStore ActiveNotificationStore,
	clk clock.Clock,
	log *slog.Logger,
) *Syncer {
	return &Syncer{
		sources: sources,
		store:   contestStore,
		clock:   clk,
		log:     log,
	}
}

type Result struct {
	Source    string
	Fetched   int
	Inserted  int
	Updated   int
	Ensured   int
	SourceErr error
}

func (s *Syncer) Run(ctx context.Context) []Result {
	results := make([]Result, 0, len(s.sources))
	for _, src := range s.sources {
		results = append(results, s.syncSource(ctx, src))
	}

	now := s.clock.Now()
	updated, err := s.store.RefreshContestStatuses(ctx, now)
	if err != nil {
		s.log.Error("refresh contest statuses failed", "error", err)
	} else if updated > 0 {
		s.log.Info("contest statuses refreshed", "updated", updated)
	}

	return results
}

func (s *Syncer) syncSource(ctx context.Context, src source.ContestSource) Result {
	result := Result{Source: src.Name()}
	now := s.clock.Now()

	contests, err := src.FetchContests(ctx)
	if err != nil {
		result.SourceErr = err
		s.log.Error("contest source failed", "source", src.Name(), "error", err)
		return result
	}
	result.Fetched = len(contests)

	for _, contest := range contests {
		contest.Platform = src.Platform()
		contest.FirstSeenAt = now
		contest.LastSeenAt = now
		contest.Status = domain.StatusAt(now, contest.StartTime, contest.EndTime)

		saved, inserted, err := s.store.UpsertContest(ctx, contest)
		if err != nil {
			s.log.Error("upsert contest failed", "source", src.Name(), "external_id", contest.ExternalID, "error", err)
			continue
		}
		if inserted {
			result.Inserted++
		} else {
			result.Updated++
		}

		dueAt := notify.ActiveDueAt(saved.StartTime)
		if err := s.store.EnsureActiveNotification(ctx, saved.ID, dueAt); err != nil {
			s.log.Error("ensure active notification failed", "contest_id", saved.ID, "error", err)
			continue
		}
		result.Ensured++
	}

	s.log.Info("contest source synchronized",
		"source", src.Name(),
		"fetched", result.Fetched,
		"inserted", result.Inserted,
		"updated", result.Updated,
		"notifications_ensured", result.Ensured,
	)
	return result
}
