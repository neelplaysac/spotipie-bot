package telegram

import (
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/neelplaysac/spotipie-go/internal/service"
)

func (h *Handler) nowHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	update := c.Update
	msg := update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := h.cooldowns.Check(c.EffectiveUser.Id, "now", 5); rem > 0 {
		h.logger.Debug("command throttled", "handler", "now", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	_, _ = b.SendChatActionWithContext(ctx, msg.Chat.Id, gotgbot.ChatActionTyping, nil)

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
	result, err := h.playback.Spotify(ctx, tgID, func() []byte { return h.fetchProfilePic(b, ctx, c.EffectiveUser.Id) })
	if err != nil {
		failure := err.(*service.PlaybackError)
		messages := map[string]string{"db": "now: db fetch failed", "spotify": "now: spotify request failed", "render": "now: image gen failed"}
		h.logger.ErrorContext(ctx, messages[failure.Stage], "err", failure.Err, "user_id", tgID, "chat_id", msg.Chat.Id)
		if failure.Stage == "render" {
			return sendText(b, ctx, msg.Chat.Id, "You are not listening to anything.", nil)
		}
		return nil
	}
	switch result.State {
	case service.PlaybackNotRegistered:
		return sendText(b, ctx, msg.Chat.Id, NOW_REG_MSG, h.contactInPMButton())
	case service.PlaybackNameRequired:
		return sendText(b, ctx, msg.Chat.Id, NOW_USR_NAME_MSG, h.contactInPMButton())
	case service.PlaybackInvalidToken:
		return sendText(b, ctx, msg.Chat.Id, NOW_TOKEN_ERR_MSG, h.contactInPMButton())
	case service.PlaybackInactive:
		return sendText(b, ctx, msg.Chat.Id, "You are not listening to anything.", nil)
	case service.PlaybackAd:
		return sendText(b, ctx, msg.Chat.Id, "You're listening to ads.", nil)
	case service.PlaybackTrack:
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Play on Spotify", Url: result.URL}},
		})
		_, err = b.SendPhotoWithContext(ctx, msg.Chat.Id, gotgbot.InputFileByReader("now.jpg", strings.NewReader(string(result.Image))), &gotgbot.SendPhotoOpts{ReplyMarkup: markup})
		if err != nil {
			h.logger.ErrorContext(ctx, "now: photo send failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
		}
		return nil
	default:
		return sendText(b, ctx, msg.Chat.Id, "Not sure what you're listening to.", nil)
	}
}

func (h *Handler) lastHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	update := c.Update
	msg := update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := h.cooldowns.Check(c.EffectiveUser.Id, "last", 5); rem > 0 {
		h.logger.Debug("command throttled", "handler", "last", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	_, _ = b.SendChatActionWithContext(ctx, msg.Chat.Id, gotgbot.ChatActionTyping, nil)

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
	result, err := h.playback.LastFM(ctx, tgID, func() []byte { return h.fetchProfilePic(b, ctx, c.EffectiveUser.Id) })
	if err != nil {
		failure := err.(*service.PlaybackError)
		messages := map[string]string{"db": "last: db fetch failed", "lastfm": "last: lastfm request failed", "render": "last: image gen failed"}
		h.logger.ErrorContext(ctx, messages[failure.Stage], "err", failure.Err, "user_id", tgID, "chat_id", msg.Chat.Id)
		if failure.Stage == "render" {
			return sendText(b, ctx, msg.Chat.Id, "Unable to get data from LastFm.", nil)
		}
		return nil
	}
	switch result.State {
	case service.PlaybackNotRegistered:
		if msg.Chat.Type == "private" {
			return sendText(b, ctx, msg.Chat.Id, "Use /linkfm command to link your lastfm account first.", nil)
		}
		return sendText(b, ctx, msg.Chat.Id, LAST_REG_MSG, h.contactInPMButton())
	case service.PlaybackNameRequired:
		if msg.Chat.Type == "private" {
			return sendText(b, ctx, msg.Chat.Id, "Use /namefm command to set a display name.", nil)
		}
		return sendText(b, ctx, msg.Chat.Id, LAST_USR_NAME_MSG, h.contactInPMButton())
	case service.PlaybackInvalidUser:
		return sendText(b, ctx, msg.Chat.Id, "LastFM username is invalid. Try to relink lastfm with the bot.", nil)
	case service.PlaybackInvalidData:
		return sendText(b, ctx, msg.Chat.Id, "Unable to get data from LastFm.", nil)
	}

	markup := kb([][]gotgbot.InlineKeyboardButton{
		{{Text: "LastFm Song Link", Url: result.URL}},
	})
	_, err = b.SendPhotoWithContext(ctx, msg.Chat.Id, gotgbot.InputFileByReader("last.jpg", strings.NewReader(string(result.Image))), &gotgbot.SendPhotoOpts{ReplyMarkup: markup})
	if err != nil {
		h.logger.ErrorContext(ctx, "last: photo send failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
	}
	return nil
}
