package integration

import (
	"context"
	"siagars/backend/internal/repository"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestMigration007RegionCodes replays 007 over pre-0.3.1 data: dotted seed codes next to the undotted
// MSI code for the same city. Afterwards one undotted code per region remains, the MSI entry (already
// present) wins, and hospitals point at the undotted codes. A second run changes nothing.
func TestMigration007RegionCodes(t *testing.T) {
	p := database(t)
	ctx := context.Background()
	regions, hospitals := p.DB.Collection("regions"), p.DB.Collection("hospitals")
	for _, r := range []bson.D{
		{{Key: "_id", Value: "31"}, {Key: "parent_code", Value: nil}, {Key: "kind", Value: "province"}, {Key: "name", Value: "DKI Jakarta"}},
		{{Key: "_id", Value: "31.71"}, {Key: "parent_code", Value: "31"}, {Key: "kind", Value: "city"}, {Key: "name", Value: "Jakarta Selatan"}},
		{{Key: "_id", Value: "3171"}, {Key: "parent_code", Value: "31"}, {Key: "kind", Value: "city"}, {Key: "name", Value: "KOTA ADM. JAKARTA SELATAN"}},
		{{Key: "_id", Value: "32"}, {Key: "parent_code", Value: nil}, {Key: "kind", Value: "province"}, {Key: "name", Value: "Jawa Barat"}},
		{{Key: "_id", Value: "32.75"}, {Key: "parent_code", Value: "32"}, {Key: "kind", Value: "city"}, {Key: "name", Value: "Kota Bekasi"}},
	} {
		if _, e := regions.InsertOne(ctx, r); e != nil {
			t.Fatal(e)
		}
	}
	at := repository.Simulation(time.Now().UTC().Truncate(time.Millisecond))
	for i, city := range []string{"31.71", "3171", "32.75"} {
		h := repository.HospitalDoc{ID: []string{"50000000-0000-4000-8000-000000000001", "50000000-0000-4000-8000-000000000002", "50000000-0000-4000-8000-000000000003"}[i], Name: "RS Migrasi", CityCode: city, ProvinceCode: city[:2], ProvenanceDoc: at, Services: []repository.ServiceDoc{}, Beds: []repository.BedDoc{}}
		if _, e := hospitals.InsertOne(ctx, h); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := p.DB.Collection("schema_migrations").DeleteOne(ctx, bson.D{{Key: "_id", Value: "007_region_codes.json"}}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e := p.Migrate(ctx); e != nil {
			t.Fatal(e)
		}
	}
	got, e := p.Regions(ctx)
	if e != nil {
		t.Fatal(e)
	}
	byCode := map[string]string{}
	for _, r := range got {
		if _, dup := byCode[r.Code]; dup {
			t.Fatal("kode ganda:", r.Code)
		}
		byCode[r.Code] = r.Name
		if r.ParentCode != nil && len(*r.ParentCode) != 2 {
			t.Fatal("parent_code tidak dinormalisasi:", *r.ParentCode)
		}
	}
	if len(byCode) != 4 || byCode["3171"] != "KOTA ADM. JAKARTA SELATAN" || byCode["3275"] != "Kota Bekasi" {
		t.Fatalf("wilayah setelah 007: %v", byCode)
	}
	for _, c := range []struct {
		code string
		want int64
	}{{"3171", 2}, {"3275", 1}, {"31.71", 0}, {"32.75", 0}} {
		if n, _ := hospitals.CountDocuments(ctx, bson.D{{Key: "city_code", Value: c.code}}); n != c.want {
			t.Errorf("RS dengan city_code %s: %d, ingin %d", c.code, n, c.want)
		}
	}
	// The city filter keeps working on the unified code, and a dotted code is no longer a known region.
	if ok, _ := p.RegionExists(ctx, "3275", "city"); !ok {
		t.Fatal("3275 tidak dikenal")
	}
	if ok, _ := p.RegionExists(ctx, "32.75", "city"); ok {
		t.Fatal("kode bertitik masih dikenal")
	}
}
