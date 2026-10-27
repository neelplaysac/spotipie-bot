package telegram

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func sendText(b *gotgbot.Bot, ctx context.Context, chatID int64, text string, markup *gotgbot.InlineKeyboardMarkup) error {
	opts := &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeMarkdown}
	if markup != nil {
		opts.ReplyMarkup = markup
	}
	_, err := b.SendMessageWithContext(ctx, chatID, text, opts)
	return err
}

func (h *Handler) contactInPMButton() *gotgbot.InlineKeyboardMarkup {
	return kb([][]gotgbot.InlineKeyboardButton{
		{{Text: "Contact in pm", Url: fmt.Sprintf(BOT_URL, h.username)}},
	})
}

func kb(rows [][]gotgbot.InlineKeyboardButton) *gotgbot.InlineKeyboardMarkup {
	return &gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func textNotCommand(msg *gotgbot.Message) bool {
	return msg != nil && msg.Text != "" && !strings.HasPrefix(msg.Text, "/")
}

func (h *Handler) fetchProfilePic(b *gotgbot.Bot, ctx context.Context, userID int64) []byte {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Warn("pfp fetch panicked", "err", r, "user_id", userID)
		}
	}()

	photos, err := b.GetUserProfilePhotosWithContext(ctx, userID, &gotgbot.GetUserProfilePhotosOpts{Limit: 1})
	if err != nil || len(photos.Photos) == 0 || len(photos.Photos[0]) == 0 {
		return nil
	}

	file, err := b.GetFileWithContext(ctx, photos.Photos[0][0].FileId, nil)
	if err != nil || file.FilePath == "" {
		return nil
	}

	// NOTE: this raw http.Get builds a token-bearing URL by hand. gotgbot's own
	// transport sanitises *url.Error URLs (sanitizeError in request.go), but this
	// call bypasses that, so the returned error must never be logged or wrapped
	// into a returned error.
	resp, err := http.Get("https://api.telegram.org/file/bot" + h.cfg.APIKey + "/" + file.FilePath)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	buf := make([]byte, 0, 64*1024)
	tmp := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf
}
