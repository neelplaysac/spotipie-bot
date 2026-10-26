package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/inlinequery"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ---------- shared state (set up in main.go) ----------

var (
	cfgConfig     *Config
	db            *Database
	spotifyClient *SpotifyClient
	lastFMClient  *LastFMClient
	cooldowns     *cooldownManager
	botUsername   string
)

// ---------- message constants (ported verbatim from python) ----------

const START_TEXT = `Hi %s,

Follow these steps to start using the bot -
1. Use /register to connect your spotify account with this bot.
2. Open the provided link, give access to the bot & you will be redirected to bot's telegram link.
3. When you open that link you will be redirected back to telegram, click start.
4. After you see a 'successful' message use /name to set a display name (this will be displayed on the song status).

thats it! you can then share your song status using -
/now or using the inline query @spotipiebot.

You can also use /linkfm to connect your lastfm account.
Then set a display name using /namefm.
and use /last to share song status.

use /style to change background style.
use /help to get the list of commands.
`

const HELP_TEXT = `Heres the list of commands -

/now - share currently playing song on spotify.
/name - change your display name on song status.
/unregister - to unlink your spotify account from the bot.
/register - to connect your spotify account with the bot.
/style - change background style to blur or black.
@spotipiebot - share song using inline query

/last - share lastfm song status.
/linkfm - link LastFm account with the bot.
/namefm - set display name for LastFm status.
/unlinkfm - unlink LastFm account from bot.
/status - recently played song via LastFm.

`

const NOW_REG_MSG = "You need to connect your Spotify account first. Contact me in pm and use /register command."
const NOW_USR_NAME_MSG = "You need to add a username to start using the bot. Contact me in pm and use /name command."
const NOW_TOKEN_ERR_MSG = `Your spotify account is not properly linked with bot :( 
please use /unregister command in pm and /register again.`

const STYLE_REG_MSG = "Use /register to connect your spotify account or /linkfm to connect your LastFm account to get started."

const NAME_PM_MSG = "Contact me in pm to change your username (*this will not change your telegram username)."
const NAME_REG_MSG = "You need to register first. use /register to get started."

const REG_REG_MSG = "Open the link below, to connect your Spotify account."
const REG_PM_MSG = "Contact me in pm to /register or /unregister your account."

const FM_PM_MSG = "Contact me in pm and use /linkfm to link or /unlinkfm to unlink your LastFm account."
const FM_REG_MSG = "Open the link below, to connect your LastFm account."
const FM_NAME_PM_MSG = "Contact me in pm to change your display name."
const FM_NAME_REG_MSG = "You need to link your LastFm account first. use /lastfm in pm to get started."

const LAST_REG_MSG = "You need to connect your LastFm account first. Contact me in pm and use /linkfm command."
const LAST_USR_NAME_MSG = "You need to add a display name to start using the bot. Contact me in pm and use /linkfm command."

// ---------- helpers ----------

func sendText(b *gotgbot.Bot, ctx context.Context, chatID int64, text string, markup *gotgbot.InlineKeyboardMarkup) error {
	opts := &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeMarkdown}
	if markup != nil {
		opts.ReplyMarkup = markup
	}
	_, err := b.SendMessageWithContext(ctx, chatID, text, opts)
	return err
}

func contactInPMButton() *gotgbot.InlineKeyboardMarkup {
	return kb([][]gotgbot.InlineKeyboardButton{
		{{Text: "Contact in pm", Url: fmt.Sprintf(BOT_URL, botUsername)}},
	})
}

