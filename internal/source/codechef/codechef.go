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
}

func New(client *http.Client) *Source {
	return &Source{
		client:  client,
		baseURL: defaultURL,
	}
}

func NewWithURL(client *http.Client, baseURL string) *Source {
	return &Source{
		client:  client,
		baseURL: baseURL,
	}
}

func (s *Source) Name() string {
	return "codechef"
}

func (s *Source) Platform() domain.Platform {
	return domain.PlatformCodeChef
}

type apiResponse struct {
	Status         string       `json:"status"`
	FutureContests []apiContest `json:"future_contests"`
}

type apiContest struct {
	ContestCode         string `json:"contest_code"`
	ContestName         string `json:"contest_name"`
	ContestStartDate    string `json:"contest_start_date"`
	ContestEndDate      string `json:"contest_end_date"`
	ContestStartDateISO string `json:"contest_start_date_iso"`
	ContestEndDateISO   string `json:"contest_end_date_iso"`
}

func (s *Source) FetchUpcoming(ctx context.Context) ([]domain.Contest, error) {
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

	now := time.Now().UTC()
	out := make([]domain.Contest, 0, len(parsed.FutureContests))
	for _, item := range parsed.FutureContests {
		if strings.TrimSpace(item.ContestCode) == "" || strings.TrimSpace(item.ContestName) == "" {
			continue
		}
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
		if start.Before(now) {
			continue
		}
		out = append(out, domain.Contest{
			Platform:   domain.PlatformCodeChef,
			ExternalID: item.ContestCode,
			Name:       item.ContestName,
			URL:        "https://www.codechef.com/" + item.ContestCode,
			StartTime:  start,
			EndTime:    end,
			Duration:   end.Sub(start),
			Status:     domain.ContestStatusUpcoming,
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
