package main

import (
	"os"
	"testing"
)

// Offline render tests: exercise drawImage (spotify card) and blurrImage
// (lastfm card) against fixture data and save JPEGs for visual inspection.
// Run with: go test -run TestRender -v

const testCover = "https://lastfm.freetls.fastly.net/i/u/300x300/2a96cbd8b46e442fc41c2b86b821562f.png"

func spotifyFixture() *SpotifyPlayback {
	res := &SpotifyPlayback{}
	res.CurrentlyPlayingType = "track"
	res.ProgressMs = 90000
	res.Item.Name = "Bohemian Rhapsody"
	res.Item.DurationMs = 240000
	res.Item.Album.Name = "A Night at the Opera"
	res.Item.Album.Images = []struct {
		URL string `json:"url"`
	}{{URL: testCover}, {URL: testCover}}
	res.Item.ExternalURLs.Spotify = "https://open.spotify.com/track/test"
	res.Item.Artists = []struct {
		Name string `json:"name"`
	}{{Name: "Queen"}}
	return res
}

func lastfmFixture() *LastFMTrack {
	t := &LastFMTrack{}
	t.Artist.Text = "Queen"
	t.Album.Text = "A Night at the Opera"
	t.Name = "Bohemian Rhapsody"
	t.URL = "https://www.last.fm/music/Queen/_/Bohemian+Rhapsody"
	t.Image = []struct {
		Text string `json:"#text"`
	}{{Text: testCover}, {Text: testCover}, {Text: testCover}, {Text: "https://picsum.photos/300/300"}}
	t.Attr = map[string]string{"nowplaying": "true"}
	return t
}

func TestRenderSpotify(t *testing.T) {
	img, err := drawImage(spotifyFixture(), "Neel", nil, "blur")
	if err != nil {
		t.Fatalf("drawImage failed: %v", err)
	}
	if err := os.WriteFile("/tmp/now.jpg", img, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	t.Logf("now.jpg written (%d bytes)", len(img))
}

func TestRenderSpotifyBlack(t *testing.T) {
	img, err := drawImage(spotifyFixture(), "Neel", nil, "black")
	if err != nil {
		t.Fatalf("drawImage (black) failed: %v", err)
	}
	if err := os.WriteFile("/tmp/now-black.jpg", img, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	t.Logf("now-black.jpg written (%d bytes)", len(img))
}

func TestRenderLastFM(t *testing.T) {
	img, err := blurrImage(lastfmFixture(), "Neel", nil, "1234")
	if err != nil {
		t.Fatalf("blurrImage failed: %v", err)
	}
	if err := os.WriteFile("/tmp/last.jpg", img, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	t.Logf("last.jpg written (%d bytes)", len(img))
}
