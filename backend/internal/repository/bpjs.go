package repository

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"siagars/backend/internal/domain"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// SetHistoryCipher enables storing and revealing the original input of BPJS checks. Without it only
// masked data is kept.
func (m *Mongo) SetHistoryCipher(c *HistoryCipher) { m.history = c }

// BPJSMemberDoc is a synthetic participant (collection bpjs_participants, _id = fictional NIK).
type BPJSMemberDoc struct {
	NIK            string  `bson:"_id"`
	Name           string  `bson:"name"`
	BirthDate      string  `bson:"birth_date"`
	Status         string  `bson:"status"`
	InactiveReason *string `bson:"inactive_reason"`
	MembershipCode *string `bson:"membership_code"`
	MembershipName *string `bson:"membership_name"`
	CareClass      *string `bson:"care_class"`
	FacilityCode   *string `bson:"facility_code"`
	FacilityName   *string `bson:"facility_name"`
}

func (d BPJSMemberDoc) domain() domain.BPJSMember {
	m := domain.BPJSMember{NIK: d.NIK, Name: d.Name, BirthDate: d.BirthDate, Status: d.Status, InactiveReason: d.InactiveReason, CareClass: d.CareClass}
	if d.MembershipCode != nil {
		m.MembershipType = &domain.Coded{Code: *d.MembershipCode, Name: d.MembershipName}
	}
	if d.FacilityCode != nil {
		m.PrimaryFacility = &domain.Coded{Code: *d.FacilityCode, Name: d.FacilityName}
	}
	return m
}

func (m *Mongo) BPJSMember(ctx context.Context, nik string) (domain.BPJSMember, error) {
	var d BPJSMemberDoc
	if e := m.DB.Collection("bpjs_participants").FindOne(ctx, bson.D{{Key: "_id", Value: nik}}).Decode(&d); e != nil {
		return domain.BPJSMember{}, one(e)
	}
	return d.domain(), nil
}

type bpjsHistoryDoc struct {
	ID         string    `bson:"_id"`
	NIKMasked  string    `bson:"nik_masked"`
	NameMasked string    `bson:"name_masked"`
	BirthYear  int       `bson:"birth_year"`
	Result     string    `bson:"result"`
	Status     *string   `bson:"status"`
	NIKHMAC    string    `bson:"nik_hmac,omitempty"`
	RawEnc     []byte    `bson:"raw_enc,omitempty"`
	CreatedAt  time.Time `bson:"created_at"`
}

func newUUID() string {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic(e)
	}
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// RecordBPJSCheck stores one history entry. The original input is kept only encrypted, and only when a
// history key is configured.
func (m *Mongo) RecordBPJSCheck(ctx context.Context, r domain.BPJSRecord) error {
	d := bpjsHistoryDoc{ID: newUUID(), NIKMasked: r.NIKMasked, NameMasked: r.NameMasked, BirthYear: r.BirthYear, Result: r.Result, Status: r.Status, CreatedAt: now()}
	if m.history != nil {
		raw, e := json.Marshal(r.Raw)
		if e != nil {
			return e
		}
		if d.RawEnc, e = m.history.Seal(raw, []byte(d.ID)); e != nil {
			return e
		}
		d.NIKHMAC = m.history.Fingerprint(r.Raw.NIK)
	}
	_, e := m.DB.Collection("bpjs_check_history").InsertOne(ctx, d)
	return e
}

// BPJSHistory lists masked entries, newest first. repeat_count is the number of stored entries with the
// same NIK fingerprint (1 when no key was configured).
func (m *Mongo) BPJSHistory(ctx context.Context, page, limit int, result string) ([]domain.BPJSHistoryItem, int, error) {
	coll := m.DB.Collection("bpjs_check_history")
	filter := bson.D{}
	if result != "" {
		filter = bson.D{{Key: "result", Value: result}}
	}
	total, e := coll.CountDocuments(ctx, filter)
	if e != nil {
		return nil, 0, e
	}
	cur, e := coll.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: 1}}).SetSkip(int64((page-1)*limit)).SetLimit(int64(limit)).
		SetProjection(bson.D{{Key: "raw_enc", Value: 0}}))
	if e != nil {
		return nil, 0, e
	}
	var docs []bpjsHistoryDoc
	if e = cur.All(ctx, &docs); e != nil {
		return nil, 0, e
	}
	var prints bson.A
	for _, d := range docs {
		if d.NIKHMAC != "" {
			prints = append(prints, d.NIKHMAC)
		}
	}
	repeats := map[string]int{}
	if len(prints) > 0 {
		agg, e := coll.Aggregate(ctx, bson.A{
			bson.D{{Key: "$match", Value: bson.D{{Key: "nik_hmac", Value: bson.D{{Key: "$in", Value: prints}}}}}},
			bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$nik_hmac"}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}}})
		if e != nil {
			return nil, 0, e
		}
		var rows []struct {
			ID string `bson:"_id"`
			N  int    `bson:"n"`
		}
		if e = agg.All(ctx, &rows); e != nil {
			return nil, 0, e
		}
		for _, r := range rows {
			repeats[r.ID] = r.N
		}
	}
	out := make([]domain.BPJSHistoryItem, 0, len(docs))
	for _, d := range docs {
		n := repeats[d.NIKHMAC]
		if n == 0 {
			n = 1
		}
		out = append(out, domain.BPJSHistoryItem{ID: d.ID, NIKMasked: d.NIKMasked, NameMasked: d.NameMasked, BirthYear: d.BirthYear, Result: d.Result, Status: d.Status, CheckedAt: d.CreatedAt, RepeatCount: n})
	}
	return out, int(total), nil
}

// RevealBPJS decrypts one entry. ErrNotFound for an unknown id, ErrRawUnavailable when the entry has no
// ciphertext or the server has no (matching) key.
func (m *Mongo) RevealBPJS(ctx context.Context, id string) (domain.BPJSReveal, error) {
	var d bpjsHistoryDoc
	if e := m.DB.Collection("bpjs_check_history").FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d); e != nil {
		return domain.BPJSReveal{}, one(e)
	}
	if m.history == nil || len(d.RawEnc) == 0 {
		return domain.BPJSReveal{}, ErrRawUnavailable
	}
	plain, e := m.history.Open(d.RawEnc, []byte(d.ID))
	if e != nil {
		return domain.BPJSReveal{}, ErrRawUnavailable
	}
	var raw domain.BPJSRaw
	if e = json.Unmarshal(plain, &raw); e != nil {
		return domain.BPJSReveal{}, ErrRawUnavailable
	}
	return domain.BPJSReveal{ID: d.ID, NIK: raw.NIK, Name: raw.Name, BirthDate: raw.BirthDate, CheckedAt: d.CreatedAt}, nil
}

// AuditBPJS records an operator action on BPJS history without any personal data.
func (m *Mongo) AuditBPJS(ctx context.Context, actor, action, id, rid string) error {
	return m.audit(ctx, actor, action, id, rid, bson.D{})
}
