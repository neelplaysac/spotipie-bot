package main

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

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/getsentry/sentry-go"
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

func setupMonitoringTest(t *testing.T, traceRate float64) *eventRecorder {
	t.Helper()
	recorder := &eventRecorder{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:                   "https://public@example.invalid/1",
		Transport:             recorder,
		AttachStacktrace:      true,
		EnableTracing:         traceRate > 0,
		TracesSampleRate:      traceRate,
		BeforeSend:            sentryScrubber(&Config{}),
		BeforeSendTransaction: sentryScrubber(&Config{}),
	})
	if err != nil {
		t.Fatal(err)
	}
	hub := sentry.CurrentHub()
	oldClient, oldLogger, oldDefault := hub.Client(), logger, slog.Default()
	hub.PushScope()
	hub.BindClient(client)
	logger = slog.New(&sentryLogHandler{next: slog.NewTextHandler(io.Discard, nil)})
	slog.SetDefault(logger)
	t.Cleanup(func() {
		client.Close()
		hub.PopScope()
		hub.BindClient(oldClient)
		logger = oldLogger
		slog.SetDefault(oldDefault)
	})
	return recorder
}

func TestMonitoringConfiguration(t *testing.T) {
	for _, rate := range []string{"-0.1", "1.1", "NaN", "Inf", "invalid"} {
		t.Run(rate, func(t *testing.T) {
			err := initMonitoring(&Config{SentryDSN: "https://public@example.invalid/1", SentryTracesSampleRate: rate})
			if err == nil {
				t.Fatal("invalid trace sample rate was accepted")
			}
		})
	}
	if err := initMonitoring(&Config{}); err != nil {
		t.Fatalf("missing DSN must leave monitoring optional: %v", err)
	}
	if err := initMonitoring(&Config{SentryDSN: "not-a-dsn", SentryTracesSampleRate: "0.01"}); err == nil {
		t.Fatal("invalid DSN was accepted")
	}
}

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
	cfg := loadConfig()
	if cfg.SentryDSN != "https://public@example.invalid/1" || cfg.SentryEnvironment != "staging" || cfg.SentryRelease != "spotipie-go@test" || cfg.SentryTracesSampleRate != "0" {
		t.Fatal("monitoring environment was not loaded correctly")
	}
}

func TestMonitoringDisabledDoesNotTraceOrFlush(t *testing.T) {
	hub := sentry.CurrentHub()
	oldClient, oldLogger := hub.Client(), logger
	hub.PushScope()
	hub.BindClient(nil)
	var output strings.Builder
	logger = slog.New(slog.NewTextHandler(&output, nil))
	t.Cleanup(func() {
		hub.PopScope()
		hub.BindClient(oldClient)
		logger = oldLogger
	})
	if err := initMonitoring(&Config{}); err != nil {
		t.Fatal(err)
	}
	d := ext.NewDispatcher(&ext.DispatcherOpts{Processor: monitoringProcessor{}})
	d.AddHandler(handlers.NewMessage(nil, func(_ *gotgbot.Bot, c *ext.Context) error {
		if c.Data[monitoringContextKey] != nil {
			t.Error("disabled monitoring created an update scope")
		}
		return nil
	}))
	if err := d.ProcessUpdate(&gotgbot.Bot{}, &gotgbot.Update{Message: &gotgbot.Message{Text: "hello"}}, nil); err != nil {
		t.Fatal(err)
	}
	flushMonitoring()
	if strings.Contains(output.String(), "timed out") {
		t.Fatal("disabled SDK produced a false flush timeout warning")
	}
}

func TestSentryLogsCaptureErrorsAndKeepStdout(t *testing.T) {
	r := setupMonitoringTest(t, 1)
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
	flushMonitoring()
	if !r.flushed {
		t.Fatal("flush did not reach the SDK transport")
	}
}

