package codeforces_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/source/codeforces"
)

func newCodeforcesTestServer(t *testing.T, listBody string, rated map[int64]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "contest.list") || r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(listBody))
			return
		}
		if strings.Contains(r.URL.Path, "contest.ratingChanges") {
			idStr := r.URL.Query().Get("contestId")
			id, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			body, ok := rated[id]
			if !ok {
				body = `{"status":"FAILED","comment":"Rating changes are unavailable for this contest"}`
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
}

func TestFetchContestsIncludesUpcomingAndRunning(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	futureStart := now.Add(48 * time.Hour).Unix()
	runningStart := now.Add(-30 * time.Minute).Unix()
	listBody := `{
		"status":"OK",
		"result":[
			{"id":101,"name":"Future Round","phase":"BEFORE","durationSeconds":7200,"startTimeSeconds":` + itoa(futureStart) + `},
			{"id":102,"name":"Live Round","phase":"CODING","durationSeconds":7200,"startTimeSeconds":` + itoa(runningStart) + `},
			{"id":100,"name":"Finished","phase":"FINISHED","durationSeconds":7200,"startTimeSeconds":100}
		]
	}`
	server := newCodeforcesTestServer(t, listBody, nil)
	defer server.Close()

	src := codeforces.NewWithURLAndClock(server.Client(), server.URL+"/contest.list", func() time.Time { return now })
	contests, err := src.FetchContests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 2 {
		t.Fatalf("got %d contests, want 2", len(contests))
	}

	byID := map[string]domain.Contest{}
	for _, c := range contests {
		byID[c.ExternalID] = c
	}
	if byID["101"].Status != domain.ContestStatusUpcoming {
		t.Fatalf("101 status = %s", byID["101"].Status)
	}
	if byID["102"].Status != domain.ContestStatusRunning {
		t.Fatalf("102 status = %s", byID["102"].Status)
	}
	if byID["102"].URL != "https://codeforces.com/contest/102" {
		t.Fatalf("url = %s", byID["102"].URL)
	}
}

func TestFetchContestsExcludesUnratedWithEmptyRatingChanges(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	futureStart := now.Add(48 * time.Hour).Unix()
	listBody := `{
		"status":"OK",
		"result":[
			{"id":201,"name":"Practice Round","phase":"BEFORE","durationSeconds":7200,"startTimeSeconds":` + itoa(futureStart) + `}
		]
	}`
	server := newCodeforcesTestServer(t, listBody, map[int64]string{
		201: `{"status":"OK","result":[]}`,
	})
	defer server.Close()

	src := codeforces.NewWithURLAndClock(server.Client(), server.URL+"/contest.list", func() time.Time { return now })
	contests, err := src.FetchContests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 0 {
		t.Fatalf("got %d contests, want 0 unrated", len(contests))
	}
}

func TestFetchContestsExcludesFinishedByTime(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	endedStart := now.Add(-4 * time.Hour).Unix()
	listBody := `{
		"status":"OK",
		"result":[
			{"id":300,"name":"Ended","phase":"CODING","durationSeconds":7200,"startTimeSeconds":` + itoa(endedStart) + `}
		]
	}`
	server := newCodeforcesTestServer(t, listBody, nil)
	defer server.Close()

	src := codeforces.NewWithURLAndClock(server.Client(), server.URL+"/contest.list", func() time.Time { return now })
	contests, err := src.FetchContests(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 0 {
		t.Fatalf("got %d contests, want 0", len(contests))
	}
}

func TestFetchContestsRejectsBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"FAILED","comment":"nope"}`))
	}))
	defer server.Close()

	src := codeforces.NewWithURL(server.Client(), server.URL)
	_, err := src.FetchContests(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func itoa(v int64) string {
	const digits = "0123456789"
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = digits[v%10]
		v /= 10
	}
	return string(buf[i:])
}
