package seed

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"siagars/backend/internal/domain"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DefaultCurationNamespace is the namespace of hospitals imported from MSI Sandbox.
const DefaultCurationNamespace = "satusehat:msi:sandbox"

// MaxCurationFile bounds the input file so a wrong path cannot load an arbitrary blob.
const MaxCurationFile = 1 << 20

var (
	curationSlug     = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	curationBedKinds = []string{"icu", "nicu", "picu", "general"}
	curationSvcSlugs = []string{"icu", "nicu", "picu", "igd-24h", "ambulance", "rawat-inap"}
)

// CurationFile is the JSON form of a curation file. Unknown fields are rejected, so
// data_origin/verified_at can never ride along silently.
type CurationFile struct {
	SchemaVersion int              `json:"schema_version"`
	Hospitals     []CurationRecord `json:"hospitals"`
}

// CurationRecord describes curated facts for one hospital that already exists from MSI.
// Pointers keep null (unknown) distinct from 0/false.
type CurationRecord struct {
	ExternalID      string            `json:"external_id"`
	SourceNamespace string            `json:"source_namespace,omitempty"`
	Name            string            `json:"name,omitempty"`
	BPJS            *bool             `json:"bpjs"`
	SourceURL       string            `json:"source_url"`
	SourceUpdatedAt string            `json:"source_updated_at"`
	Services        []CurationService `json:"services,omitempty"`
	Beds            []CurationBed     `json:"beds,omitempty"`
	// Rejected inputs: present only to produce a clear error instead of a generic one.
	DataOrigin any `json:"data_origin,omitempty"`
	VerifiedAt any `json:"verified_at,omitempty"`
}
type CurationService struct {
	Slug            string `json:"slug"`
	Status          string `json:"status"`
	SourceURL       string `json:"source_url,omitempty"`
	SourceUpdatedAt string `json:"source_updated_at,omitempty"`
}
type CurationBed struct {
	Category        string `json:"category"`
	Total           *int   `json:"total"`
	Available       *int   `json:"available"`
	SourceURL       string `json:"source_url,omitempty"`
	SourceUpdatedAt string `json:"source_updated_at,omitempty"`
}

// Curated* are validated records ready to apply: provenance is resolved per item,
// data_origin is always curated and there is deliberately no verified_at field.
type CuratedHospital struct {
	ExternalID      string
	SourceNamespace string
	Name            string
	BPJS            *bool
	// SourceURL/SourceUpdatedAt are the record-level source; they become the BPJS provenance.
	SourceURL       string
	SourceUpdatedAt time.Time
	Services        []CuratedService
	Beds            []CuratedBed
}
type CuratedService struct {
	Slug, Status, SourceURL string
	SourceUpdatedAt         time.Time
}
type CuratedBed struct {
	Category, SourceURL string
	Total, Available    *int
	SourceUpdatedAt     time.Time
}

// CurationIssue is one validation finding. Warnings do not reject a record.
type CurationIssue struct {
	Record  int // 1-based position in the file
	ID      string
	Field   string
	Message string
	Warning bool
}

func (i CurationIssue) String() string {
	kind := "ERROR"
	if i.Warning {
		kind = "WARN"
	}
	return fmt.Sprintf("%s record %d (%s) %s: %s", kind, i.Record, i.ID, i.Field, i.Message)
}

// ParseCuration decodes a .json or .csv curation file by its extension.
func ParseCuration(name string, data []byte) ([]CurationRecord, error) {
	if len(data) > MaxCurationFile {
		return nil, errors.New("file terlalu besar (maks 1 MB)")
	}
	switch strings.ToLower(name[strings.LastIndex(name, ".")+1:]) {
	case "json":
		var f CurationFile
		d := json.NewDecoder(bytes.NewReader(data))
		d.DisallowUnknownFields()
		if e := d.Decode(&f); e != nil {
			return nil, fmt.Errorf("JSON tidak valid: %w", e)
		}
		if d.More() {
			return nil, errors.New("JSON tidak valid: data tambahan setelah objek")
		}
		if f.SchemaVersion != 1 {
			return nil, errors.New("schema_version harus 1")
		}
		return f.Hospitals, nil
	case "csv":
		return parseCurationCSV(data)
	}
	return nil, errors.New("ekstensi file harus .json atau .csv")
}

