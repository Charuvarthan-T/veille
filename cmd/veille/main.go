package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Charuvarthan-T/veille/internal/clock"
	"github.com/Charuvarthan-T/veille/internal/config"
	"github.com/Charuvarthan-T/veille/internal/domain"
	"github.com/Charuvarthan-T/veille/internal/migrate"
	"github.com/Charuvarthan-T/veille/internal/notify"
	"github.com/Charuvarthan-T/veille/internal/notify/resend"
	"github.com/Charuvarthan-T/veille/internal/notify/twilio"
	"github.com/Charuvarthan-T/veille/internal/schedule"
	"github.com/Charuvarthan-T/veille/internal/source"
	"github.com/Charuvarthan-T/veille/internal/source/codechef"
	"github.com/Charuvarthan-T/veille/internal/source/codeforces"
	"github.com/Charuvarthan-T/veille/internal/store/postgres"
	"github.com/Charuvarthan-T/veille/internal/syncer"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("failed to load .env", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		logger.Error("invalid timezone", "timezone", cfg.Timezone, "error", err)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(rootCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("database close failed", "error", err)
		}
	}()

	migrationsDir := filepath.Join("migrations")
	if err := migrate.Up(db.SQL(), migrationsDir); err != nil {
		logger.Error("migration failed", "error", err)
		os.Exit(1)
	}
	logger.Info("migrations applied")

	httpClient := &http.Client{Timeout: cfg.HTTPTimeout}
	sources := []source.ContestSource{
		codeforces.New(httpClient),
		codechef.New(httpClient),
	}

	channels := []domain.Channel{domain.ChannelWhatsApp, domain.ChannelEmail}
	clk := clock.System{}
	contestSyncer := syncer.New(sources, db, clk, channels, cfg.ReminderLead, logger)

	senders := []notify.ChannelSender{
		twilio.New(httpClient, cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioWhatsAppFrom, cfg.WhatsAppTo),
		resend.New(httpClient, cfg.ResendAPIKey, cfg.EmailFrom, cfg.EmailTo),
	}
	orchestrator := notify.NewOrchestrator(
		db,
		senders,
		clk,
		location,
		cfg.ReminderLead,
		cfg.ReminderWindow,
		cfg.NotificationMaxAttempts,
		logger,
	)

	scheduler := schedule.New(logger)
	scheduler.Every(rootCtx, "collect", cfg.CollectInterval, func(ctx context.Context) error {
		results := contestSyncer.Run(ctx)
		for _, result := range results {
			if result.SourceErr != nil {
				logger.Error("collection source error", "source", result.Source, "error", result.SourceErr)
			}
		}
		return nil
	})
	scheduler.Every(rootCtx, "notify", cfg.NotifyInterval, func(ctx context.Context) error {
		_, err := orchestrator.Run(ctx)
		return err
	})

	logger.Info("veille started",
		"timezone", cfg.Timezone,
		"collect_interval", cfg.CollectInterval.String(),
		"notify_interval", cfg.NotifyInterval.String(),
	)

	<-rootCtx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		scheduler.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("scheduler drained")
	case <-shutdownCtx.Done():
		logger.Warn("shutdown timed out waiting for scheduler")
	}

	logger.Info("veille stopped")
}
