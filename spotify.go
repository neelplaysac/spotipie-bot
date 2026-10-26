package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	spotifyAuthorizeURL = "https://accounts.spotify.com/authorize"
	spotifyTokenURL     = "https://accounts.spotify.com/api/token"
	spotifyPlayerURL    = "https://api.spotify.com/v1/me/player/currently-playing"
)

// SpotifyPlayback mirrors the /v1/me/player/currently-playing response.
type SpotifyPlayback struct {
	CurrentlyPlayingType string `json:"currently_playing_type"`
	ProgressMs           int64  `json:"progress_ms"`
	Item                 struct {
		Name       string `json:"name"`
		DurationMs int64  `json:"duration_ms"`
		Album      struct {
			Name   string `json:"name"`
			Images []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"album"`
		ExternalURLs struct {
			Spotify string `json:"spotify"`
		} `json:"external_urls"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
	} `json:"item"`
}

type SpotifyClient struct {
	clientID     string
	clientSecret string
	redirectURI  string
	http         *http.Client
}

func newSpotifyClient(cfg *Config) *SpotifyClient {
	return &SpotifyClient{
		clientID:     cfg.SpotifyClientID,
		clientSecret: cfg.SpotifyClientSecret,
		redirectURI:  cfg.RedirectURI,
		http:         &http.Client{Timeout: 30 * 1e9},
	}
}

// GetAuthUrl builds the Spotify authorization URL (same shape as the python bot:
// the redirect_uri is concatenated raw, not query-escaped).
func (s *SpotifyClient) GetAuthUrl() string {
	return spotifyAuthorizeURL + "?response_type=code&client_id=" + s.clientID +
		"&redirect_uri=" + s.redirectURI + "&scope=user-read-currently-playing"
}

// GetAccessToken exchanges an authorization code for a refresh token.
// It returns the refresh token, or "error" (mirroring python's sentinel).
func (s *SpotifyClient) GetAccessToken(ctx context.Context, authCode string) (string, error) {
	start := time.Now()
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", authCode)
	form.Set("redirect_uri", s.redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "error", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.clientID, s.clientSecret)

	resp, err := s.http.Do(req)
	if err != nil {
		// Strip *url.Error: its Error() embeds the request URL. Keep only the cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return "error", urlErr.Err
		}
		return "error", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logger.Warn("spotify api non-2xx", "status", resp.StatusCode, "method", "GetAccessToken", logDuration(start))
		return "error", nil
	}

	var res struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "error", err
	}
	return res.RefreshToken, nil
}

// CurrentlyPlaying is the raw HTTP result of the currently-playing request.
type CurrentlyPlaying struct {
	Status int
	Body   []byte
}

// GetCurrentlyPlaying refreshes the Spotify token and fetches the current
// playback state. Errors are returned as-is; the caller decides what to show.
func (s *SpotifyClient) GetCurrentlyPlaying(ctx context.Context, refreshToken string) (*CurrentlyPlaying, error) {
	start := time.Now()
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("redirect_uri", s.redirectURI)
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)

	treq, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		logger.Error("spotify token refresh failed", "err", err)
		return nil, err
	}
	treq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tresp, err := s.http.Do(treq)
	if err != nil {
		// Strip *url.Error: its Error() embeds the request URL. Keep only the cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		logger.Error("spotify token refresh failed", "err", err)
		return nil, err
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	decodeErr := json.NewDecoder(tresp.Body).Decode(&token)
	tresp.Body.Close()
	if decodeErr != nil {
		logger.Error("spotify token refresh failed", "err", decodeErr)
		return nil, decodeErr
	}
	if token.AccessToken == "" {
		err := fmt.Errorf("spotify: no access_token in refresh response")
		logger.Error("spotify token refresh failed", "err", err)
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spotifyPlayerURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := s.http.Do(req)
	if err != nil {
		// Strip *url.Error: its Error() embeds the request URL. Keep only the cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return nil, urlErr.Err
		}
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		logger.Warn("spotify api non-2xx", "status", resp.StatusCode, "method", "GetCurrentlyPlaying", logDuration(start))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &CurrentlyPlaying{Status: resp.StatusCode, Body: body}, nil
}
