package main

import (
	"fmt"
	"os"
	"siagars/backend/internal/app"
	"strings"
)

func main() {
	if len(os.Args) < 2 || (len(os.Args) > 2 && os.Args[1] != "curate") {
		fmt.Fprintln(os.Stderr, "Perintah gagal: usage: go run ./cmd/manage "+strings.Join(app.Commands, "|"))
		os.Exit(1)
	}
	if e := app.Manage(os.Args[1]); e != nil {
		fmt.Fprintln(os.Stderr, "Perintah gagal:", e)
		os.Exit(1)
	}
}
