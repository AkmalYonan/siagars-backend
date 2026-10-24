package repository

import (
	"siagars/backend/internal/service"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestHospitalMatchPushesEveryBaseFilter(t *testing.T) {
	now := time.Date(2026, 10, 24, 10, 0, 0, 0, time.UTC)
	yes := true
	s := service.Search{Dataset: "real", Province: "31", City: "3171", BPJS: &yes, Services: []string{"icu", "nicu"}, HasBeds: true}
	raw, e := bson.MarshalExtJSON(hospitalMatch(s, now), false, false)
	if e != nil {
		t.Fatal(e)
	}
	want := `{"$and":[{"source_active":{"$ne":false}},{"data_origin":{"$ne":"simulation"}},{"province_code":"31"},{"city_code":"3171"},{"bpjs":true},` +
		`{"services":{"$elemMatch":{"slug":"icu","status":{"$in":["available","limited"]}}}},{"services":{"$elemMatch":{"slug":"nicu","status":{"$in":["available","limited"]}}}},` +
		`{"beds":{"$elemMatch":{"available":{"$gt":0},"data_origin":{"$ne":"simulation"},"source_updated_at":{"$gt":{"$date":"2026-10-24T09:00:00Z"},"$lte":{"$date":"2026-10-24T10:00:00Z"}}}}}]}`
	if string(raw) != want {
		t.Fatalf("got  %s\nwant %s", raw, want)
	}
}

func TestHospitalMatchDatasets(t *testing.T) {
	now := time.Now()
	for dataset, want := range map[string]string{
		"":           `{"$and":[{"source_active":{"$ne":false}},{"data_origin":{"$ne":"simulation"}}]}`,
		"real":       `{"$and":[{"source_active":{"$ne":false}},{"data_origin":{"$ne":"simulation"}}]}`,
		"simulation": `{"$and":[{"source_active":{"$ne":false}},{"data_origin":"simulation"}]}`,
		"all":        `{"$and":[{"source_active":{"$ne":false}}]}`,
	} {
		raw, _ := bson.MarshalExtJSON(hospitalMatch(service.Search{Dataset: dataset}, now), false, false)
		if string(raw) != want {
			t.Errorf("%q: got %s", dataset, raw)
		}
	}
}

func TestSearchPipelineStages(t *testing.T) {
	now := time.Now()
	if n := len(searchPipeline(service.Search{Dataset: "real"}, now)); n != 1 {
		t.Fatalf("no q must be a single $match, got %d stages", n)
	}
	// The user's text only ever appears as a BSON string value, never as an operator or regex.
	p := searchPipeline(service.Search{Dataset: "real", Q: `{"$ne":1} .*`}, now)
	if len(p) != 5 {
		t.Fatalf("q needs match, 2 lookups, expr match and unset: %d", len(p))
	}
	raw, _ := bson.MarshalExtJSON(bson.D{{Key: "p", Value: p}}, false, false)
	if !strings.Contains(string(raw), `"{\"$ne\":1} .*"`) {
		t.Fatal(string(raw))
	}
}
