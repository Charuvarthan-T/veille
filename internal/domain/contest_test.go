package domain_test

import (
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

func TestContestIdentityKey(t *testing.T) {
	c := domain.Contest{Platform: domain.PlatformCodeforces, ExternalID: "123"}
	if c.IdentityKey() != "codeforces:123" {
		t.Fatalf("got %s", c.IdentityKey())
	}
}

func TestStatusAt(t *testing.T) {
	start := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	cases := []struct {
		name string
		now  time.Time
		want domain.ContestStatus
	}{
		{name: "upcoming", now: start.Add(-time.Minute), want: domain.ContestStatusUpcoming},
		{name: "running at start", now: start, want: domain.ContestStatusRunning},
		{name: "running mid", now: start.Add(time.Hour), want: domain.ContestStatusRunning},
		{name: "finished at end", now: end, want: domain.ContestStatusFinished},
		{name: "finished after", now: end.Add(time.Minute), want: domain.ContestStatusFinished},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.StatusAt(tc.now, start, end)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
