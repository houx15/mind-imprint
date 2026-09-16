package api

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"mindimprint/api/internal/store/sqlc"
)

func TestMissionEvidenceSnapshotPreservesInstructionsAndStates(t *testing.T) {
	stamp := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	rows := []sqlc.PblMissionItem{
		{ID: uuid.New(), ToolID: uuid.New(), Prompt: "当前：饭和菜可多选", WantKind: "observation"},
		{ID: uuid.New(), Prompt: "当前已勾选，不代表内容已证实", WantKind: "question", DoneAt: stamp, EditedByStudent: true},
		{ID: uuid.New(), Prompt: "历史未勾选", WantKind: "observation", SupersededAt: stamp},
		{ID: uuid.New(), Prompt: "历史已勾选", WantKind: "quote", DoneAt: stamp, SupersededAt: stamp},
	}
	full := missionSnapshot(rows)
	compact, err := missionEvidenceSnapshot(full)
	if err != nil {
		t.Fatal(err)
	}
	var got []struct {
		Prompt        string `json:"prompt"`
		Kind          string `json:"kind"`
		Done          bool   `json:"marked_done"`
		Historical    bool   `json:"historical"`
		StudentEdited bool   `json:"student_edited"`
	}
	if err := json.Unmarshal([]byte(compact), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(rows) {
		t.Fatalf("lost rows: %s", compact)
	}
	for i, row := range rows {
		if got[i].Prompt != row.Prompt || got[i].Kind != row.WantKind || got[i].Done != row.DoneAt.Valid || got[i].Historical != row.SupersededAt.Valid || got[i].StudentEdited != row.EditedByStudent {
			t.Fatalf("lost source meaning at %d: %+v", i, got[i])
		}
	}
	if len(compact) >= len(full) {
		t.Fatal("projection did not reduce source")
	}
	t.Logf("snapshot bytes: %d -> %d", len(full), len(compact))
	if _, err := missionEvidenceSnapshot("invalid"); err == nil {
		t.Fatal("invalid snapshot silently accepted")
	}
}