func TestSentryScrubsCredentials(t *testing.T) {
	cfg := &Config{APIKey: "123456789:abcdefghijklmnopqrstuvwxyzABCDEFG", SpotifyClientSecret: "test-spotify-secret", MongoPass: "test-p@ss:/word", LastFMAPIKey: "test-lastfm-key"}
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

func TestMonitoringScopesAndTransactionNames(t *testing.T) {
	r := setupMonitoringTest(t, 1)
	d := ext.NewDispatcher(&ext.DispatcherOpts{Processor: monitoringProcessor{}})
	d.AddHandler(handlers.NewCommand("start", func(_ *gotgbot.Bot, c *ext.Context) error {
		hub := sentry.GetHubFromContext(updateContext(c))
		hub.Scope().SetTag("scope_test", c.Message.Text)
		logger.ErrorContext(updateContext(c), "handler failed", "err", errors.New("test failure"))
		return nil
	}))
	b := &gotgbot.Bot{}
	var wg sync.WaitGroup
	for _, text := range []string{"/start first-oauth-code", "/start second-oauth-code"} {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			update := &gotgbot.Update{Message: &gotgbot.Message{Chat: gotgbot.Chat{Id: 42, Type: "private"}, From: &gotgbot.User{Id: 42}, Text: text}}
			if err := d.ProcessUpdate(b, update, nil); err != nil {
				t.Error(err)
			}
		}(text)
	}
	wg.Wait()
	var errorsSeen, transactionsSeen int
	for _, event := range r.events {
		if event.Transaction != "telegram.start" {
			t.Errorf("unsafe or missing transaction name: %q", event.Transaction)
		}
		if _, ok := event.Contexts["trace"]; !ok {
			t.Error("event is not correlated with its update trace")
		}
		if event.Type == "transaction" {
			transactionsSeen++
		} else {
			errorsSeen++
		}
	}
	if errorsSeen != 2 || transactionsSeen != 2 {
		t.Fatalf("got %d errors and %d transactions", errorsSeen, transactionsSeen)
	}
	// No update-local tags may escape onto the shared scope.
	sentry.CaptureMessage("outside update")
	if _, ok := r.events[len(r.events)-1].Tags["scope_test"]; ok {
		t.Fatal("concurrent update scope leaked to the global hub")
	}
	unknown := ext.NewContext(b, &gotgbot.Update{Message: &gotgbot.Message{Text: "/private-secret arbitrary-content"}}, nil)
	if updateOperation(unknown) != "telegram.message" {
		t.Fatal("unknown command contents leaked into transaction names")
	}
}

func TestMonitoringCapturesRecoveredDispatcherPanic(t *testing.T) {
	r := setupMonitoringTest(t, 1)
	d := ext.NewDispatcher(&ext.DispatcherOpts{Processor: monitoringProcessor{}, Panic: panicHandler})
	d.AddHandler(handlers.NewMessage(nil, func(_ *gotgbot.Bot, _ *ext.Context) error {
		panic("test handler panic")
	}))
	update := &gotgbot.Update{Message: &gotgbot.Message{Chat: gotgbot.Chat{Id: 42, Type: "private"}, From: &gotgbot.User{Id: 42}, Text: "private-message"}}
	if err := d.ProcessUpdate(&gotgbot.Bot{}, update, nil); err != nil {
		t.Fatal(err)
	}
	for _, event := range r.events {
		if event.Level == sentry.LevelFatal && len(event.Exception) > 0 && event.Exception[0].Value == "test handler panic" {
			if event.Exception[0].Stacktrace == nil {
				t.Fatal("panic stack trace missing")
			}
			return
		}
	}
	t.Fatal("recovered handler panic was not reported")
}

func TestMonitoringReportsErrorsWithTracingDisabled(t *testing.T) {
	r := setupMonitoringTest(t, 0)
	d := ext.NewDispatcher(&ext.DispatcherOpts{Processor: monitoringProcessor{}})
	d.AddHandler(handlers.NewMessage(nil, func(_ *gotgbot.Bot, c *ext.Context) error {
		logger.ErrorContext(updateContext(c), "handler failed", "err", errors.New("test error"))
		return nil
	}))
	update := &gotgbot.Update{Message: &gotgbot.Message{Text: "private-message"}}
	if err := d.ProcessUpdate(&gotgbot.Bot{}, update, nil); err != nil {
		t.Fatal(err)
	}
	if len(r.events) != 1 || r.events[0].Type == "transaction" || len(r.events[0].Exception) == 0 {
		t.Fatal("disabling tracing must retain error reports without transactions")
	}
}

type blockedDispatcher struct{ release chan struct{} }

func (d blockedDispatcher) Start(*gotgbot.Bot, <-chan json.RawMessage) {}
func (d blockedDispatcher) Stop()                                      { <-d.release }

func TestMonitoringShutdownIsBounded(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	updater := ext.NewUpdater(blockedDispatcher{release: release}, nil)
	if err := stopUpdater(updater, 10*time.Millisecond); err == nil {
		t.Fatal("a stalled dispatcher must time out so monitoring can flush")
	}
}
