package notify

import (
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

func ActiveDueAt(start time.Time) time.Time {
	return start.UTC()
}

func IsContestRunning(now, start, end time.Time) bool {
	return domain.StatusAt(now, start, end) == domain.ContestStatusRunning
}

func ShouldSend(n domain.Notification, contest domain.Contest, now time.Time, maxAttempts int) bool {
	if n.Status == domain.NotificationStatusSent {
		return false
	}
	if n.AttemptCount > maxAttempts {
		return false
	}
	if !IsContestRunning(now, contest.StartTime, contest.EndTime) {
		return false
	}
	return !now.Before(n.DueAt.UTC())
}
