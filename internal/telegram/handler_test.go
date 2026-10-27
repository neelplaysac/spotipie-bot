package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

type apiCall struct {
	method string
	params map[string]any
}

type telegramClientStub struct {
	*gotgbot.BaseBotClient
	calls []apiCall
}

func (a *telegramClientStub) RequestWithContext(_ context.Context, _, method string, params map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
	a.calls = append(a.calls, apiCall{method: method, params: params})
	if method == "deleteMessage" || method == "sendChatAction" || method == "answerInlineQuery" {
		return json.RawMessage(`true`), nil
	}
	return json.RawMessage(`{"message_id":1,"chat":{"id":42,"type":"private"}}`), nil
}

type accountStoreStub struct {
	service.AccountRepository
	service.AuthorizationRepository
	user       *service.User
	name       string
	fmName     string
	createdFor string
	createErr  error
}

func (r *accountStoreStub) CreateAuthorization(_ context.Context, _, owner string, _ time.Time) error {
	r.createdFor = owner
	return r.createErr
}

type spotifyAuthorizationStub struct{ service.SpotifyAuthorization }

func (spotifyAuthorizationStub) AuthURL(state string) string {
	return "https://accounts.spotify.com/authorize?state=" + state
}

func (r *accountStoreStub) ConsumeAuthorization(context.Context, string, string, time.Time) (*service.Code, error) {
	return nil, nil
}
func (r *accountStoreStub) FetchData(context.Context, string) (*service.User, error) {
	return r.user, nil
}
func (r *accountStoreStub) UpdateData(_ context.Context, _, value string) error {
	r.name = value
	return nil
}
func (r *accountStoreStub) GetLastFMUser(context.Context, string) (*service.LastFMUser, error) {
	return nil, nil
}
func (r *accountStoreStub) AddLastFMUser(_ context.Context, _, value string) error {
	r.fmName = value
	return nil
}

func newTestHandler(repo *accountStoreStub) (*Handler, *gotgbot.Bot, *telegramClientStub) {
	api := &telegramClientStub{BaseBotClient: &gotgbot.BaseBotClient{}}
	bot := &gotgbot.Bot{User: gotgbot.User{Username: "testbot"}, BotClient: api}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(&config.Config{}, service.NewAccounts(repo, spotifyAuthorizationStub{}, service.NewAuthorization(repo)), service.NewPlayback(repo, nil, nil, nil), logger, "testbot")
	return handler, bot, api
}

func TestRegisterCreatesUserBoundLinkAndHandlesStorageFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		repo := &accountStoreStub{}
		if fail {
			repo.createErr = errors.New("database unavailable")
		}
		handler, bot, api := newTestHandler(repo)
		if err := handler.Dispatcher().ProcessUpdate(bot, messageUpdate("/register"), nil); err != nil {
			t.Fatal(err)
		}
		if repo.createdFor != "42" || len(api.calls) != 1 {
			t.Fatalf("owner=%s calls=%v", repo.createdFor, api.calls)
		}
		if fail {
			if api.calls[0].params["text"] != "Unable to start registration. Please try /register again shortly." {
				t.Fatal("missing registration failure response")
			}
		} else {
			raw, err := json.Marshal(api.calls[0].params)
			if err != nil || !strings.Contains(string(raw), "https://accounts.spotify.com/authorize?state=") {
				t.Fatal("registration reply did not contain the state-bearing Spotify link")
			}
		}
	}
}

func messageUpdate(text string) *gotgbot.Update {
	return &gotgbot.Update{Message: &gotgbot.Message{MessageId: 1, Chat: gotgbot.Chat{Id: 42, Type: "private"}, From: &gotgbot.User{Id: 42, FirstName: "Name"}, Text: text}}
}

func TestStartAndAccountPrompts(t *testing.T) {
	for _, test := range []struct {
		command string
		want    string
		deleted bool
	}{
		{"/start register", "To register your account use /register command.", true},
		{"/start username", "To change your username use /name command.", true},
		{"/start invalid", "Please use /register command to initiate the login process.", false},
		{"/start 0123456789abcdef01234567", "Please use /register command to initiate the login process.", true},
		{"/name", "Send me a username (max 15 characters)", false},
		{"/linkfm", "Send me your LastFm username. use /cancel to cancel.", false},
		{"/namefm", "Send me a username (max 15 characters)", false},
	} {
		t.Run(test.command, func(t *testing.T) {
			handler, bot, api := newTestHandler(&accountStoreStub{})
			if err := handler.Dispatcher().ProcessUpdate(bot, messageUpdate(test.command), nil); err != nil {
				t.Fatal(err)
			}
			if len(api.calls) == 0 || api.calls[0].method != "sendMessage" || api.calls[0].params["text"] != test.want {
				t.Fatalf("unexpected Telegram calls: %+v", api.calls)
			}
			deleted := len(api.calls) > 1 && api.calls[1].method == "deleteMessage"
			if deleted != test.deleted {
				t.Fatal("/start deletion behavior changed")
			}
		})
	}
}

func TestConversationUpdatesReachAccountService(t *testing.T) {
	for _, test := range []struct{ command, value, want string }{
		{"/name", " New Name ", "Username updated to New Name"},
		{"/linkfm", " fm-user ", "LastFM account linked. Now set a display name using /namefm command."},
	} {
		t.Run(test.command, func(t *testing.T) {
			repo := &accountStoreStub{user: &service.User{Username: "Old Name"}}
			handler, bot, api := newTestHandler(repo)
			dispatcher := handler.Dispatcher()
			for _, text := range []string{test.command, test.value} {
				if err := dispatcher.ProcessUpdate(bot, messageUpdate(text), nil); err != nil {
					t.Fatal(err)
				}
			}
			if len(api.calls) != 2 || api.calls[1].params["text"] != test.want {
				t.Fatalf("unexpected replies: %+v", api.calls)
			}
			if test.command == "/name" && repo.name != "New Name" {
				t.Fatal("display name did not reach storage")
			}
			if test.command == "/linkfm" && repo.fmName != "fm-user" {
				t.Fatal("Last.fm username did not reach storage")
			}
		})
	}
}

func TestPlaybackRegistrationResponses(t *testing.T) {
	for _, test := range []struct{ command, want string }{
		{"/now", NOW_REG_MSG},
		{"/last", "Use /linkfm command to link your lastfm account first."},
	} {
		handler, bot, api := newTestHandler(&accountStoreStub{})
		if err := handler.Dispatcher().ProcessUpdate(bot, messageUpdate(test.command), nil); err != nil {
			t.Fatal(err)
		}
		if len(api.calls) != 2 || api.calls[0].method != "sendChatAction" || api.calls[1].params["text"] != test.want {
			t.Fatalf("unexpected playback replies: %+v", api.calls)
		}
	}
}
