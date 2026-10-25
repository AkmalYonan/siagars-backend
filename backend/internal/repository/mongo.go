package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"siagars/backend/internal/domain"
	"siagars/backend/migrations"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrConflict  = errors.New("conflict")
)

type Mongo struct {
	Client *mongo.Client
	DB     *mongo.Database
}

// Open configures the client lazily; the first operation establishes the connection.
func Open(uri, database string) (*Mongo, error) {
	c, e := mongo.Connect(options.Client().ApplyURI(uri).SetAppName("siagars-backend").SetServerSelectionTimeout(5 * time.Second).SetMaxPoolSize(20))
	if e != nil {
		return nil, e
	}
	return &Mongo{c, c.Database(database)}, nil
}
func (m *Mongo) Close(ctx context.Context) error { return m.Client.Disconnect(ctx) }
func Hash(v string) string                       { s := sha256.Sum256([]byte(v)); return hex.EncodeToString(s[:]) }

// now uses millisecond precision because BSON dates cannot store more.
func now() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }

func (m *Mongo) transaction(ctx context.Context, fn func(context.Context) error, opts ...options.Lister[options.TransactionOptions]) error {
	s, e := m.Client.StartSession()
	if e != nil {
		return e
	}
	defer s.EndSession(context.Background())
	_, e = s.WithTransaction(ctx, func(ctx context.Context) (any, error) { return nil, fn(ctx) }, opts...)
	return e
}
func snapshot() *options.TransactionOptionsBuilder {
	return options.Transaction().SetReadConcern(readconcern.Snapshot()).SetWriteConcern(writeconcern.Majority())
}

// Ready reports whether the database is reachable and at least one migration has been applied.
func (m *Mongo) Ready(ctx context.Context) error {
	var v struct {
		ID string `bson:"_id"`
	}
	return m.DB.Collection("schema_migrations").FindOne(ctx, bson.D{}, options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})).Decode(&v)
}

// Migrate applies numbered JSON command files once each. DDL is not transactional in MongoDB,
// so every command is safe to re-run: create falls back to collMod and createIndexes is idempotent.
func (m *Mongo) Migrate(ctx context.Context) error {
	locks := m.DB.Collection("locks")
	if _, e := locks.DeleteOne(ctx, bson.D{{Key: "_id", Value: "migrate"}, {Key: "expires_at", Value: bson.D{{Key: "$lt", Value: now()}}}}); e != nil {
		return e
	}
	if _, e := locks.InsertOne(ctx, bson.D{{Key: "_id", Value: "migrate"}, {Key: "expires_at", Value: now().Add(5 * time.Minute)}}); e != nil {
		if mongo.IsDuplicateKeyError(e) {
			return errors.New("another migration is running")
		}
		return e
	}
	defer locks.DeleteOne(context.Background(), bson.D{{Key: "_id", Value: "migrate"}})
	entries, e := migrations.Files.ReadDir(".")
	if e != nil {
		return e
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	applied := m.DB.Collection("schema_migrations")
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, e := migrations.Files.ReadFile(entry.Name())
		if e != nil {
			return e
		}
		checksum := Hash(string(b))
		var existing struct {
			Checksum string `bson:"checksum"`
		}
		e = applied.FindOne(ctx, bson.D{{Key: "_id", Value: entry.Name()}}).Decode(&existing)
		if e == nil {
			if existing.Checksum != checksum {
				return fmt.Errorf("migration checksum mismatch: %s", entry.Name())
			}
			continue
		}
		if !errors.Is(e, mongo.ErrNoDocuments) {
			return e
		}
		var file struct {
			Commands []bson.D `bson:"commands"`
		}
		if e = bson.UnmarshalExtJSON(b, false, &file); e != nil {
			return fmt.Errorf("migration %s: %w", entry.Name(), e)
		}
		for _, command := range file.Commands {
			if e = m.runCommand(ctx, command); e != nil {
				return fmt.Errorf("migration %s: %w", entry.Name(), e)
			}
		}
		if _, e = applied.InsertOne(ctx, bson.D{{Key: "_id", Value: entry.Name()}, {Key: "checksum", Value: checksum}, {Key: "applied_at", Value: now()}}); e != nil {
			return e
		}
	}
	return nil
}
func (m *Mongo) runCommand(ctx context.Context, command bson.D) error {
	e := m.DB.RunCommand(ctx, command).Err()
	var ce mongo.CommandError
	if len(command) > 0 && command[0].Key == "create" && errors.As(e, &ce) && ce.Code == 48 {
		// NamespaceExists: reconcile the validator of a collection left by an interrupted run.
		modify := append(bson.D{{Key: "collMod", Value: command[0].Value}}, command[1:]...)
		return m.DB.RunCommand(ctx, modify).Err()
	}
	return e
}

