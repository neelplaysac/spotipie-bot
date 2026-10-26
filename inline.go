package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func randID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// answerInlineError replies to an inline query with no results and a guidance button.
func answerInlineError(b *gotgbot.Bot, c *ext.Context, text, startParam string) error {
	ctx := updateContext(c)
	cache := int64(0)
	_, err := b.AnswerInlineQueryWithContext(ctx, c.InlineQuery.Id, []gotgbot.InlineQueryResult{}, &gotgbot.AnswerInlineQueryOpts{
		CacheTime: &cache,
		Button:    &gotgbot.InlineQueryResultsButton{Text: text, StartParameter: startParam},
	})
	return err
}

// parseSpotifyBody unmarshals the currently-playing response; nil on failure.
func parseSpotifyBody(body []byte) *SpotifyPlayback {
	var res SpotifyPlayback
	if err := json.Unmarshal(body, &res); err != nil {
		return nil
	}
	return &res
}

// parseLastFMResponse unmarshals the lastfm recent-tracks response.
func parseLastFMResponse(body []byte) (*lastFMResponse, bool) {
	var res lastFMResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, false
	}
	return &res, true
}

func inlineNowPlaying(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	iq := c.InlineQuery
	if iq == nil {
		return nil
	}

	tgID := strconv.FormatInt(iq.From.Id, 10)

	user, err := db.FetchData(ctx, tgID)
	if err != nil {
		logger.ErrorContext(ctx, "inline: db fetch failed", "err", err, "user_id", iq.From.Id)
		return nil
	}

	if user == nil {
		return answerInlineError(b, c, "You need to register first.", "register")
	}
	if user.Username == "User" {
		return answerInlineError(b, c, "You need to set a username.", "username")
	}
	if user.Token == "00000" {
		return answerInlineError(b, c, "Registration error, please click here to fix.", "token")
	}

	cp, err := spotifyClient.GetCurrentlyPlaying(ctx, user.Token)
	if err != nil {
		logger.ErrorContext(ctx, "inline: spotify request failed", "err", err, "user_id", iq.From.Id)
		return nil
	}

	pfp := fetchProfilePic(b, ctx, iq.From.Id)

	if cp.Status == 204 || len(cp.Body) == 0 {
		return answerInlineError(b, c, "You're not listening to anything.", "notlistening")
	}

	res := parseSpotifyBody(cp.Body)
	if res == nil {
		return answerInlineError(b, c, "You're not listening to anything.", "notlistening")
	}

	switch res.CurrentlyPlayingType {
	case "ad":
		return answerInlineError(b, c, "You are listening to ads.", "ads")
	case "track":
		if res.Item.Name == "" {
			return answerInlineError(b, c, "You're not listening to anything.", "notlistening")
		}

		img, err := drawImage(res, user.Username, pfp, user.Style)
		if err != nil {
			logger.ErrorContext(ctx, "inline: image gen failed", "err", err, "user_id", iq.From.Id)
			return answerInlineError(b, c, "You're not listening to anything.", "notlistening")
		}

		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Play on Spotify", Url: res.Item.ExternalURLs.Spotify}},
		})

		temp, err := b.SendPhotoWithContext(ctx, cfgConfig.TempChannel, gotgbot.InputFileByReader("now.jpg", strings.NewReader(string(img))), nil)
		if err != nil {
			logger.ErrorContext(ctx, "inline: temp channel send failed", "err", err, "user_id", iq.From.Id)
			return answerInlineError(b, c, "You're not listening to anything.", "notlistening")
		}

		fileID := ""
		if len(temp.Photo) > 1 {
			fileID = temp.Photo[1].FileId
		}
		_, _ = b.DeleteMessageWithContext(ctx, cfgConfig.TempChannel, temp.MessageId, nil)

		cache := int64(0)
		_, err = b.AnswerInlineQueryWithContext(ctx, iq.Id, []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultCachedPhoto{
				Id:          randID(),
				PhotoFileId: fileID,
				ReplyMarkup: markup,
			},
		}, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cache})
		if err != nil {
			logger.ErrorContext(ctx, "inline: answer failed", "err", err, "user_id", iq.From.Id)
		}
		return nil
	default:
		return answerInlineError(b, c, "Not sure what you're listening to.", "notsure")
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
