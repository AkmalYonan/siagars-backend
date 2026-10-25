package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"siagars/backend/internal/config"
	"siagars/backend/internal/integrations/satusehat"
	"siagars/backend/internal/repository"
	"siagars/backend/internal/service"
	"siagars/backend/seed"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Commands lists the management commands accepted by Manage.
var Commands = []string{"migrate", "seed", "operator", "sync-satusehat", "curate", "dummy-facilities"}

// MinOperatorPassword is 12 bytes everywhere except local development, where 8 keeps throwaway
// test accounts practical. Unset APP_ENV is treated as development, matching config.Load.
func MinOperatorPassword(appEnv string) int {
	if appEnv == "" || appEnv == "development" {
		return 8
	}
	return 12
}

// Manage runs one management command against MongoDB. It never starts the API.
func Manage(command string) error {
	if command == "curate" {
		// Dry-run is offline; only --apply opens the database (see seed.RunCurate).
		return seed.RunCurate(os.Args[2:], os.Stdout)
	}
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		return fmt.Errorf("MONGODB_URI required")
	}
	database := os.Getenv("MONGODB_DATABASE")
	if database == "" {
		database = "siagars"
	}
	p, e := repository.Open(uri, database)
	if e != nil {
		return fmt.Errorf("invalid database configuration")
	}
	defer p.Close(context.Background())
	if command == "sync-satusehat" {
		return syncSATUSEHAT(p)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	switch command {
	case "migrate":
		if e = p.Migrate(ctx); e != nil {
			return fmt.Errorf("migration failed; check MongoDB availability and database permissions: %w", e)
		}
	case "seed":
		if os.Getenv("DEMO_MODE") == "false" {
			return fmt.Errorf("seed requires DEMO_MODE=true")
		}
		if e = seed.Apply(ctx, p); e != nil {
			return fmt.Errorf("seed failed; run migrate first on the intended demo database: %w", e)
		}
	case "dummy-facilities":
		out, e := seed.ApplyDummyFacilities(ctx, p.DB, time.Now())
		if e != nil {
			return fmt.Errorf("dummy-facilities failed: %w", e)
		}
		fmt.Printf("dummy-facilities (data_origin=simulation): %d hospitals, %d service items, %d bed items, %d BPJS values in total (new or earlier); existing data was not overwritten\n", out.Hospitals, out.ServicesAdded, out.BedsAdded, out.BPJSSet)
	case "operator":
		email := strings.ToLower(strings.TrimSpace(os.Getenv("OPERATOR_EMAIL")))
		password := os.Getenv("OPERATOR_PASSWORD")
		hospitals := []string{}
		seen := map[string]bool{}
		for _, hospital := range strings.Split(os.Getenv("OPERATOR_HOSPITAL_IDS"), ",") {
			if hospital = strings.TrimSpace(hospital); hospital != "" && !seen[hospital] {
				seen[hospital] = true
				hospitals = append(hospitals, hospital)
			}
		}
		address, err := mail.ParseAddress(email)
		if err != nil || address.Address != email || len(email) > 254 || len(password) < MinOperatorPassword(os.Getenv("APP_ENV")) || len(password) > 72 || len(hospitals) == 0 {
			return fmt.Errorf("set OPERATOR_EMAIL, OPERATOR_PASSWORD (12..72 bytes; 8..72 only when APP_ENV=development), OPERATOR_HOSPITAL_IDS (comma-separated UUIDs)")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			return err
		}
		var b [16]byte
		if _, err = rand.Read(b[:]); err != nil {
			return err
		}
		b[6] = (b[6] & 15) | 64
		b[8] = (b[8] & 63) | 128
		s := hex.EncodeToString(b[:])
		id := s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
		switch err = p.CreateOperator(ctx, id, email, string(hash), hospitals); {
		case errors.Is(err, repository.ErrConflict):
			return fmt.Errorf("operator creation failed; email must be new")
		case errors.Is(err, repository.ErrNotFound):
			return fmt.Errorf("hospital assignment failed; all hospital UUIDs must exist")
		case err != nil:
			return fmt.Errorf("operator transaction failed")
		}
	default:
		return fmt.Errorf("unknown command")
	}
	fmt.Println("Selesai:", command)
	return nil
}

// syncSATUSEHAT imports MSI Sandbox hospitals. It never migrates, seeds or deletes, and
// prints only counts and error categories.
func syncSATUSEHAT(p *repository.Mongo) error {
	cfg, e := config.LoadSATUSEHAT()
	if e != nil {
		return e
	}
	if !cfg.Enabled {
		return fmt.Errorf("sync-satusehat requires SATUSEHAT_ENABLED=true")
	}
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(root, 30*time.Minute)
	defer cancel()
	if applied, e := p.Applied(ctx, "002_satusehat_msi.json"); e != nil || !applied {
		return fmt.Errorf("database is not migrated to 002_satusehat_msi.json; run migrate first")
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	tokens := satusehat.NewTokenProvider(satusehat.SandboxBaseURL, satusehat.Credentials{ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret}, client)
	summary, e := service.SyncSATUSEHAT(ctx, satusehat.NewMSIClient(satusehat.SandboxBaseURL, tokens, client), p, cfg.KabKota, cfg.PageLimit, time.Now)
	fmt.Printf("sync-satusehat env=%s scopes=%d pages=%d fetched=%d created=%d updated=%d unchanged=%d skipped=%d failed=%d duration=%s\n",
		summary.Environment, summary.Scopes, summary.Pages, summary.Fetched, summary.Created, summary.Updated, summary.Unchanged, summary.Skipped, summary.Failed, summary.FinishedAt.Sub(summary.StartedAt).Round(time.Millisecond))
	reasons := make([]string, 0, len(summary.SkipReasons))
	for reason := range summary.SkipReasons {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		fmt.Printf("  skipped %s=%d\n", reason, summary.SkipReasons[reason])
	}
	if e != nil {
		if summary.FailedPage > 0 {
			return fmt.Errorf("sync stopped at kabkota %q page %d (%s); previously imported data is unchanged", summary.FailedScope, summary.FailedPage, summary.ErrorCategory)
		}
		return fmt.Errorf("sync incomplete (%s)", summary.ErrorCategory)
	}
	return nil
}