type ProvenanceDoc struct {
	DataOrigin      string     `bson:"data_origin"`
	SourceURL       *string    `bson:"source_url"`
	SourceUpdatedAt *time.Time `bson:"source_updated_at"`
	SyncedAt        *time.Time `bson:"synced_at"`
	VerifiedAt      *time.Time `bson:"verified_at"`
}
type ServiceDoc struct {
	Slug          string `bson:"slug"`
	Status        string `bson:"status"`
	ProvenanceDoc `bson:",inline"`
	Version       int `bson:"version"`
}
type BedDoc struct {
	Category      string `bson:"category"`
	Total         *int   `bson:"total"`
	Available     *int   `bson:"available"`
	ProvenanceDoc `bson:",inline"`
	Version       int `bson:"version"`
}
type HospitalDoc struct {
	ID              string   `bson:"_id,omitempty"`
	ExternalID      *string  `bson:"external_id,omitempty"`
	SourceNamespace *string  `bson:"source_namespace,omitempty"`
	Name            string   `bson:"name"`
	Address         *string  `bson:"address"`
	CityCode        string   `bson:"city_code"`
	ProvinceCode    string   `bson:"province_code"`
	Latitude        *float64 `bson:"latitude"`
	Longitude       *float64 `bson:"longitude"`
	Phone           *string  `bson:"phone"`
	BPJS            *bool    `bson:"bpjs"`
	// Upstream facility attributes; absent on seed/curated-only documents.
	BPJSSourceURL       *string    `bson:"bpjs_source_url,omitempty"`
	BPJSSourceUpdatedAt *time.Time `bson:"bpjs_source_updated_at,omitempty"`
	FacilityClass       *string    `bson:"facility_class,omitempty"`
	FacilityClassName   *string    `bson:"facility_class_name,omitempty"`
	FacilitySubtype     *string    `bson:"facility_subtype,omitempty"`
	FacilitySubtypeName *string    `bson:"facility_subtype_name,omitempty"`
	Operational         *bool      `bson:"operational,omitempty"`
	ProvenanceDoc       `bson:",inline"`
	Services            []ServiceDoc `bson:"services"`
	Beds                []BedDoc     `bson:"beds"`
}

// Simulation returns provenance for synthetic data: no source URL and never verified.
func Simulation(at time.Time) ProvenanceDoc {
	return ProvenanceDoc{DataOrigin: "simulation", SourceUpdatedAt: &at, SyncedAt: &at}
}
func (p ProvenanceDoc) domain() domain.Provenance {
	return domain.Provenance{DataOrigin: p.DataOrigin, SourceURL: p.SourceURL, SourceUpdatedAt: p.SourceUpdatedAt, SyncedAt: p.SyncedAt, VerifiedAt: p.VerifiedAt}
}

type referralDoc struct {
	ID                    string    `bson:"_id"`
	OwnerHospitalID       string    `bson:"owner_hospital_id"`
	DestinationHospitalID *string   `bson:"destination_hospital_id"`
	Status                string    `bson:"status"`
	PublicNote            string    `bson:"public_note"`
	NextStep              string    `bson:"next_step"`
	Version               int       `bson:"version"`
	IsDemo                bool      `bson:"is_demo"`
	UpdatedAt             time.Time `bson:"updated_at"`
}

