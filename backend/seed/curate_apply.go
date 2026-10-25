package seed

import (
	"context"
	"fmt"
	"siagars/backend/internal/repository"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// CurateOutcome counts what happened to one hospital. Existing values are never overwritten, so
// operator updates and earlier curation always win over the file.
type CurateOutcome struct {
	ExternalID                  string
	Found, NameMismatch         bool
	BPJSSet, BPJSKept           bool
	ServicesAdded, ServicesKept []string
	BedsAdded, BedsKept         []string
	Missing                     []string // service slugs absent from the master collection
}

// ApplyCuration adds curated facts to hospitals that already exist (matched by namespace + external_id).
// Every write is a conditional $set/$push, so repeating it is a no-op and nothing existing is replaced.
// Hospital-level origin, verified_at and operator versions are never touched; new items are created with
// data_origin=curated, version 1 and verified_at=null.
func ApplyCuration(ctx context.Context, db *mongo.Database, records []CuratedHospital, at time.Time) ([]CurateOutcome, error) {
	at = at.UTC().Truncate(time.Millisecond)
	masters := map[string]bool{}
	cur, e := db.Collection("services").Find(ctx, bson.D{})
	if e != nil {
		return nil, e
	}
	var rows []struct {
		ID string `bson:"_id"`
	}
	if e = cur.All(ctx, &rows); e != nil {
		return nil, e
	}
	for _, r := range rows {
		masters[r.ID] = true
	}
	hospitals := db.Collection("hospitals")
	var out []CurateOutcome
	for _, r := range records {
		o := CurateOutcome{ExternalID: r.ExternalID}
		var h struct {
			ID   string `bson:"_id"`
			Name string `bson:"name"`
		}
		match := bson.D{{Key: "source_namespace", Value: r.SourceNamespace}, {Key: "external_id", Value: r.ExternalID}}
		switch e = hospitals.FindOne(ctx, match).Decode(&h); {
		case e == mongo.ErrNoDocuments:
			out = append(out, o)
			continue
		case e != nil:
			return out, e
		}
		o.Found = true
		if r.Name != "" && !strings.EqualFold(strings.TrimSpace(h.Name), r.Name) {
			// A mismatched name suggests a wrong external_id; touch nothing.
			o.NameMismatch = true
			out = append(out, o)
			continue
		}
		if r.BPJS != nil {
			res, e := hospitals.UpdateOne(ctx, bson.D{{Key: "_id", Value: h.ID}, {Key: "bpjs", Value: nil}}, bson.D{{Key: "$set", Value: bson.D{{Key: "bpjs", Value: *r.BPJS}, {Key: "bpjs_source_url", Value: r.SourceURL}, {Key: "bpjs_source_updated_at", Value: r.SourceUpdatedAt.UTC()}}}})
			if e != nil {
				return out, e
			}
			o.BPJSSet, o.BPJSKept = res.ModifiedCount == 1, res.ModifiedCount == 0
		}
		for _, s := range r.Services {
			if !masters[s.Slug] {
				o.Missing = append(o.Missing, s.Slug)
				continue
			}
			doc := repository.ServiceDoc{Slug: s.Slug, Status: s.Status, ProvenanceDoc: curatedProvenance(s.SourceURL, s.SourceUpdatedAt, at), Version: 1}
			added, e := pushIfAbsent(ctx, hospitals, h.ID, "services", "slug", s.Slug, doc)
			if e != nil {
				return out, e
			}
			o.ServicesAdded, o.ServicesKept = tally(o.ServicesAdded, o.ServicesKept, s.Slug, added)
		}
		for _, b := range r.Beds {
			doc := repository.BedDoc{Category: b.Category, Total: b.Total, Available: b.Available, ProvenanceDoc: curatedProvenance(b.SourceURL, b.SourceUpdatedAt, at), Version: 1}
			added, e := pushIfAbsent(ctx, hospitals, h.ID, "beds", "category", b.Category, doc)
			if e != nil {
				return out, e
			}
			o.BedsAdded, o.BedsKept = tally(o.BedsAdded, o.BedsKept, b.Category, added)
		}
		out = append(out, o)
	}
	return out, nil
}

// curatedProvenance never sets verified_at; only a human check after the fact may do that.
func curatedProvenance(sourceURL string, sourceUpdatedAt, syncedAt time.Time) repository.ProvenanceDoc {
	u, s := sourceURL, sourceUpdatedAt.UTC()
	return repository.ProvenanceDoc{DataOrigin: "curated", SourceURL: &u, SourceUpdatedAt: &s, SyncedAt: &syncedAt}
}

func pushIfAbsent(ctx context.Context, c *mongo.Collection, id, field, column, value string, item any) (bool, error) {
	res, e := c.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}, {Key: field + "." + column, Value: bson.D{{Key: "$ne", Value: value}}}}, bson.D{{Key: "$push", Value: bson.D{{Key: field, Value: item}}}})
	if e != nil {
		return false, fmt.Errorf("%s %s: %w", field, value, e)
	}
	return res.ModifiedCount == 1, nil
}

func tally(added, kept []string, v string, wasAdded bool) ([]string, []string) {
	if wasAdded {
		return append(added, v), kept
	}
	return added, append(kept, v)
}
