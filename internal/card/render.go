package card

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/disintegration/imaging"
	"github.com/fogleman/gg"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/neelplaysac/spotipie-go/internal/observability"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

const (
	canvasW = 600
	canvasH = 250

	// lastfm fallback cover art (replaces empty / placeholder images)
	lastFMPlaceholder = "https://lastfm.freetls.fastly.net/i/u/300x300/2a96cbd8b46e442fc41c2b86b821562f.png"
	lastFMLogo        = "https://files.catbox.moe/098341.png"
)

type Renderer struct {
	logger   *slog.Logger
	fontsDir string
	http     *http.Client
}

func New(logger *slog.Logger, fontsDir string) *Renderer {
	return &Renderer{logger: logger, fontsDir: fontsDir, http: &http.Client{Timeout: 30 * 1e9}}
}

type fontSet struct {
	name   string
	face   font.Face
	ascent float64 // ascender height in px (PIL's draw.text is top-anchored)
}

var fontKey = map[string]string{
	"open_sans": "OpenSans-Regular.ttf",
	"open_bold": "OpenSans-Bold.ttf",
	"poppins":   "Poppins-SemiBold.ttf",
	"arial":     "arial-unicode-ms.ttf",
}

func (r *Renderer) loadFont(name string, size float64) (*fontSet, error) {
	path := r.fontsDir + "/" + fontKey[name]
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	otf, err := opentype.Parse(data)
	if err != nil {
		return nil, err
	}

	face, err := opentype.NewFace(otf, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return nil, err
	}

	ascent := float64(face.Metrics().Ascent) / 64.0
	return &fontSet{name: name, face: face, ascent: ascent}, nil
}

// textWidth returns the rendered width of text in pixels.
func textWidth(face font.Face, text string) float64 {
	return float64(font.MeasureString(face, text)) / 64.0
}

// checkUnicode mirrors the python bot: True when the string can be rendered by
// the standard TTF fonts (ascii, no control chars, no backslash).
func checkUnicode(text string) bool {
	for _, r := range text {
		if r > 127 || r == '\\' || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// truncate shortens text until it fits within the given width (in px),
// appending ".." when it had to be cut. Works on runes, not bytes.
func truncate(face font.Face, text string, limit float64) string {
	edited := textWidth(face, text) > limit

	runes := []rune(text)
	for textWidth(face, string(runes)) > limit && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}

	out := strings.TrimSpace(string(runes))
	if edited {
		return out + ".."
	}
	return out
}

// fillRectCrisp fills an axis-aligned rectangle with sharp, non-blended edges.
func fillRectCrisp(dst draw.Image, x, y, w, h int, c color.Color) {
	if w <= 0 || h <= 0 {
		return
	}
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), &image.Uniform{c}, image.Point{}, draw.Src)
}

// drawStringAt draws a string at a PIL-style top-left anchor.
func drawStringAt(img draw.Image, f *fontSet, x, y float64, fill color.Color, text string) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(fill),
		Face: f.face,
		Dot:  fixed.P(int(x), int(y+f.ascent)),
	}
	d.DrawString(text)
}

// getCover downloads the album art bytes.
func (r *Renderer) getCover(url string) ([]byte, error) {
	resp, err := r.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cover fetch failed: %d", resp.StatusCode)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// makeBlurBackground builds the blurred + darkened + desaturated background.
func makeBlurBackground(cover []byte) (image.Image, error) {
	art, _, err := image.Decode(bytes.NewReader(cover))
	if err != nil {
		return nil, err
	}

	bg := imaging.Resize(art, canvasW, canvasW, imaging.CatmullRom)
	bg = imaging.Crop(bg, image.Rect(0, 175, canvasW, 425))
	bg = imaging.Blur(bg, 25)
	bg = adjustBrightnessPillow(bg, 0.9) // PIL enhance(0.9)
	bg = adjustContrastPillow(bg, 0.8)   // PIL enhance(0.8)
	return bg, nil
}

// adjustBrightnessPillow mirrors PIL ImageEnhance.Brightness.enhance(factor):
// Image.blend(black, img, factor), i.e. per channel (uint8)(x*factor) using C
// truncation toward zero.
func adjustBrightnessPillow(img image.Image, factor float64) *image.NRGBA {
	var lut [256]uint8
	for i := 0; i < 256; i++ {
		v := int(float64(i) * factor)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		lut[i] = uint8(v)
	}
	return imaging.AdjustFunc(img, func(c color.NRGBA) color.NRGBA {
		c.R, c.G, c.B = lut[c.R], lut[c.G], lut[c.B]
		return c
	})
}

// adjustContrastPillow mirrors PIL ImageEnhance.Contrast.enhance(factor):
//
//	mean := int(ImageStat.Stat(img.convert("L")).mean[0] + 0.5)
//	out  := (uint8)(mean + (x-mean)*factor)   // C truncation toward zero
//
// Pillow blends toward the image's own measured mean, not a fixed grey.
func adjustContrastPillow(img image.Image, factor float64) *image.NRGBA {
	b := img.Bounds()
	var sum, n uint64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			// Pillow's L conversion for RGB: (R*19595 + G*38470 + B*7471) >> 16
			l := (int(c.R)*19595 + int(c.G)*38470 + int(c.B)*7471 + 32768) >> 16
			sum += uint64(l)
			n++
		}
	}
	if n == 0 {
		return imaging.Clone(img)
	}
	mean := int(float64(sum)/float64(n) + 0.5)

	var lut [256]uint8
	for i := 0; i < 256; i++ {
		// Pillow truncates the WHOLE blend result, not just the delta:
		//   (uint8)(mean + (x-mean)*factor)
		// Empirically verified against Pillow 12.3.0 for mean=128 and mean=113
		// (a per-delta truncation is off by one on ~100 of 256 levels).
		v := int(float64(mean) + float64(i-mean)*factor)
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		lut[i] = uint8(v)
	}
	return imaging.AdjustFunc(img, func(c color.NRGBA) color.NRGBA {
		c.R, c.G, c.B = lut[c.R], lut[c.G], lut[c.B]
		return c
	})
}