func (r referralDoc) domain() domain.Referral {
	return domain.Referral{ID: r.ID, OwnerHospitalID: r.OwnerHospitalID, DestinationHospitalID: r.DestinationHospitalID, Status: r.Status, PublicNote: r.PublicNote, NextStep: r.NextStep, Version: r.Version, IsDemo: r.IsDemo, UpdatedAt: r.UpdatedAt}
}
func all[T any](ctx context.Context, c *mongo.Collection, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	cursor, e := c.Find(ctx, filter, opts...)
	if e != nil {
		return nil, e
	}
	out := []T{}
	return out, cursor.All(ctx, &out)
}
func one(e error) error {
	if errors.Is(e, mongo.ErrNoDocuments) {
		return ErrNotFound
	}
	return e
}
func (m *Mongo) Hospitals(ctx context.Context) ([]domain.Hospital, error) {
	regions, e := all[struct {
		Code string `bson:"_id"`
		Name string `bson:"name"`
	}](ctx, m.DB.Collection("regions"), bson.D{})
	if e != nil {
		return nil, e
	}
	names := map[string]string{}
	for _, r := range regions {
		names[r.Code] = r.Name
	}
	masters, e := m.Services(ctx)
	if e != nil {
		return nil, e
	}
	serviceNames := map[string]string{}
	for _, s := range masters {
		serviceNames[s.Slug] = s.Name
	}
	// Services and beds are embedded, so each hospital is read as one atomic document.
	// Only documents explicitly marked inactive upstream are hidden; seed/curated documents have no flag.
	docs, e := all[HospitalDoc](ctx, m.DB.Collection("hospitals"), bson.D{{Key: "source_active", Value: bson.D{{Key: "$ne", Value: false}}}}, options.Find().SetSort(bson.D{{Key: "name", Value: 1}, {Key: "_id", Value: 1}}))
	if e != nil {
		return nil, e
	}
	return hospitalDTOs(docs, names, serviceNames), nil
}

// coded returns nil when the source never provided a code.
func coded(code, name *string) *domain.Coded {
	if code == nil || *code == "" {
		return nil
	}
	return &domain.Coded{Code: *code, Name: name}
}

