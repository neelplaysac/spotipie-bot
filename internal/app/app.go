package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/neelplaysac/spotipie-go/internal/card"
	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/lastfm"
	"github.com/neelplaysac/spotipie-go/internal/observability"
	"github.com/neelplaysac/spotipie-go/internal/service"
	"github.com/neelplaysac/spotipie-go/internal/spotify"
	mongostore "github.com/neelplaysac/spotipie-go/internal/storage/mongo"
	"github.com/neelplaysac/spotipie-go/internal/telegram"
)

func Run(testMonitoring bool) int {
	cfg := config.Load()
	logger, logLevel := observability.NewLogger()
	logger, err := observability.Init(cfg, logger)
	slog.SetDefault(logger)
	if err != nil {
		logger.Error("failed to initialize GlitchTip monitoring", "err", err)
		return 1
	}
	defer observability.Flush(logger)
	defer func() {
		if r := recover(); r != nil {
			logger.Error("fatal panic", "panic", r)
			panic(r)
		}
	}()
	if testMonitoring {
		if cfg.SentryDSN == "" {
			logger.Error("set SENTRY_DSN before running -sentry-test")
			return 1
		}
		logger.Error("GlitchTip connection test")
		return 0
	}

	shutdownCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	if missing := cfg.MissingRequired(); len(missing) > 0 {
		logger.Error("missing required configuration", "missing", strings.Join(missing, ","))
		return 1
	}

	logger.Info("configuration loaded", "bot_username", cfg.BotUsername, "mongo_db", cfg.DatabaseName(), "temp_channel", cfg.TempChannel, "admins", len(cfg.AdminIDs), "log_level", logLevel.Level().String())

	db, err := mongostore.New(cfg, logger)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		return 1
	}
	defer db.Close()

	logger.Info("connected to database", "db", cfg.DatabaseName())

	spotifyClient := spotify.New(cfg, logger)
	lastFMClient := lastfm.New(cfg, logger)
	renderer := card.New(logger, "fonts")
	accounts := service.NewAccounts(db, spotifyClient, service.NewAuthorization(db))
	playback := service.NewPlayback(db, spotifyClient, lastFMClient, renderer)

	if cfg.TempChannel == 0 {
		logger.Warn("TEMP_CHANNEL is not set or invalid - inline queries will fail")
	}

	bot, err := gotgbot.NewBot(cfg.APIKey, nil)
	if err != nil {
		logger.Error("failed to create bot", "err", err)
		return 1
	}
	botUsername := bot.User.Username
	if err := cfg.ValidateBotIdentity(botUsername); err != nil {
		logger.Error("invalid Telegram return link", "err", err)
		return 1
	}

	logger.Info("Starting Spotipie Bot...", "username", botUsername)

	handler := telegram.New(cfg, accounts, playback, logger, botUsername)
	dispatcher := handler.Dispatcher()

	updater := ext.NewUpdater(dispatcher, &ext.UpdaterOpts{Logger: logger})

	if err := updater.StartPolling(bot, &ext.PollingOpts{
		DropPendingUpdates: false,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			// Long-poll: Telegram holds the connection up to 20s when idle.
			Timeout:     20,
			RequestOpts: &gotgbot.RequestOpts{Timeout: 25 * time.Second},
		},
	}); err != nil {
		logger.Error("failed to start polling", "err", err)
		return 1
	}

	logger.Info("Starting Telegram bot polling...")
	<-shutdownCtx.Done()
	logger.Info("stopping Telegram bot polling...")
	if err := stopUpdater(updater, 20*time.Second); err != nil {
		logger.Error("failed to stop Telegram updater", "err", err)
		return 1
	}
	logger.Info("shutdown complete")
	return 0
}

func stopUpdater(updater *ext.Updater, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- updater.Stop() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return fmt.Errorf("Telegram updater did not stop within %s", timeout)
	}
}
