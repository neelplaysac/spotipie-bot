package main

import (
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"
)

var logLevel = new(slog.LevelVar)

var logger *slog.Logger

var botTokenRe = regexp.MustCompile(`bot[0-9]{5,}:[A-Za-z0-9_-]{20,}`)

var querySecretRe = regexp.MustCompile(`(?i)\b(api_key|api_token|access_token|refresh_token|client_secret|client_id|code|auth)=([^&\s"']+)`)

var bearerRe = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)

var bareTokenRe = regexp.MustCompile(`\b[0-9]{6,}:[A-Za-z0-9_-]{25,}\b`)

var uriCredsRe = regexp.MustCompile(`//[^/@\s:]+:[^/@\s]+@`)

// redactSecrets is a defence-in-depth net for secrets that reach the logger.
// The primary protection is not putting secrets into errors (or any log
// attributes) in the first place; this only scrubs what slips through.
func redactSecrets(s string) string {
	s = botTokenRe.ReplaceAllString(s, "bot<redacted>")
	s = querySecretRe.ReplaceAllString(s, "$1=<redacted>")
	s = bearerRe.ReplaceAllString(s, "Bearer <redacted>")
	s = bareTokenRe.ReplaceAllString(s, "<redacted>")
	s = uriCredsRe.ReplaceAllString(s, "//<redacted>@")
	return s
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	switch v := a.Value.Any().(type) {
	case error:
		a.Value = slog.StringValue(redactSecrets(v.Error()))
		return a
	case string:
		a.Value = slog.StringValue(redactSecrets(v))
		return a
	default:
		return a
	}
}

func logDuration(start time.Time) slog.Attr {
	return slog.Duration("took", time.Since(start))
}

func init() {
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		logLevel.Set(slog.LevelDebug)
	case "info":
		logLevel.Set(slog.LevelInfo)
	case "warn":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}

	opts := &slog.HandlerOptions{AddSource: true, Level: logLevel, ReplaceAttr: redactAttr}

	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, opts))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	slog.SetDefault(logger)
}
