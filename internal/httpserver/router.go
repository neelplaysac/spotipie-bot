// Package httpserver adapts the Spotify callback workflow to Gin. It does not
// start Telegram polling or exchange Spotify tokens.
package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gin-gonic/gin"
	"github.com/neelplaysac/spotipie-go/internal/service"
)

type Authorization interface {
	Complete(ctx context.Context, state, code string) (string, error)
}

type HealthChecker interface {
	Ping(context.Context) error
}

func NewRouter(auth Authorization, health HealthChecker, botURL string, logger *slog.Logger) *gin.Engine {
	router := gin.New()
	// No client IP is used for authentication or logging. Ignore forwarded IPs.
	router.ForwardedByClientIP = false
	_ = router.SetTrustedProxies(nil)
	router.Use(requestMiddleware(logger))

	home := func(c *gin.Context) { c.String(http.StatusOK, "Welcome to spotipie authserver") }
	router.GET("/", home)
	router.HEAD("/", home)
	live := func(c *gin.Context) { c.Status(http.StatusOK) }
	router.GET("/healthz", live)
	router.HEAD("/healthz", live)
	ready := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := health.Ping(ctx); err != nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusOK)
	}
	router.GET("/readyz", ready)
	router.HEAD("/readyz", ready)
	callback := func(c *gin.Context) {
		if c.Query("error") != "" {
			c.String(http.StatusOK, "Cancelled. Return to Telegram and use /register to try again.")
			return
		}
		// Reject duplicates rather than letting different parsers pick values.
		query, err := parseCallback(c.Request)
		if err != nil {
			c.String(http.StatusBadRequest, "Invalid authorization. Return to Telegram and use /register again.")
			return
		}
		id, err := auth.Complete(c.Request.Context(), query.state, query.code)
		if errors.Is(err, service.ErrInvalidAuthorization) {
			c.String(http.StatusBadRequest, "Authorization expired or already used. Return to Telegram and use /register again.")
			return
		}
		if err != nil {
			logger.ErrorContext(c.Request.Context(), "failed to save Spotify authorization", "err", err, "component", "authserver")
			c.String(http.StatusInternalServerError, "Unable to complete authorization. Please try /register again shortly.")
			return
		}
		c.Redirect(http.StatusFound, botURL+id)
	}
	router.GET("/callback", callback)
	router.GET("/callback/", callback)
	// Link previews and HEAD probes must never consume a session.
	for _, path := range []string{"/callback", "/callback/"} {
		router.HEAD(path, func(c *gin.Context) { c.Status(http.StatusMethodNotAllowed) })
	}
	return router
}

type callbackQuery struct{ state, code string }

func parseCallback(r *http.Request) (callbackQuery, error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query["state"]) != 1 || len(query["code"]) != 1 || query.Get("state") == "" || query.Get("code") == "" {
		return callbackQuery{}, service.ErrInvalidAuthorization
	}
	return callbackQuery{state: query.Get("state"), code: query.Get("code")}, nil
}

func requestMiddleware(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		var nonce [16]byte
		_, _ = rand.Read(nonce[:])
		requestID := hex.EncodeToString(nonce[:])
		c.Header("X-Request-ID", requestID)
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		hub := sentry.CurrentHub().Clone()
		hub.Scope().SetTag("component", "authserver")
		ctx := sentry.SetHubOnContext(c.Request.Context(), hub)
		// Route patterns are fixed; never name transactions using a raw URL.
		span := sentry.StartTransaction(ctx, c.Request.Method+" "+c.FullPath(), sentry.WithOpName("http.server"))
		c.Request = c.Request.WithContext(span.Context())
		defer span.Finish()
		defer func() {
			if r := recover(); r != nil {
				// Gin's default recovery dumps the request, including OAuth codes.
				logger.ErrorContext(c.Request.Context(), "panic handling HTTP request", "panic", r, "component", "authserver", "request_id", requestID)
				c.AbortWithStatus(http.StatusInternalServerError)
			}
			span.Status = sentry.HTTPtoSpanStatus(c.Writer.Status())
			logger.InfoContext(c.Request.Context(), "HTTP request", "request_id", requestID,
				"method", c.Request.Method, "handler", c.FullPath(), "status", c.Writer.Status(), "duration", time.Since(start))
		}()
		c.Next()
	}
}
