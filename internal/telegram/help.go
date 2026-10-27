package telegram

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func (h *Handler) helpHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Help", Url: fmt.Sprintf(BOT_URL, h.username) + "?start=help"}},
		})
		return sendText(b, ctx, msg.Chat.Id, "Contact me in PM to get the list of possible commands.", markup)
	}

	return sendText(b, ctx, msg.Chat.Id, HELP_TEXT, nil)
}

func (h *Handler) cancelHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	if msg := c.Update.Message; msg != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Canceled.", nil)
	}
	return handlers.EndConversation()
}
