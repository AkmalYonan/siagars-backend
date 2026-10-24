package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"siagars/backend/internal/repository"
	"siagars/backend/seed"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// database creates an isolated siagars_test_<nanos> database and drops only that database afterwards.
func database(t *testing.T) *repository.Mongo {
	t.Helper()
	uri := os.Getenv("TEST_MONGODB_URI")
	if uri == "" {
		t.Skip("TEST_MONGODB_URI not set; MongoDB integration not executed")
	}
	name := fmt.Sprintf("siagars_test_%d", time.Now().UnixNano())
	p, e := repository.Open(uri, name)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if p.DB.Name() != name {
			t.Fatal("refusing to drop unexpected database")
		}
		if err := p.DB.Drop(ctx); err != nil {
			t.Error(err)
		}
		p.Close(ctx)
	})
	if e = p.Migrate(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = p.Migrate(context.Background()); e != nil {
		t.Fatal("idempotent migration", e)
	}
	return p
}
func validator(t *testing.T, p *repository.Mongo, collection string) bson.Raw {
	t.Helper()
	specs, e := p.DB.ListCollectionSpecifications(context.Background(), bson.D{{Key: "name", Value: collection}})
	if e != nil || len(specs) != 1 {
		t.Fatal("collection spec", e)
	}
	v, e := specs[0].Options.LookupErr("validator")
	if e != nil {
		t.Fatal(e)
	}
	return v.Document()
}
func TestMongoTransactions(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	if e := p.Ready(ctx); e != nil {
		t.Fatal("ready after migrate", e)
	}
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	if e := seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	if count, e := p.DB.Collection("hospitals").CountDocuments(ctx, bson.D{}); e != nil || count != 30 {
		t.Fatalf("seed %d %v", count, e)
	}
	actor := "30000000-0000-4000-8000-000000000001"
	other := "30000000-0000-4000-8000-000000000002"
	if e := p.CreateOperator(ctx, actor, "operator0@example.invalid", "test-hash", []string{seed.HospitalID(1)}); e != nil {
		t.Fatal(e)
	}
	if e := p.CreateOperator(ctx, other, "operator1@example.invalid", "test-hash", []string{seed.HospitalID(3)}); e != nil {
		t.Fatal(e)
	}
	if e := p.CreateOperator(ctx, "30000000-0000-4000-8000-000000000003", "operator0@example.invalid", "test-hash", []string{seed.HospitalID(1)}); !errors.Is(e, repository.ErrConflict) {
		t.Fatal("duplicate email", e)
	}
	if e := p.CreateOperator(ctx, "30000000-0000-4000-8000-000000000004", "operator4@example.invalid", "test-hash", []string{"10000000-0000-4000-8000-999999999999"}); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("missing hospital", e)
	}
	if _, e := p.UpdateReferral(ctx, other, seed.ReferralID, "confirmed", "simulasi", 1, "scope"); !errors.Is(e, repository.ErrForbidden) {
		t.Fatal("scope", e)
	}
	if refs, e := p.Referrals(ctx, other); e != nil || len(refs) != 0 {
		t.Fatal("cross-hospital read", refs, e)
	}
	// Force audit insert failure to prove current status and history roll back.
	original := validator(t, p, "audit_logs")
	failing := bson.D{{Key: "$and", Value: bson.A{original, bson.D{{Key: "action", Value: bson.D{{Key: "$ne", Value: "referral.status"}}}}}}}
	if e := p.DB.RunCommand(ctx, bson.D{{Key: "collMod", Value: "audit_logs"}, {Key: "validator", Value: failing}}).Err(); e != nil {
		t.Fatal(e)
	}
	if _, e := p.UpdateReferral(ctx, actor, seed.ReferralID, "confirmed", "simulasi", 1, "rollback"); e == nil {
		t.Fatal("expected rollback")
	}
	ref, history, e := p.Lookup(ctx, "SR-DEMO-001")
	if e != nil || ref.Version != 1 || ref.Status != "searching" || len(history) != 3 || history[0].ToStatus != "submitted" || history[2].ToStatus != "searching" {
		t.Fatal("rollback failed", ref, history, e)
	}
	if e = p.DB.RunCommand(ctx, bson.D{{Key: "collMod", Value: "audit_logs"}, {Key: "validator", Value: original}}).Err(); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.UpdateReferral(ctx, actor, seed.ReferralID, "confirmed", "simulasi", 1, "race")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, repository.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatal(success, conflicts)
	}
	ref, history, e = p.Lookup(ctx, "SR-DEMO-001")
	if e != nil || ref.Version != 2 || ref.Status != "confirmed" || len(history) != 4 || history[3].ToStatus != "confirmed" {
		t.Fatal("atomic history", ref, history, e)
	}
	if audits, e := p.DB.Collection("audit_logs").CountDocuments(ctx, bson.D{{Key: "action", Value: "referral.status"}}); e != nil || audits != 1 {
		t.Fatal("audit count", audits, e)
	}
	if e = seed.Apply(ctx, p); e != nil {
		t.Fatal(e)
	}
	if ref, _, e = p.Lookup(ctx, "SR-DEMO-001"); e != nil || ref.Version != 2 {
		t.Fatal("seed overwrote update")
	}
	if _, _, e = p.Lookup(ctx, "SR-DEMO-404"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("unknown code", e)
	}
	if e = p.CreateSession(ctx, actor, "token", "csrf", time.Now().Add(time.Hour), ""); e != nil {
		t.Fatal(e)
	}
	if op, csrf, e := p.Session(ctx, "token"); e != nil || op.ID != actor || len(op.HospitalIDs) != 1 || csrf != repository.Hash("csrf") {
		t.Fatal(op, e)
	}
	if e = p.CreateSession(ctx, actor, "rotated", "csrf", time.Now().Add(time.Hour), "token"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = p.Session(ctx, "token"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("rotation", e)
	}
	if e = p.Revoke(ctx, "rotated"); e != nil {
		t.Fatal(e)
	}
	if _, _, e = p.Session(ctx, "rotated"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("revoked", e)
	}
	if e = p.CreateSession(ctx, actor, "expired", "csrf", time.Now().Add(-time.Second), ""); e != nil {
		t.Fatal(e)
	}
	if _, _, e = p.Session(ctx, "expired"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal("expired", e)
	}
	status := "limited"
	if e = p.UpdateFacility(ctx, other, seed.HospitalID(1), "icu", 1, &status, nil, nil, "scope"); !errors.Is(e, repository.ErrForbidden) {
		t.Fatal(e)
	}
	if e = p.UpdateFacility(ctx, actor, seed.HospitalID(1), "icu", 1, &status, nil, nil, "update"); e != nil {
		t.Fatal(e)
	}
	if e = p.UpdateFacility(ctx, actor, seed.HospitalID(1), "icu", 1, &status, nil, nil, "stale"); !errors.Is(e, repository.ErrConflict) {
		t.Fatal(e)
	}
	if e = p.UpdateFacility(ctx, actor, seed.HospitalID(1), "missing", 1, &status, nil, nil, "missing"); !errors.Is(e, repository.ErrNotFound) {
		t.Fatal(e)
	}
	total, available := 5, 0
	if e = p.UpdateFacility(ctx, actor, seed.HospitalID(1), "nicu", 1, nil, &total, &available, "zero"); e != nil {
		t.Fatal(e)
	}
	if e = p.UpdateFacility(ctx, actor, seed.HospitalID(1), "picu", 1, nil, nil, nil, "unknown"); e != nil {
		t.Fatal(e)
	}
	if _, e = p.DB.Collection("hospitals").UpdateOne(ctx, bson.D{{Key: "_id", Value: seed.HospitalID(1)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "beds.0.total", Value: 1}, {Key: "beds.0.available", Value: 2}}}}); e == nil {
		t.Fatal("bed constraint missing")
	}
	if _, e = p.DB.Collection("hospitals").UpdateOne(ctx, bson.D{{Key: "_id", Value: seed.HospitalID(1)}}, bson.D{{Key: "$set", Value: bson.D{{Key: "services.0.status", Value: "maybe"}}}}); e == nil {
		t.Fatal("service enum missing")
	}
	hs, e := p.Hospitals(ctx)
	if e != nil || len(hs) != 30 || len(hs[0].Services) != 5 || len(hs[0].Beds) != 4 {
		t.Fatal("DTO query", e)
	}
	h := hs[0]
	if h.ID != seed.HospitalID(1) || h.CityName == "" || h.ProvinceName == "" || h.Services[0].Name == "" {
		t.Fatal("joined names", h)
	}
	for _, b := range h.Beds {
		switch b.Category {
		case "nicu":
			if b.Total == nil || *b.Total != 5 || b.Available == nil || *b.Available != 0 || b.Version != 2 || b.DataOrigin != "simulation" || b.VerifiedAt != nil {
				t.Fatal("bed zero", b)
			}
		case "picu":
			if b.Total != nil || b.Available != nil {
				t.Fatal("bed null", b)
			}
		}
	}
	for _, s := range h.Services {
		if s.Slug == "icu" && (s.Status != "limited" || s.Version != 2) {
			t.Fatal("service update", s)
		}
	}
}
