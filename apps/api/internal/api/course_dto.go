package api

// course_dto.go — Task 5's course v2 wire shapes. All four DTOs mirror
// packages/contracts/src/course.ts exactly (CourseSummary/CoursePlayerPayload/
// CourseProgress/CourseReport) — snake_case where the contract says
// snake_case (summary + course_slug + teaching_thread), camelCase where it
// says camelCase (cardIds/renderCache/completedStepTitles/secondsSpent).
// structure/renderCache travel as json.RawMessage: this layer never
// interprets them beyond the one narrow header (title/course_goal/
// teaching_thread) getCourseReport needs — the store already treats them the
// same way (coursestore.go's own narrow report-only parse types).

import (
	"encoding/json"
	"time"

	"mindimprint/api/internal/agent"
)

type courseSummaryDTO struct {
	Slug      string   `json:"slug"`
	Branch    string   `json:"branch"`
	Title     string   `json:"title"`
	Blurb     string   `json:"blurb"`
	TimeLabel string   `json:"time_label"`
	CardIDs   []string `json:"card_ids"`
	StepCount int      `json:"step_count"`
}

func toCourseSummaryDTO(r agent.CourseSummaryRow) courseSummaryDTO {
	cardIDs := r.CardIDs
	if cardIDs == nil {
		cardIDs = []string{}
	}
	return courseSummaryDTO{
		Slug: r.Slug, Branch: r.Branch, Title: r.Title, Blurb: r.Blurb,
		TimeLabel: r.TimeLabel, CardIDs: cardIDs, StepCount: r.StepCount,
	}
}

// coursePayloadDTO is CoursePlayerPayload (contract): structure/renderCache
// are emitted verbatim as raw JSON — the store never parses them beyond its
// own report-only needs, and neither does this DTO.
type coursePayloadDTO struct {
	Slug        string          `json:"slug"`
	Title       string          `json:"title"`
	Branch      string          `json:"branch"`
	CardIDs     []string        `json:"cardIds"`
	Structure   json.RawMessage `json:"structure"`
	RenderCache json.RawMessage `json:"renderCache"`
}

func toCoursePayloadDTO(p agent.CoursePlayerPayload) coursePayloadDTO {
	cardIDs := p.CardIDs
	if cardIDs == nil {
		cardIDs = []string{}
	}
	structure := p.Structure
	if len(structure) == 0 {
		structure = json.RawMessage("{}")
	}
	renderCache := p.RenderCache
	if len(renderCache) == 0 {
		renderCache = json.RawMessage("{}")
	}
	return coursePayloadDTO{
		Slug: p.Slug, Title: p.Title, Branch: p.Branch,
		CardIDs: cardIDs, Structure: structure, RenderCache: renderCache,
	}
}

// courseProgressDTO is CourseProgress (contract) — keyed by slug (course_slug),
// not the internal course uuid; the store's CourseProgressRow only carries
// CourseID, so the slug travels in from the handler's path param.
type courseProgressDTO struct {
	CourseSlug        string  `json:"course_slug"`
	CurrentOrdinal    int     `json:"current_ordinal"`
	CompletedOrdinals []int   `json:"completed_ordinals"`
	StartedAt         *string `json:"started_at"`
	CompletedAt       *string `json:"completed_at"`
	UpdatedAt         string  `json:"updated_at"`
}

func toCourseProgressDTO(slug string, p agent.CourseProgressRow) courseProgressDTO {
	completed := p.CompletedOrdinals
	if completed == nil {
		completed = []int{}
	}
	// A first-time visitor (GetProgress's no-row default) has a zero UpdatedAt —
	// format that as "" rather than the zero-time string, mirroring the pre-v2
	// handler's own explicit no-row default.
	updatedAt := ""
	if !p.UpdatedAt.IsZero() {
		updatedAt = p.UpdatedAt.Format(tsLayout)
	}
	return courseProgressDTO{
		CourseSlug: slug, CurrentOrdinal: p.CurrentOrdinal, CompletedOrdinals: completed,
		StartedAt: timePtrToTSPtr(p.StartedAt), CompletedAt: timePtrToTSPtr(p.CompletedAt),
		UpdatedAt: updatedAt,
	}
}

func timePtrToTSPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(tsLayout)
	return &s
}

// courseStructureHeader is the narrow slice of `course.structure` getCourseReport
// reads for the report's title/goal/teaching_thread fields — CourseReportData
// (the store's return value) does not carry these; they live only in the
// authored structure, so the handler parses them itself rather than asking
// the store to grow a header-only return field.
type courseStructureHeader struct {
	Title          string `json:"title"`
	CourseGoal     string `json:"course_goal"`
	TeachingThread string `json:"teaching_thread"`
}

type courseQuizDTO struct {
	Total   int `json:"total"`
	Correct int `json:"correct"`
}

// courseReportDTO is CourseReport (contract) — note the contract's own mixed
// casing (teaching_thread is snake_case; completedStepTitles/cardIds/
// secondsSpent are camelCase), reproduced verbatim, not normalized.
type courseReportDTO struct {
	Title               string        `json:"title"`
	Goal                string        `json:"goal"`
	TeachingThread      string        `json:"teaching_thread"`
	CompletedStepTitles []string      `json:"completedStepTitles"`
	CardIDs             []string      `json:"cardIds"`
	SecondsSpent        int           `json:"secondsSpent"`
	Quiz                courseQuizDTO `json:"quiz"`
}

func toCourseReportDTO(h courseStructureHeader, rep agent.CourseReportData) courseReportDTO {
	titles := rep.CompletedStepTitles
	if titles == nil {
		titles = []string{}
	}
	cardIDs := rep.CardIDs
	if cardIDs == nil {
		cardIDs = []string{}
	}
	return courseReportDTO{
		Title: h.Title, Goal: h.CourseGoal, TeachingThread: h.TeachingThread,
		CompletedStepTitles: titles, CardIDs: cardIDs, SecondsSpent: rep.SecondsSpent,
		Quiz: courseQuizDTO{Total: rep.Quiz.Total, Correct: rep.Quiz.Correct},
	}
}
