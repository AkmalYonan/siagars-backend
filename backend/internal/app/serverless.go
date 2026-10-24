package app

import (
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"siagars/backend/internal/config"
	api "siagars/backend/internal/http"
	"siagars/backend/internal/repository"
)

var (
	handlerMu sync.Mutex
	handler   http.Handler
)

// ServeHTTP serves one request for serverless runtimes (Vercel). The router and the MongoDB
// client are built on first use and reused by warm invocations; a failed init is retried.
func ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, e := sharedHandler()
	if e != nil {
		slog.Error("serverless init failed", "error", e.Error())
		http.Error(w, `{"error":"service_unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	h.ServeHTTP(w, r)
}

func sharedHandler() (http.Handler, error) {
	handlerMu.Lock()
	defer handlerMu.Unlock()
	if handler != nil {
		return handler, nil
	}
	cfg, e := config.Load()
	if e != nil {
		return nil, e
	}
	repo, e := repository.Open(cfg.MongoURI, cfg.MongoDatabase)
	if e != nil {
		return nil, errors.New("invalid database configuration")
	}
	handler = api.New(repo, cfg, repo.Ready)
	return handler, nil
}
