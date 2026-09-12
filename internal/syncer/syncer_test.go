package syncer_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/Charuvarthan-T/veille/internal/clock"
	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/notify"
	"github.com/Charuvarthan-T/veille/internal/source"
	"github.com/Charuvarthan-T/veille/internal/syncer"
)

type fakeSource struct {
	platform domain.Platform
	name     string
	contests []domain.Contest
	err      error
}

func (f *fakeSource) Name() string              { return f.name }
func (f *fakeSource) Platform() domain.Platform { return f.platform }
func (f *fakeSource) FetchContests(context.Context) ([]domain.Contest, error) {
	return f.contests, f.err
}

type memoryStore struct {
	byKey         map[string]domain.Contest
	nextID        int64
	notifications map[string]time.Time
	upsertCalls   int
	ensureCalls   int
	refreshCalls  int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		byKey:         make(map[string]domain.Contest),
		nextID:        1,
		notifications: make(map[string]time.Time),
	}
}

func (m *memoryStore) UpsertContest(_ context.Context, contest domain.Contest) (domain.Contest, bool, error) {
	m.upsertCalls++
	key := contest.IdentityKey()
	existing, ok := m.byKey[key]
	if !ok {
		contest.ID = m.nextID
		m.nextID++
		m.byKey[key] = contest
		return contest, true, nil
	}
	contest.ID = existing.ID
	contest.FirstSeenAt = existing.FirstSeenAt
	contest.CreatedAt = existing.CreatedAt
	m.byKey[key] = contest
	return contest, false, nil
}

func (m *memoryStore) GetContest(_ context.Context, id int64) (domain.Contest, error) {
	for _, c := range m.byKey {
		if c.ID == id {
			return c, nil
		}
	}
	return domain.Contest{}, errors.New("not found")
}

func (m *memoryStore) EnsureActiveNotification(_ context.Context, contestID int64, dueAt time.Time) error {
	m.ensureCalls++
	m.notifications["email:"+strconv.FormatInt(contestID, 10)] = dueAt
	return nil
}

func (m *memoryStore) RefreshContestStatuses(context.Context, time.Time) (int64, error) {
	m.refreshCalls++
	return 0, nil
}

func TestSyncerInsertsUpdatesAndEnsuresActiveNotifications(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	start := now.Add(48 * time.Hour)
	src := &fakeSource{
		platform: domain.PlatformCodeforces,
		name:     "codeforces",
		contests: []domain.Contest{{
			ExternalID: "1001",
			Name:       "Round 1001",
			URL:        "https://codeforces.com/contest/1001",
			StartTime:  start,
			EndTime:    start.Add(2 * time.Hour),
			Duration:   2 * time.Hour,
		}},
	}
	st := newMemoryStore()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := syncer.New(
		[]source.ContestSource{src},
		st,
		clock.Fixed{Instant: now},
		log,
	)

	results := s.Run(context.Background())
	if len(results) != 1 || results[0].Inserted != 1 || results[0].Ensured != 1 {
		t.Fatalf("unexpected first sync result: %+v", results[0])
	}
	if st.refreshCalls != 1 {
		t.Fatal("expected status refresh after sync")
	}

	dueAt := st.notifications["email:1"]
	if !dueAt.Equal(notify.ActiveDueAt(start)) {
		t.Fatalf("due_at = %v want %v", dueAt, notify.ActiveDueAt(start))
	}

	src.contests[0].Name = "Round 1001 Div.2"
	src.contests[0].StartTime = start.Add(time.Hour)
	src.contests[0].EndTime = src.contests[0].StartTime.Add(2 * time.Hour)
	results = s.Run(context.Background())
	if results[0].Inserted != 0 || results[0].Updated != 1 {
		t.Fatalf("expected update, got %+v", results[0])
	}
	saved := st.byKey["codeforces:1001"]
	if saved.Status != domain.ContestStatusUpcoming {
		t.Fatalf("status = %s want upcoming", saved.Status)
	}
}

func TestSyncerSetsRunningStatusForActiveContest(t *testing.T) {
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	start := now.Add(-30 * time.Minute)
	end := start.Add(2 * time.Hour)
	src := &fakeSource{
		platform: domain.PlatformCodeChef,
		name:     "codechef",
		contests: []domain.Contest{{
			ExternalID: "START100",
			Name:       "Starters 100",
			URL:        "https://www.codechef.com/START100",
			StartTime:  start,
			EndTime:    end,
			Duration:   2 * time.Hour,
		}},
	}
	st := newMemoryStore()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := syncer.New([]source.ContestSource{src}, st, clock.Fixed{Instant: now}, log)

	results := s.Run(context.Background())
	if results[0].Inserted != 1 {
		t.Fatalf("unexpected result: %+v", results[0])
	}
	saved := st.byKey["codechef:START100"]
	if saved.Status != domain.ContestStatusRunning {
		t.Fatalf("status = %s want running", saved.Status)
	}
}

func TestSyncerSurvivesSourceFailure(t *testing.T) {
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	failing := &fakeSource{platform: domain.PlatformCodeChef, name: "codechef", err: errors.New("boom")}
	ok := &fakeSource{
		platform: domain.PlatformCodeforces,
		name:     "codeforces",
		contests: []domain.Contest{{
			ExternalID: "42",
			Name:       "Test",
			URL:        "https://codeforces.com/contest/42",
			StartTime:  now.Add(30 * time.Hour),
			EndTime:    now.Add(32 * time.Hour),
			Duration:   2 * time.Hour,
		}},
	}
	st := newMemoryStore()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	s := syncer.New(
		[]source.ContestSource{failing, ok},
		st,
		clock.Fixed{Instant: now},
		log,
	)
	results := s.Run(context.Background())
	if results[0].SourceErr == nil {
		t.Fatal("expected source error")
	}
	if results[1].Inserted != 1 {
		t.Fatalf("healthy source should still sync: %+v", results[1])
	}
}