// Spotify generates the Spotify "now playing" card (600x250 JPEG).
// style is "blur" or "black".
func (r *Renderer) Spotify(res *service.SpotifyPlayback, username string, pfp []byte, style string) ([]byte, error) {
	start := time.Now()
	songname := res.Item.Name
	albumname := res.Item.Album.Name
	totaltime := res.Item.DurationMs
	currtime := res.ProgressMs
	coverart := ""
	if len(res.Item.Album.Images) > 1 {
		coverart = res.Item.Album.Images[1].URL
	}
	artists := []string{}
	for _, a := range res.Item.Artists {
		if a.Name != "" {
			artists = append(artists, a.Name)
		}
	}
	artistsStr := strings.Join(artists, ", ")

	// background object
	canvas := imaging.New(canvasW, canvasH, color.RGBA{18, 18, 18, 255})

	// album art
	if coverart != "" {
		if cover, err := r.getCover(coverart); err == nil {
			if style == "blur" {
				if bg, err := makeBlurBackground(cover); err == nil {
					canvas = imaging.Paste(canvas, bg, image.Pt(0, 0))
				}
			}

			if art, _, err := image.Decode(bytes.NewReader(cover)); err == nil {
				thumb := imaging.Fit(art, 200, 200, imaging.Lanczos)
				canvas = imaging.Paste(canvas, thumb, image.Pt(25, 25))
			}
		} else {
			r.logger.Warn("failed to fetch spotify cover", "err", err, "url", coverart)
		}
	}

	// profile pic
	if pfp != nil {
		if profile, _, err := image.Decode(bytes.NewReader(pfp)); err == nil {
			thumb := imaging.Fit(profile, 52, 52, imaging.Lanczos)
			canvas = imaging.Paste(canvas, thumb, image.Pt(523, 25))
		}
	}

	// fonts
	openSans, err := r.loadFont("open_sans", 24)
	if err != nil {
		return nil, err
	}
	poppins, err := r.loadFont("poppins", 26)
	if err != nil {
		return nil, err
	}
	arial, err := r.loadFont("arial", 26)
	if err != nil {
		return nil, err
	}
	arial23, err := r.loadFont("arial", 23)
	if err != nil {
		return nil, err
	}

	// assign fonts (unicode goes through arial-unicode-ms)
	songfont := poppins
	artistfont := openSans
	albumfont := openSans
	if !checkUnicode(songname) {
		songfont = arial
	}
	if !checkUnicode(artistsStr) {
		artistfont = arial23
	}
	if !checkUnicode(albumname) {
		albumfont = arial23
	}

	white := color.White
	drawStringAt(canvas, poppins, 248, 18, white, truncate(poppins.face, username, 250))
	drawStringAt(canvas, openSans, 248, 53, white, "is listening to")
	drawStringAt(canvas, songfont, 248, 115, white, truncate(songfont.face, songname, 315))
	drawStringAt(canvas, artistfont, 248, 150, white, truncate(artistfont.face, artistsStr, 315))
	drawStringAt(canvas, albumfont, 248, 180, white, truncate(albumfont.face, albumname, 315))

	// progress bar
	filled := 0.0
	if totaltime > 0 {
		filled = float64(currtime) / float64(totaltime) * 330
		if filled > 330 {
			filled = 330
		}
	}
	// Progress bars: Pillow's ImageDraw does not anti-alias, while gg's Fill
	// does (gg v1.3.0 has no toggle), so paint the bars directly with sharp
	// edges. Bounds are inclusive in Pillow: x=248..578, y=222..224 = 331x3.
	fillRectCrisp(canvas, 248, 222, 331, 3, color.RGBA{0x40, 0x40, 0x40, 0xFF}) // #404040
	if filled > 0 {
		w := int(filled + 1)
		if w > 331 {
			w = 331
		}
		fillRectCrisp(canvas, 248, 222, w, 3, color.RGBA{0xB3, 0xB3, 0xB3, 0xFF}) // #B3B3B3
	}
	dc := gg.NewContextForImage(canvas)

	img, err := encodeJPEG(dc.Image())
	if err == nil {
		r.logger.Debug("rendered card", "kind", "now", "bytes", len(img), observability.Duration(start))
	}
	return img, err
}

