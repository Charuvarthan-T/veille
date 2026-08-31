package notify_test

import (
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
)

func TestIsWithinReminderWindow(t *testing.T) {
	start := time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC)
	lead := 24 * time.Hour
	window := 24 * time.Hour

	cases := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "too early", now: start.Add(-25 * time.Hour), want: false},
		{name: "exactly due", now: start.Add(-24 * time.Hour), want: true},
		{name: "delayed but inside window", now: start.Add(-12 * time.Hour), want: true},
		{name: "near start", now: start.Add(-time.Minute), want: true},
		{name: "after start", now: start.Add(time.Minute), want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := notify.IsWithinReminderWindow(tc.now, start, lead, window)
			if got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestShouldSendRejectsSentAndCancelled(t *testing.T) {
	start := time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC)
	now := start.Add(-20 * time.Hour)
	contest := domain.Contest{Status: domain.ContestStatusUpcoming, StartTime: start}
	n := domain.Notification{Status: domain.NotificationStatusSent, AttemptCount: 1}

	if notify.ShouldSend(n, contest, now, 24*time.Hour, 24*time.Hour, 5) {
		t.Fatal("sent notification must not resend")
	}

	n.Status = domain.NotificationStatusPending
	contest.Status = domain.ContestStatusCancelled
	if notify.ShouldSend(n, contest, now, 24*time.Hour, 24*time.Hour, 5) {
		t.Fatal("cancelled contest must not notify")
	}
}

func TestReminderDueAt(t *testing.T) {
	start := time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC)
	due := notify.ReminderDueAt(start, 24*time.Hour)
	want := time.Date(2026, 8, 31, 18, 0, 0, 0, time.UTC)
	if !due.Equal(want) {
		t.Fatalf("due = %v want %v", due, want)
	}
}
