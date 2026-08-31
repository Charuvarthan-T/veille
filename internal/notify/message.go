package notify

import (
	"fmt"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

func BuildReminderMessage(contest domain.Contest, location *time.Location) Message {
	localStart := contest.StartTime.In(location)
	localEnd := contest.EndTime.In(location)
	subject := fmt.Sprintf("Contest in 24h: %s", contest.Name)
	body := fmt.Sprintf(
		"Upcoming competitive programming contest reminder\n\n"+
			"Platform: %s\n"+
			"Contest: %s\n"+
			"Starts: %s\n"+
			"Ends: %s\n"+
			"Duration: %s\n"+
			"URL: %s\n",
		contest.Platform,
		contest.Name,
		localStart.Format("Mon, 02 Jan 2006 15:04 MST"),
		localEnd.Format("Mon, 02 Jan 2006 15:04 MST"),
		contest.Duration.String(),
		contest.URL,
	)
	return Message{Subject: subject, Body: body}
}
