package notify_test

import (
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
)

func TestIsContestRunning(t *testing.T) {
	start := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "before start", now: start.Add(-time.Minute), want: false},
		{name: "at start", now: start, want: true},
		{name: "mid contest", now: start.Add(time.Hour), want: true},
		{name: "at end", now: end, want: false},
		{name: "after end", now: end.Add(time.Minute), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := notify.IsContestRunning(tc.now, start, end)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestShouldSend(t *testing.T) {
	start := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	dueAt := notify.ActiveDueAt(start)

	t.Run("running contest due", func(t *testing.T) {
		now := start.Add(30 * time.Minute)
		contest := domain.Contest{StartTime: start, EndTime: end, Status: domain.ContestStatusRunning}
		n := domain.Notification{Status: domain.NotificationStatusPending, DueAt: dueAt, AttemptCount: 0}
		if !notify.ShouldSend(n, contest, now, 5) {
			t.Fatal("expected send while running")
		}
	})

	t.Run("upcoming contest waits", func(t *testing.T) {
		now := start.Add(-time.Hour)
		contest := domain.Contest{StartTime: start, EndTime: end, Status: domain.ContestStatusUpcoming}
		n := domain.Notification{Status: domain.NotificationStatusPending, DueAt: dueAt, AttemptCount: 0}
		if notify.ShouldSend(n, contest, now, 5) {
			t.Fatal("must not send before contest starts")
		}
	})

	t.Run("finished contest rejected", func(t *testing.T) {
		now := end.Add(time.Minute)
		contest := domain.Contest{StartTime: start, EndTime: end, Status: domain.ContestStatusFinished}
		n := domain.Notification{Status: domain.NotificationStatusPending, DueAt: dueAt, AttemptCount: 0}
		if notify.ShouldSend(n, contest, now, 5) {
			t.Fatal("must not send after contest ends")
		}
	})

	t.Run("already sent rejected", func(t *testing.T) {
		now := start.Add(time.Minute)
		contest := domain.Contest{StartTime: start, EndTime: end}
		n := domain.Notification{Status: domain.NotificationStatusSent, DueAt: dueAt, AttemptCount: 1}
		if notify.ShouldSend(n, contest, now, 5) {
			t.Fatal("sent notification must not resend")
		}
	})

	t.Run("discovered already running sends immediately", func(t *testing.T) {
		now := start.Add(time.Hour)
		contest := domain.Contest{StartTime: start, EndTime: end}
		n := domain.Notification{Status: domain.NotificationStatusPending, DueAt: dueAt, AttemptCount: 0}
		if !notify.ShouldSend(n, contest, now, 5) {
			t.Fatal("past due_at with running contest must send")
		}
	})
}

func TestActiveDueAt(t *testing.T) {
	start := time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC)
	due := notify.ActiveDueAt(start)
	if !due.Equal(start) {
		t.Fatalf("due = %v want %v", due, start)
	}
}
