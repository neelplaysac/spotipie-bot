package telegram

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/getsentry/sentry-go"

	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/observability"
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
	hub := sentry.CurrentHub()
	oldClient := hub.Client()
	hub.PushScope()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	logger, err := observability.Init(&config.Config{SentryDSN: "https://public@example.invalid/1", SentryTracesSampleRate: strconv.FormatFloat(traceRate, 'f', -1, 64)}, logger)
	if err != nil {
		t.Fatal(err)
	}
	sdkClient := hub.Client()
	client, err := sentry.NewClient(sentry.ClientOptions{Dsn: "https://public@example.invalid/1", Transport: recorder, AttachStacktrace: true, EnableTracing: traceRate > 0, TracesSampleRate: traceRate})
	if err != nil {
		t.Fatal(err)
	}
	hub.BindClient(client)
	t.Cleanup(func() { client.Close(); sdkClient.Close(); hub.PopScope(); hub.BindClient(oldClient) })
	return recorder, logger
}

func TestMonitoringScopesAndTransactionNames(t *testing.T) {
	r, logger := setupMonitoringTest(t, 1)
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
	r, logger := setupMonitoringTest(t, 1)
	d := ext.NewDispatcher(&ext.DispatcherOpts{Processor: monitoringProcessor{}, Panic: New(&config.Config{}, nil, nil, logger, "testbot").panicHandler})
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
	r, logger := setupMonitoringTest(t, 0)
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