func kb(rows [][]gotgbot.InlineKeyboardButton) *gotgbot.InlineKeyboardMarkup {
	return &gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// textNotCommand matches plain text messages (conversation state filters).
func textNotCommand(msg *gotgbot.Message) bool {
	return msg != nil && msg.Text != "" && !strings.HasPrefix(msg.Text, "/")
}

// fetchProfilePic downloads the user's newest telegram profile picture, if any.
func fetchProfilePic(b *gotgbot.Bot, ctx context.Context, userID int64) []byte {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn("pfp fetch panicked", "err", r, "user_id", userID)
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
	resp, err := http.Get("https://api.telegram.org/file/bot" + cfgConfig.APIKey + "/" + file.FilePath)
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

// ---------- /start ----------

func startHandler(b *gotgbot.Bot, c *ext.Context) error {
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
		idHex := text[7:]
		oid, err := bson.ObjectIDFromHex(idHex)
		if err != nil {
			_ = sendText(b, ctx, msg.Chat.Id, "Please use /register command to initiate the login process.", nil)
			return nil
		}

		codeObj, ferr := db.FetchCode(ctx, oid)
		_ = db.DeleteCode(ctx, oid)
		if ferr != nil {
			_ = sendText(b, ctx, msg.Chat.Id, "Please use /register command to initiate the login process.", nil)
			return nil
		}

		if codeObj == nil {
			_ = sendText(b, ctx, msg.Chat.Id, "Please use /register command to initiate the login process.", nil)
		} else {
			tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

			existing, eerr := db.FetchData(ctx, tgID)
			if eerr != nil {
				_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
				return nil
			}
			if existing != nil {
				_ = sendText(b, ctx, msg.Chat.Id, "You are already registered. If the bot is not working /unregister and /register again.", nil)
				return nil
			}

			refreshToken, terr := spotifyClient.GetAccessToken(ctx, codeObj.AuthCode)
			if terr != nil {
				_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
				return nil
			}
			if refreshToken == "error" {
				_ = sendText(b, ctx, msg.Chat.Id, "Unable to authenticate. Please try again using /register. If you are having issues using the bot contact in support chat (check bot info)", nil)
				return nil
			}

			if _, aerr := db.AddUser(ctx, tgID, refreshToken); aerr != nil {
				_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
				return nil
			}

			_ = sendText(b, ctx, msg.Chat.Id, "Account successfully linked. Now use /name to set a display name then use /now to use the bot.", nil)
		}
	}

	// delete the /start message (python: update.effective_message.delete())
	_, _ = b.DeleteMessageWithContext(ctx, msg.Chat.Id, msg.MessageId, nil)
	return nil
}

// ---------- /help ----------

func helpHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Help", Url: fmt.Sprintf(BOT_URL, botUsername) + "?start=help"}},
		})
		return sendText(b, ctx, msg.Chat.Id, "Contact me in PM to get the list of possible commands.", markup)
	}

	return sendText(b, ctx, msg.Chat.Id, HELP_TEXT, nil)
}

// ---------- /register /unregister ----------

func registerHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := cooldowns.Check(c.EffectiveUser.Id, "register", 3); rem > 0 {
		logger.Debug("command throttled", "handler", "register", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	if msg.Chat.Type == "private" {
		authURL := spotifyClient.GetAuthUrl()
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Open this link to register", Url: authURL}},
		})
		return sendText(b, ctx, msg.Chat.Id, REG_REG_MSG, markup)
	}

	markup := kb([][]gotgbot.InlineKeyboardButton{
		{{Text: "Register here", Url: fmt.Sprintf(BOT_URL, botUsername)}},
	})
	return sendText(b, ctx, msg.Chat.Id, REG_PM_MSG, markup)
}

func unregisterHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := cooldowns.Check(c.EffectiveUser.Id, "unregister", 3); rem > 0 {
		logger.Debug("command throttled", "handler", "unregister", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	if msg.Chat.Type == "private" {
		user, err := db.FetchData(ctx, tgID)
		if err != nil {
			return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
		}
		if user == nil {
			return sendText(b, ctx, msg.Chat.Id, "You haven't registered your account yet.", nil)
		}
		if err := db.DeleteData(ctx, tgID); err != nil {
			return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
		}
		return sendText(b, ctx, msg.Chat.Id, "Account successfully removed.", nil)
	}

	return sendText(b, ctx, msg.Chat.Id, REG_PM_MSG, contactInPMButton())
}

// ---------- conversation: /name ----------

const nameState = "NAME"

func getName(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Change username", Url: fmt.Sprintf(BOT_URL, botUsername)}},
		})
		return sendText(b, ctx, msg.Chat.Id, NAME_PM_MSG, markup)
	}

	_ = sendText(b, ctx, msg.Chat.Id, "Send me a username (max 15 characters)", nil)
	return handlers.NextConversationState(nameState)
}