// LastFM generates the Last.fm card.
func (r *Renderer) LastFM(track *service.LastFMTrack, username string, pfp []byte, scrobbles string) ([]byte, error) {
	start := time.Now()
	coverart := ""
	if len(track.Image) > 3 {
		coverart = track.Image[3].Text
	}
	if coverart == "" || coverart == lastFMPlaceholder {
		coverart = lastFMLogo
	}

	artists := track.Artist.Text
	albumname := track.Album.Text
	songname := track.Name
	tense := "was"
	if _, ok := track.Attr["nowplaying"]; ok {
		tense = "is"
	}

	// background object
	canvas := imaging.New(canvasW, canvasH, color.RGBA{18, 18, 18, 255})

	// album art (always blurred for lastfm)
	if cover, err := r.getCover(coverart); err == nil {
		if bg, err := makeBlurBackground(cover); err == nil {
			canvas = imaging.Paste(canvas, bg, image.Pt(0, 0))
		}
		if art, _, err := image.Decode(bytes.NewReader(cover)); err == nil {
			thumb := imaging.Fit(art, 200, 200, imaging.Lanczos)
			canvas = imaging.Paste(canvas, thumb, image.Pt(25, 25))
		}
	} else {
		r.logger.Warn("failed to fetch lastfm cover", "err", err, "url", coverart)
	}

	// profile pic
	if pfp != nil {
		if profile, _, err := image.Decode(bytes.NewReader(pfp)); err == nil {
			thumb := imaging.Fit(profile, 52, 52, imaging.Lanczos)
			canvas = imaging.Paste(canvas, thumb, image.Pt(523, 25))
		}
	}

	// fonts
	openSans, err := r.loadFont("open_sans", 24)
	if err != nil {
		return nil, err
	}
	poppins, err := r.loadFont("poppins", 26)
	if err != nil {
		return nil, err
	}
	arial, err := r.loadFont("arial", 26)
	if err != nil {
		return nil, err
	}
	arial23, err := r.loadFont("arial", 23)
	if err != nil {
		return nil, err
	}
	bold, err := r.loadFont("open_bold", 22)
	if err != nil {
		return nil, err
	}

	// assign fonts
	songfont := poppins
	artistfont := openSans
	albumfont := openSans
	if !checkUnicode(songname) {
		songfont = arial
	}
	if !checkUnicode(artists) {
		artistfont = arial23
	}
	if !checkUnicode(albumname) {
		albumfont = arial23
	}

	white := color.White
	drawStringAt(canvas, poppins, 248, 18, white, truncate(poppins.face, username, 250))
	drawStringAt(canvas, openSans, 248, 53, white, tense+" listening to")
	drawStringAt(canvas, songfont, 248, 105, white, truncate(songfont.face, songname, 315))
	drawStringAt(canvas, artistfont, 248, 140, white, truncate(artistfont.face, artists, 315))
	drawStringAt(canvas, albumfont, 248, 170, white, truncate(albumfont.face, albumname, 315))

	// Progress bar: Pillow bounds are inclusive (x=248..578, y=221..223 =
	// 331x3) and ImageDraw does not anti-alias, so paint directly.
	fillRectCrisp(canvas, 248, 221, 331, 3, color.RGBA{0xB3, 0xB3, 0xB3, 0xFF}) // #B3B3B3
	dc := gg.NewContextForImage(canvas)

	if scrobbles != "off" {
		widths := map[int]float64{1: 45, 2: 58, 3: 73, 4: 86, 5: 100, 6: 111}
		w := 111.0
		if v, ok := widths[len([]rune(scrobbles))]; ok {
			w = v
		}
		dc.SetHexColor("#121212")
		dc.DrawRoundedRectangle(18, 200, w-18, 31, 13)
		dc.Fill()
		if d, ok := dc.Image().(draw.Image); ok {
			drawStringAt(d, bold, 26, 200, white, scrobbles)
		}
	}

	img, err := encodeJPEG(dc.Image())
	if err == nil {
		r.logger.Debug("rendered card", "kind", "last", "bytes", len(img), observability.Duration(start))
	}
	return img, err
}

func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
