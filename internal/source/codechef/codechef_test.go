package codechef_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/source/codechef"
)

func newCodeChefTestServer(t *testing.T, listPayload string, ratedByCode map[string]bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/api/list/contests/") {
			_, _ = w.Write([]byte(listPayload))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/contests/") {
			code := strings.TrimPrefix(r.URL.Path, "/api/contests/")
			rated := ratedByCode[code]
			flag := "0"
			if rated {
				flag = "1"
			}
			_, _ = w.Write([]byte(`{"status":"success","isRatedContest":"` + flag + `"}`))
			return
		}
		http.NotFound(w, r)
	}))
}

func TestFetchContestsNormalizesFutureContests(t *testing.T) {
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	end := start.Add(3 * time.Hour)
	payload := `{
		"status":"success",
		"future_contests":[
			{
				"contest_code":"START999",
				"contest_name":"Starters 999",
				"contest_start_date_iso":"` + start.Format(time.RFC3339) + `",
				"contest_end_date_iso":"` + end.Format(time.RFC3339) + `"
			}
		]
	}`
	server := newCodeChefTestServer(t, payload, map[string]bool{"START999": true})
	defer server.Close()

	src := codechef.NewWithURL(server.Client(), server.URL+"/api/list/contests/all")
	contests, err := src.FetchContests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 1 {
		t.Fatalf("got %d contests", len(contests))
	}
	if contests[0].ExternalID != "START999" {
		t.Fatalf("external id = %s", contests[0].ExternalID)
	}
	if contests[0].Duration != 3*time.Hour {
		t.Fatalf("duration = %s", contests[0].Duration)
	}
	if contests[0].Status != domain.ContestStatusUpcoming {
		t.Fatalf("status = %s want upcoming", contests[0].Status)
	}
}

func TestFetchContestsExcludesUnratedContest(t *testing.T) {
	start := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Second)
	end := start.Add(3 * time.Hour)
	payload := `{
		"status":"success",
		"future_contests":[
			{
				"contest_code":"PRACTICE1",
				"contest_name":"Practice",
				"contest_start_date_iso":"` + start.Format(time.RFC3339) + `",
				"contest_end_date_iso":"` + end.Format(time.RFC3339) + `"
			}
		]
	}`
	server := newCodeChefTestServer(t, payload, map[string]bool{"PRACTICE1": false})
	defer server.Close()

	src := codechef.NewWithURL(server.Client(), server.URL+"/api/list/contests/all")
	contests, err := src.FetchContests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 0 {
		t.Fatalf("got %d contests, want 0 unrated", len(contests))
	}
}

func TestFetchContestsIncludesPresentRunningContest(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(-45 * time.Minute)
	end := now.Add(2 * time.Hour)
	payload := `{
		"status":"success",
		"present_contests":[
			{
				"contest_code":"LIVE100",
				"contest_name":"Live Starters",
				"contest_start_date_iso":"` + start.Format(time.RFC3339) + `",
				"contest_end_date_iso":"` + end.Format(time.RFC3339) + `"
			}
		],
		"future_contests":[]
	}`
	server := newCodeChefTestServer(t, payload, map[string]bool{"LIVE100": true})
	defer server.Close()

	src := codechef.NewWithURL(server.Client(), server.URL+"/api/list/contests/all")
	contests, err := src.FetchContests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 1 {
		t.Fatalf("got %d contests", len(contests))
	}
	if contests[0].ExternalID != "LIVE100" {
		t.Fatalf("external id = %s", contests[0].ExternalID)
	}
	if contests[0].Status != domain.ContestStatusRunning {
		t.Fatalf("status = %s want running", contests[0].Status)
	}
}

func TestFetchContestsDeduplicatesPresentAndFuture(t *testing.T) {
	start := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	end := start.Add(3 * time.Hour)
	payload := `{
		"status":"success",
		"present_contests":[],
		"future_contests":[
			{
				"contest_code":"DUP1",
				"contest_name":"Dup",
				"contest_start_date_iso":"` + start.Format(time.RFC3339) + `",
				"contest_end_date_iso":"` + end.Format(time.RFC3339) + `"
			},
			{
				"contest_code":"DUP1",
				"contest_name":"Dup Again",
				"contest_start_date_iso":"` + start.Format(time.RFC3339) + `",
				"contest_end_date_iso":"` + end.Format(time.RFC3339) + `"
			}
		]
	}`
	server := newCodeChefTestServer(t, payload, map[string]bool{"DUP1": true})
	defer server.Close()

	src := codechef.NewWithURL(server.Client(), server.URL+"/api/list/contests/all")
	contests, err := src.FetchContests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 1 {
		t.Fatalf("got %d contests", len(contests))
	}
}

func TestFetchContestsHandlesMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer server.Close()

	src := codechef.NewWithURL(server.Client(), server.URL+"/api/list/contests/all")
	_, err := src.FetchContests(context.Background())
	if err == nil {
		t.Fatal("expected decode error")
	}
}
