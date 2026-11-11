package repository

import (
	"context"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func rateLimitBucket(key string, window time.Duration, at time.Time) (string, time.Time) {
	start := at.UTC().Truncate(window)
	return Hash(key) + ":" + strconv.FormatInt(start.Unix(), 10), start.Add(window)
}

// TakeRateLimit atomically charges one request against a shared, fixed window.
func (m *Mongo) TakeRateLimit(ctx context.Context, key string, max int, window time.Duration, at time.Time) (bool, error) {
	id, expires := rateLimitBucket(key, window, at)
	collection := m.DB.Collection("rate_limits")
	filter := bson.D{{Key: "_id", Value: id}}
	update := bson.D{
		{Key: "$inc", Value: bson.D{{Key: "count", Value: int64(1)}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "expires_at", Value: expires}}},
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var result struct {
		Count int64 `bson:"count"`
	}
	err := collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if mongo.IsDuplicateKeyError(err) {
		// Another instance created the bucket during the upsert.
		err = collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	}
	return result.Count <= int64(max) && err == nil, err
}

func (m *Mongo) ClearRateLimit(ctx context.Context, key string, window time.Duration, at time.Time) error {
	id, _ := rateLimitBucket(key, window, at)
	_, err := m.DB.Collection("rate_limits").DeleteOne(ctx, bson.D{{Key: "_id", Value: id}})
	return err
}
