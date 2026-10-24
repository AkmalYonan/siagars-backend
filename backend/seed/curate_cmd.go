package seed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"siagars/backend/internal/repository"
	"strings"
	"time"
)

// RunCurate implements `curate <file> [--apply]`. Without --apply it only validates and reports; it
// does not open a database connection. With --apply it writes through ApplyCuration to the database
// named by MONGODB_URI/MONGODB_DATABASE. CURATE_FILE and CURATE_APPLY=true are fallbacks for callers
// that cannot pass extra arguments.
func RunCurate(args []string, out io.Writer) error {
	file, apply := "", os.Getenv("CURATE_APPLY") == "true"
	for _, a := range args {
		switch {
		case a == "--apply":
			apply = true
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf("opsi tidak dikenal: %s (usage: curate <file> [--apply])", a)
		case file != "":
			return errors.New("hanya satu file per perintah")
		default:
			file = a
		}
	}
	if file == "" {
		file = os.Getenv("CURATE_FILE")
	}
	if file == "" {
		return errors.New("usage: curate <file.json|file.csv> [--apply]")
	}
	f, e := os.Open(file)
	if e != nil {
		return fmt.Errorf("file tidak dapat dibuka: %w", e)
	}
	defer f.Close()
	data, e := readAll(f)
	if e != nil {
		return e
	}
	records, e := ParseCuration(file, data)
	if e != nil {
		return e
	}
	now := time.Now().UTC()
	valid, issues := ValidateCuration(records, now)
	mode := "DRY-RUN (tidak ada yang ditulis)"
	if apply {
		mode = "APPLY"
	}
	fmt.Fprintf(out, "curate %s mode=%s\n", file, mode)
	rejected := len(records) - len(valid)
	for _, i := range issues {
		fmt.Fprintln(out, "  "+i.String())
	}
	for _, c := range valid {
		fmt.Fprintf(out, "  OK   %s/%s bpjs=%s services=%d beds=%d\n", c.SourceNamespace, c.ExternalID, bpjsLabel(c), len(c.Services), len(c.Beds))
	}
	fmt.Fprintf(out, "ringkasan: records=%d valid=%d ditolak=%d placeholder=%t verified_at=tidak-pernah-diisi\n", len(records), len(valid), rejected, HasPlaceholder(issues))
	if rejected > 0 {
		return fmt.Errorf("%d record ditolak validator; perbaiki file dan ulangi", rejected)
	}
	if !apply {
		fmt.Fprintln(out, "dry-run selesai; tambahkan --apply hanya setelah checklist verifikasi manusia (data/curation/README.md)")
		return nil
	}
	if HasPlaceholder(issues) {
		return errors.New("apply dibatalkan: ada source_url placeholder")
	}
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		return errors.New("MONGODB_URI required")
	}
	database := os.Getenv("MONGODB_DATABASE")
	if database == "" {
		database = "siagars"
	}
	p, e := repository.Open(uri, database)
	if e != nil {
		return errors.New("invalid database configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	defer p.Close(context.Background())
	if applied, e := p.Applied(ctx, "002_satusehat_msi.json"); e != nil || !applied {
		return errors.New("database is not migrated to 002_satusehat_msi.json; run migrate first")
	}
	outcomes, e := ApplyCuration(ctx, p.DB, valid, now)
	failed := 0
	for _, o := range outcomes {
		fmt.Fprintln(out, "  "+o.String())
		if !o.Found || o.NameMismatch || len(o.Missing) > 0 {
			failed++
		}
	}
	if e != nil {
		return fmt.Errorf("apply terhenti: %w", e)
	}
	if failed > 0 {
		return fmt.Errorf("%d record tidak seluruhnya diterapkan", failed)
	}
	return nil
}

func bpjsLabel(c CuratedHospital) string {
	if c.BPJS == nil {
		return "null"
	}
	return fmt.Sprint(*c.BPJS)
}

func (o CurateOutcome) String() string {
	switch {
	case !o.Found:
		return o.ExternalID + ": TIDAK DITEMUKAN (jalankan sync-satusehat dahulu); tidak ada yang ditulis"
	case o.NameMismatch:
		return o.ExternalID + ": nama tidak cocok dengan RS di database; tidak ada yang ditulis"
	}
	s := fmt.Sprintf("%s: bpjs set=%t dipertahankan=%t; layanan +%v tetap%v; bed +%v tetap%v", o.ExternalID, o.BPJSSet, o.BPJSKept, o.ServicesAdded, o.ServicesKept, o.BedsAdded, o.BedsKept)
	if len(o.Missing) > 0 {
		s += fmt.Sprintf("; slug layanan tidak ada di master: %v", o.Missing)
	}
	return s
}
