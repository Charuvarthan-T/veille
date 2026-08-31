package notify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Charuvarthan-T/veille/internal/clock"
	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/store"
)

type Orchestrator struct {
	store       store.Store
	senders     map[domain.Channel]ChannelSender
	clock       clock.Clock
	location    *time.Location
	lead        time.Duration
	window      time.Duration
	maxAttempts int
	claimLimit  int
	staleAfter  time.Duration
	log         *slog.Logger
}

func NewOrchestrator(
	st store.Store,
	senders []ChannelSender,
	clk clock.Clock,
	location *time.Location,
	lead, window time.Duration,
	maxAttempts int,
	log *slog.Logger,
) *Orchestrator {
	indexed := make(map[domain.Channel]ChannelSender, len(senders))
	for _, sender := range senders {
		indexed[sender.Channel()] = sender
	}
	return &Orchestrator{
		store:       st,
		senders:     indexed,
		clock:       clk,
		location:    location,
		lead:        lead,
		window:      window,
		maxAttempts: maxAttempts,
		claimLimit:  50,
		staleAfter:  10 * time.Minute,
		log:         log,
	}
}

type DispatchResult struct {
	Claimed int
	Sent    int
	Failed  int
}

func (o *Orchestrator) Run(ctx context.Context) (DispatchResult, error) {
	now := o.clock.Now()
	released, err := o.store.ReleaseStaleSending(ctx, now.Add(-o.staleAfter))
	if err != nil {
		return DispatchResult{}, fmt.Errorf("release stale notifications: %w", err)
	}
	if released > 0 {
		o.log.Warn("released stale sending claims", "count", released)
	}

	claimed, err := o.store.ClaimDue(ctx, now, o.claimLimit, o.maxAttempts)
	if err != nil {
		return DispatchResult{}, fmt.Errorf("claim due notifications: %w", err)
	}

	result := DispatchResult{Claimed: len(claimed)}
	for _, item := range claimed {
		if err := o.dispatchOne(ctx, item, now); err != nil {
			result.Failed++
			o.log.Error("notification dispatch failed",
				"notification_id", item.ID,
				"contest_id", item.ContestID,
				"channel", item.Channel,
				"error", err,
			)
			_ = o.store.MarkFailed(ctx, item.ID, err.Error())
			continue
		}
		result.Sent++
	}
	if result.Claimed > 0 {
		o.log.Info("notification dispatch completed",
			"claimed", result.Claimed,
			"sent", result.Sent,
			"failed", result.Failed,
		)
	}
	return result, nil
}

func (o *Orchestrator) dispatchOne(ctx context.Context, item domain.Notification, now time.Time) error {
	contest, err := o.store.GetContest(ctx, item.ContestID)
	if err != nil {
		return err
	}
	if !ShouldSend(item, contest, now, o.lead, o.window, o.maxAttempts) {
		return fmt.Errorf("notification no longer due for contest %d", contest.ID)
	}

	sender, ok := o.senders[item.Channel]
	if !ok {
		return fmt.Errorf("no sender configured for channel %s", item.Channel)
	}

	msg := BuildReminderMessage(contest, o.location)
	if err := sender.Send(ctx, msg); err != nil {
		return err
	}
	if err := o.store.MarkSent(ctx, item.ID, now); err != nil {
		return fmt.Errorf("persist sent status: %w", err)
	}
	o.log.Info("notification sent",
		"notification_id", item.ID,
		"contest_id", contest.ID,
		"channel", item.Channel,
		"platform", contest.Platform,
	)
	return nil
}
