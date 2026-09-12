package runner

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Charuvarthan-T/veille/internal/notify"
	"github.com/Charuvarthan-T/veille/internal/syncer"
)

type Syncer interface {
	Run(ctx context.Context) []syncer.Result
}

type Orchestrator interface {
	Run(ctx context.Context) (notify.DispatchResult, error)
}

type Result struct {
	Sync     []syncer.Result
	Dispatch notify.DispatchResult
}

func Once(ctx context.Context, sync Syncer, orch Orchestrator, log *slog.Logger) (Result, error) {
	syncResults := sync.Run(ctx)
	for _, result := range syncResults {
		if result.SourceErr != nil {
			log.Error("collection source error", "source", result.Source, "error", result.SourceErr)
		}
	}

	dispatch, err := orch.Run(ctx)
	if err != nil {
		return Result{Sync: syncResults}, fmt.Errorf("dispatch notifications: %w", err)
	}

	log.Info("veille run completed",
		"sync_sources", len(syncResults),
		"claimed", dispatch.Claimed,
		"sent", dispatch.Sent,
		"failed", dispatch.Failed,
		"skipped", dispatch.Skipped,
	)

	return Result{Sync: syncResults, Dispatch: dispatch}, nil
}
