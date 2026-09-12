package codechef

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

const defaultURL = "https://www.codechef.com/api/list/contests/all"

type Source struct {
	client  *http.Client
	baseURL string
	now     func() time.Time
	rated   *ratedChecker
}

func New(client *http.Client) *Source {
	return NewWithURL(client, defaultURL)
}

func NewWithURL(client *http.Client, baseURL string) *Source {
	return NewWithURLAndClock(client, baseURL, nil)
}

func NewWithURLAndClock(client *http.Client, baseURL string, now func() time.Time) *Source {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Source{
		client:  client,
		baseURL: baseURL,
		now:     now,
		rated:   newRatedChecker(client, baseURL),
	}
}

func (s *Source) Name() string {
	return "codechef"
}

func (s *Source) Platform() domain.Platform {
	return domain.PlatformCodeChef
}

type apiResponse struct {
	Status          string       `json:"status"`
	PresentContests []apiContest `json:"present_contests"`
	FutureContests  []apiContest `json:"future_contests"`
}

type apiContest struct {
	ContestCode         string `json:"contest_code"`
	ContestName         string `json:"contest_name"`
	ContestStartDate    string `json:"contest_start_date"`
	ContestEndDate      string `json:"contest_end_date"`
	ContestStartDateISO string `json:"contest_start_date_iso"`
	ContestEndDateISO   string `json:"contest_end_date_iso"`
}

func (s *Source) FetchContests(ctx context.Context) ([]domain.Contest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("codechef request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "veille-contest-radar/1.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("codechef fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("codechef read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("codechef status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("codechef decode: %w", err)
	}
	if parsed.Status != "" && !strings.EqualFold(parsed.Status, "success") {
		return nil, fmt.Errorf("codechef api status %q", parsed.Status)
	}

	now := s.now()
	seen := make(map[string]struct{})
	out := make([]domain.Contest, 0, len(parsed.PresentContests)+len(parsed.FutureContests))
	for _, item := range append(parsed.PresentContests, parsed.FutureContests...) {
		code := strings.TrimSpace(item.ContestCode)
		if code == "" || strings.TrimSpace(item.ContestName) == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}

		start, err := parseCodeChefTime(item.ContestStartDateISO, item.ContestStartDate)
		if err != nil {
			continue
		}
		end, err := parseCodeChefTime(item.ContestEndDateISO, item.ContestEndDate)
		if err != nil {
			continue
		}
		if !end.After(start) {
			continue
		}
		if !end.After(now) {
			continue
		}
		rated, err := s.rated.isRated(ctx, code)
		if err != nil {
			return nil, err
		}
		if !rated {
			continue
		}
		out = append(out, domain.Contest{
			Platform:   domain.PlatformCodeChef,
			ExternalID: code,
			Name:       item.ContestName,
			URL:        "https://www.codechef.com/" + code,
			StartTime:  start,
			EndTime:    end,
			Duration:   end.Sub(start),
			Status:     domain.StatusAt(now, start, end),
		})
	}
	return out, nil
}

func parseCodeChefTime(isoValue, legacyValue string) (time.Time, error) {
	candidates := []string{strings.TrimSpace(isoValue), strings.TrimSpace(legacyValue)}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"02 Jan 2006 15:04:05",
		"January 2 2006 15:04:05 GMT+0530",
		"02 Jan 2006  15:04:05",
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, candidate); err == nil {
				return t.UTC(), nil
			}
		}
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", candidate, time.FixedZone("IST", 5*3600+30*60)); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized codechef time")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
