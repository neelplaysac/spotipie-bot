package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/observability"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

const (
	spotifyAuthorizeURL = "https://accounts.spotify.com/authorize"
	spotifyTokenURL     = "https://accounts.spotify.com/api/token"
	spotifyPlayerURL    = "https://api.spotify.com/v1/me/player/currently-playing"
)

type Client struct {
	logger       *slog.Logger
	clientID     string
	clientSecret string
	redirectURI  string
	http         *http.Client
}

func New(cfg *config.Config, logger *slog.Logger) *Client {
	return &Client{
		logger:       logger,
		clientID:     cfg.SpotifyClientID,
		clientSecret: cfg.SpotifyClientSecret,
		redirectURI:  cfg.RedirectURI,
		http:         &http.Client{Timeout: 30 * 1e9},
	}
}

// GetAuthUrl builds the Spotify authorization URL (same shape as the python bot:
// the redirect_uri is concatenated raw, not query-escaped).
func (s *Client) GetAuthUrl() string {
	return spotifyAuthorizeURL + "?response_type=code&client_id=" + s.clientID +
		"&redirect_uri=" + s.redirectURI + "&scope=user-read-currently-playing"
}

// GetAccessToken exchanges an authorization code for a refresh token.
// It returns the refresh token, or "error" (mirroring python's sentinel).
func (s *Client) GetAccessToken(ctx context.Context, authCode string) (string, error) {
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
		s.logger.Warn("spotify api non-2xx", "status", resp.StatusCode, "method", "GetAccessToken", observability.Duration(start))
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

// GetCurrentlyPlaying refreshes the Spotify token and fetches the current
// playback state. Errors are returned as-is; the caller decides what to show.
func (s *Client) GetCurrentlyPlaying(ctx context.Context, refreshToken string) (*service.CurrentlyPlaying, error) {
	start := time.Now()
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("redirect_uri", s.redirectURI)
	form.Set("client_id", s.clientID)
	form.Set("client_secret", s.clientSecret)

	treq, err := http.NewRequestWithContext(ctx, http.MethodPost, spotifyTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		s.logger.ErrorContext(ctx, "spotify token refresh failed", "err", err)
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
		s.logger.ErrorContext(ctx, "spotify token refresh failed", "err", err)
		return nil, err
	}

	var token struct {
		AccessToken string `json:"access_token"`
	}
	decodeErr := json.NewDecoder(tresp.Body).Decode(&token)
	tresp.Body.Close()
	if decodeErr != nil {
		s.logger.ErrorContext(ctx, "spotify token refresh failed", "err", decodeErr)
		return nil, decodeErr
	}
	if token.AccessToken == "" {
		err := fmt.Errorf("spotify: no access_token in refresh response")
		s.logger.ErrorContext(ctx, "spotify token refresh failed", "err", err)
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
		s.logger.Warn("spotify api non-2xx", "status", resp.StatusCode, "method", "GetCurrentlyPlaying", observability.Duration(start))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	result := &service.CurrentlyPlaying{Status: resp.StatusCode}
	if len(body) > 0 {
		var playback service.SpotifyPlayback
		if json.Unmarshal(body, &playback) == nil {
			result.Playback = &playback
		}
	}
	return result, nil
}
