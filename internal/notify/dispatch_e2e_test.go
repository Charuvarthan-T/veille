package notify_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/clock"
	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
)

type e2eStore struct {
	contest       domain.Contest
	notifications map[int64]*domain.Notification
}

func newE2EStore(contest domain.Contest, n domain.Notification) *e2eStore {
	copied := n
	return &e2eStore{
		contest:       contest,
		notifications: map[int64]*domain.Notification{n.ID: &copied},
	}
}

func (s *e2eStore) UpsertContest(context.Context, domain.Contest) (domain.Contest, bool, error) {
	return domain.Contest{}, false, errors.New("unused")
}

func (s *e2eStore) GetContest(_ context.Context, id int64) (domain.Contest, error) {
	if s.contest.ID != id {
		return domain.Contest{}, errors.New("contest not found")
	}
	return s.contest, nil
}

func (s *e2eStore) EnsureActiveNotification(context.Context, int64, time.Time) error {
	return nil
}

func (s *e2eStore) RefreshContestStatuses(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *e2eStore) DeleteFinishedContests(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *e2eStore) ClaimDue(_ context.Context, now time.Time, limit int, maxAttempts int) ([]domain.Notification, error) {
	var claimed []domain.Notification
	for _, n := range s.notifications {
		if len(claimed) >= limit {
			break
		}
		if n.Status != domain.NotificationStatusPending && n.Status != domain.NotificationStatusFailed {
			continue
		}
		if n.AttemptCount >= maxAttempts {
			continue
		}
		if n.DueAt.After(now) {
			continue
		}
		if !s.contest.StartTime.After(now) && !now.Before(s.contest.EndTime) {
			continue
		}
		if s.contest.StartTime.After(now) || !now.Before(s.contest.EndTime) {
			continue
		}
		n.Status = domain.NotificationStatusSending
		n.AttemptCount++
		n.UpdatedAt = now
		claimed = append(claimed, *n)
	}
	return claimed, nil
}

func (s *e2eStore) MarkSent(_ context.Context, id int64, sentAt time.Time) error {
	n, ok := s.notifications[id]
	if !ok {
		return errors.New("notification not found")
	}
	n.Status = domain.NotificationStatusSent
	n.SentAt = &sentAt
	n.LastError = ""
	n.UpdatedAt = sentAt
	return nil
}

func (s *e2eStore) MarkFailed(_ context.Context, id int64, errMsg string) error {
	n, ok := s.notifications[id]
	if !ok {
		return errors.New("notification not found")
	}
	n.Status = domain.NotificationStatusFailed
	n.LastError = errMsg
	return nil
}

func (s *e2eStore) ReleaseClaim(_ context.Context, id int64) error {
	n, ok := s.notifications[id]
	if !ok {
		return errors.New("notification not found")
	}
	if n.Status == domain.NotificationStatusSending {
		n.Status = domain.NotificationStatusPending
		if n.AttemptCount > 0 {
			n.AttemptCount--
		}
	}
	return nil
}

func (s *e2eStore) ReleaseStaleSending(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func (s *e2eStore) Ping(context.Context) error { return nil }
func (s *e2eStore) Close() error               { return nil }

func TestDispatchRunningContestExactlyOnce(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-30 * time.Minute)
	end := start.Add(2 * time.Hour)
	dueAt := notify.ActiveDueAt(start)

	contest := domain.Contest{
		ID:         42,
		Platform:   domain.PlatformCodeforces,
		ExternalID: "42",
		Name:       "Div. 2 Round",
		URL:        "https://codeforces.com/contest/42",
		StartTime:  start,
		EndTime:    end,
		Duration:   2 * time.Hour,
		Status:     domain.ContestStatusRunning,
	}
	pending := domain.Notification{
		ID:           7,
		ContestID:    contest.ID,
		Channel:      domain.ChannelEmail,
		Kind:         domain.NotificationKindContestStarted,
		Status:       domain.NotificationStatusPending,
		DueAt:        dueAt,
		AttemptCount: 0,
	}

	if !notify.ShouldSend(pending, contest, now, 5) {
		t.Fatal("fixture must be eligible while running")
	}

	st := newE2EStore(contest, pending)
	email := &fakeSender{channel: domain.ChannelEmail}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := notify.NewOrchestrator(
		st,
		[]notify.ChannelSender{email},
		clock.Fixed{Instant: now},
		time.UTC,
		5,
		log,
	)

	first, err := orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Claimed != 1 || first.Sent != 1 || first.Failed != 0 {
		t.Fatalf("first dispatch result = %+v", first)
	}
	if email.calls != 1 {
		t.Fatalf("email sends = %d, want 1", email.calls)
	}
	stored := st.notifications[pending.ID]
	if stored.Status != domain.NotificationStatusSent {
		t.Fatalf("status = %s, want sent", stored.Status)
	}

	second, err := orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Claimed != 0 || second.Sent != 0 || second.Failed != 0 {
		t.Fatalf("second dispatch result = %+v", second)
	}
	if email.calls != 1 {
		t.Fatalf("email resent: calls = %d", email.calls)
	}
}

func TestDispatchDoesNotSendBeforeContestStarts(t *testing.T) {
	now := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	contest := domain.Contest{
		ID:        1,
		StartTime: start,
		EndTime:   end,
		Status:    domain.ContestStatusUpcoming,
	}
	pending := domain.Notification{
		ID:        2,
		ContestID: 1,
		Channel:   domain.ChannelEmail,
		Status:    domain.NotificationStatusPending,
		DueAt:     notify.ActiveDueAt(start),
	}
	if notify.ShouldSend(pending, contest, now, 5) {
		t.Fatal("must not send before start")
	}
}
