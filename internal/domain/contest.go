package domain

import "time"

type Platform string

const (
	PlatformCodeforces Platform = "codeforces"
	PlatformCodeChef   Platform = "codechef"
)

type ContestStatus string

const (
	ContestStatusUpcoming  ContestStatus = "upcoming"
	ContestStatusRunning   ContestStatus = "running"
	ContestStatusFinished  ContestStatus = "finished"
	ContestStatusCancelled ContestStatus = "cancelled"
)

type Contest struct {
	ID          int64
	Platform    Platform
	ExternalID  string
	Name        string
	URL         string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	Status      ContestStatus
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (c Contest) IdentityKey() string {
	return string(c.Platform) + ":" + c.ExternalID
}

// StatusAt derives contest lifecycle from start/end times and the current instant.
func StatusAt(now, start, end time.Time) ContestStatus {
	now = now.UTC()
	start = start.UTC()
	end = end.UTC()
	if now.Before(start) {
		return ContestStatusUpcoming
	}
	if now.Before(end) {
		return ContestStatusRunning
	}
	return ContestStatusFinished
}
