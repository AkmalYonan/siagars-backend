package httpapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"net/http"
	"siagars/backend/internal/domain"
	"siagars/backend/internal/repository"
	"siagars/backend/internal/service"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// BPJSStore backs the BPJS membership simulation and its operator history. Stores without it answer 501.
type BPJSStore interface {
	BPJSMember(ctx context.Context, nik string) (domain.BPJSMember, error)
	RecordBPJSCheck(ctx context.Context, r domain.BPJSRecord) error
	BPJSHistory(ctx context.Context, page, limit int, result string) ([]domain.BPJSHistoryItem, int, error)
	RevealBPJS(ctx context.Context, id string) (domain.BPJSReveal, error)
	AuditBPJS(ctx context.Context, actor, action, id, rid string) error
}

func (s *Server) bpjsStore(w http.ResponseWriter, r *http.Request) (BPJSStore, bool) {
	store, ok := s.repo.(BPJSStore)
	if !ok {
		fail(w, r, 501, "not_implemented", "Fitur ini belum tersedia di server")
	}
	return store, ok
}

// equal compares in constant time; both sides are always evaluated so the result does not leak which
// field differed.
func equal(a, b string) bool { return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1 }

// bpjsCheck simulates a membership lookup against synthetic participants. A miss never says which field
// was wrong. The original input is stored only encrypted (see Mongo.RecordBPJSCheck) for operator
// history and is never logged or returned.
func (s *Server) bpjsCheck(w http.ResponseWriter, r *http.Request) {
	store, have := s.bpjsStore(w, r)
	if !have || !s.rate(w, r, "bpjs:"+s.clientIP(r), 10, time.Minute) {
		return
	}
	var b struct {
		NIK       string `json:"nik"`
		Name      string `json:"name"`
		BirthDate string `json:"birth_date"`
	}
	if !decode(w, r, &b) {
		return
	}
	now := time.Now().UTC()
	in, e := service.ParseBPJSCheck(b.NIK, b.Name, b.BirthDate, now)
	if e != nil {
		fail(w, r, 422, "validation_error", e.Error())
		return
	}
	member, e := store.BPJSMember(r.Context(), in.NIK)
	if e != nil && !errors.Is(e, repository.ErrNotFound) {
		s.dbError(w, r, e)
		return
	}
	nameOK := equal(service.NameKey(member.Name), in.NameKey)
	birthOK := equal(member.BirthDate, in.BirthDate)
	matched := e == nil && nameOK && birthOK
	out := domain.BPJSCheck{Result: "not_found", DataOrigin: "simulation", CheckedAt: now.Truncate(time.Second)}
	rec := domain.BPJSRecord{NIKMasked: domain.MaskNIK(in.NIK), NameMasked: domain.MaskName(in.Name), Result: "not_found", Raw: domain.BPJSRaw{NIK: in.NIK, Name: in.Name, BirthDate: in.BirthDate}}
	rec.BirthYear, _ = strconv.Atoi(in.BirthDate[:4])
	if matched {
		out.Result, out.Participant = "found", member.PublicParticipant()
		rec.Result, rec.Status = "found", &member.Status
	}
	// History must never delay or fail the answer; a failure is logged without any input.
	rctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Second)
	defer cancel()
	if e := store.RecordBPJSCheck(rctx, rec); e != nil {
		slog.Error("bpjs history write failed", "request_id", rid(r))
	}
	ok(w, out)
}

// bpjsHistory lists stored checks, masked only. Any signed-in operator may read it.
func (s *Server) bpjsHistory(w http.ResponseWriter, r *http.Request) {
	store, have := s.bpjsStore(w, r)
	if !have {
		return
	}
	if _, yes := s.auth(w, r, false); !yes {
		return
	}
	page, limit, result := 1, 20, ""
	for k, v := range r.URL.Query() {
		if len(v) != 1 || v[0] == "" {
			fail(w, r, 422, "validation_error", "Parameter kosong atau berulang: "+k)
			return
		}
		switch k {
		case "page":
			n, e := strconv.Atoi(v[0])
			if e != nil || n < 1 || n > 100000 {
				fail(w, r, 422, "validation_error", "page harus bilangan bulat positif")
				return
			}
			page = n
		case "limit":
			n, e := strconv.Atoi(v[0])
			if e != nil || n < 1 || n > 50 {
				fail(w, r, 422, "validation_error", "limit harus 1 sampai 50")
				return
			}
			limit = n
		case "result":
			if v[0] != "found" && v[0] != "not_found" {
				fail(w, r, 422, "validation_error", "result harus found atau not_found")
				return
			}
			result = v[0]
		default:
			fail(w, r, 422, "validation_error", "Parameter tidak didukung: "+k)
			return
		}
	}
	items, total, e := store.BPJSHistory(r.Context(), page, limit, result)
	if e != nil {
		s.dbError(w, r, e)
		return
	}
	pages := (total + limit - 1) / limit
	send(w, 200, map[string]any{"data": items, "meta": map[string]any{"page": page, "limit": limit, "total": total, "total_pages": pages, "as_of": time.Now().UTC()}})
}

// bpjsReveal shows the original input of one stored check. It needs a session, Origin, CSRF token and a
// fresh proof of the operator's own password, and every attempt is audited without personal data.
func (s *Server) bpjsReveal(w http.ResponseWriter, r *http.Request) {
	store, have := s.bpjsStore(w, r)
	if !have {
		return
	}
	op, yes := s.auth(w, r, true)
	if !yes {
		return
	}
	id := r.PathValue("id")
	if !validUUID(id) {
		fail(w, r, 404, "not_found", "Resource tidak ditemukan")
		return
	}
	if !s.rate(w, r, "bpjs-reveal:"+op.ID, 5, 15*time.Minute) {
		return
	}
	var b struct {
		Password string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	if len(b.Password) < 1 || len(b.Password) > 72 {
		fail(w, r, 422, "validation_error", "Password tidak valid")
		return
	}
	_, hash, e := s.repo.Credentials(r.Context(), op.Email)
	if e != nil && !errors.Is(e, repository.ErrNotFound) {
		s.dbError(w, r, e)
		return
	}
	compare := []byte(hash)
	if e != nil {
		compare = s.dummy
	}
	if e != nil || bcrypt.CompareHashAndPassword(compare, []byte(b.Password)) != nil {
		_ = store.AuditBPJS(r.Context(), op.ID, "bpjs.reveal_denied", id, rid(r))
		fail(w, r, 401, "invalid_credentials", "Password tidak valid")
		return
	}
	// The access is recorded before any data leaves; if the audit write fails nothing is revealed.
	if e = store.AuditBPJS(r.Context(), op.ID, "bpjs.reveal", id, rid(r)); e != nil {
		s.dbError(w, r, e)
		return
	}
	reveal, e := store.RevealBPJS(r.Context(), id)
	switch {
	case errors.Is(e, repository.ErrRawUnavailable):
		fail(w, r, 409, "history_raw_unavailable", "Data asli tidak tersedia untuk entri ini")
		return
	case e != nil:
		s.dbError(w, r, e)
		return
	}
	ok(w, reveal)
}
