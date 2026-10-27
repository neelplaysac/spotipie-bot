package lastfm

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/neelplaysac/spotipie-go/internal/config"
)

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRecentTracksResponseBoundary(t *testing.T) {
	for _, test := range []struct {
		body      string
		valid     bool
		errorCode int
		tracks    int
	}{
		{"invalid json", false, 0, 0},
		{`{"error":6,"message":"User not found"}`, true, 6, 0},
		{`{"recenttracks":{"track":[]}}`, true, 0, 0},
		{`{"recenttracks":{"track":[{"name":"Song","artist":{"#text":"Artist"},"album":{"#text":"Album"}}]}}`, true, 0, 1},
	} {
		client := New(&config.Config{LastFMAPIKey: "api-key"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
		client.http.Transport = fixtureTransport(func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			if query.Get("user") != "fm-user" || query.Get("api_key") != "api-key" || query.Get("method") != "user.getrecenttracks" || query.Get("limit") != "1" {
				t.Fatal("Last.fm query parameters changed")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})
		result, err := client.GetRecentTracks(context.Background(), "fm-user")
		if err != nil || result.Valid != test.valid || result.Error != test.errorCode || len(result.Tracks) != test.tracks || result.Status != 200 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if test.tracks > 0 && (result.Tracks[0].Artist.Text != "Artist" || result.Tracks[0].Album.Text != "Album") {
			t.Fatal("Last.fm metadata mapping changed")
		}
	}
}
