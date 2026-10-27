package service

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

type CurrentlyPlaying struct {
	Status   int
	Playback *SpotifyPlayback
}

type RecentTracks struct {
	Status int
	Valid  bool
	Error  int
	Tracks []LastFMTrack
}
