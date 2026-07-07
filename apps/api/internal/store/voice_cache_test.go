package store_test

import (
	"context"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func TestVoiceTTSCacheRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testcontainers integration in -short mode")
	}
	pool := newStoreTestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	_, err := q.UpsertVoiceTTSCache(ctx, sqlc.UpsertVoiceTTSCacheParams{
		Key:   "k1",
		Audio: []byte{1, 2, 3},
		Voice: "v",
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := q.GetVoiceTTSCache(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Audio) != string([]byte{1, 2, 3}) {
		t.Fatalf("audio = % x", got.Audio)
	}
	if got.Voice != "v" {
		t.Fatalf("voice = %q, want v", got.Voice)
	}

	// Second upsert on the same key must overwrite, not error.
	_, err = q.UpsertVoiceTTSCache(ctx, sqlc.UpsertVoiceTTSCacheParams{
		Key:   "k1",
		Audio: []byte{4, 5, 6},
		Voice: "v2",
	})
	if err != nil {
		t.Fatal(err)
	}

	got2, err := q.GetVoiceTTSCache(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if string(got2.Audio) != string([]byte{4, 5, 6}) {
		t.Fatalf("audio after overwrite = % x", got2.Audio)
	}
	if got2.Voice != "v2" {
		t.Fatalf("voice after overwrite = %q, want v2", got2.Voice)
	}
}
