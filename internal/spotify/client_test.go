package spotify

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/neelplaysac/spotipie-go/internal/config"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAuthURLPreservesRedirectAndState(t *testing.T) {
	redirect := "https://example.invalid/callback?existing=a&other=b"
	client := New(&config.Config{SpotifyClientID: "client", RedirectURI: redirect}, slog.New(slog.DiscardHandler))
	u, err := url.Parse(client.AuthURL("unguessable-state"))
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	if query.Get("redirect_uri") != redirect || query.Get("state") != "unguessable-state" || query.Get("scope") != "user-read-currently-playing" || query.Get("response_type") != "code" {
		t.Fatalf("incorrect authorization URL: %s", u)
	}
}

func TestPlaybackResponseBoundary(t *testing.T) {
	for _, test := range []struct {
		name         string
		status       int
		body         string
		playbackType string
		valid        bool
	}{
		{"empty", 204, "", "", false},
		{"malformed", 200, "invalid json", "", false},
		{"advertisement", 200, `{"currently_playing_type":"ad"}`, "ad", true},
		{"track", 200, `{"currently_playing_type":"track","item":{"name":"Song"}}`, "track", true},
		{"API error", 403, `{"error":{"status":403}}`, "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := New(&config.Config{SpotifyClientID: "client-id", SpotifyClientSecret: "client-secret", RedirectURI: "https://example.invalid/callback"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
			client.http.Transport = fixtureTransport(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() == spotifyTokenURL {
					body, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatal(err)
					}
					values, err := url.ParseQuery(string(body))
					if err != nil || values.Get("client_id") != "client-id" || values.Get("client_secret") != "client-secret" || values.Get("refresh_token") != "refresh-token" {
						t.Fatal("token refresh parameters changed")
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"access-token"}`))}, nil
				}
				if req.URL.String() != spotifyPlayerURL || req.Header.Get("Authorization") != "Bearer access-token" {
					t.Fatal("playback request changed")
				}
				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
			})
			result, err := client.GetCurrentlyPlaying(context.Background(), "refresh-token")
			if err != nil || result.Status != test.status || (result.Playback != nil) != test.valid {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if result.Playback != nil && result.Playback.CurrentlyPlayingType != test.playbackType {
				t.Fatal("playback response mapping changed")
			}
		})
	}
}
