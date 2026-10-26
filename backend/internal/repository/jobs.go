package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RestampSimulation sets source_updated_at and synced_at to at on every simulation item: embedded
// services and beds with data_origin=simulation, and seeded hospitals whose own origin is simulation.
// The array filters match on data_origin, so curated, satusehat_* or any other item is never written,
// and verified_at is never set. It returns how many simulation hospitals were restamped.
func (m *Mongo) RestampSimulation(ctx context.Context, at time.Time) (int, error) {
	at = at.UTC().Truncate(time.Millisecond)
	hospitals := m.DB.Collection("hospitals")
	for _, field := range []string{"services", "beds"} {
		_, e := hospitals.UpdateMany(ctx,
			bson.D{{Key: field + ".data_origin", Value: "simulation"}},
			bson.D{{Key: "$set", Value: bson.D{{Key: field + ".$[x].source_updated_at", Value: at}, {Key: field + ".$[x].synced_at", Value: at}}}},
			options.UpdateMany().SetArrayFilters([]any{bson.D{{Key: "x.data_origin", Value: "simulation"}}}))
		if e != nil {
			return 0, e
		}
	}
	r, e := hospitals.UpdateMany(ctx, bson.D{{Key: "data_origin", Value: "simulation"}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "source_updated_at", Value: at}, {Key: "synced_at", Value: at}}}})
	if e != nil {
		return 0, e
	}
	return int(r.MatchedCount), nil
}

// CountSimulationItems counts embedded simulation services and beds stamped exactly at.
func (m *Mongo) CountSimulationItems(ctx context.Context, at time.Time) (services, beds int, err error) {
	at = at.UTC().Truncate(time.Millisecond)
	count := func(field string) (int, error) {
		cur, e := m.DB.Collection("hospitals").Aggregate(ctx, bson.A{
			bson.D{{Key: "$unwind", Value: "$" + field}},
			bson.D{{Key: "$match", Value: bson.D{{Key: field + ".data_origin", Value: "simulation"}, {Key: field + ".synced_at", Value: at}}}},
			bson.D{{Key: "$count", Value: "n"}}})
		if e != nil {
			return 0, e
		}
		var r struct {
			N int `bson:"n"`
		}
		if cur.Next(ctx) {
			if e = cur.Decode(&r); e != nil {
				return 0, e
			}
		}
		return r.N, cur.Err()
	}
	if services, err = count("services"); err != nil {
		return
	}
	beds, err = count("beds")
	return
}

// AuditJob records one run of a system job (actor such as "system:cron" or "system:curate") against an
// entity label such as "simulation", with its counts. It never carries personal data.
func (m *Mongo) AuditJob(ctx context.Context, actor, action, entity, rid string, counts map[string]int) error {
	changes := bson.D{}
	for _, k := range []string{"imported_hospitals", "simulation_hospitals", "services", "beds", "created", "updated", "unchanged", "failed"} {
		if v, ok := counts[k]; ok {
			changes = append(changes, bson.E{Key: k, Value: v})
		}
	}
	return m.audit(ctx, actor, action, entity, rid, changes)
}