func setName(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	text := strings.TrimSpace(msg.Text)
	if text == "" || len([]rune(text)) > 15 || strings.HasPrefix(text, "/") {
		_ = sendText(b, ctx, msg.Chat.Id, "Invalid username. Must be 1-15 characters and not start with /. Try again using /name", nil)
		return handlers.EndConversation()
	}

	user, err := db.FetchData(ctx, tgID)
	if err != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
		return handlers.EndConversation()
	}
	if user == nil {
		_ = sendText(b, ctx, msg.Chat.Id, NAME_REG_MSG, nil)
		return handlers.EndConversation()
	}

	if err := db.UpdateData(ctx, tgID, text); err != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
		return handlers.EndConversation()
	}

	_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Username updated to %s", text), nil)
	return handlers.EndConversation()
}

// ---------- /now ----------

func nowHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	update := c.Update
	msg := update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := cooldowns.Check(c.EffectiveUser.Id, "now", 5); rem > 0 {
		logger.Debug("command throttled", "handler", "now", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	_, _ = b.SendChatActionWithContext(ctx, msg.Chat.Id, gotgbot.ChatActionTyping, nil)

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
	user, err := db.FetchData(ctx, tgID)
	if err != nil {
		logger.ErrorContext(ctx, "now: db fetch failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
		return nil
	}
	if user == nil {
		return sendText(b, ctx, msg.Chat.Id, NOW_REG_MSG, contactInPMButton())
	}
	if user.Username == "User" {
		return sendText(b, ctx, msg.Chat.Id, NOW_USR_NAME_MSG, contactInPMButton())
	}
	if user.Token == "00000" {
		return sendText(b, ctx, msg.Chat.Id, NOW_TOKEN_ERR_MSG, contactInPMButton())
	}

	cp, err := spotifyClient.GetCurrentlyPlaying(ctx, user.Token)
	if err != nil {
		logger.ErrorContext(ctx, "now: spotify request failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
		return nil
	}

	pfp := fetchProfilePic(b, ctx, c.EffectiveUser.Id)

	if cp.Status == 204 || len(cp.Body) == 0 {
		return sendText(b, ctx, msg.Chat.Id, "You are not listening to anything.", nil)
	}

	res := parseSpotifyBody(cp.Body)
	if res == nil {
		return sendText(b, ctx, msg.Chat.Id, "You are not listening to anything.", nil)
	}

	switch res.CurrentlyPlayingType {
	case "ad":
		return sendText(b, ctx, msg.Chat.Id, "You're listening to ads.", nil)
	case "track":
		if res.Item.Name == "" {
			return sendText(b, ctx, msg.Chat.Id, "You are not listening to anything.", nil)
		}
		img, err := drawImage(res, user.Username, pfp, user.Style)
		if err != nil {
			logger.ErrorContext(ctx, "now: image gen failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
			return sendText(b, ctx, msg.Chat.Id, "You are not listening to anything.", nil)
		}

		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Play on Spotify", Url: res.Item.ExternalURLs.Spotify}},
		})
		_, err = b.SendPhotoWithContext(ctx, msg.Chat.Id, gotgbot.InputFileByReader("now.jpg", strings.NewReader(string(img))), &gotgbot.SendPhotoOpts{ReplyMarkup: markup})
		if err != nil {
			logger.ErrorContext(ctx, "now: photo send failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
		}
		return nil
	default:
		return sendText(b, ctx, msg.Chat.Id, "Not sure what you're listening to.", nil)
	}
}

// ---------- /last ----------

func lastHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	update := c.Update
	msg := update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := cooldowns.Check(c.EffectiveUser.Id, "last", 5); rem > 0 {
		logger.Debug("command throttled", "handler", "last", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	_, _ = b.SendChatActionWithContext(ctx, msg.Chat.Id, gotgbot.ChatActionTyping, nil)

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)
	fmUser, err := db.GetLastFMUser(ctx, tgID)
	if err != nil {
		logger.ErrorContext(ctx, "last: db fetch failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
		return nil
	}
	if fmUser == nil {
		if msg.Chat.Type == "private" {
			return sendText(b, ctx, msg.Chat.Id, "Use /linkfm command to link your lastfm account first.", nil)
		}
		return sendText(b, ctx, msg.Chat.Id, LAST_REG_MSG, contactInPMButton())
	}
	if fmUser.Name == "User" {
		if msg.Chat.Type == "private" {
			return sendText(b, ctx, msg.Chat.Id, "Use /namefm command to set a display name.", nil)
		}
		return sendText(b, ctx, msg.Chat.Id, LAST_USR_NAME_MSG, contactInPMButton())
	}

	status, body, err := lastFMClient.GetRecentTracks(ctx, fmUser.FMUsername)
	if err != nil {
		logger.ErrorContext(ctx, "last: lastfm request failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
		return nil
	}

	pfp := fetchProfilePic(b, ctx, c.EffectiveUser.Id)

	lfResp, ok := parseLastFMResponse(body)
	// python: r.json() raise -> "Unable to get data from LastFm." ; 'error' in json -> invalid username
	if !ok {
		return sendText(b, ctx, msg.Chat.Id, "Unable to get data from LastFm.", nil)
	}
	if lfResp.Error != 0 {
		return sendText(b, ctx, msg.Chat.Id, "LastFM username is invalid. Try to relink lastfm with the bot.", nil)
	}
	if status < 200 || status > 299 {
		return sendText(b, ctx, msg.Chat.Id, "Unable to get data from LastFm.", nil)
	}
	if len(lfResp.RecentTracks.Track) == 0 {
		return sendText(b, ctx, msg.Chat.Id, "Unable to get data from LastFm.", nil)
	}

	track := lfResp.RecentTracks.Track[0]
	username := fmUser.Name

	scrobbles := "off"
	if fmUser.Counter != "off" {
		scrobbles = lastFMClient.GetScrobbles(ctx, fmUser.FMUsername, track.Artist.Text, track.Name)
	}

	img, err := blurrImage(&track, username, pfp, scrobbles)
	if err != nil {
		logger.ErrorContext(ctx, "last: image gen failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
		return sendText(b, ctx, msg.Chat.Id, "Unable to get data from LastFm.", nil)
	}

	markup := kb([][]gotgbot.InlineKeyboardButton{
		{{Text: "LastFm Song Link", Url: track.URL}},
	})
	_, err = b.SendPhotoWithContext(ctx, msg.Chat.Id, gotgbot.InputFileByReader("last.jpg", strings.NewReader(string(img))), &gotgbot.SendPhotoOpts{ReplyMarkup: markup})
	if err != nil {
		logger.ErrorContext(ctx, "last: photo send failed", "err", err, "user_id", tgID, "chat_id", msg.Chat.Id)
	}
	return nil
}

// ---------- /style + settings callback ----------

func styleHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := cooldowns.Check(c.EffectiveUser.Id, "style", 3); rem > 0 {
		logger.Debug("command throttled", "handler", "style", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	if msg.Chat.Type != "private" {
		return sendText(b, ctx, msg.Chat.Id, "Contact me in pm and use /style command to change background style.", contactInPMButton())
	}

	user, err := db.FetchData(ctx, tgID)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
	}
	fmUser, err := db.GetLastFMUser(ctx, tgID)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
	}

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
		if fmUser.Counter == "" {
			_ = db.ToggleCounter(ctx, tgID, "on")
			fmUser.Counter = "on"
		}
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

func styleButton(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	cq := c.CallbackQuery
	if cq == nil {
		return nil
	}

	if rem := cooldowns.Check(cq.From.Id, "style_button", 3); rem > 0 {
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
		_ = db.UpdateStyle(ctx, tgID, v)
		updatedField = "Style updated to"
		updatedValue = v
	}
	if v, ok := counterMap[cq.Data]; ok {
		_ = db.ToggleCounter(ctx, tgID, v)
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
		logger.ErrorContext(ctx, "style: edit failed", "err", err, "user_id", cq.From.Id, "chat_id", chatID)
	}
	return nil
}

// ---------- /linkfm /unlinkfm ----------

const linkFMState = "LINKFM"

func getLastFMUserName(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Link LastFm account.", Url: fmt.Sprintf(BOT_URL, botUsername)}},
		})
		return sendText(b, ctx, msg.Chat.Id, FM_PM_MSG, markup)
	}

	_ = sendText(b, ctx, msg.Chat.Id, "Send me your LastFm username. use /cancel to cancel.", nil)
	return handlers.NextConversationState(linkFMState)
}

func linkLastFMUser(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	text := strings.TrimSpace(msg.Text)
	if text == "" || len([]rune(text)) > 15 || strings.HasPrefix(text, "/") || !validFmUsername(text) {
		_ = sendText(b, ctx, msg.Chat.Id, "Invalid LastFm username. Must be 1-15 alphanumeric characters (hyphens and underscores allowed). Try again using /linkfm", nil)
		return handlers.EndConversation()
	}

	fmUser, err := db.GetLastFMUser(ctx, tgID)
	if err != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
		return handlers.EndConversation()
	}
	if fmUser != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Your LastFm account is already linked. Use /unlinkfm to unlink.", nil)
		return handlers.EndConversation()
	}

	if _, err := db.AddLastFMUser(ctx, tgID, text); err != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
		return handlers.EndConversation()
	}

	_ = sendText(b, ctx, msg.Chat.Id, "LastFM account linked. Now set a display name using /namefm command.", nil)
	return handlers.EndConversation()
}

