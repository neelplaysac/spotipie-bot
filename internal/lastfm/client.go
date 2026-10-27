package lastfm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/observability"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

const (
	scrobblerURL = "http://ws.audioscrobbler.com/2.0/"
)

type lastFMResponse struct {
	Error        int    `json:"error"`
	Message      string `json:"message"`
	RecentTracks struct {
		Track []service.LastFMTrack `json:"track"`
	} `json:"recenttracks"`
}

type trackInfoResponse struct {
	Track struct {
		UserPlayCount string `json:"userplaycount"`
	} `json:"track"`
}

type Client struct {
	logger *slog.Logger
	apiKey string
	http   *http.Client
}

func New(cfg *config.Config, logger *slog.Logger) *Client {
	return &Client{
		logger: logger,
		apiKey: cfg.LastFMAPIKey,
		http:   &http.Client{Timeout: 30 * 1e9},
	}
}

// GetRecentTracks maps user.getrecenttracks into the service response model.
func (l *Client) GetRecentTracks(ctx context.Context, fmUsername string) (*service.RecentTracks, error) {
	q := url.Values{}
	q.Set("method", "user.getrecenttracks")
	q.Set("limit", "1")
	q.Set("user", fmUsername)
	q.Set("api_key", l.apiKey)
	q.Set("format", "json")

	status, body, err := l.get(ctx, scrobblerURL+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var response lastFMResponse
	valid := json.Unmarshal(body, &response) == nil
	return &service.RecentTracks{Status: status, Valid: valid, Error: response.Error, Tracks: response.RecentTracks.Track}, nil
}

// GetScrobbles returns the user's play count for a track, or "00" on any failure.
func (l *Client) GetScrobbles(ctx context.Context, user, artist, track string) string {
	q := url.Values{}
	q.Set("method", "track.getInfo")
	q.Set("username", user)
	q.Set("api_key", l.apiKey)
	q.Set("artist", artist)
	q.Set("track", track)
	q.Set("format", "json")

	_, body, err := l.get(ctx, scrobblerURL+"?"+q.Encode())
	if err != nil {
		return "00"
	}

	var res trackInfoResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return "00"
	}
	if res.Track.UserPlayCount == "" {
		return "00"
	}
	return res.Track.UserPlayCount
}

func (l *Client) get(ctx context.Context, rawURL string) (int, []byte, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}

	resp, err := l.http.Do(req)
	if err != nil {
		// Do NOT wrap the *url.Error directly: its Error() embeds the full request
		// URL, which carries api_key. Keep only the cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return 0, nil, fmt.Errorf("lastfm request failed: %w", urlErr.Err)
		}
		return 0, nil, fmt.Errorf("lastfm request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		l.logger.Warn("lastfm api non-2xx", "status", resp.StatusCode, "method", "get", observability.Duration(start))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}
