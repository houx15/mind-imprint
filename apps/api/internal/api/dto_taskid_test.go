package api

import (
	"testing"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// Regression for the P0 reload bug: the task-detail DTOs must carry task_id.
// The web store filters rehydrated messages/cards by `x.task_id === taskId`
// (WorkspaceView.tsx); when the DTO omits task_id, every rehydrated row is
// dropped and the task opens empty after a reload — even though the DB has it.
func TestMessageAndCardDTOsCarryTaskID(t *testing.T) {
	tid := uuid.New()

	msgs := toMessageDTOs([]sqlc.Message{{ID: uuid.New(), TaskID: tid, Role: "user", Content: "hi"}})
	if len(msgs) != 1 {
		t.Fatalf("want 1 message dto, got %d", len(msgs))
	}
	if msgs[0].TaskID != tid.String() {
		t.Fatalf("messageDTO.TaskID = %q, want %q", msgs[0].TaskID, tid.String())
	}

	card := toCardDTO(sqlc.CardInstance{ID: uuid.New(), TaskID: tid, CardID: "sift_craap", Status: "active"})
	if card.TaskID != tid.String() {
		t.Fatalf("cardDTO.TaskID = %q, want %q", card.TaskID, tid.String())
	}
}
