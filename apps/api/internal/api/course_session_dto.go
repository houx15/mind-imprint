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
// student is in, and this session's dialogue. Page position stays in
// course_progress and ships through the existing progress endpoints.
type CourseSessionDTO struct {
	ID         string             `json:"id"`
	CourseID   string             `json:"courseId"`
	Phase      string             `json:"phase"`
	PhaseTitle string             `json:"phaseTitle"`
	Status     string             `json:"status"`
	Messages   []CourseMessageDTO `json:"messages"`
}

// CourseMessageDTO is one row of a course session's dialogue.
type CourseMessageDTO struct {
	ID        string `json:"id"`
	Phase     string `json:"phase"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

// toCourseSessionDTO converts a sqlc.CourseSession row + its resolved
// phaseTitle (the skill Contract.Title for the session's current phase,
// resolved by the caller since it needs the skill) + the session's messages.
func toCourseSessionDTO(s sqlc.CourseSession, phaseTitle string, messages []CourseMessageDTO) CourseSessionDTO {
	return CourseSessionDTO{
		ID:         s.ID.String(),
		CourseID:   s.CourseID.String(),
		Phase:      s.Phase,
		PhaseTitle: phaseTitle,
		Status:     s.Status,
		Messages:   messages,
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
