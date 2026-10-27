package config

import (
	"testing"
)

func TestMonitoringReadsEnvironment(t *testing.T) {
	t.Chdir(t.TempDir()) // Do not load the developer's real .env file.
	for key, value := range map[string]string{
		"SENTRY_DSN":                " https://public@example.invalid/1 ",
		"SENTRY_ENVIRONMENT":        "staging",
		"SENTRY_RELEASE":            "spotipie-go@test",
		"SENTRY_TRACES_SAMPLE_RATE": "0",
	} {
		t.Setenv(key, value)
	}
	cfg := Load()
	if cfg.SentryDSN != "https://public@example.invalid/1" || cfg.SentryEnvironment != "staging" || cfg.SentryRelease != "spotipie-go@test" || cfg.SentryTracesSampleRate != "0" {
		t.Fatal("monitoring environment was not loaded correctly")
	}
}

func TestAuthServerConfiguration(t *testing.T) {
	for _, test := range []struct {
		name, port, uri, botURL string
		valid                   bool
	}{
		{"defaults", "6969", "mongodb://localhost:27017", "", true},
		{"legacy bot URL", "6969", "mongodb://localhost:27017", "t.me/testbot?start=", true},
		{"bad port", "0", "mongodb://localhost:27017", "", false},
		{"out of range", "65536", "mongodb://localhost:27017", "", false},
		{"no database", "6969", "", "", false},
		{"wrong redirect host", "6969", "mongodb://localhost:27017", "https://evil.invalid/testbot?start=", false},
		{"redirect userinfo", "6969", "mongodb://localhost:27017", "https://user@t.me/testbot?start=", false},
		{"bad query", "6969", "mongodb://localhost:27017", "https://t.me/testbot?start=x&next=evil", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := &Config{MongoURI: test.uri, HTTPPort: test.port, BotUsername: "testbot", BotURL: test.botURL}
			if err := cfg.ValidateAuthServer(); (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
		})
	}
	if got := (&Config{MongoColl: "legacy"}).DatabaseName(); got != "legacy" {
		t.Fatal(got)
	}
	if got := (&Config{MongoColl: "legacy", MongoDatabase: "explicit"}).DatabaseName(); got != "explicit" {
		t.Fatal(got)
	}
	if got := (&Config{}).DatabaseName(); got != "spotipie" {
		t.Fatal(got)
	}
}

func TestBotAcceptsMongoURIWithoutLegacyCredentials(t *testing.T) {
	cfg := &Config{APIKey: "token", SpotifyClientID: "id", SpotifyClientSecret: "secret", MongoURI: "mongodb://localhost:27017", RedirectURI: "https://example.invalid/callback", LastFMAPIKey: "lastfm"}
	if missing := cfg.MissingRequired(); len(missing) != 0 {
		t.Fatalf("unexpected requirements: %v", missing)
	}
}

func TestTelegramReturnLinkMatchesAuthenticatedBot(t *testing.T) {
	cfg := &Config{BotUsername: "testbot"}
	if err := cfg.ValidateBotIdentity("TestBot"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateBotIdentity("anotherbot"); err == nil {
		t.Fatal("accepted a return link to a different bot")
	}
	cfg.BotURL = "https://t.me/anotherbot?start="
	if err := cfg.ValidateBotIdentity("testbot"); err == nil {
		t.Fatal("BOT_URL overrode the authenticated bot identity")
	}
}
