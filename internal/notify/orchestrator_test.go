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
	releasedIDs  []int64
	claimCalls   int
	releaseCalls int
	releaseStale int
}

func (f *fakeStore) UpsertContest(context.Context, domain.Contest) (domain.Contest, bool, error) {
	return domain.Contest{}, false, errors.New("unused")
}
func (f *fakeStore) GetContest(context.Context, int64) (domain.Contest, error) {
	return f.contest, nil
}
func (f *fakeStore) EnsureActiveNotification(context.Context, int64, time.Time) error {
	return nil
}
func (f *fakeStore) RefreshContestStatuses(context.Context, time.Time) (int64, error) {
	return 0, nil
}
func (f *fakeStore) DeleteFinishedContests(context.Context, time.Time) (int64, error) {
	return 0, nil
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
func (f *fakeStore) ReleaseClaim(_ context.Context, id int64) error {
	f.releaseCalls++
	f.releasedIDs = append(f.releasedIDs, id)
	return nil
}
func (f *fakeStore) ReleaseStaleSending(context.Context, time.Time) (int64, error) {
	f.releaseStale++
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
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-30 * time.Minute)
	end := start.Add(2 * time.Hour)
	st := &fakeStore{
		contest: domain.Contest{
			ID:        7,
			Platform:  domain.PlatformCodeforces,
			Name:      "Round",
			URL:       "https://codeforces.com/contest/7",
			StartTime: start,
			EndTime:   end,
			Duration:  2 * time.Hour,
			Status:    domain.ContestStatusRunning,
		},
		claimed: []domain.Notification{{
			ID:           1,
			ContestID:    7,
			Channel:      domain.ChannelEmail,
			Kind:         domain.NotificationKindContestStarted,
			Status:       domain.NotificationStatusSending,
			DueAt:        start,
			AttemptCount: 1,
		}},
	}
	sender := &fakeSender{channel: domain.ChannelEmail}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := notify.NewOrchestrator(st, []notify.ChannelSender{sender}, clock.Fixed{Instant: now}, time.UTC, 5, log)

	result, err := orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Sent != 1 || sender.calls != 1 || len(st.sentIDs) != 1 {
		t.Fatalf("result=%+v sender.calls=%d sent=%v", result, sender.calls, st.sentIDs)
	}

	st.claimed = nil
	result, err = orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Claimed != 0 || sender.calls != 1 {
		t.Fatalf("duplicate send detected: result=%+v calls=%d", result, sender.calls)
	}
}

func TestOrchestratorMarksFailureAndAllowsRetryPath(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-15 * time.Minute)
	end := start.Add(2 * time.Hour)
	st := &fakeStore{
		contest: domain.Contest{
			ID:        7,
			Name:      "Round",
			URL:       "https://example.com",
			StartTime: start,
			EndTime:   end,
			Duration:  2 * time.Hour,
			Status:    domain.ContestStatusRunning,
		},
		claimed: []domain.Notification{{
			ID:           9,
			ContestID:    7,
			Channel:      domain.ChannelEmail,
			Status:       domain.NotificationStatusSending,
			DueAt:        start,
			AttemptCount: 1,
		}},
	}
	sender := &fakeSender{channel: domain.ChannelEmail, err: errors.New("resend down")}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := notify.NewOrchestrator(st, []notify.ChannelSender{sender}, clock.Fixed{Instant: now}, time.UTC, 5, log)

	result, err := orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Failed != 1 || len(st.failedIDs) != 1 {
		t.Fatalf("expected failure persistence: %+v failedIDs=%v", result, st.failedIDs)
	}
}

func TestOrchestratorSkipsIneligibleWithoutFailure(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(2 * time.Hour)
	end := start.Add(2 * time.Hour)
	st := &fakeStore{
		contest: domain.Contest{
			ID:        7,
			StartTime: start,
			EndTime:   end,
			Status:    domain.ContestStatusUpcoming,
		},
		claimed: []domain.Notification{{
			ID:           3,
			ContestID:    7,
			Channel:      domain.ChannelEmail,
			Status:       domain.NotificationStatusSending,
			DueAt:        start,
			AttemptCount: 1,
		}},
	}
	sender := &fakeSender{channel: domain.ChannelEmail}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	orch := notify.NewOrchestrator(st, []notify.ChannelSender{sender}, clock.Fixed{Instant: now}, time.UTC, 5, log)

	result, err := orch.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("result=%+v", result)
	}
	if st.releaseCalls != 1 {
		t.Fatalf("releaseCalls = %d", st.releaseCalls)
	}
	if sender.calls != 0 {
		t.Fatal("must not send for upcoming contest")
	}
}