func validFmUsername(text string) bool {
	for _, r := range text {
		if r != '-' && r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func unlinkfmHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	if rem := cooldowns.Check(c.EffectiveUser.Id, "unlinkfm", 3); rem > 0 {
		logger.Debug("command throttled", "handler", "unlinkfm", "user_id", c.EffectiveUser.Id, "retry_in_s", rem)
		//_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Please wait %ds before using this again.", rem), nil)
		return nil
	}

	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	if msg.Chat.Type == "private" {
		fmUser, err := db.GetLastFMUser(ctx, tgID)
		if err != nil {
			return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
		}
		if fmUser == nil {
			return sendText(b, ctx, msg.Chat.Id, "You haven't registered your account yet.", nil)
		}
		if err := db.RemoveLastFMUser(ctx, tgID); err != nil {
			return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
		}
		return sendText(b, ctx, msg.Chat.Id, "Account successfully removed.", nil)
	}

	return sendText(b, ctx, msg.Chat.Id, FM_PM_MSG, contactInPMButton())
}

// ---------- conversation: /namefm ----------

const nameFMState = "NAMEFM"

func getLastFMUserData(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil {
		return nil
	}

	if msg.Chat.Type != "private" {
		markup := kb([][]gotgbot.InlineKeyboardButton{
			{{Text: "Change display name", Url: fmt.Sprintf(BOT_URL, botUsername)}},
		})
		return sendText(b, ctx, msg.Chat.Id, FM_NAME_PM_MSG, markup)
	}

	_ = sendText(b, ctx, msg.Chat.Id, "Send me a username (max 15 characters)", nil)
	return handlers.NextConversationState(nameFMState)
}

func setLastFMUserData(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	tgID := strconv.FormatInt(c.EffectiveUser.Id, 10)

	text := strings.TrimSpace(msg.Text)
	if text == "" || len([]rune(text)) > 15 || strings.HasPrefix(text, "/") {
		_ = sendText(b, ctx, msg.Chat.Id, "Invalid username. Must be 1-15 characters and not start with /. Try again using /namefm", nil)
		return handlers.EndConversation()
	}

	fmUser, err := db.GetLastFMUser(ctx, tgID)
	if err != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
		return handlers.EndConversation()
	}
	if fmUser == nil {
		_ = sendText(b, ctx, msg.Chat.Id, FM_NAME_REG_MSG, nil)
		return handlers.EndConversation()
	}

	if err := db.UpdateLastFMData(ctx, tgID, text); err != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Database Error", nil)
		return handlers.EndConversation()
	}

	_ = sendText(b, ctx, msg.Chat.Id, fmt.Sprintf("Username updated to %s. Use /last to share lastfm song status.", text), nil)
	return handlers.EndConversation()
}

