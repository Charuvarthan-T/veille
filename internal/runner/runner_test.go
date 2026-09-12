package runner_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/Charuvarthan-T/veille/internal/notify"
	"github.com/Charuvarthan-T/veille/internal/runner"
	"github.com/Charuvarthan-T/veille/internal/syncer"
)

type fakeSyncer struct {
	results []syncer.Result
	calls   int
}

func (f *fakeSyncer) Run(context.Context) []syncer.Result {
	f.calls++
	return f.results
}

type fakeOrchestrator struct {
	result notify.DispatchResult
	err    error
	calls  int
}

func (f *fakeOrchestrator) Run(context.Context) (notify.DispatchResult, error) {
	f.calls++
	return f.result, f.err
}

func TestOnceRunsSyncThenDispatch(t *testing.T) {
	sync := &fakeSyncer{
		results: []syncer.Result{{
			Source:   "codeforces",
			Fetched:  2,
			Inserted: 1,
		}},
	}
	orch := &fakeOrchestrator{
		result: notify.DispatchResult{Claimed: 1, Sent: 1},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, err := runner.Once(context.Background(), sync, orch, log)
	if err != nil {
		t.Fatal(err)
	}
	if sync.calls != 1 || orch.calls != 1 {
		t.Fatalf("sync calls=%d orch calls=%d", sync.calls, orch.calls)
	}
	if result.Dispatch.Sent != 1 {
		t.Fatalf("dispatch = %+v", result.Dispatch)
	}
}

func TestOnceReturnsErrorWhenDispatchFails(t *testing.T) {
	sync := &fakeSyncer{}
	orch := &fakeOrchestrator{err: errors.New("db down")}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := runner.Once(context.Background(), sync, orch, log)
	if err == nil {
		t.Fatal("expected error")
	}
	if sync.calls != 1 {
		t.Fatal("sync should run before dispatch failure")
	}
}

func TestOnceContinuesAfterSourceError(t *testing.T) {
	sync := &fakeSyncer{
		results: []syncer.Result{{
			Source:    "codechef",
			SourceErr: errors.New("timeout"),
		}},
	}
	orch := &fakeOrchestrator{result: notify.DispatchResult{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	result, err := runner.Once(context.Background(), sync, orch, log)
	if err != nil {
		t.Fatal(err)
	}
	if result.Sync[0].SourceErr == nil {
		t.Fatal("expected source error preserved")
	}
	if orch.calls != 1 {
		t.Fatal("dispatch should still run")
	}
}