// parseCurationCSV reads one hospital per row. Columns: external_id, source_namespace, name, bpjs,
// source_url, source_updated_at, svc_<slug>, bed_<category>_total, bed_<category>_available.
// An empty cell means "not provided" (null), never 0.
func parseCurationCSV(data []byte) ([]CurationRecord, error) {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord = -1
	rows, e := r.ReadAll()
	if e != nil {
		return nil, fmt.Errorf("CSV tidak valid: %w", e)
	}
	if len(rows) == 0 {
		return nil, errors.New("CSV kosong")
	}
	head := map[string]int{}
	for i, h := range rows[0] {
		h = strings.TrimSpace(h)
		if _, dup := head[h]; dup {
			return nil, fmt.Errorf("kolom ganda: %s", h)
		}
		head[h] = i
	}
	known := map[string]bool{"external_id": true, "source_namespace": true, "name": true, "bpjs": true, "source_url": true, "source_updated_at": true, "data_origin": true, "verified_at": true}
	for _, s := range curationSvcSlugs {
		known["svc_"+s] = true
	}
	for _, c := range curationBedKinds {
		known["bed_"+c+"_total"], known["bed_"+c+"_available"] = true, true
	}
	for h := range head {
		if !known[h] {
			return nil, fmt.Errorf("kolom tidak dikenal: %s", h)
		}
	}
	var out []CurationRecord
	for n, row := range rows[1:] {
		cell := func(h string) string {
			if i, ok := head[h]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		line := n + 2
		allBlank := true
		for _, c := range row {
			allBlank = allBlank && strings.TrimSpace(c) == ""
		}
		if allBlank {
			continue
		}
		rec := CurationRecord{ExternalID: cell("external_id"), SourceNamespace: cell("source_namespace"), Name: cell("name"), SourceURL: cell("source_url"), SourceUpdatedAt: cell("source_updated_at")}
		if v := cell("data_origin"); v != "" {
			rec.DataOrigin = v
		}
		if v := cell("verified_at"); v != "" {
			rec.VerifiedAt = v
		}
		switch strings.ToLower(cell("bpjs")) {
		case "":
		case "true", "false":
			b := strings.ToLower(cell("bpjs")) == "true"
			rec.BPJS = &b
		default:
			return nil, fmt.Errorf("baris %d: bpjs harus kosong, true atau false", line)
		}
		for _, s := range curationSvcSlugs {
			if v := cell("svc_" + s); v != "" {
				rec.Services = append(rec.Services, CurationService{Slug: s, Status: v})
			}
		}
		for _, c := range curationBedKinds {
			var b CurationBed
			b.Category = c
			for _, f := range []struct {
				col string
				dst **int
			}{{"bed_" + c + "_total", &b.Total}, {"bed_" + c + "_available", &b.Available}} {
				v := cell(f.col)
				if v == "" {
					continue
				}
				x, err := strconv.Atoi(v)
				if err != nil {
					return nil, fmt.Errorf("baris %d: %s harus bilangan bulat atau kosong", line, f.col)
				}
				*f.dst = &x
			}
			if b.Total != nil || b.Available != nil {
				rec.Beds = append(rec.Beds, b)
			}
		}
		out = append(out, rec)
	}
	return out, nil
}

// placeholderHost reports hosts that can never be a real public source.
func placeholderHost(host string) bool {
	host = strings.ToLower(host)
	for _, s := range []string{".invalid", ".example", ".test", ".localhost"} {
		if strings.HasSuffix(host, s) {
			return true
		}
	}
	switch host {
	case "localhost", "example.com", "example.org", "example.net":
		return true
	}
	return strings.HasSuffix(host, ".example.com") || strings.HasSuffix(host, ".example.org") || strings.HasSuffix(host, ".example.net")
}

func checkSource(rawURL, rawAt string, now time.Time) (u string, at time.Time, problems []string, placeholder bool) {
	rawURL, rawAt = strings.TrimSpace(rawURL), strings.TrimSpace(rawAt)
	if rawURL == "" {
		problems = append(problems, "source_url wajib diisi")
	} else if p, e := url.Parse(rawURL); e != nil || p.Scheme != "https" || p.Hostname() == "" || len(rawURL) > 2048 {
		problems = append(problems, "source_url harus URL https yang valid")
	} else {
		u = rawURL
		placeholder = placeholderHost(p.Hostname())
	}
	if rawAt == "" {
		problems = append(problems, "source_updated_at wajib diisi (RFC3339)")
	} else if t, e := time.Parse(time.RFC3339, rawAt); e != nil {
		problems = append(problems, "source_updated_at harus RFC3339, mis. 2026-10-20T08:00:00Z")
	} else if t.After(now) {
		problems = append(problems, "source_updated_at tidak boleh di masa depan")
	} else {
		at = t.UTC()
	}
	return
}

// ValidateCuration checks every record. A record with any error is rejected (absent from the
// result) and the caller must not apply a partially valid file. data_origin is not an input.
func ValidateCuration(records []CurationRecord, now time.Time) ([]CuratedHospital, []CurationIssue) {
	var out []CuratedHospital
	var issues []CurationIssue
	seen := map[string]bool{}
	for i, r := range records {
		n, id := i+1, strings.TrimSpace(r.ExternalID)
		bad := false
		fail := func(field, msg string) { bad = true; issues = append(issues, CurationIssue{n, id, field, msg, false}) }
		warn := func(field, msg string) { issues = append(issues, CurationIssue{n, id, field, msg, true}) }
		ns := strings.TrimSpace(r.SourceNamespace)
		if ns == "" {
			ns = DefaultCurationNamespace
		}
		if id == "" || len(id) > 128 || strings.ContainsAny(id, " \t\r\n") {
			fail("external_id", "wajib diisi (kode MSI), tanpa spasi, maks 128 karakter")
		} else if seen[ns+"|"+id] {
			fail("external_id", "duplikat dalam file")
		}
		seen[ns+"|"+id] = true
		if r.DataOrigin != nil {
			fail("data_origin", "tidak boleh diisi; kurasi selalu bernilai curated")
		}
		if r.VerifiedAt != nil {
			fail("verified_at", "tidak boleh diisi; hanya manusia yang memverifikasi setelah pemeriksaan nyata")
		}
		src, at, problems, ph := checkSource(r.SourceURL, r.SourceUpdatedAt, now)
		for _, p := range problems {
			fail("source", p)
		}
		if ph {
			warn("source_url", "host placeholder; --apply akan menolak")
		}
		c := CuratedHospital{ExternalID: id, SourceNamespace: ns, Name: strings.TrimSpace(r.Name), BPJS: r.BPJS, SourceURL: src, SourceUpdatedAt: at}
		svc := map[string]bool{}
		for _, s := range r.Services {
			slug := strings.TrimSpace(s.Slug)
			if !curationSlug.MatchString(slug) || len(slug) > 40 {
				fail("services.slug", fmt.Sprintf("slug %q tidak valid", s.Slug))
				continue
			}
			if svc[slug] {
				fail("services."+slug, "duplikat")
				continue
			}
			svc[slug] = true
			if !domain.ValidServiceStatus(s.Status) {
				fail("services."+slug, "status harus available|limited|unavailable|unknown")
			}
			u, t, p, sph := src, at, []string(nil), ph
			if s.SourceURL != "" || s.SourceUpdatedAt != "" {
				u, t, p, sph = checkSource(orDefault(s.SourceURL, r.SourceURL), orDefault(s.SourceUpdatedAt, r.SourceUpdatedAt), now)
			}
			for _, x := range p {
				fail("services."+slug, x)
			}
			if sph && !ph {
				warn("services."+slug+".source_url", "host placeholder; --apply akan menolak")
			}
			c.Services = append(c.Services, CuratedService{Slug: slug, Status: s.Status, SourceURL: u, SourceUpdatedAt: t})
		}
		bed := map[string]bool{}
		for _, b := range r.Beds {
			cat := strings.TrimSpace(b.Category)
			if !slices.Contains(curationBedKinds, cat) {
				fail("beds.category", fmt.Sprintf("kategori %q harus icu|nicu|picu|general", b.Category))
				continue
			}
			if bed[cat] {
				fail("beds."+cat, "duplikat")
				continue
			}
			bed[cat] = true
			if b.Total == nil && b.Available == nil {
				fail("beds."+cat, "total dan available keduanya kosong; hapus entri (null berarti tidak diketahui, bukan 0)")
			}
			if !domain.ValidBeds(b.Total, b.Available) {
				fail("beds."+cat, "nilai tidak boleh negatif dan available harus <= total")
			}
			u, t, p, sph := src, at, []string(nil), ph
			if b.SourceURL != "" || b.SourceUpdatedAt != "" {
				u, t, p, sph = checkSource(orDefault(b.SourceURL, r.SourceURL), orDefault(b.SourceUpdatedAt, r.SourceUpdatedAt), now)
			}
			for _, x := range p {
				fail("beds."+cat, x)
			}
			if sph && !ph {
				warn("beds."+cat+".source_url", "host placeholder; --apply akan menolak")
			}
			c.Beds = append(c.Beds, CuratedBed{Category: cat, Total: b.Total, Available: b.Available, SourceURL: u, SourceUpdatedAt: t})
		}
		if r.BPJS == nil && len(r.Services) == 0 && len(r.Beds) == 0 {
			fail("data", "tidak ada bpjs, layanan, atau bed untuk dikurasi")
		}
		if !bad {
			out = append(out, c)
		}
	}
	return out, issues
}

// HasPlaceholder reports whether any warning marks a placeholder source host.
func HasPlaceholder(issues []CurationIssue) bool {
	for _, i := range issues {
		if i.Warning && strings.Contains(i.Message, "placeholder") {
			return true
		}
	}
	return false
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return d
}

// readAll is split out so tests can feed readers without touching the filesystem.
func readAll(r io.Reader) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, MaxCurationFile+1))
	if e == nil && len(b) > MaxCurationFile {
		e = errors.New("file terlalu besar (maks 1 MB)")
	}
	return b, e
}
