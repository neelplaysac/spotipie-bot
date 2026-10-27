package observability

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"

	"github.com/neelplaysac/spotipie-go/internal/config"
)

func Init(cfg *config.Config, logger *slog.Logger) (*slog.Logger, error) {
	// Do not initialize the SDK with an empty DSN: error monitoring is opt-in.
	if cfg.SentryDSN == "" {
		logger.Info("GlitchTip monitoring disabled; SENTRY_DSN is not set")
		return logger, nil
	}

	rate, err := strconv.ParseFloat(cfg.SentryTracesSampleRate, 64)
	if err != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
		return logger, fmt.Errorf("SENTRY_TRACES_SAMPLE_RATE must be a number between 0 and 1")
	}

	scrub := sentryScrubber(cfg)
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:                   cfg.SentryDSN,
		Environment:           cfg.SentryEnvironment,
		Release:               cfg.SentryRelease,
		ServerName:            "spotipie-bot",
		AttachStacktrace:      true,
		SendDefaultPII:        false,
		EnableTracing:         rate > 0,
		TracesSampleRate:      rate,
		BeforeSend:            scrub,
		BeforeSendTransaction: scrub,
	}); err != nil {
		// SDK configuration errors may contain the DSN. Keep it out of logs.
		return logger, fmt.Errorf("invalid SENTRY_DSN; copy the Go project DSN from GlitchTip")
	}

	logger = slog.New(&sentryLogHandler{next: logger.Handler()})
	logger.Info("GlitchTip monitoring enabled", "environment", cfg.SentryEnvironment, "release", cfg.SentryRelease, "traces_sample_rate", rate)
	return logger, nil
}

func Flush(logger *slog.Logger) {
	if !sentry.CurrentHub().Client().IsEnabled() {
		return
	}
	if !sentry.Flush(2 * time.Second) {
		logger.Warn("timed out flushing GlitchTip events")
	}
}

type sentryLogHandler struct {
	next  slog.Handler
	attrs []slog.Attr
}

func (h *sentryLogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= slog.LevelError || h.next.Enabled(ctx, level)
}

func (h *sentryLogHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= slog.LevelError {
		hub := sentry.GetHubFromContext(ctx)
		if hub == nil {
			hub = sentry.CurrentHub()
		}
		event := sentry.NewEvent()
		event.Level = sentry.LevelError
		event.Logger = "spotipie"
		event.Message = record.Message
		event.Timestamp = record.Time
		if span := sentry.SpanFromContext(ctx); span != nil {
			span.Status = sentry.SpanStatusInternalError
			event.Transaction = span.Name
		}
		var panicValue any
		visit := func(attr slog.Attr) bool {
			value := attr.Value.Resolve().Any()
			switch attr.Key {
			case "err", "error":
				if err, ok := value.(error); ok {
					event.SetException(err, -1)
				}
			case "panic":
				panicValue = value
			case "component", "handler", "method", "status", "kind":
				event.Tags[attr.Key] = fmt.Sprint(value)
			}
			return true
		}
		for _, attr := range h.attrs {
			visit(attr)
		}
		record.Attrs(visit)
		if panicValue != nil {
			panicErr, ok := panicValue.(error)
			if !ok {
				panicErr = fmt.Errorf("%v", panicValue)
			}
			event.Level = sentry.LevelFatal
			event.SetException(panicErr, -1)
		}
		hub.CaptureEventWithHint(event, &sentry.EventHint{Context: ctx, RecoveredException: panicValue})
	}
	if h.next.Enabled(ctx, record.Level) {
		return h.next.Handle(ctx, record)
	}
	return nil
}

func (h *sentryLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &sentryLogHandler{
		next:  h.next.WithAttrs(attrs),
		attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...),
	}
}

func (h *sentryLogHandler) WithGroup(name string) slog.Handler {
	return &sentryLogHandler{next: h.next.WithGroup(name), attrs: h.attrs}
}

func sentryScrubber(cfg *config.Config) func(*sentry.Event, *sentry.EventHint) *sentry.Event {
	var replacements []string
	for _, secret := range []string{cfg.APIKey, cfg.SpotifyClientID, cfg.SpotifyClientSecret, cfg.MongoPass, cfg.LastFMAPIKey, cfg.SentryDSN} {
		if secret != "" {
			for _, value := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret)} {
				replacements = append(replacements, value, "<redacted>")
			}
		}
	}
	replacer := strings.NewReplacer(replacements...)
	redact := func(value string) string { return redactSecrets(replacer.Replace(value)) }
	return func(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
		event.Message = redact(event.Message)
		for i := range event.Exception {
			event.Exception[i].Value = redact(event.Exception[i].Value)
			stripStackData(event.Exception[i].Stacktrace)
		}
		for i := range event.Threads {
			stripStackData(event.Threads[i].Stacktrace)
		}
		for key, value := range event.Tags {
			event.Tags[key] = redact(value)
		}
		// Operational tags and stack traces are enough to diagnose bot errors.
		event.Request = nil
		event.User = sentry.User{}
		event.Breadcrumbs = nil
		event.Attachments = nil
		return event
	}
}

func stripStackData(stack *sentry.Stacktrace) {
	if stack == nil {
		return
	}
	for i := range stack.Frames {
		stack.Frames[i].Vars = nil
		stack.Frames[i].PreContext = nil
		stack.Frames[i].ContextLine = ""
		stack.Frames[i].PostContext = nil
	}
}
