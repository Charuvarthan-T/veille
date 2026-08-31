package syncer

import (
	"context"
	"log/slog"
	"time"

	"github.com/Charuvarthan-T/veille/internal/clock"
	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/source"
	"github.com/Charuvarthan-T/veille/internal/store"
)

type ReminderStore interface {
	store.ContestStore
	EnsureReminder(ctx context.Context, contestID int64, channel domain.Channel, dueAt time.Time) error
}

type Syncer struct {
	sources  []source.ContestSource
	store    ReminderStore
	clock    clock.Clock
	channels []domain.Channel
	lead     time.Duration
	log      *slog.Logger
}

func New(
	sources []source.ContestSource,
	contestStore ReminderStore,
	clk clock.Clock,
	channels []domain.Channel,
	lead time.Duration,
	log *slog.Logger,
) *Syncer {
	return &Syncer{
		sources:  sources,
		store:    contestStore,
		clock:    clk,
		channels: channels,
		lead:     lead,
		log:      log,
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
	return results
}

func (s *Syncer) syncSource(ctx context.Context, src source.ContestSource) Result {
	result := Result{Source: src.Name()}
	now := s.clock.Now()

	contests, err := src.FetchUpcoming(ctx)
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
		if contest.Status == "" {
			contest.Status = domain.ContestStatusUpcoming
		}

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

		dueAt := saved.StartTime.Add(-s.lead)
		for _, channel := range s.channels {
			if err := s.store.EnsureReminder(ctx, saved.ID, channel, dueAt); err != nil {
				s.log.Error("ensure reminder failed", "contest_id", saved.ID, "channel", channel, "error", err)
				continue
			}
			result.Ensured++
		}
	}

	s.log.Info("contest source synchronized",
		"source", src.Name(),
		"fetched", result.Fetched,
		"inserted", result.Inserted,
		"updated", result.Updated,
		"reminders_ensured", result.Ensured,
	)
	return result
}
