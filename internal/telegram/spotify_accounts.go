package telegram

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"

	"github.com/neelplaysac/spotipie-go/internal/service"
)

const nameState = "NAME"

func (h *Handler) startHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	update := c.Update
	msg := update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		return sendText(b, ctx, msg.Chat.Id, "Hmm?", nil)
	}

	text := msg.Text
	firstName := ""
	if c.EffectiveUser != nil {
		firstName = c.EffectiveUser.FirstName
	}

	if len(text) <= 10 {
		_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf(START_TEXT, firstName), nil)
	} else if strings.HasSuffix(text, "register") {
		_ = sendText(b, ctx, msg.Chat.Id, "To register your account use /register command.", nil)
	} else if strings.HasSuffix(text, "username") {
		_ = sendText(b, ctx, msg.Chat.Id, "To change your username use /name command.", nil)
	} else if strings.HasSuffix(text, "token") {
		_ = sendText(b, ctx, msg.Chat.Id, "use /unregister to unlink your account & register again using /register command.", nil)
	} else if strings.HasSuffix(text, "notsure") {
		_ = sendText(b, ctx, msg.Chat.Id, "I'm not sure what you're listening to.", nil)
	} else if strings.HasSuffix(text, "ads") {
		_ = sendText(b, ctx, msg.Chat.Id, "You're listening to ads!", nil)
	} else if strings.HasSuffix(text, "notlistening") {
		_ = sendText(b, ctx, msg.Chat.Id, "You're not listening to anything on Spotify at the moment.", nil)
	} else if strings.HasSuffix(text, "lastfm") {
		_ = sendText(b, ctx, msg.Chat.Id, "To link your lastfm account use /lastfm command.", nil)
	} else {
		tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
		status, err := h.accounts.CompleteSpotifyRegistration(ctx, text[7:], tgID)
		if err != nil {
			h.logger.ErrorContext(ctx, "failed to complete Spotify registration", "err", err)
			_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
			return nil
		}
		switch status {
		case service.AccountInvalidCode:
			_ = sendText(b, ctx, msg.Chat.Id, "Please use /register command to initiate the login process.", nil)
			return nil
		case service.AccountMissingCode:
			_ = sendText(b, ctx, msg.Chat.Id, "Please use /register command to initiate the login process.", nil)
		case service.AccountAlreadyLinked:
			_ = sendText(b, ctx, msg.Chat.Id, "You are already registered. If the bot is not working /unregister and /register again.", nil)
			return nil
		case service.AccountAuthFailed:
			_ = sendText(b, ctx, msg.Chat.Id, "Unable to authenticate. Please try again using /register. If you are having issues using the bot contact in support chat (check bot info)", nil)
			return nil
		case service.AccountUpdated:
			_ = sendText(b, ctx, msg.Chat.Id, "Account successfully linked. Now use /name to set a display name then use /now to use the bot.", nil)
		}
	}

	// delete the /start message (python: update.effective_message.delete())
	_, _ = b.DeleteMessageWithContext(ctx, msg.Chat.Id, msg.MessageId, nil)
	return nil
}

func (h *Handler) registerHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := h.cooldowns.Check(c.EffectiveUser.Id, "register", 3); rem > 0 {
		h.logger.Debug("command throttled", "handler", "register", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	if msg.Chat.Type == "private" {
		authURL, err := h.accounts.SpotifyAuthURL(ctx, strconv.FormatInt(c.EffectiveUser.Id, 10))
		if err != nil {
			h.logger.ErrorContext(ctx, "failed to create authorization session", "err", err)
			return sendText(b, ctx, msg.Chat.Id, "Unable to start registration. Please try /register again shortly.", nil)
		}
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Open this link to register", Url: authURL}},
		})
		return sendText(b, ctx, msg.Chat.Id, REG_REG_MSG, markup)
	}

	markup := kb([][]gotgbot.InlineKeyboardButton{
		{{Text: "Register here", Url: fmt.Sprintf(BOT_URL, h.username)}},
	})
	return sendText(b, ctx, msg.Chat.Id, REG_PM_MSG, markup)
}

func (h *Handler) unregisterHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := h.cooldowns.Check(c.EffectiveUser.Id, "unregister", 3); rem > 0 {
		h.logger.Debug("command throttled", "handler", "unregister", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	if msg.Chat.Type == "private" {
		removed, err := h.accounts.UnlinkSpotify(ctx, tgID)
		if err != nil {
			return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
		}
		if !removed {
			return sendText(b, ctx, msg.Chat.Id, "You haven't registered your account yet.", nil)
		}

		return sendText(b, ctx, msg.Chat.Id, "Account successfully removed.", nil)
	}

	return sendText(b, ctx, msg.Chat.Id, REG_PM_MSG, h.contactInPMButton())
}

func (h *Handler) getName(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Change username", Url: fmt.Sprintf(BOT_URL, h.username)}},
		})
		return sendText(b, ctx, msg.Chat.Id, NAME_PM_MSG, markup)
	}

	_ = sendText(b, ctx, msg.Chat.Id, "Send me a username (max 15 characters)", nil)
	return handlers.NextConversationState(nameState)
}

func (h *Handler) setName(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
	text := strings.TrimSpace(msg.Text)
	status, err := h.accounts.SetSpotifyName(ctx, tgID, text)
	switch {
	case err != nil:
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
	case status == service.AccountInvalidInput:
		_ = sendText(b, ctx, msg.Chat.Id, "Invalid username. Must be 1-15 characters and not start with /. Try again using /name", nil)
	case status == service.AccountNotRegistered:
		_ = sendText(b, ctx, msg.Chat.Id, NAME_REG_MSG, nil)
	default:
		_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Username updated to %s", text), nil)
	}
	return handlers.EndConversation()
}
