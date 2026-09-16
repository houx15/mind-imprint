package api

import (
	"fmt"
	"github.com/google/uuid"
	"mindimprint/api/internal/store/sqlc"
	"testing"
	"time"
)

func TestSupersededArtifactsRespectVersionRelations(t *testing.T) {
	atom := uuid.New()
	source := sqlc.PblArtifact{ID: uuid.New(), AtomID: atom, Kind: "draft", CreatedAt: time.Now()}
	revision := source
	revision.ID = uuid.New()
	revision.CreatedAt = source.CreatedAt.Add(time.Second)
	revision.Payload = []byte(fmt.Sprintf(`{"baseArtifactId":%q}`, source.ID.String()))
	newer := revision
	newer.ID = uuid.New()
	newer.CreatedAt = revision.CreatedAt.Add(time.Second)
	newer.Payload = []byte(fmt.Sprintf(`{"replacesArtifactId":%q}`, revision.ID.String()))
	ids := supersededArtifactIDs([]sqlc.PblArtifact{newer, source, revision})
	if !ids[source.ID] || !ids[revision.ID] || ids[newer.ID] {
		t.Fatalf("wrong chain: %v", ids)
	}
	for _, mutate := range []func(*sqlc.PblArtifact){func(r *sqlc.PblArtifact) { r.AtomID = uuid.New() }, func(r *sqlc.PblArtifact) { r.Kind = "spec" }, func(r *sqlc.PblArtifact) { r.CreatedAt = source.CreatedAt }, func(r *sqlc.PblArtifact) { r.Payload = []byte(`{"body":"independent"}`) }} {
		invalid := revision
		mutate(&invalid)
		if len(supersededArtifactIDs([]sqlc.PblArtifact{source, invalid})) != 0 {
			t.Fatal("unrelated version hid a pending artifact")
		}
	}
}
