package service

import (
	"context"
	"errors"
	"testing"
)

type spotifyPlayerStub struct {
	current *CurrentlyPlaying
	err     error
	called  bool
}

func (s *spotifyPlayerStub) GetCurrentlyPlaying(context.Context, string) (*CurrentlyPlaying, error) {
	s.called = true
	return s.current, s.err
}

type lastFMPlayerStub struct {
	recent  *RecentTracks
	counted bool
	err     error
}

func (s *lastFMPlayerStub) GetRecentTracks(context.Context, string) (*RecentTracks, error) {
	return s.recent, s.err
}
func (s *lastFMPlayerStub) GetScrobbles(context.Context, string, string, string) string {
	s.counted = true
	return "1234"
}

type rendererStub struct {
	name      string
	style     string
	profile   []byte
	scrobbles string
	err       error
}

func (r *rendererStub) Spotify(_ *SpotifyPlayback, name string, profile []byte, style string) ([]byte, error) {
	r.name, r.profile, r.style = name, profile, style
	return []byte("card"), r.err
}
func (r *rendererStub) LastFM(_ *LastFMTrack, name string, profile []byte, scrobbles string) ([]byte, error) {
	r.name, r.profile, r.scrobbles = name, profile, scrobbles
	return []byte("card"), r.err
}

func TestSpotifyPlaybackStates(t *testing.T) {
	track := &SpotifyPlayback{CurrentlyPlayingType: "track"}
	track.Item.Name = "Song"
	track.Item.ExternalURLs.Spotify = "https://example.invalid/track"
	for _, test := range []struct {
		name    string
		user    *User
		current *CurrentlyPlaying
		state   PlaybackState
	}{
		{"unregistered", nil, nil, PlaybackNotRegistered},
		{"name required", &User{Username: "User"}, nil, PlaybackNameRequired},
		{"token invalid", &User{Username: "Name", Token: "00000"}, nil, PlaybackInvalidToken},
		{"no playback", &User{Username: "Name"}, &CurrentlyPlaying{Status: 204}, PlaybackInactive},
		{"malformed playback", &User{Username: "Name"}, &CurrentlyPlaying{Status: 200}, PlaybackInactive},
		{"ad", &User{Username: "Name"}, &CurrentlyPlaying{Playback: &SpotifyPlayback{CurrentlyPlayingType: "ad"}}, PlaybackAd},
		{"episode", &User{Username: "Name"}, &CurrentlyPlaying{Playback: &SpotifyPlayback{CurrentlyPlayingType: "episode"}}, PlaybackUnknown},
		{"empty track", &User{Username: "Name"}, &CurrentlyPlaying{Playback: &SpotifyPlayback{CurrentlyPlayingType: "track"}}, PlaybackInactive},
		{"track", &User{Username: "Name", Style: "black"}, &CurrentlyPlaying{Playback: track}, PlaybackTrack},
	} {
		t.Run(test.name, func(t *testing.T) {
			player := &spotifyPlayerStub{current: test.current}
			renderer := &rendererStub{}
			profileCalled := false
			result, err := NewPlayback(&accountRepositoryStub{user: test.user}, player, nil, renderer).Spotify(context.Background(), "42", func() []byte {
				profileCalled = true
				if !player.called {
					t.Fatal("profile requested before Spotify playback")
				}
				return []byte("profile")
			})
			if err != nil || result.State != test.state {
				t.Fatalf("result=%+v err=%v; want %q", result, err, test.state)
			}
			if profileCalled != (test.current != nil) {
				t.Fatal("profile fetch order or short-circuit behavior changed")
			}
			if test.state == PlaybackTrack && (result.URL != track.Item.ExternalURLs.Spotify || string(result.Image) != "card" || renderer.name != "Name" || renderer.style != "black" || string(renderer.profile) != "profile") {
				t.Fatal("playback metadata changed across service boundaries")
			}
		})
	}
}

func TestLastFMPlaybackAndCounter(t *testing.T) {
	for _, counter := range []string{"on", "off", ""} {
		t.Run(counter, func(t *testing.T) {
			track := LastFMTrack{Name: "Song", URL: "https://example.invalid/track"}
			player := &lastFMPlayerStub{recent: &RecentTracks{Status: 200, Valid: true, Tracks: []LastFMTrack{track}}}
			renderer := &rendererStub{}
			repo := &accountRepositoryStub{fm: &LastFMUser{Name: "Name", FMUsername: "fm-user", Counter: counter}}
			result, err := NewPlayback(repo, nil, player, renderer).LastFM(context.Background(), "42", func() []byte { return []byte("profile") })
			if err != nil || result.State != PlaybackTrack || result.URL != track.URL || renderer.name != "Name" || player.counted != (counter != "off") {
				t.Fatalf("Last.fm workflow changed: result=%+v err=%v counter=%q", result, err, counter)
			}
			want := "1234"
			if counter == "off" {
				want = "off"
			}
			if renderer.scrobbles != want {
				t.Fatalf("scrobbles=%q, want %q", renderer.scrobbles, want)
			}
		})
	}
}

func TestLastFMErrorResponsePrecedence(t *testing.T) {
	for _, test := range []struct {
		recent *RecentTracks
		state  PlaybackState
	}{
		{&RecentTracks{Status: 500}, PlaybackInvalidData},
		{&RecentTracks{Status: 500, Valid: true, Error: 6}, PlaybackInvalidUser},
		{&RecentTracks{Status: 200, Valid: true}, PlaybackInvalidData},
	} {
		repo := &accountRepositoryStub{fm: &LastFMUser{Name: "Name"}}
		result, err := NewPlayback(repo, nil, &lastFMPlayerStub{recent: test.recent}, nil).LastFM(context.Background(), "42", func() []byte { return nil })
		if err != nil || result.State != test.state {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
}

func TestPlaybackErrorsRetainCauseAndStage(t *testing.T) {
	cause := errors.New("Spotify unavailable")
	_, err := NewPlayback(&accountRepositoryStub{user: &User{Username: "Name"}}, &spotifyPlayerStub{err: cause}, nil, nil).Spotify(context.Background(), "42", func() []byte { t.Fatal("profile fetched after API failure"); return nil })
	var failure *PlaybackError
	if !errors.As(err, &failure) || failure.Stage != "spotify" || !errors.Is(err, cause) {
		t.Fatalf("lost error classification or cause: %v", err)
	}
}
