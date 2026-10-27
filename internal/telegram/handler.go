package telegram

import (
	"log/slog"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/inlinequery"

	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

type Handler struct {
	cfg       *config.Config
	accounts  *service.Accounts
	playback  *service.Playback
	logger    *slog.Logger
	username  string
	cooldowns *cooldownManager
}

func New(cfg *config.Config, accounts *service.Accounts, playback *service.Playback, logger *slog.Logger, username string) *Handler {
	return &Handler{cfg: cfg, accounts: accounts, playback: playback, logger: logger, username: username, cooldowns: newCooldownManager(logger)}
}

func (h *Handler) Dispatcher() *ext.Dispatcher {
	d := ext.NewDispatcher(&ext.DispatcherOpts{Processor: monitoringProcessor{}, Error: h.errorHandler, Panic: h.panicHandler, Logger: h.logger})
	h.registerHandlers(d)
	return d
}

func (h *Handler) registerHandlers(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("start", h.startHandler))
	d.AddHandler(handlers.NewCommand("help", h.helpHandler))
	d.AddHandler(handlers.NewCommand("register", h.registerHandler))
	d.AddHandler(handlers.NewCommand("unregister", h.unregisterHandler))
	d.AddHandler(handlers.NewCommand("now", h.nowHandler))
	d.AddHandler(handlers.NewCommand("last", h.lastHandler))
	d.AddHandler(handlers.NewCommand("style", h.styleHandler))
	d.AddHandler(handlers.NewCommand("unlinkfm", h.unlinkfmHandler))
	d.AddHandler(handlers.NewCommand("statss", h.statssHandler))
	d.AddHandler(handlers.NewCommand("details", h.detailsHandler))
	d.AddHandler(handlers.NewCallback(callbackquery.All, h.styleButton))
	d.AddHandler(handlers.NewInlineQuery(inlinequery.All, h.inlineNowPlaying))

	d.AddHandler(h.nameConversation())
	d.AddHandler(h.linkfmConversation())
	d.AddHandler(h.namefmConversation())
}

func (h *Handler) nameConversation() handlers.Conversation {
	return handlers.NewConversation(
		[]ext.Handler{handlers.NewCommand("name", h.getName)},
		map[string][]ext.Handler{
			nameState: {handlers.NewMessage(textNotCommand, h.setName)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", h.cancelHandler)},
			AllowReEntry: true,
			StateStorage: newExpiringStorage(time.Minute),
		},
	)
}

func (h *Handler) linkfmConversation() handlers.Conversation {
	return handlers.NewConversation(
		[]ext.Handler{handlers.NewCommand("linkfm", h.getLastFMUserName)},
		map[string][]ext.Handler{
			linkFMState: {handlers.NewMessage(textNotCommand, h.linkLastFMUser)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", h.cancelHandler)},
			AllowReEntry: true,
			StateStorage: newExpiringStorage(time.Minute),
		},
	)
}

func (h *Handler) namefmConversation() handlers.Conversation {
	return handlers.NewConversation(
		[]ext.Handler{handlers.NewCommand("namefm", h.getLastFMUserData)},
		map[string][]ext.Handler{
			nameFMState: {handlers.NewMessage(textNotCommand, h.setLastFMUserData)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", h.cancelHandler)},
			AllowReEntry: true,
			StateStorage: newExpiringStorage(time.Minute),
		},
	)
}
