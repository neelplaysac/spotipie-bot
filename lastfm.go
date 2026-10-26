package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	scrobblerURL = "http://ws.audioscrobbler.com/2.0/"
)

// LastFMTrack is one entry of user.getrecenttracks.
type LastFMTrack struct {
	Artist struct {
		Text string `json:"#text"`
	} `json:"artist"`
	Album struct {
		Text string `json:"#text"`
	} `json:"album"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Image []struct {
		Text string `json:"#text"`
	} `json:"image"`
	Attr map[string]string `json:"@attr"`
}

type lastFMResponse struct {
	Error        int    `json:"error"`
	Message      string `json:"message"`
	RecentTracks struct {
		Track []LastFMTrack `json:"track"`
	} `json:"recenttracks"`
}

type trackInfoResponse struct {
	Track struct {
		UserPlayCount string `json:"userplaycount"`
	} `json:"track"`
}

type LastFMClient struct {
	apiKey string
	http   *http.Client
}

func newLastFMClient(cfg *Config) *LastFMClient {
	return &LastFMClient{
		apiKey: cfg.LastFMAPIKey,
		http:   &http.Client{Timeout: 30 * 1e9},
	}
}

// GetRecentTracks returns the status code and body of user.getrecenttracks.
func (l *LastFMClient) GetRecentTracks(ctx context.Context, fmUsername string) (int, []byte, error) {
	q := url.Values{}
	q.Set("method", "user.getrecenttracks")
	q.Set("limit", "1")
	q.Set("user", fmUsername)
	q.Set("api_key", l.apiKey)
	q.Set("format", "json")

	return l.get(ctx, scrobblerURL+"?"+q.Encode())
}

// GetScrobbles returns the user's play count for a track, or "00" on any failure.
func (l *LastFMClient) GetScrobbles(ctx context.Context, user, artist, track string) string {
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

func (l *LastFMClient) get(ctx context.Context, rawURL string) (int, []byte, error) {
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
		logger.Warn("lastfm api non-2xx", "status", resp.StatusCode, "method", "get", logDuration(start))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}
