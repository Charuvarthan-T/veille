package codeforces_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/source/codeforces"
)

func TestFetchUpcomingNormalizesBeforePhase(t *testing.T) {
	start := time.Now().UTC().Add(48 * time.Hour).Unix()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status":"OK",
			"result":[
				{"id":101,"name":"Future Round","phase":"BEFORE","durationSeconds":7200,"startTimeSeconds":` + itoa(start) + `},
				{"id":100,"name":"Finished","phase":"FINISHED","durationSeconds":7200,"startTimeSeconds":100}
			]
		}`))
	}))
	defer server.Close()

	src := codeforces.NewWithURL(server.Client(), server.URL)
	contests, err := src.FetchUpcoming(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(contests) != 1 {
		t.Fatalf("got %d contests, want 1", len(contests))
	}
	if contests[0].ExternalID != "101" {
		t.Fatalf("external id = %s", contests[0].ExternalID)
	}
	if contests[0].URL != "https://codeforces.com/contest/101" {
		t.Fatalf("url = %s", contests[0].URL)
	}
}

func TestFetchUpcomingRejectsBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"FAILED","comment":"nope"}`))
	}))
	defer server.Close()

	src := codeforces.NewWithURL(server.Client(), server.URL)
	_, err := src.FetchUpcoming(context.Background())
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
