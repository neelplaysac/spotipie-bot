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
