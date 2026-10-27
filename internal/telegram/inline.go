package telegram

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/neelplaysac/spotipie-go/internal/service"
)

func randID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *Handler) answerInlineError(b *gotgbot.Bot, c *ext.Context, text, startParam string) error {
	ctx := updateContext(c)
	cache := int64(0)
	_, err := b.AnswerInlineQueryWithContext(ctx, c.InlineQuery.Id, []gotgbot.InlineQueryResult{}, &gotgbot.AnswerInlineQueryOpts{
		CacheTime: &cache,
		Button:    &gotgbot.InlineQueryResultsButton{Text: text, StartParameter: startParam},
	})
	return err
}

func (h *Handler) inlineNowPlaying(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	iq := c.InlineQuery
	if iq == nil {
		return nil
	}

	tgID := strconv.FormatInt(iq.From.Id, 10)

	result, err := h.playback.Spotify(ctx, tgID, func() []byte { return h.fetchProfilePic(b, ctx, iq.From.Id) })
	if err != nil {
		failure := err.(*service.PlaybackError)
		messages := map[string]string{"db": "inline: db fetch failed", "spotify": "inline: spotify request failed", "render": "inline: image gen failed"}
		h.logger.ErrorContext(ctx, messages[failure.Stage], "err", failure.Err, "user_id", iq.From.Id)
		if failure.Stage == "render" {
			return h.answerInlineError(b, c, "You're not listening to anything.", "notlistening")
		}
		return nil
	}
	switch result.State {
	case service.PlaybackNotRegistered:
		return h.answerInlineError(b, c, "You need to register first.", "register")
	case service.PlaybackNameRequired:
		return h.answerInlineError(b, c, "You need to set a username.", "username")
	case service.PlaybackInvalidToken:
		return h.answerInlineError(b, c, "Registration error, please click here to fix.", "token")
	case service.PlaybackInactive:
		return h.answerInlineError(b, c, "You're not listening to anything.", "notlistening")
	case service.PlaybackAd:
		return h.answerInlineError(b, c, "You are listening to ads.", "ads")
	case service.PlaybackTrack:
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Play on Spotify", Url: result.URL}},
		})

		temp, err := b.SendPhotoWithContext(ctx, h.cfg.TempChannel, gotgbot.InputFileByReader("now.jpg", strings.NewReader(string(result.Image))), nil)
		if err != nil {
			h.logger.ErrorContext(ctx, "inline: temp channel send failed", "err", err, "user_id", iq.From.Id)
			return h.answerInlineError(b, c, "You're not listening to anything.", "notlistening")
		}

		fileID := ""
		if len(temp.Photo) > 1 {
			fileID = temp.Photo[1].FileId
		}
		_, _ = b.DeleteMessageWithContext(ctx, h.cfg.TempChannel, temp.MessageId, nil)

		cache := int64(0)
		_, err = b.AnswerInlineQueryWithContext(ctx, iq.Id, []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultCachedPhoto{
				Id:          randID(),
				PhotoFileId: fileID,
				ReplyMarkup: markup,
			},
		}, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cache})
		if err != nil {
			h.logger.ErrorContext(ctx, "inline: answer failed", "err", err, "user_id", iq.From.Id)
		}
		return nil
	default:
		return h.answerInlineError(b, c, "Not sure what you're listening to.", "notsure")
	}
}

// stringReader lets us feed []byte as an io.Reader for InputFileByReader.
func stringReader(b []byte) *byteSliceReader {
	return &byteSliceReader{data: b}
}

type byteSliceReader struct {
	data []byte
	pos  int
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, stopReading
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

var stopReading = sentinelErr("eof")

type sentinelErr string

func (s sentinelErr) Error() string { return string(s) }
