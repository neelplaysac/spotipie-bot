package card

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/neelplaysac/spotipie-go/internal/service"
)

// Render fixtures exercise Spotify and Last.fm cards and save JPEGs for visual
// inspection. TestRenderMatchesOriginalCards uses deterministic offline images.
// Run with: go test -run TestRender -v ./internal/card

const testCover = "https://lastfm.freetls.fastly.net/i/u/300x300/2a96cbd8b46e442fc41c2b86b821562f.png"

func spotifyFixture() *service.SpotifyPlayback {
	res := &service.SpotifyPlayback{}
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

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Hashes were captured from the original renderer before the package move,
// using this deterministic cover and profile picture. No network is needed.
func TestRenderMatchesOriginalCards(t *testing.T) {
	cover := image.NewRGBA(image.Rect(0, 0, 300, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 300; x++ {
			cover.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, cover); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	renderer := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "../../fonts")
	renderer.http.Transport = fixtureTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data))}, nil
	})
	for _, test := range []struct{ style, hash string }{
		{"blur", "5367c0fa49d37213fb7b9888f37d6c95ae68808b139ba49b943ab10167d92a32"},
		{"black", "4430b035d8839714b7260669e037e9e23fbbd18e883abe7f10c43f356fe4a615"},
	} {
		img, err := renderer.Spotify(spotifyFixture(), "Neel", data, test.style)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(img)); got != test.hash {
			t.Fatalf("%s card changed: %s", test.style, got)
		}
	}
	img, err := renderer.LastFM(lastfmFixture(), "Neel", data, "1234")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(img)); got != "48eea6b7e8ba50287afcf4035973197e5854ae0f83703476936f3d768d2dddc8" {
		t.Fatalf("Last.fm card changed: %s", got)
	}
}

func lastfmFixture() *service.LastFMTrack {
	t := &service.LastFMTrack{}
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
	renderer := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "../../fonts")
	img, err := renderer.Spotify(spotifyFixture(), "Neel", nil, "blur")
	if err != nil {
		t.Fatalf("drawImage failed: %v", err)
	}
	if err := os.WriteFile("/tmp/now.jpg", img, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	t.Logf("now.jpg written (%d bytes)", len(img))
}

func TestRenderSpotifyBlack(t *testing.T) {
	renderer := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "../../fonts")
	img, err := renderer.Spotify(spotifyFixture(), "Neel", nil, "black")
	if err != nil {
		t.Fatalf("drawImage (black) failed: %v", err)
	}
	if err := os.WriteFile("/tmp/now-black.jpg", img, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	t.Logf("now-black.jpg written (%d bytes)", len(img))
}

func TestRenderLastFM(t *testing.T) {
	renderer := New(slog.New(slog.NewTextHandler(io.Discard, nil)), "../../fonts")
	img, err := renderer.LastFM(lastfmFixture(), "Neel", nil, "1234")
	if err != nil {
		t.Fatalf("blurrImage failed: %v", err)
	}
	if err := os.WriteFile("/tmp/last.jpg", img, 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	t.Logf("last.jpg written (%d bytes)", len(img))
}
