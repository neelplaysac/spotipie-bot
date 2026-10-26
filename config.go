package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	BOT_URL = "t.me/%s"
)

type Config struct {
	APIKey              string
	BotUsername         string
	SpotifyClientID     string
	SpotifyClientSecret string
	MongoUser           string
	MongoPass           string
	MongoColl           string
	TempChannel         int64
	RedirectURI         string
	LastFMAPIKey        string
	AdminIDs            map[string]bool
}

func loadConfig() *Config {
	// Load environment variables from .env file
	_ = godotenv.Load()

	cfg := &Config{
		APIKey:              os.Getenv("API_KEY"),
		BotUsername:         envOr("BOT_USERNAME", "spotipiebot"),
		SpotifyClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		SpotifyClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		MongoUser:           os.Getenv("MONGO_USR"),
		MongoPass:           os.Getenv("MONGO_PASS"),
		MongoColl:           os.Getenv("MONGO_COLL"),
		RedirectURI:         os.Getenv("REDIRECT_URI"),
		LastFMAPIKey:        os.Getenv("LASTFM_API_KEY"),
		AdminIDs:            map[string]bool{},
	}

	for _, id := range strings.Split(os.Getenv("ADMIN_IDS"), ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			cfg.AdminIDs[id] = true
		}
	}

	if ch := os.Getenv("TEMP_CHANNEL"); ch != "" {
		if v, err := strconv.ParseInt(ch, 10, 64); err == nil {
			cfg.TempChannel = v
		}
	}

	return cfg
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func (c *Config) missingRequired() []string {
	var missing []string
	if c.APIKey == "" {
		missing = append(missing, "API_KEY")
	}
	if c.SpotifyClientID == "" {
		missing = append(missing, "SPOTIFY_CLIENT_ID")
	}
	if c.SpotifyClientSecret == "" {
		missing = append(missing, "SPOTIFY_CLIENT_SECRET")
	}
	if c.MongoUser == "" {
		missing = append(missing, "MONGO_USR")
	}
	if c.MongoPass == "" {
		missing = append(missing, "MONGO_PASS")
	}
	if c.MongoColl == "" {
		missing = append(missing, "MONGO_COLL")
	}
	if c.RedirectURI == "" {
		missing = append(missing, "REDIRECT_URI")
	}
	if c.LastFMAPIKey == "" {
		missing = append(missing, "LASTFM_API_KEY")
	}
	return missing
}
