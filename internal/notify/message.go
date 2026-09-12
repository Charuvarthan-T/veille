package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

func platformLabel(p domain.Platform) string {
	switch p {
	case domain.PlatformCodeforces:
		return "Codeforces"
	case domain.PlatformCodeChef:
		return "CodeChef"
	default:
		return string(p)
	}
}

func BuildActiveMessage(contest domain.Contest, location *time.Location) Message {
	localStart := contest.StartTime.In(location)
	localEnd := contest.EndTime.In(location)
	label := platformLabel(contest.Platform)
	subject := fmt.Sprintf("%s contest is LIVE: %s", label, contest.Name)
	body := fmt.Sprintf(
		"%s Contest is LIVE\n\n"+
			"Contest: %s\n\n"+
			"Started: %s\n"+
			"Ends: %s\n"+
			"Duration: %s\n\n"+
			"Join contest:\n%s\n",
		strings.ToUpper(label),
		contest.Name,
		localStart.Format("Mon, 02 Jan 2006 15:04 MST"),
		localEnd.Format("Mon, 02 Jan 2006 15:04 MST"),
		contest.Duration.String(),
		contest.URL,
	)
	return Message{Subject: subject, Body: body}
}
