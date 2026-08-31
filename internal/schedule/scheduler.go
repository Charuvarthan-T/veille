package schedule

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Job func(ctx context.Context) error

type Scheduler struct {
	log *slog.Logger
	wg  sync.WaitGroup
}

func New(log *slog.Logger) *Scheduler {
	return &Scheduler{log: log}
}

func (s *Scheduler) Every(ctx context.Context, name string, interval time.Duration, job Job) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runLoop(ctx, name, interval, job)
	}()
}

func (s *Scheduler) Wait() {
	s.wg.Wait()
}

func (s *Scheduler) runLoop(ctx context.Context, name string, interval time.Duration, job Job) {
	s.log.Info("scheduler started", "job", name, "interval", interval.String())
	s.execute(ctx, name, job)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped", "job", name)
			return
		case <-ticker.C:
			s.execute(ctx, name, job)
		}
	}
}

func (s *Scheduler) execute(ctx context.Context, name string, job Job) {
	if ctx.Err() != nil {
		return
	}
	s.log.Info("scheduler tick", "job", name)
	if err := job(ctx); err != nil {
		s.log.Error("scheduler job failed", "job", name, "error", err)
	}
}