// hospitalDTOs maps stored documents to domain hospitals, joining region and service names.
func hospitalDTOs(docs []HospitalDoc, names, serviceNames map[string]string) []domain.Hospital {
	hs := make([]domain.Hospital, 0, len(docs))
	for _, d := range docs {
		h := domain.Hospital{ID: d.ID, ExternalID: d.ExternalID, SourceNamespace: d.SourceNamespace, BPJSSourceURL: d.BPJSSourceURL, BPJSSourceUpdatedAt: d.BPJSSourceUpdatedAt, FacilityClass: coded(d.FacilityClass, d.FacilityClassName), FacilitySubtype: coded(d.FacilitySubtype, d.FacilitySubtypeName), Operational: d.Operational, Name: d.Name, Address: d.Address, CityCode: d.CityCode, CityName: names[d.CityCode], ProvinceCode: d.ProvinceCode, ProvinceName: names[d.ProvinceCode], Latitude: d.Latitude, Longitude: d.Longitude, Phone: d.Phone, BPJS: d.BPJS, Provenance: d.domain(), Services: []domain.Service{}, Beds: []domain.Bed{}}
		for _, s := range d.Services {
			h.Services = append(h.Services, domain.Service{Slug: s.Slug, Name: serviceNames[s.Slug], Status: s.Status, Provenance: s.domain(), Version: s.Version})
		}
		for _, b := range d.Beds {
			h.Beds = append(h.Beds, domain.Bed{Category: b.Category, Total: b.Total, Available: b.Available, Provenance: b.domain(), Version: b.Version})
		}
		sort.Slice(h.Services, func(i, j int) bool { return h.Services[i].Slug < h.Services[j].Slug })
		sort.Slice(h.Beds, func(i, j int) bool { return h.Beds[i].Category < h.Beds[j].Category })
		hs = append(hs, h)
	}
	return hs
}
func (m *Mongo) Services(ctx context.Context) ([]domain.Service, error) {
	docs, e := all[struct {
		Slug     string `bson:"_id"`
		Name     string `bson:"name"`
		Category string `bson:"category"`
	}](ctx, m.DB.Collection("services"), bson.D{}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if e != nil {
		return nil, e
	}
	out := make([]domain.Service, 0, len(docs))
	for _, d := range docs {
		out = append(out, domain.Service{Slug: d.Slug, Name: d.Name, Category: d.Category})
	}
	return out, nil
}
func (m *Mongo) RegionExists(ctx context.Context, code, kind string) (bool, error) {
	n, e := m.DB.Collection("regions").CountDocuments(ctx, bson.D{{Key: "_id", Value: code}, {Key: "kind", Value: kind}})
	return n > 0, e
}
func (m *Mongo) Credentials(ctx context.Context, email string) (string, string, error) {
	var op struct {
		ID   string `bson:"_id"`
		Hash string `bson:"password_hash"`
	}
	e := m.DB.Collection("operators").FindOne(ctx, bson.D{{Key: "email", Value: email}, {Key: "active", Value: true}}).Decode(&op)
	return op.ID, op.Hash, one(e)
}

// CreateOperator inserts an operator with hospital assignments; all hospitals must exist.
func (m *Mongo) CreateOperator(ctx context.Context, id, email, hash string, hospitals []string) error {
	return m.transaction(ctx, func(ctx context.Context) error {
		n, e := m.DB.Collection("hospitals").CountDocuments(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: hospitals}}}})
		if e != nil {
			return e
		}
		if n != int64(len(hospitals)) {
			return ErrNotFound
		}
		_, e = m.DB.Collection("operators").InsertOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "email", Value: email}, {Key: "password_hash", Value: hash}, {Key: "active", Value: true}, {Key: "hospital_ids", Value: hospitals}})
		if mongo.IsDuplicateKeyError(e) {
			return ErrConflict
		}
		return e
	})
}
func (m *Mongo) CreateSession(ctx context.Context, id, token, csrf string, expires time.Time, old string) error {
	return m.transaction(ctx, func(ctx context.Context) error {
		sessions := m.DB.Collection("sessions")
		if old != "" {
			if _, e := sessions.UpdateOne(ctx, bson.D{{Key: "_id", Value: Hash(old)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "revoked_at", Value: now()}}}}); e != nil {
				return e
			}
		}
		_, e := sessions.InsertOne(ctx, bson.D{{Key: "_id", Value: Hash(token)}, {Key: "operator_id", Value: id}, {Key: "csrf_hash", Value: Hash(csrf)}, {Key: "expires_at", Value: expires.UTC()}, {Key: "revoked_at", Value: nil}})
		return e
	})
}
func (m *Mongo) Session(ctx context.Context, token string) (domain.Operator, string, error) {
	var s struct {
		OperatorID string `bson:"operator_id"`
		CSRF       string `bson:"csrf_hash"`
	}
	e := m.DB.Collection("sessions").FindOne(ctx, bson.D{{Key: "_id", Value: Hash(token)}, {Key: "expires_at", Value: bson.D{{Key: "$gt", Value: now()}}}, {Key: "revoked_at", Value: nil}}).Decode(&s)
	if e != nil {
		return domain.Operator{}, "", one(e)
	}
	var op struct {
		ID          string   `bson:"_id"`
		Email       string   `bson:"email"`
		HospitalIDs []string `bson:"hospital_ids"`
	}
	if e = m.DB.Collection("operators").FindOne(ctx, bson.D{{Key: "_id", Value: s.OperatorID}, {Key: "active", Value: true}}).Decode(&op); e != nil {
		return domain.Operator{}, "", one(e)
	}
	ids := append([]string{}, op.HospitalIDs...)
	sort.Strings(ids)
	return domain.Operator{ID: op.ID, Email: op.Email, HospitalIDs: ids}, s.CSRF, nil
}
func (m *Mongo) Revoke(ctx context.Context, token string) error {
	_, e := m.DB.Collection("sessions").UpdateOne(ctx, bson.D{{Key: "_id", Value: Hash(token)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "revoked_at", Value: now()}}}})
	return e
}
func (m *Mongo) hospitalIDs(ctx context.Context, operator string) ([]string, error) {
	var op struct {
		HospitalIDs []string `bson:"hospital_ids"`
	}
	e := m.DB.Collection("operators").FindOne(ctx, bson.D{{Key: "_id", Value: operator}}).Decode(&op)
	if errors.Is(e, mongo.ErrNoDocuments) {
		return []string{}, nil
	}
	return op.HospitalIDs, e
}
func (m *Mongo) Referrals(ctx context.Context, operator string) ([]domain.Referral, error) {
	ids, e := m.hospitalIDs(ctx, operator)
	if e != nil {
		return nil, e
	}
	docs, e := all[referralDoc](ctx, m.DB.Collection("referrals"), bson.D{{Key: "owner_hospital_id", Value: bson.D{{Key: "$in", Value: ids}}}}, options.Find().SetSort(bson.D{{Key: "updated_at", Value: -1}, {Key: "_id", Value: 1}}))
	if e != nil {
		return nil, e
	}
	out := make([]domain.Referral, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.domain())
	}
	return out, nil
}
func (m *Mongo) Lookup(ctx context.Context, code string) (domain.Referral, []domain.History, error) {
	var ref domain.Referral
	var h []domain.History
	e := m.transaction(ctx, func(ctx context.Context) error {
		var d referralDoc
		if e := m.DB.Collection("referrals").FindOne(ctx, bson.D{{Key: "tracking_token_hash", Value: Hash(code)}, {Key: "is_demo", Value: true}}).Decode(&d); e != nil {
			return one(e)
		}
		docs, e := all[struct {
			FromStatus *string   `bson:"from_status"`
			ToStatus   string    `bson:"to_status"`
			PublicNote string    `bson:"public_note"`
			CreatedAt  time.Time `bson:"created_at"`
		}](ctx, m.DB.Collection("referral_status_history"), bson.D{{Key: "referral_id", Value: d.ID}}, options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}, {Key: "_id", Value: 1}}))
		if e != nil {
			return e
		}
		ref, h = d.domain(), make([]domain.History, 0, len(docs))
		for _, x := range docs {
			h = append(h, domain.History{FromStatus: x.FromStatus, ToStatus: x.ToStatus, PublicNote: x.PublicNote, CreatedAt: x.CreatedAt})
		}
		return nil
	}, snapshot())
	return ref, h, e
}
func (m *Mongo) scope(ctx context.Context, actor, hospital string) error {
	n, e := m.DB.Collection("operators").CountDocuments(ctx, bson.D{{Key: "_id", Value: actor}, {Key: "hospital_ids", Value: hospital}})
	if e != nil {
		return e
	}
	if n == 0 {
		return ErrForbidden
	}
	return nil
}
func (m *Mongo) audit(ctx context.Context, actor, action, id, rid string, changes bson.D) error {
	_, e := m.DB.Collection("audit_logs").InsertOne(ctx, bson.D{{Key: "actor_id", Value: actor}, {Key: "action", Value: action}, {Key: "entity_id", Value: id}, {Key: "changes", Value: changes}, {Key: "request_id", Value: rid}, {Key: "created_at", Value: now()}})
	return e
}

