package observability

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/neelplaysac/spotipie-go/internal/config"
)

// No test sends events to a real GlitchTip/Sentry endpoint.
type eventRecorder struct {
	mu      sync.Mutex
	events  []*sentry.Event
	flushed bool
}

func (r *eventRecorder) Configure(sentry.ClientOptions) {}
func (r *eventRecorder) Close()                         {}
func (r *eventRecorder) Flush(time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushed = true
	return true
}
func (r *eventRecorder) FlushWithContext(context.Context) bool { return r.Flush(0) }
func (r *eventRecorder) SendEvent(event *sentry.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func setupMonitoringTest(t *testing.T, traceRate float64) (*eventRecorder, *slog.Logger) {
	t.Helper()
	recorder := &eventRecorder{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:                   "https://public@example.invalid/1",
		Transport:             recorder,
		AttachStacktrace:      true,
		EnableTracing:         traceRate > 0,
		TracesSampleRate:      traceRate,
		BeforeSend:            sentryScrubber(&config.Config{}),
		BeforeSendTransaction: sentryScrubber(&config.Config{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	hub := sentry.CurrentHub()
	oldClient := hub.Client()
	hub.PushScope()
	hub.BindClient(client)
	logger := slog.New(&sentryLogHandler{next: slog.NewTextHandler(io.Discard, nil)})
	t.Cleanup(func() {
		client.Close()
		hub.PopScope()
		hub.BindClient(oldClient)
	})
	return recorder, logger
}

func TestMonitoringConfiguration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, rate := range []string{"-0.1", "1.1", "NaN", "Inf", "invalid"} {
		t.Run(rate, func(t *testing.T) {
			_, err := Init(&config.Config{SentryDSN: "https://public@example.invalid/1", SentryTracesSampleRate: rate}, logger)
			if err == nil {
				t.Fatal("invalid trace sample rate was accepted")
			}
		})
	}
	if _, err := Init(&config.Config{}, logger); err != nil {
		t.Fatalf("missing DSN must leave monitoring optional: %v", err)
	}
	if _, err := Init(&config.Config{SentryDSN: "not-a-dsn", SentryTracesSampleRate: "0.01"}, logger); err == nil {
		t.Fatal("invalid DSN was accepted")
	}
}

func TestMonitoringDisabledDoesNotFlush(t *testing.T) {
	hub := sentry.CurrentHub()
	oldClient := hub.Client()
	hub.PushScope()
	hub.BindClient(nil)
	defer func() { hub.PopScope(); hub.BindClient(oldClient) }()
	var output strings.Builder
	logger := slog.New(slog.NewTextHandler(&output, nil))
	if _, err := Init(&config.Config{}, logger); err != nil {
		t.Fatal(err)
	}
	Flush(logger)
	if strings.Contains(output.String(), "timed out") {
		t.Fatal("disabled SDK produced a false flush timeout warning")
	}
}

func TestSentryLogsCaptureErrorsAndKeepStdout(t *testing.T) {
	r, logger := setupMonitoringTest(t, 1)
	var output strings.Builder
	log := slog.New(&sentryLogHandler{next: slog.NewTextHandler(&output, nil)}).With("component", "updater").WithGroup("polling")
	log.Info("started")
	log.Warn("retrying")
	log.Error("failed to get updates", "error", errors.New("Bad Gateway"), "user_id", "private-user-id")
	if len(r.events) != 1 {
		t.Fatalf("got %d events, want only the error", len(r.events))
	}
	event := r.events[0]
	if len(event.Exception) == 0 || event.Exception[0].Value != "Bad Gateway" || event.Exception[0].Stacktrace == nil {
		t.Fatalf("exception or stack trace missing: %+v", event.Exception)
	}
	if event.Tags["component"] != "updater" {
		t.Fatalf("logger's bound attributes missing: %+v", event.Tags)
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-user-id") {
		t.Fatal("Telegram user ID was sent to monitoring")
	}
	for _, message := range []string{"started", "retrying", "failed to get updates"} {
		if !strings.Contains(output.String(), message) {
			t.Fatalf("stdout log missing %q", message)
		}
	}
	Flush(logger)
	if !r.flushed {
		t.Fatal("flush did not reach the SDK transport")
	}
}

func TestSentryScrubsCredentials(t *testing.T) {
	cfg := &config.Config{APIKey: "123456789:abcdefghijklmnopqrstuvwxyzABCDEFG", SpotifyClientSecret: "test-spotify-secret", MongoPass: "test-p@ss:/word", LastFMAPIKey: "test-lastfm-key"}
	frame := sentry.Frame{Filename: "handlers.go", Lineno: 42, Vars: map[string]any{"token": cfg.APIKey}, ContextLine: cfg.SpotifyClientSecret}
	event := sentry.NewEvent()
	event.Message = "failure: " + cfg.SpotifyClientSecret + " " + cfg.LastFMAPIKey
	event.Exception = []sentry.Exception{{Value: "https://api.telegram.org/bot" + cfg.APIKey + "/getUpdates Bearer dynamic-access-token mongodb://user:" + cfg.MongoPass + "@db", Stacktrace: &sentry.Stacktrace{Frames: []sentry.Frame{frame}}}}
	event.Threads = []sentry.Thread{{Stacktrace: &sentry.Stacktrace{Frames: []sentry.Frame{frame}}}}
	event.User = sentry.User{ID: "private-user-id"}
	event.Request = &sentry.Request{Data: "private-message-body"}
	event.Breadcrumbs = []*sentry.Breadcrumb{{Message: "private-breadcrumb"}}
	scrubbed := sentryScrubber(cfg)(event, nil)
	data, err := json.Marshal(scrubbed)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{cfg.APIKey, cfg.SpotifyClientSecret, cfg.MongoPass, cfg.LastFMAPIKey, "dynamic-access-token", "private-user-id", "private-message-body", "private-breadcrumb"} {
		if strings.Contains(string(data), secret) {
			t.Errorf("sensitive data was not scrubbed: %q", secret)
		}
	}
	if scrubbed.Exception[0].Stacktrace.Frames[0].Lineno != 42 {
		t.Fatal("scrubbing removed useful stack locations")
	}
}