// ---------- /statss /details (admin only) ----------

func statssHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	user := strconv.FormatInt(c.EffectiveUser.Id, 10)
	if !cfgConfig.AdminIDs[user] {
		return sendText(b, ctx, msg.Chat.Id, "Only bot admins can use this command ^-^", nil)
	}

	total, err := db.CountAll(ctx)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
	}
	return sendText(b, ctx, msg.Chat.Id, strconv.FormatInt(total, 10), nil)
}

func detailsHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	msg := c.Update.Message
	if msg == nil || c.EffectiveUser == nil {
		return nil
	}

	user := strconv.FormatInt(c.EffectiveUser.Id, 10)
	if !cfgConfig.AdminIDs[user] {
		return sendText(b, ctx, msg.Chat.Id, "Only bot admins can use this command ^-^", nil)
	}

	totalUsers, err := db.CountAll(ctx)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
	}
	totalFMUsers, err := db.CountAllLastFM(ctx)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
	}

	usersStats, err := db.AggregateUsers(ctx)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
	}
	fmStats, err := db.AggregateLastFMUsers(ctx)
	if err != nil {
		return sendText(b, ctx, msg.Chat.Id, "Database Error.", nil)
	}

	blurCount := int64(0)
	counterOn := int64(0)
	counterOff := int64(0)

	for _, s := range usersStats {
		if s.ID == "blur" {
			blurCount += s.Count
		}
	}
	for _, s := range fmStats {
		if s.ID == "off" {
			counterOff += s.Count
		}
		if s.ID == "on" {
			counterOn += s.Count
		}
	}

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

