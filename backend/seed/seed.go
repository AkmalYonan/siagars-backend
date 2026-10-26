// Package seed installs synthetic fixtures without overwriting operator changes.
package seed

import (
	"context"
	"errors"
	"fmt"
	"siagars/backend/internal/repository"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func HospitalID(n int) string { return fmt.Sprintf("10000000-0000-4000-8000-%012d", n) }

const ReferralID = "20000000-0000-4000-8000-000000000001"

func key(id any) bson.D { return bson.D{{Key: "_id", Value: id}} }

// insertOnly upserts with $setOnInsert so existing documents are never modified.
func insertOnly(id any, doc any) mongo.WriteModel {
	return mongo.NewUpdateOneModel().SetFilter(key(id)).SetUpdate(bson.D{{Key: "$setOnInsert", Value: doc}}).SetUpsert(true)
}

// pushMissing appends an embedded item only when no item with the same key exists yet.
func pushMissing(id, field, column, value string, item any) mongo.WriteModel {
	return mongo.NewUpdateOneModel().SetFilter(bson.D{{Key: "_id", Value: id}, {Key: field + "." + column, Value: bson.D{{Key: "$ne", Value: value}}}}).SetUpdate(bson.D{{Key: "$push", Value: bson.D{{Key: field, Value: item}}}})
}

func Apply(ctx context.Context, p *repository.Mongo) error {
	if e := p.Ready(ctx); e != nil {
		return errors.New("database is not migrated")
	}
	s, e := p.Client.StartSession()
	if e != nil {
		return e
	}
	defer s.EndSession(context.Background())
	_, e = s.WithTransaction(ctx, func(ctx context.Context) (any, error) { return nil, apply(ctx, p.DB) })
	return e
}
func apply(ctx context.Context, db *mongo.Database) error {
	at := time.Now().UTC().Truncate(time.Millisecond)
	ordered := options.BulkWrite().SetOrdered(true)
	regions := [][4]string{{"31", "", "province", "DKI Jakarta"}, {"32", "", "province", "Jawa Barat"}, {"36", "", "province", "Banten"}, {"3171", "31", "city", "Jakarta Selatan"}, {"3271", "32", "city", "Kota Bogor"}, {"3275", "32", "city", "Kota Bekasi"}, {"3276", "32", "city", "Kota Depok"}, {"3671", "36", "city", "Kota Tangerang"}, {"3674", "36", "city", "Kota Tangerang Selatan"}}
	models := []mongo.WriteModel{}
	for _, r := range regions {
		var parent any
		if r[1] != "" {
			parent = r[1]
		}
		models = append(models, insertOnly(r[0], bson.D{{Key: "parent_code", Value: parent}, {Key: "kind", Value: r[2]}, {Key: "name", Value: r[3]}}))
	}
	if _, e := db.Collection("regions").BulkWrite(ctx, models, ordered); e != nil {
		return e
	}
	masters := [][3]string{{"icu", "ICU", "intensive"}, {"nicu", "NICU", "intensive"}, {"picu", "PICU", "intensive"}, {"igd-24h", "IGD 24 Jam", "emergency"}, {"ambulance", "Ambulans", "transport"}, {"rawat-inap", "Rawat Inap", "inpatient"}}
	models = []mongo.WriteModel{}
	for _, s := range masters {
		models = append(models, insertOnly(s[0], bson.D{{Key: "name", Value: s[1]}, {Key: "category", Value: s[2]}}))
	}
	if _, e := db.Collection("services").BulkWrite(ctx, models, ordered); e != nil {
		return e
	}
	cities := [][2]string{{"3171", "31"}, {"3271", "32"}, {"3275", "32"}, {"3276", "32"}, {"3671", "36"}, {"3674", "36"}}
	address := "Alamat sintetis untuk pengujian, bukan fasilitas nyata"
	models = []mongo.WriteModel{}
	for n := 1; n <= 30; n++ {
		c := cities[(n-1)%len(cities)]
		id := HospitalID(n)
		var bpjs *bool
		if n%3 != 0 {
			b := n%3 == 1
			bpjs = &b
		}
		// Synthetic coordinates are deliberately approximate, never a real facility location.
		lat, lng := -6.2-float64(n%6)*0.02, 106.7+float64(n%5)*0.03
		h := repository.HospitalDoc{Name: fmt.Sprintf("RS Simulasi Siaga %02d", n), Address: &address, CityCode: c[0], ProvinceCode: c[1], Latitude: &lat, Longitude: &lng, BPJS: bpjs, ProvenanceDoc: repository.Simulation(at), Services: []repository.ServiceDoc{}, Beds: []repository.BedDoc{}}
		models = append(models, insertOnly(id, h))
		for j, s := range masters {
			status := []string{"available", "limited", "unavailable", "unknown"}[(n+j)%4]
			if n <= 10 {
				status = "available"
			}
			models = append(models, pushMissing(id, "services", "slug", s[0], repository.ServiceDoc{Slug: s[0], Status: status, ProvenanceDoc: repository.Simulation(at), Version: 1}))
		}
		for j, category := range []string{"icu", "nicu", "picu", "general"} {
			var total, available *int
			if n <= 10 || n%3 != 0 {
				t, a := 10+j, (n+j)%4
				total, available = &t, &a
			}
			models = append(models, pushMissing(id, "beds", "category", category, repository.BedDoc{Category: category, Total: total, Available: available, ProvenanceDoc: repository.Simulation(at), Version: 1}))
		}
	}
	if _, e := db.Collection("hospitals").BulkWrite(ctx, models, ordered); e != nil {
		return e
	}
	models = []mongo.WriteModel{}
	for _, m := range BPJSDemoMembers {
		models = append(models, insertOnly(m.NIK, bson.D{{Key: "name", Value: m.Name}, {Key: "birth_date", Value: m.BirthDate}, {Key: "status", Value: m.Status}, {Key: "inactive_reason", Value: m.InactiveReason},
			{Key: "membership_code", Value: m.MembershipCode}, {Key: "membership_name", Value: m.MembershipName}, {Key: "care_class", Value: m.CareClass}, {Key: "facility_code", Value: m.FacilityCode}, {Key: "facility_name", Value: m.FacilityName}}))
	}
	if _, e := db.Collection("bpjs_participants").BulkWrite(ctx, models, ordered); e != nil {
		return e
	}
	result, e := db.Collection("referrals").UpdateOne(ctx, key(ReferralID), bson.D{{Key: "$setOnInsert", Value: bson.D{
		{Key: "owner_hospital_id", Value: HospitalID(1)}, {Key: "destination_hospital_id", Value: HospitalID(2)}, {Key: "tracking_token_hash", Value: repository.Hash("SR-DEMO-001")},
		{Key: "status", Value: "searching"}, {Key: "public_note", Value: "Pencarian tujuan dalam skenario simulasi"}, {Key: "next_step", Value: "Menunggu konfirmasi simulasi"},
		{Key: "version", Value: 1}, {Key: "is_demo", Value: true}, {Key: "updated_at", Value: at}}}}, options.UpdateOne().SetUpsert(true))
	if e != nil {
		return e
	}
	if result.UpsertedCount == 1 {
		history := []any{}
		for _, s := range []struct {
			from any
			to   string
		}{{nil, "submitted"}, {"submitted", "verified"}, {"verified", "searching"}} {
			// ObjectIDs are generated in order so equal timestamps keep their sequence.
			history = append(history, bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "referral_id", Value: ReferralID}, {Key: "from_status", Value: s.from}, {Key: "to_status", Value: s.to}, {Key: "public_note", Value: "Skenario publik sintetis"}, {Key: "actor_id", Value: nil}, {Key: "created_at", Value: at}})
		}
		if _, e = db.Collection("referral_status_history").InsertMany(ctx, history); e != nil {
			return e
		}
	}
	return nil
}
