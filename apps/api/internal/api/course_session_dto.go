package api

// course_session_dto.go — Task 7 (Slice 12): the Course session surface's own
// wire DTOs. Mirrors chat_dto.go's camelCase convention (a fresh runtime
// surface) rather than course_dto.go's older snake_case convention — page
// position (course_progress) stays snake_case via the existing progress
// endpoints; this is runtime state (DEC-12.1: pages are content, phases are
// runtime).

import (
	"time"

	"mindimprint/api/internal/store/sqlc"
)

// CourseSessionDTO is the Course surface's runtime state: which phase the
// student is in, this session's dialogue, and any card offer still open
// (proposed/active). Page position stays in course_progress and ships
// through the existing progress endpoints.
type CourseSessionDTO struct {
	ID         string               `json:"id"`
	CourseID   string               `json:"courseId"`
	Phase      string               `json:"phase"`
	PhaseTitle string               `json:"phaseTitle"`
	Status     string               `json:"status"`
	Messages   []CourseMessageDTO   `json:"messages"`
	OpenCards  []CourseCardOfferDTO `json:"openCards"`
}

// CourseMessageDTO is one row of a course session's dialogue.
type CourseMessageDTO struct {
	ID        string `json:"id"`
	Phase     string `json:"phase"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// CourseCardOfferDTO is one card offer the session has not yet dispositioned
// (status proposed or active) — carried on session load so a reload can
// restore an offer that would otherwise live only in React state (Slice-12
// whole-branch Critical-2: without this, a reload during `guided` erased the
// offer and the card_dispositioned floor could never be met again).
type CourseCardOfferDTO struct {
	CardInstanceID string `json:"cardInstanceId"`
	CardID         string `json:"cardId"`
	MaterialID     string `json:"materialId"`
}

// toCourseSessionDTO converts a sqlc.CourseSession row + its resolved
// phaseTitle (the skill Contract.Title for the session's current phase,
// resolved by the caller since it needs the skill) + the session's messages +
// its open card offers.
func toCourseSessionDTO(s sqlc.CourseSession, phaseTitle string, messages []CourseMessageDTO, openCards []CourseCardOfferDTO) CourseSessionDTO {
	return CourseSessionDTO{
		ID:         s.ID.String(),
		CourseID:   s.CourseID.String(),
		Phase:      s.Phase,
		PhaseTitle: phaseTitle,
		Status:     s.Status,
		Messages:   messages,
		OpenCards:  openCards,
	}
}

// toCourseMessageDTO converts a sqlc.CourseMessage row.
func toCourseMessageDTO(m sqlc.CourseMessage) CourseMessageDTO {
	return CourseMessageDTO{
		ID:        m.ID.String(),
		Phase:     m.Phase,
		Role:      m.Role,
		Content:   m.Content,
		CreatedAt: m.CreatedAt.Format(time.RFC3339),
	}
}