// UpdateReferral changes status, history and audit in one transaction. A concurrent write
// conflict makes the driver retry the callback, which then observes the new version and returns ErrConflict.
func (m *Mongo) UpdateReferral(ctx context.Context, actor, id, status, note string, version int, rid string) (domain.Referral, error) {
	var r domain.Referral
	e := m.transaction(ctx, func(ctx context.Context) error {
		var d referralDoc
		if e := m.DB.Collection("referrals").FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&d); e != nil {
			return one(e)
		}
		r = d.domain()
		if e := m.scope(ctx, actor, r.OwnerHospitalID); e != nil {
			return e
		}
		if r.Version != version || !domain.CanTransition(r.Status, status, note) {
			return ErrConflict
		}
		at, next := now(), NextStep(status)
		result, e := m.DB.Collection("referrals").UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: "version", Value: version}}, bson.D{
			{Key: "$set", Value: bson.D{{Key: "status", Value: status}, {Key: "public_note", Value: note}, {Key: "next_step", Value: next}, {Key: "updated_at", Value: at}}},
			{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}})
		if e != nil {
			return e
		}
		if result.MatchedCount != 1 {
			return ErrConflict
		}
		if _, e = m.DB.Collection("referral_status_history").InsertOne(ctx, bson.D{{Key: "referral_id", Value: id}, {Key: "from_status", Value: r.Status}, {Key: "to_status", Value: status}, {Key: "public_note", Value: note}, {Key: "actor_id", Value: actor}, {Key: "created_at", Value: at}}); e != nil {
			return e
		}
		if e = m.audit(ctx, actor, "referral.status", id, rid, bson.D{{Key: "from", Value: r.Status}, {Key: "to", Value: status}, {Key: "version", Value: version + 1}}); e != nil {
			return e
		}
		r.Status, r.PublicNote, r.NextStep, r.UpdatedAt = status, note, next, at
		r.Version++
		return nil
	})
	return r, e
}
func NextStep(s string) string {
	switch s {
	case "submitted":
		return "Menunggu verifikasi simulasi"
	case "verified":
		return "Pencarian tujuan simulasi"
	case "searching":
		return "Menunggu konfirmasi simulasi"
	case "confirmed":
		return "Lanjutkan skenario simulasi"
	default:
		return "Skenario simulasi selesai"
	}
}

