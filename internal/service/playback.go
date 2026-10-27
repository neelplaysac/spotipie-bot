package service

import "context"

type PlaybackRepository interface {
	FetchData(context.Context, string) (*User, error)
	GetLastFMUser(context.Context, string) (*LastFMUser, error)
}

type SpotifyPlayer interface {
	GetCurrentlyPlaying(context.Context, string) (*CurrentlyPlaying, error)
}

type LastFMPlayer interface {
	GetRecentTracks(context.Context, string) (*RecentTracks, error)
	GetScrobbles(context.Context, string, string, string) string
}

type CardRenderer interface {
	Spotify(*SpotifyPlayback, string, []byte, string) ([]byte, error)
	LastFM(*LastFMTrack, string, []byte, string) ([]byte, error)
}

type PlaybackState string

const (
	PlaybackNotRegistered PlaybackState = "not_registered"
	PlaybackNameRequired  PlaybackState = "name_required"
	PlaybackInvalidToken  PlaybackState = "invalid_token"
	PlaybackInactive      PlaybackState = "inactive"
	PlaybackAd            PlaybackState = "ad"
	PlaybackTrack         PlaybackState = "track"
	PlaybackUnknown       PlaybackState = "unknown"
	PlaybackInvalidData   PlaybackState = "invalid_data"
	PlaybackInvalidUser   PlaybackState = "invalid_user"
)

type PlaybackResult struct {
	State PlaybackState
	Image []byte
	URL   string
}

// Stage lets each Telegram entry point retain its existing error response and
// logging policy while sharing the same playback workflow.
type PlaybackError struct {
	Stage string
	Err   error
}

func (e *PlaybackError) Error() string { return e.Err.Error() }
func (e *PlaybackError) Unwrap() error { return e.Err }

type Playback struct {
	repo     PlaybackRepository
	spotify  SpotifyPlayer
	lastfm   LastFMPlayer
	renderer CardRenderer
}

func NewPlayback(repo PlaybackRepository, spotify SpotifyPlayer, lastfm LastFMPlayer, renderer CardRenderer) *Playback {
	return &Playback{repo: repo, spotify: spotify, lastfm: lastfm, renderer: renderer}
}

func (p *Playback) Spotify(ctx context.Context, tgID string, profilePic func() []byte) (*PlaybackResult, error) {
	user, err := p.repo.FetchData(ctx, tgID)
	if err != nil {
		return nil, &PlaybackError{Stage: "db", Err: err}
	}
	if user == nil {
		return &PlaybackResult{State: PlaybackNotRegistered}, nil
	}
	if user.Username == "User" {
		return &PlaybackResult{State: PlaybackNameRequired}, nil
	}
	if user.Token == "00000" {
		return &PlaybackResult{State: PlaybackInvalidToken}, nil
	}
	current, err := p.spotify.GetCurrentlyPlaying(ctx, user.Token)
	if err != nil {
		return nil, &PlaybackError{Stage: "spotify", Err: err}
	}
	pfp := profilePic()
	if current.Status == 204 || current.Playback == nil {
		return &PlaybackResult{State: PlaybackInactive}, nil
	}
	track := current.Playback
	switch track.CurrentlyPlayingType {
	case "ad":
		return &PlaybackResult{State: PlaybackAd}, nil
	case "track":
		if track.Item.Name == "" {
			return &PlaybackResult{State: PlaybackInactive}, nil
		}
		img, err := p.renderer.Spotify(track, user.Username, pfp, user.Style)
		if err != nil {
			return nil, &PlaybackError{Stage: "render", Err: err}
		}
		return &PlaybackResult{State: PlaybackTrack, Image: img, URL: track.Item.ExternalURLs.Spotify}, nil
	default:
		return &PlaybackResult{State: PlaybackUnknown}, nil
	}
}

func (p *Playback) LastFM(ctx context.Context, tgID string, profilePic func() []byte) (*PlaybackResult, error) {
	user, err := p.repo.GetLastFMUser(ctx, tgID)
	if err != nil {
		return nil, &PlaybackError{Stage: "db", Err: err}
	}
	if user == nil {
		return &PlaybackResult{State: PlaybackNotRegistered}, nil
	}
	if user.Name == "User" {
		return &PlaybackResult{State: PlaybackNameRequired}, nil
	}
	recent, err := p.lastfm.GetRecentTracks(ctx, user.FMUsername)
	if err != nil {
		return nil, &PlaybackError{Stage: "lastfm", Err: err}
	}
	pfp := profilePic()
	if !recent.Valid {
		return &PlaybackResult{State: PlaybackInvalidData}, nil
	}
	if recent.Error != 0 {
		return &PlaybackResult{State: PlaybackInvalidUser}, nil
	}
	if recent.Status < 200 || recent.Status > 299 || len(recent.Tracks) == 0 {
		return &PlaybackResult{State: PlaybackInvalidData}, nil
	}
	track := recent.Tracks[0]
	scrobbles := "off"
	if user.Counter != "off" {
		scrobbles = p.lastfm.GetScrobbles(ctx, user.FMUsername, track.Artist.Text, track.Name)
	}
	img, err := p.renderer.LastFM(&track, user.Name, pfp, scrobbles)
	if err != nil {
		return nil, &PlaybackError{Stage: "render", Err: err}
	}
	return &PlaybackResult{State: PlaybackTrack, Image: img, URL: track.URL}, nil
}
