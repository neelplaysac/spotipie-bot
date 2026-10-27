package telegram

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func (h *Handler) errorHandler(b *gotgbot.Bot, c *ext.Context, err error) ext.DispatcherAction {
	ctx := updateContext(c)
	attrs := []any{"err", err}
	if c.EffectiveUser != nil {
		attrs = append(attrs, "user_id", c.EffectiveUser.Id)
	}
	if c.EffectiveChat != nil {
		attrs = append(attrs, "chat_id", c.EffectiveChat.Id)
	}
	h.logger.ErrorContext(ctx, "Exception while handling an update", attrs...)

	if c.EffectiveMessage != nil {
		_, _ = b.SendMessageWithContext(ctx, c.EffectiveChat.Id, "Something went wrong. Please try again later.", &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeMarkdown})
	}
	return ext.DispatcherActionContinueGroups
}

func (h *Handler) panicHandler(b *gotgbot.Bot, c *ext.Context, r any) {
	attrs := []any{"panic", r}
	if c.EffectiveUser != nil {
		attrs = append(attrs, "user_id", c.EffectiveUser.Id)
	}
	h.logger.ErrorContext(updateContext(c), "panic while handling update", attrs...)
}