// ---------- admin only? no: /cancel for conversations ----------

func cancelHandler(b *gotgbot.Bot, c *ext.Context) error {
	ctx := updateContext(c)
	if msg := c.Update.Message; msg != nil {
		_ = sendText(b, ctx, msg.Chat.Id, "Canceled.", nil)
	}
	return handlers.EndConversation()
}

// ---------- handlers registration ----------

func registerHandlers(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("start", startHandler))
	d.AddHandler(handlers.NewCommand("help", helpHandler))
	d.AddHandler(handlers.NewCommand("register", registerHandler))
	d.AddHandler(handlers.NewCommand("unregister", unregisterHandler))
	d.AddHandler(handlers.NewCommand("now", nowHandler))
	d.AddHandler(handlers.NewCommand("last", lastHandler))
	d.AddHandler(handlers.NewCommand("style", styleHandler))
	d.AddHandler(handlers.NewCommand("unlinkfm", unlinkfmHandler))
	d.AddHandler(handlers.NewCommand("statss", statssHandler))
	d.AddHandler(handlers.NewCommand("details", detailsHandler))
	d.AddHandler(handlers.NewCallback(callbackquery.All, styleButton))
	d.AddHandler(handlers.NewInlineQuery(inlinequery.All, inlineNowPlaying))

	d.AddHandler(nameConversation())
	d.AddHandler(linkfmConversation())
	d.AddHandler(namefmConversation())
}

func nameConversation() handlers.Conversation {
	return handlers.NewConversation(
		[]ext.Handler{handlers.NewCommand("name", getName)},
		map[string][]ext.Handler{
			nameState: {handlers.NewMessage(textNotCommand, setName)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", cancelHandler)},
			AllowReEntry: true,
			StateStorage: newExpiringStorage(time.Minute),
		},
	)
}

func linkfmConversation() handlers.Conversation {
	return handlers.NewConversation(
		[]ext.Handler{handlers.NewCommand("linkfm", getLastFMUserName)},
		map[string][]ext.Handler{
			linkFMState: {handlers.NewMessage(textNotCommand, linkLastFMUser)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", cancelHandler)},
			AllowReEntry: true,
			StateStorage: newExpiringStorage(time.Minute),
		},
	)
}

func namefmConversation() handlers.Conversation {
	return handlers.NewConversation(
		[]ext.Handler{handlers.NewCommand("namefm", getLastFMUserData)},
		map[string][]ext.Handler{
			nameFMState: {handlers.NewMessage(textNotCommand, setLastFMUserData)},
		},
		&handlers.ConversationOpts{
			Exits:        []ext.Handler{handlers.NewCommand("cancel", cancelHandler)},
			AllowReEntry: true,
			StateStorage: newExpiringStorage(time.Minute),
		},
	)
}
