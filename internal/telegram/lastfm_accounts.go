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

const linkFMState = "LINKFM"
const nameFMState = "NAMEFM"

func (h *Handler) getLastFMUserName(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Link LastFm account.", Url: fmt.Sprintf(BOT_URL, h.username)}},
		})
		return sendText(b, ctx, msg.Chat.Id, FM_PM_MSG, markup)
	}

	_ = sendText(b, ctx, msg.Chat.Id, "Send me your LastFm username. use /cancel to cancel.", nil)
	return handlers.NextConversationState(linkFMState)
}

func (h *Handler) linkLastFMUser(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
	status, err := h.accounts.LinkLastFM(ctx, tgID, msg.Text)
	switch {
	case err != nil:
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
	case status == service.AccountInvalidInput:
		_ = sendText(b, ctx, msg.Chat.Id, "Invalid LastFm username. Must be 1-15 alphanumeric characters (hyphens and underscores allowed). Try again using /linkfm", nil)
	case status == service.AccountAlreadyLinked:
		_ = sendText(b, ctx, msg.Chat.Id, "Your LastFm account is already linked. Use /unlinkfm to unlink.", nil)
	default:
		_ = sendText(b, ctx, msg.Chat.Id, "LastFM account linked. Now set a display name using /namefm command.", nil)
	}
	return handlers.EndConversation()
}

func (h *Handler) unlinkfmHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := h.cooldowns.Check(c.EffectiveUser.Id, "unlinkfm", 3); rem > 0 {
		h.logger.Debug("command throttled", "handler", "unlinkfm", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	if msg.Chat.Type == "private" {
		removed, err := h.accounts.UnlinkLastFM(ctx, tgID)
		if err != nil {
			return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
		}
		if !removed {
			return sendText(b, ctx, msg.Chat.Id, "You haven't registered your account yet.", nil)
		}

		return sendText(b, ctx, msg.Chat.Id, "Account successfully removed.", nil)
	}

	return sendText(b, ctx, msg.Chat.Id, FM_PM_MSG, h.contactInPMButton())
}

func (h *Handler) getLastFMUserData(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Change display name", Url: fmt.Sprintf(BOT_URL, h.username)}},
		})
		return sendText(b, ctx, msg.Chat.Id, FM_NAME_PM_MSG, markup)
	}

	_ = sendText(b, ctx, msg.Chat.Id, "Send me a username (max 15 characters)", nil)
	return handlers.NextConversationState(nameFMState)
}

func (h *Handler) setLastFMUserData(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
	text := strings.TrimSpace(msg.Text)
	status, err := h.accounts.SetLastFMName(ctx, tgID, text)
	switch {
	case err != nil:
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
	case status == service.AccountInvalidInput:
		_ = sendText(b, ctx, msg.Chat.Id, "Invalid username. Must be 1-15 characters and not start with /. Try again using /namefm", nil)
	case status == service.AccountNotRegistered:
		_ = sendText(b, ctx, msg.Chat.Id, FM_NAME_REG_MSG, nil)
	default:
		_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Username updated to %s. Use /last to share lastfm song status.", text), nil)
	}
	return handlers.EndConversation()
}
