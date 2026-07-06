package api_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	. "mindimprint/api/internal/api"
	"mindimprint/api/internal/store/sqlc"
)

func TestMaterialQueriesRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()

	task, err := q.CreateTask(ctx, sqlc.CreateTaskParams{UserID: mustUUID(SeedUserID.String()), Title: "material rt"})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	url := "https://example.com/a"
	m, err := q.CreateMaterial(ctx, sqlc.CreateMaterialParams{
		TaskID: task.ID, Kind: "article", Source: "fetched", Title: "标题",
		SourceUrl: &url, Blocks: []byte(`[{"id":"b0","text":"第一段。"}]`),
	})
	if err != nil {
		t.Fatalf("create material: %v", err)
	}
	if m.Scratch != "" || m.Kind != "article" {
		t.Fatalf("defaults wrong: %+v", m)
	}

	list, err := q.ListMaterialsByTask(ctx, task.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}

	upd, err := q.UpdateMaterialScratch(ctx, sqlc.UpdateMaterialScratchParams{ID: m.ID, TaskID: task.ID, Scratch: "记一笔"})
	if err != nil || upd.Scratch != "记一笔" {
		t.Fatalf("scratch update: %v scratch=%q", err, upd.Scratch)
	}

	// Ownership scope: wrong task id → no rows.
	if _, err := q.UpdateMaterialScratch(ctx, sqlc.UpdateMaterialScratchParams{ID: m.ID, TaskID: uuid.New(), Scratch: "x"}); err == nil {
		t.Fatalf("expected no-rows error for wrong task scope")
	}
}
