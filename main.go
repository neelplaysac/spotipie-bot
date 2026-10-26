package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func main() {
	testMonitoring := flag.Bool("sentry-test", false, "send a test error to GlitchTip and exit without starting the bot")
	flag.Parse()
	os.Exit(run(*testMonitoring))
}

// Return an exit code so startup failures still run the Sentry flush defer.
func run(testMonitoring bool) int {
	cfgConfig = loadConfig()
	configureLogging() // .env has now been loaded.
	if err := initMonitoring(cfgConfig); err != nil {
		logger.Error("failed to initialize GlitchTip monitoring", "err", err)
		return 1
	}
	defer flushMonitoring()
	defer func() {
		if r := recover(); r != nil {
			logger.Error("fatal panic", "panic", r)
			panic(r)
		}
	}()
	if testMonitoring {
		if cfgConfig.SentryDSN == "" {
			logger.Error("set SENTRY_DSN before running -sentry-test")
			return 1
		}
		logger.Error("GlitchTip connection test")
		return 0
	}

	shutdownCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	if missing := cfgConfig.missingRequired(); len(missing) > 0 {
		logger.Error("missing required configuration", "missing", strings.Join(missing, ","))
		return 1
	}

	logger.Info("configuration loaded", "bot_username", cfgConfig.BotUsername, "mongo_db", cfgConfig.MongoColl, "temp_channel", cfgConfig.TempChannel, "admins", len(cfgConfig.AdminIDs), "log_level", logLevel.Level().String())

	var err error
	db, err = newDatabase(cfgConfig)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		return 1
	}
	defer db.Close()

	logger.Info("connected to database", "db", cfgConfig.MongoColl)

	spotifyClient = newSpotifyClient(cfgConfig)
	lastFMClient = newLastFMClient(cfgConfig)
	cooldowns = newCooldownManager()

	if cfgConfig.TempChannel == 0 {
		logger.Warn("TEMP_CHANNEL is not set or invalid - inline queries will fail")
	}

	bot, err := gotgbot.NewBot(cfgConfig.APIKey, nil)
	if err != nil {
		logger.Error("failed to create bot", "err", err)
		return 1
	}
	botUsername = bot.User.Username

	logger.Info("Starting Spotipie Bot...", "username", botUsername)

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Processor: monitoringProcessor{},
		Error:     errorHandler,
		Panic:     panicHandler,
		Logger:    logger,
	})
	registerHandlers(dispatcher)

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

// errorHandler mirrors the python bot's global error handler: log and notify the user.
func errorHandler(b *gotgbot.Bot, c *ext.Context, err error) ext.DispatcherAction {
	ctx := updateContext(c)
	attrs := []any{"err", err}
	if c.EffectiveUser != nil {
		attrs = append(attrs, "user_id", c.EffectiveUser.Id)
	}
	if c.EffectiveChat != nil {
		attrs = append(attrs, "chat_id", c.EffectiveChat.Id)
	}
	logger.ErrorContext(ctx, "Exception while handling an update", attrs...)

	if c.EffectiveMessage != nil {
		_, _ = b.SendMessageWithContext(ctx, c.EffectiveChat.Id, "Something went wrong. Please try again later.", &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeMarkdown})
	}
	return ext.DispatcherActionContinueGroups
}

func panicHandler(b *gotgbot.Bot, c *ext.Context, r any) {
	attrs := []any{"panic", r}
	if c.EffectiveUser != nil {
		attrs = append(attrs, "user_id", c.EffectiveUser.Id)
	}
	logger.ErrorContext(updateContext(c), "panic while handling update", attrs...)
}
