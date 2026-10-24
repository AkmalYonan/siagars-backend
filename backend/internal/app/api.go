// Package app holds the process entrypoints shared by cmd/api, cmd/manage and the root main package.
package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"siagars/backend/internal/config"
	api "siagars/backend/internal/http"
	"siagars/backend/internal/repository"
	"syscall"
	"time"
)

// RunAPI starts the HTTP server and blocks until it fails or receives SIGINT/SIGTERM.
func RunAPI() error {
	cfg, e := config.Load()
	if e != nil {
		return e
	}
	repo, e := repository.Open(cfg.MongoURI, cfg.MongoDatabase)
	if e != nil {
		return errors.New("invalid database configuration")
	}
	defer repo.Close(context.Background())
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: ":" + cfg.Port, Handler: api.New(repo, cfg, repo.Ready), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	result := make(chan error, 1)
	go func() {
		slog.Info("api listening", "port", cfg.Port, "demo_mode", true)
		result <- server.ListenAndServe()
	}()
	select {
	case e = <-result:
		if errors.Is(e, http.ErrServerClosed) {
			return nil
		}
		return e
	case <-root.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
