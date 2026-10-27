package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/neelplaysac/spotipie-go/internal/config"
	"github.com/neelplaysac/spotipie-go/internal/httpserver"
	"github.com/neelplaysac/spotipie-go/internal/observability"
	"github.com/neelplaysac/spotipie-go/internal/service"
	mongostore "github.com/neelplaysac/spotipie-go/internal/storage/mongo"
)

func RunAuthServer() int {
	cfg := config.Load()
	logger, _ := observability.NewLogger()
	logger, err := observability.Init(cfg, logger)
	slog.SetDefault(logger)
	if err != nil {
		logger.Error("failed to initialize monitoring", "err", err)
		return 1
	}
	defer observability.Flush(logger)
	if err := cfg.ValidateAuthServer(); err != nil {
		logger.Error("invalid auth server configuration", "err", err)
		return 1
	}
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	db, err := mongostore.New(cfg, logger)
	if err != nil {
		logger.Error("failed to connect to database", "err", err)
		return 1
	}
	defer db.Close()
	botURL, _ := cfg.TelegramReturnURL() // Validated before opening resources.
	gin.SetMode(gin.ReleaseMode)
	router := httpserver.NewRouter(service.NewAuthorization(db), db, botURL, logger)
	server := httpserver.New(":"+cfg.HTTPPort, router)
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		logger.Error("failed to listen for HTTP requests", "err", err)
		return 1
	}
	logger.Info("Spotify auth server listening", "port", cfg.HTTPPort, "db", cfg.DatabaseName())
	if err := httpserver.Serve(ctx, server, listener); err != nil {
		logger.Error("HTTP server stopped with an error", "err", err)
		return 1
	}
	logger.Info("auth server shutdown complete")
	return 0
}

// CheckAuthServer uses only local HTTP and needs no SDK or database credentials.
func CheckAuthServer() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "6969"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/readyz")
	if err != nil {
		return fmt.Errorf("auth server health check failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth server is not ready (HTTP %d)", resp.StatusCode)
	}
	return nil
}
