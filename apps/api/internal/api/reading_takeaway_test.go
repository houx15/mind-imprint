package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
)

func TestReadingBriefAndTakeawayRoundTrip(t *testing.T) {
	pool := newAPITestPool(t)
	q := sqlc.New(pool)
	ctx := context.Background()
	projectID := mustUUID(seedProjectID)

	// Seed a reference in the project. tags/search_hints are jsonb NOT NULL
	// (DEFAULT '[]' only applies when the column is omitted from the INSERT,
	// but CreateReference's :one always supplies all columns) — a nil []byte
	// param encodes as SQL NULL, so they must be set explicitly here.
	ref, err := q.CreateReference(ctx, sqlc.CreateReferenceParams{
		ProjectID: projectID, Title: "NASA 报告",
		Tags: []byte("[]"), SearchHints: []byte("[]"),
	})
	if err != nil {
		t.Fatalf("CreateReference: %v", err)
	}

	reason, focus, phase := "验证碳排放是否构成反例", "看它引用了谁", "反例检验"
	updated, err := q.UpdateReadingBrief(ctx, sqlc.UpdateReadingBriefParams{
		ID: ref.ID, ProjectID: projectID,
		ReadingReason: &reason, ReadingFocus: &focus, PhaseTag: &phase,
	})
	if err != nil {
		t.Fatalf("UpdateReadingBrief: %v", err)
	}
	if updated.ReadingReason == nil || *updated.ReadingReason != reason {
		t.Fatalf("reading_reason not persisted: %+v", updated.ReadingReason)
	}
	if updated.TakeawayFinalizedAt.Valid {
		t.Fatalf("finalized_at should be null before finalize")
	}

	obj := map[string]any{"proposal_impact": "把它作为让步段的证据"}
	raw, _ := json.Marshal(obj)
	fin, err := q.FinalizeReadingTakeaway(ctx, sqlc.FinalizeReadingTakeawayParams{
		ID: ref.ID, ProjectID: projectID, Takeaway: raw,
	})
	if err != nil {
		t.Fatalf("FinalizeReadingTakeaway: %v", err)
	}
	if !fin.TakeawayFinalizedAt.Valid {
		t.Fatalf("finalized_at should be set after finalize")
	}
	var got map[string]any
	if err := json.Unmarshal(fin.Takeaway, &got); err != nil || got["proposal_impact"] != obj["proposal_impact"] {
		t.Fatalf("takeaway round-trip mismatch: %v / %v", err, got)
	}

	// Scope guard: wrong project id must not find it.
	if _, err := q.GetReferenceForProject(ctx, sqlc.GetReferenceForProjectParams{
		ID: ref.ID, ProjectID: uuid.New(),
	}); err == nil {
		t.Fatalf("GetReferenceForProject should 404 across projects")
	}
}
