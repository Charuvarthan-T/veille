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

type fakeStore struct {
	contest      domain.Contest
	claimed      []domain.Notification
	sentIDs      []int64
	failedIDs    []int64
	claimCalls   int
	releaseCalls int
}

func (f *fakeStore) UpsertContest(context.Context, domain.Contest) (domain.Contest, bool, error) {
	return domain.Contest{}, false, errors.New("unused")
}
func (f *fakeStore) GetContest(context.Context, int64) (domain.Contest, error) {
	return f.contest, nil
}
func (f *fakeStore) EnsureReminder(context.Context, int64, domain.Channel, time.Time) error {
	return nil
}
func (f *fakeStore) ClaimDue(context.Context, time.Time, int, int) ([]domain.Notification, error) {
	f.claimCalls++
	out := f.claimed
	f.claimed = nil
	return out, nil
}
func (f *fakeStore) MarkSent(_ context.Context, id int64, _ time.Time) error {
	f.sentIDs = append(f.sentIDs, id)
	return nil
}
func (f *fakeStore) MarkFailed(_ context.Context, id int64, _ string) error {
	f.failedIDs = append(f.failedIDs, id)
	return nil
}
func (f *fakeStore) ReleaseStaleSending(context.Context, time.Time) (int64, error) {
	f.releaseCalls++
	return 0, nil
}
func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }

type fakeSender struct {
	channel domain.Channel
	calls   int
	err     error
}

func (f *fakeSender) Channel() domain.Channel { return f.channel }
func (f *fakeSender) Send(context.Context, notify.Message) error {
	f.calls++
	return f.err
}

func TestOrchestratorSendsOncePerClaim(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	start := now.Add(20 * time.Hour)
	st := &fakeStore{
		contest: domain.Contest{
			ID:        7,
			Platform:  domain.PlatformCodeforces,
			Name:      "Round",
			URL:       "https://codeforces.com/contest/7",
			StartTime: start,
			EndTime:   start.Add(2 * time.Hour),
			Duration:  2 * time.Hour,
			Status:    domain.ContestStatusUpcoming,
		},
		claimed: []domain.Notification{{
			ID:           1,
			ContestID:    7,
			Channel:      domain.ChannelEmail,
			Kind:         domain.NotificationKindReminder24h,
			Status:       domain.NotificationStatusSending,
			DueAt:        start.Add(-24 * time.Hour),
			AttemptCount: 1,
		}},
	}
	sender := &fakeSender{channel: domain.ChannelEmail}
	loc := time.UTC
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := notify.NewOrchestrator(st, []notify.ChannelSender{sender}, clock.Fixed{Instant: now}, loc, 24*time.Hour, 24*time.Hour, 5, log)

	result, err := orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent != 1 || sender.calls != 1 || len(st.sentIDs) != 1 {
		t.Fatalf("result=%+v sender.calls=%d sent=%v", result, sender.calls, st.sentIDs)
	}

	result, err = orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 0 || sender.calls != 1 {
		t.Fatalf("duplicate send detected: result=%+v calls=%d", result, sender.calls)
	}
}

func TestOrchestratorMarksFailureAndAllowsRetryPath(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	start := now.Add(20 * time.Hour)
	st := &fakeStore{
		contest: domain.Contest{
			ID:        7,
			Name:      "Round",
			URL:       "https://example.com",
			StartTime: start,
			EndTime:   start.Add(time.Hour),
			Duration:  time.Hour,
			Status:    domain.ContestStatusUpcoming,
		},
		claimed: []domain.Notification{{
			ID:           9,
			ContestID:    7,
			Channel:      domain.ChannelWhatsApp,
			Status:       domain.NotificationStatusSending,
			DueAt:        start.Add(-24 * time.Hour),
			AttemptCount: 1,
		}},
	}
	sender := &fakeSender{channel: domain.ChannelWhatsApp, err: errors.New("twilio down")}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := notify.NewOrchestrator(st, []notify.ChannelSender{sender}, clock.Fixed{Instant: now}, time.UTC, 24*time.Hour, 24*time.Hour, 5, log)

	result, err := orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != 1 || len(st.failedIDs) != 1 {
		t.Fatalf("expected failure persistence: %+v failedIDs=%v", result, st.failedIDs)
	}
}
