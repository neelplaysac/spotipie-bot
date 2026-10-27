package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	APIKey              string
	BotUsername         string
	SpotifyClientID     string
	SpotifyClientSecret string
	MongoUser           string
	MongoPass           string
	MongoColl           string
	MongoURI            string
	MongoDatabase       string
	HTTPPort            string
	BotURL              string
	TempChannel         int64
	RedirectURI         string
	LastFMAPIKey        string
	AdminIDs            map[string]bool

	SentryDSN              string
	SentryEnvironment      string
	SentryRelease          string
	SentryTracesSampleRate string
}

func Load() *Config {
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
		MongoURI:            os.Getenv("MONGODB_URI"),
		MongoDatabase:       os.Getenv("MONGODB_DATABASE"),
		HTTPPort:            envOr("PORT", "6969"),
		BotURL:              os.Getenv("BOT_URL"),
		RedirectURI:         os.Getenv("REDIRECT_URI"),
		LastFMAPIKey:        os.Getenv("LASTFM_API_KEY"),
		AdminIDs:            map[string]bool{},

		SentryDSN:              strings.TrimSpace(os.Getenv("SENTRY_DSN")),
		SentryEnvironment:      envOr("SENTRY_ENVIRONMENT", "production"),
		SentryRelease:          os.Getenv("SENTRY_RELEASE"),
		SentryTracesSampleRate: envOr("SENTRY_TRACES_SAMPLE_RATE", "0.01"),
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

func (c *Config) MissingRequired() []string {
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
	if c.MongoURI == "" && c.MongoUser == "" {
		missing = append(missing, "MONGO_USR")
	}
	if c.MongoURI == "" && c.MongoPass == "" {
		missing = append(missing, "MONGO_PASS")
	}
	if c.MongoURI == "" && c.MongoColl == "" {
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

// DatabaseName preserves the bot's legacy database selection when using the
// original Atlas variables. Both commands must select the same database.
func (c *Config) DatabaseName() string {
	if c.MongoDatabase != "" {
		return c.MongoDatabase
	}
	if c.MongoColl != "" {
		return c.MongoColl
	}
	return "spotipie"
}

func (c *Config) ValidateAuthServer() error {
	port, err := strconv.Atoi(c.HTTPPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("PORT must be between 1 and 65535")
	}
	if c.MongoURI == "" && (c.MongoUser == "" || c.MongoPass == "" || c.MongoColl == "") {
		return fmt.Errorf("set MONGODB_URI or all of MONGO_USR, MONGO_PASS and MONGO_COLL")
	}
	_, err = c.TelegramReturnURL()
	return err
}

var botNamePattern = regexp.MustCompile(`^[A-Za-z0-9_]{5,32}$`)

// TelegramReturnURL is a fixed redirect destination; callback input can never
// choose its host or bot. BOT_URL supports the previous server's configuration.
func (c *Config) TelegramReturnURL() (string, error) {
	raw := strings.TrimSpace(c.BotURL)
	if raw == "" {
		raw = "https://t.me/" + c.BotUsername + "?start="
	} else if strings.HasPrefix(raw, "t.me/") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "t.me" || u.User != nil || u.Fragment != "" ||
		!botNamePattern.MatchString(strings.TrimPrefix(u.Path, "/")) || u.RawQuery != "start=" {
		return "", fmt.Errorf("BOT_URL must be https://t.me/<bot_username>?start= (or set BOT_USERNAME)")
	}
	return u.String(), nil
}

func (c *Config) ValidateBotIdentity(username string) error {
	returnURL, err := c.TelegramReturnURL()
	if err != nil {
		return err
	}
	u, _ := url.Parse(returnURL)
	if !strings.EqualFold(strings.TrimPrefix(u.Path, "/"), username) {
		return fmt.Errorf("BOT_USERNAME/BOT_URL must name the bot authenticated by API_KEY")
	}
	return nil
}
