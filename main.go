package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func main() {
	cfgConfig = loadConfig()

	if missing := cfgConfig.missingRequired(); len(missing) > 0 {
		logger.Error("missing required configuration", "missing", strings.Join(missing, ","))
		os.Exit(1)
	}

	logger.Info("configuration loaded", "bot_username", cfgConfig.BotUsername, "mongo_db", cfgConfig.MongoColl, "temp_channel", cfgConfig.TempChannel, "admins", len(cfgConfig.AdminIDs), "log_level", logLevel.Level().String())

	var err error
	db, err = newDatabase(cfgConfig)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		os.Exit(1)
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
		os.Exit(1)
	}
	botUsername = bot.User.Username

	logger.Info("Starting Spotipie Bot...", "username", botUsername)

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error:  errorHandler,
		Panic:  panicHandler,
		Logger: logger,
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
		os.Exit(1)
	}

	logger.Info("Starting Telegram bot polling...")
	updater.Idle()
	logger.Info("shutdown complete")
}

// errorHandler mirrors the python bot's global error handler: log and notify the user.
func errorHandler(b *gotgbot.Bot, c *ext.Context, err error) ext.DispatcherAction {
	attrs := []any{"err", err}
	if c.EffectiveUser != nil {
		attrs = append(attrs, "user_id", c.EffectiveUser.Id)
	}
	if c.EffectiveChat != nil {
		attrs = append(attrs, "chat_id", c.EffectiveChat.Id)
	}
	logger.Error("Exception while handling an update", attrs...)

	if c.EffectiveMessage != nil {
		_, _ = b.SendMessageWithContext(context.Background(), c.EffectiveChat.Id, "Something went wrong. Please try again later.", &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeMarkdown})
	}
	return ext.DispatcherActionContinueGroups
}

func panicHandler(b *gotgbot.Bot, c *ext.Context, r any) {
	attrs := []any{"panic", r}
	if c.EffectiveUser != nil {
		attrs = append(attrs, "user_id", c.EffectiveUser.Id)
	}
	logger.Error("panic while handling update", attrs...)
}
