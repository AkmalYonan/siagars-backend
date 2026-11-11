package integration

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"siagars/backend/internal/repository"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSharedRateLimit(t *testing.T) {
	first := database(t)
	second, err := repository.Open(os.Getenv("TEST_MONGODB_URI"), first.DB.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := second.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	indexes, err := first.DB.Collection("rate_limits").Indexes().ListSpecifications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundTTL := false
	for _, index := range indexes {
		if index.Name == "rate_limits_expiry" && index.ExpireAfterSeconds != nil && *index.ExpireAfterSeconds == 0 {
			foundTTL = true
		}
	}
	if !foundTTL {
		t.Fatal("rate limit TTL index missing")
	}

	at := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
	const key = "lookup:test"
	const max = 5
	var wg sync.WaitGroup
	var allowed atomic.Int32
	for i := range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := first
			if i%2 != 0 {
				m = second
			}
			ok, err := m.TakeRateLimit(ctx, key, max, time.Minute, at)
			if err != nil {
				t.Error(err)
			}
			if ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if allowed.Load() != max {
		t.Fatalf("allowed = %d, want %d", allowed.Load(), max)
	}
	var bucket struct {
		ExpiresAt time.Time `bson:"expires_at"`
	}
	if err := first.DB.Collection("rate_limits").FindOne(ctx, bson.D{}).Decode(&bucket); err != nil || !bucket.ExpiresAt.Equal(at.Add(time.Minute)) {
		t.Fatalf("unexpected expiration %s, err %v", bucket.ExpiresAt, err)
	}
	ok, err := second.TakeRateLimit(ctx, key, max, time.Minute, at.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("new window: allowed = %v, err = %v", ok, err)
	}
	if err := second.ClearRateLimit(ctx, key, time.Minute, at); err != nil {
		t.Fatal(err)
	}
	ok, err = first.TakeRateLimit(ctx, key, max, time.Minute, at)
	if err != nil || !ok {
		t.Fatalf("cleared window: allowed = %v, err = %v", ok, err)
	}
}
