package api_test

import (
	"bytes"
	"context"
	"testing"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestSetCardAnchorsRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "anchors rt"})
	if err != nil {
		t.Fatal(err)
	}
	card, err := q.CreateCardInstance(ctx, sqlc.CreateCardInstanceParams{CardID: "sift_craap", TaskID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(card.Anchors, []byte("[]")) {
		t.Fatalf("default anchors want []; got %s", card.Anchors)
	}

	anchors := []byte(`[{"id":"a0","material_id":"m1","block_id":"b0","start":0,"end":5,"quote":"美航局发现","dimension":"权威性","author":"ai","question":"可信吗？","answer":""}]`)
	upd, err := q.SetCardAnchors(ctx, sqlc.SetCardAnchorsParams{ID: card.ID, TaskID: task.ID, Anchors: anchors})
	if err != nil {
		t.Fatalf("set anchors: %v", err)
	}
	if !bytes.Contains(upd.Anchors, []byte("美航局发现")) {
		t.Fatalf("anchors not persisted: %s", upd.Anchors)
	}
}
