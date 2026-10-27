package telegram

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func (h *Handler) statssHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	user := strconv.FormatInt(c.EffectiveUser.Id, 10)
	if !h.cfg.AdminIDs[user] {
		return sendText(b, ctx, msg.Chat.Id, "Only bot admins can use this command ^-^", nil)
	}

	total, err := h.accounts.CountSpotify(ctx)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
	}
	return sendText(b, ctx, msg.Chat.Id, strconv.FormatInt(total, 10), nil)
}

func (h *Handler) detailsHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	user := strconv.FormatInt(c.EffectiveUser.Id, 10)
	if !h.cfg.AdminIDs[user] {
		return sendText(b, ctx, msg.Chat.Id, "Only bot admins can use this command ^-^", nil)
	}

	stats, err := h.accounts.Stats(ctx)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
	}
	totalUsers, totalFMUsers := stats.SpotifyUsers, stats.LastFMUsers
	blurCount, counterOn, counterOff := stats.Blur, stats.CounterOn, stats.CounterOff

	result := []string{
		fmt.Sprintf("Spotify Users: %d", totalUsers),
		fmt.Sprintf("LastFm Users: %d", totalFMUsers),
		"",
		fmt.Sprintf("Blur style: %d, Black Style: %d", blurCount, totalUsers-blurCount),
		fmt.Sprintf("On: %d, Off: %d", counterOn, counterOff),
		fmt.Sprintf("Others: %d", totalFMUsers-counterOn-counterOff),
	}

	return sendText(b, ctx, msg.Chat.Id, strings.Join(result, "\n"), nil)
}
