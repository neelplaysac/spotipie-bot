package telegram

import (
	"fmt"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func (h *Handler) styleHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := h.cooldowns.Check(c.EffectiveUser.Id, "style", 3); rem > 0 {
		h.logger.Debug("command throttled", "handler", "style", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	if msg.Chat.Type != "private" {
		return sendText(b, ctx, msg.Chat.Id, "Contact me in pm and use /style command to change background style.", h.contactInPMButton())
	}

	settings, err := h.accounts.Settings(ctx, tgID)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
	}
	user, fmUser := settings.Spotify, settings.LastFM

	if user == nil && fmUser == nil {
		return sendText(b, ctx, msg.Chat.Id, STYLE_REG_MSG, nil)
	}

	var buttons []gotgbot.InlineKeyboardButton

	if user != nil {
		currStyle := "Blur"
		if user.Style != "blur" {
			currStyle = "Black"
		}
		buttons = append(buttons, gotgbot.InlineKeyboardButton{
			Text:         fmt.Sprintf("Current Style: %s", currStyle),
			CallbackData: currStyle,
		})
	}

	if fmUser != nil {
		counterStatus := "Enabled"
		if fmUser.Counter == "off" {
			counterStatus = "Disabled"
		}
		buttons = append(buttons, gotgbot.InlineKeyboardButton{
			Text:         fmt.Sprintf("LastFm Scrobbles: %s", counterStatus),
			CallbackData: counterStatus,
		})
	}

	markup := kb([][]gotgbot.InlineKeyboardButton{buttons})
	return sendText(b, ctx, msg.Chat.Id, "Settings : ", markup)
}

func (h *Handler) styleButton(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	cq := c.CallbackQuery
	if cq == nil {
		return nil
	}

	if rem := h.cooldowns.Check(cq.From.Id, "style_button", 3); rem > 0 {
		_, _ = b.AnswerCallbackQueryWithContext(ctx, cq.Id, &gotgbot.AnswerCallbackQueryOpts{
			Text: fmt.Sprintf("Please wait %ds.", rem),
		})
		return nil
	}

	_, _ = b.AnswerCallbackQueryWithContext(ctx, cq.Id, nil)

	tgID := strconv.FormatInt(cq.From.Id, 10)

	styleMap := map[string]string{"Black": "blur", "Blur": "black"}
	counterMap := map[string]string{"Enabled": "off", "Disabled": "on"}

	updatedField := ""
	updatedValue := ""

	if v, ok := styleMap[cq.Data]; ok {
		_ = h.accounts.UpdateStyle(ctx, tgID, v)
		updatedField = "Style updated to"
		updatedValue = v
	}
	if v, ok := counterMap[cq.Data]; ok {
		_ = h.accounts.ToggleCounter(ctx, tgID, v)
		updatedField = "Scrobble Counter is now"
		updatedValue = v
	}

	if updatedField == "" {
		return nil
	}

	_, _, err := b.EditMessageTextWithContext(ctx, &gotgbot.EditMessageTextOpts{
		ChatId:    cq.Message.GetChat().Id,
		MessageId: cq.Message.GetMessageId(),
		Text:      fmt.Sprintf("%s: %s", updatedField, updatedValue),
	})
	if err != nil {
		chatID := int64(0)
		if c.EffectiveChat != nil {
			chatID = c.EffectiveChat.Id
		}
		h.logger.ErrorContext(ctx, "style: edit failed", "err", err, "user_id", cq.From.Id, "chat_id", chatID)
	}
	return nil
}
