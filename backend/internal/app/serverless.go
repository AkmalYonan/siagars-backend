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
		// Same Error schema as every other response, so clients parse one shape.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error":{"code":"service_unavailable","message":"Layanan sedang tidak tersedia; coba lagi sebentar"},"request_id":""}` + "\n"))
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
	if e = enableBPJSHistory(repo, cfg); e != nil {
		return nil, e
	}
	handler = api.New(store{repo}, cfg, repo.Ready)
	return handler, nil
}
