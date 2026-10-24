// Command backend is the local development shortcut:
//
//	go run .                 load .env and start the API
//	go run . migrate         load .env and run a management command (migrate|seed|operator|sync-satusehat)
//
// cmd/api and cmd/manage remain the production entrypoints; they read only the process environment.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"siagars/backend/internal/app"
	"siagars/backend/internal/config"
	"slices"
	"strings"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	loaded, e := config.LoadEnvFile(".env")
	switch {
	case errors.Is(e, fs.ErrNotExist):
		slog.Info(".env not found; using process environment only")
	case e != nil:
		slog.Error("startup failed", "reason", e.Error())
		os.Exit(1)
	default:
		slog.Info(".env loaded", "variables", loaded)
	}
	switch {
	case len(os.Args) == 1:
		if e = app.RunAPI(); e != nil {
			slog.Error("startup failed", "reason", e.Error())
			os.Exit(1)
		}
	case len(os.Args) >= 2 && slices.Contains(app.Commands, os.Args[1]) && (len(os.Args) == 2 || os.Args[1] == "curate"):
		if e = app.Manage(os.Args[1]); e != nil {
			fmt.Fprintln(os.Stderr, "Perintah gagal:", e)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: go run . ["+strings.Join(app.Commands, "|")+"]")
		os.Exit(2)
	}
}