// UpdateFacility updates one embedded service or bed item matched by key and version.
func (m *Mongo) UpdateFacility(ctx context.Context, actor, hospital, key string, version int, status *string, total, available *int, rid string) error {
	return m.transaction(ctx, func(ctx context.Context) error {
		if e := m.scope(ctx, actor, hospital); e != nil {
			return e
		}
		field, column, action := "beds", "category", "bed_availability.update"
		if status != nil {
			field, column, action = "services", "slug", "hospital_services.update"
		}
		at := now()
		set := bson.D{{Key: field + ".$.data_origin", Value: "simulation"}, {Key: field + ".$.source_url", Value: nil}, {Key: field + ".$.verified_at", Value: nil}, {Key: field + ".$.source_updated_at", Value: at}, {Key: field + ".$.synced_at", Value: at}}
		if status != nil {
			set = append(set, bson.E{Key: "services.$.status", Value: *status})
		} else {
			set = append(set, bson.E{Key: "beds.$.total", Value: total}, bson.E{Key: "beds.$.available", Value: available})
		}
		hospitals := m.DB.Collection("hospitals")
		result, e := hospitals.UpdateOne(ctx, bson.D{{Key: "_id", Value: hospital}, {Key: field, Value: bson.D{{Key: "$elemMatch", Value: bson.D{{Key: column, Value: key}, {Key: "version", Value: version}}}}}}, bson.D{{Key: "$set", Value: set}, {Key: "$inc", Value: bson.D{{Key: field + ".$.version", Value: 1}}}})
		if e != nil {
			return e
		}
		if result.MatchedCount != 1 {
			n, e := hospitals.CountDocuments(ctx, bson.D{{Key: "_id", Value: hospital}, {Key: field + "." + column, Value: key}})
			if e != nil {
				return e
			}
			if n == 0 {
				return ErrNotFound
			}
			return ErrConflict
		}
		return m.audit(ctx, actor, action, hospital, rid, bson.D{{Key: "key", Value: key}, {Key: "status", Value: status}, {Key: "total", Value: total}, {Key: "available", Value: available}, {Key: "version", Value: version + 1}})
	})
}

// Applied reports whether a named migration has been recorded.
func (m *Mongo) Applied(ctx context.Context, name string) (bool, error) {
	n, e := m.DB.Collection("schema_migrations").CountDocuments(ctx, bson.D{{Key: "_id", Value: name}})
	return n > 0, e
}
