package telegram

import (
	"context"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/getsentry/sentry-go"
)

// Clone the hub for each concurrent update. Never put Telegram payloads or
// account details on the shared global scope.
const monitoringContextKey = "spotipie.monitoring.context"

type monitoringProcessor struct{}

func (monitoringProcessor) ProcessUpdate(d *ext.Dispatcher, b *gotgbot.Bot, c *ext.Context) (err error) {
	if !sentry.CurrentHub().Client().IsEnabled() {
		return (ext.BaseProcessor{}).ProcessUpdate(d, b, c)
	}
	hub := sentry.CurrentHub().Clone()
	ctx := sentry.SetHubOnContext(context.Background(), hub)
	transaction := sentry.StartTransaction(ctx, updateOperation(c), sentry.WithOpName("telegram.update"))
	if c.Data == nil {
		c.Data = make(map[string]any)
	}
	c.Data[monitoringContextKey] = transaction.Context()
	completed := false
	defer func() {
		if !completed || err != nil {
			transaction.Status = sentry.SpanStatusInternalError
		}
		transaction.Finish()
	}()
	err = (ext.BaseProcessor{}).ProcessUpdate(d, b, c)
	completed = true
	return err
}

func updateContext(c *ext.Context) context.Context {
	if c != nil {
		if ctx, ok := c.Data[monitoringContextKey].(context.Context); ok {
			return ctx
		}
	}
	return context.Background()
}

func updateOperation(c *ext.Context) string {
	switch {
	case c.Message != nil:
		// Whitelist command names: /start arguments contain OAuth code IDs, and
		// ordinary messages can contain names or other private information.
		fields := strings.Fields(c.Message.Text)
		if len(fields) > 0 {
			command, _, _ := strings.Cut(strings.ToLower(fields[0]), "@")
			switch command {
			case "/start", "/help", "/register", "/unregister", "/now", "/last", "/style", "/name", "/linkfm", "/unlinkfm", "/namefm", "/statss", "/details", "/cancel":
				return "telegram." + strings.TrimPrefix(command, "/")
			}
		}
		return "telegram.message"
	case c.CallbackQuery != nil:
		return "telegram.callback_query"
	case c.InlineQuery != nil:
		return "telegram.inline_query"
	default:
		return "telegram.other"
	}
}
