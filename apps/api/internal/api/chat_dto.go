package api

// chat_dto.go — Task 5 (Slice 11): the Chat surface's own wire DTOs. Chat is
// project-free (agent/chat_step.go), so these mirror the studio package's
// camelCase convention (studioState.ts contract shape) rather than this
// package's snake_case convention (classes_dto.go/course_dto.go) — Chat's
// web client (Task 6) is a fresh surface, not bound to the older snake_case
// contract.

import (
	"time"

	"mindimprint/api/internal/store/sqlc"
)

// ChatThreadDTO is one row of GET/POST /chat/threads.
type ChatThreadDTO struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

// ChatMessageDTO is one row of GET /chat/threads/{id}/messages.
type ChatMessageDTO struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Modality  string `json:"modality"`
	CreatedAt string `json:"createdAt"`
}

// ChatCardOfferDTO is the SSE `card` frame's wire shape for a fresh chat card
// offer (agent.ChatCardOffer) — kept as a named type for the DTO parity test
// even though postChatTurn (chat.go) emits its fields inline via
// studioEmitter.Card, not by marshaling this struct directly.
type ChatCardOfferDTO struct {
	CardInstanceID string `json:"cardInstanceId"`
	CardID         string `json:"cardId"`
	MaterialID     string `json:"materialId"`
}

// toChatThreadDTO converts a sqlc.ChatThread row. ID/UserID are plain
// uuid.UUID (never pgtype.UUID) — chat_thread.id and .user_id are NOT NULL.
func toChatThreadDTO(t sqlc.ChatThread) ChatThreadDTO {
	return ChatThreadDTO{ID: t.ID.String(), Title: t.Title, CreatedAt: t.CreatedAt.Format(time.RFC3339)}
}

// toChatMessageDTO converts a sqlc.ChatMessage row.
func toChatMessageDTO(m sqlc.ChatMessage) ChatMessageDTO {
	return ChatMessageDTO{
		ID: m.ID.String(), Role: m.Role, Content: m.Content, Modality: m.Modality,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
}
