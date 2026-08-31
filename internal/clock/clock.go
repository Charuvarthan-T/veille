package clock

import "time"

type Clock interface {
	Now() time.Time
}

type System struct{}

func (System) Now() time.Time {
	return time.Now().UTC()
}

type Fixed struct {
	Instant time.Time
}

func (f Fixed) Now() time.Time {
	return f.Instant.UTC()
}
