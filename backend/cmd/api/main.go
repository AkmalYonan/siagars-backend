package main

import (
	"log/slog"
	"os"
	"siagars/backend/internal/app"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if e := app.RunAPI(); e != nil {
		slog.Error("startup failed", "reason", e.Error())
		os.Exit(1)
	}
}
