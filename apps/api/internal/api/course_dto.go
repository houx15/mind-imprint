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

// courseCatalogProgressDTO is the student's own state on one catalog card:
// enough to draw the ring and the 进行中/已学完 pill, and to order the list by
// what they last worked on. Deliberately NOT courseProgressDTO — the catalog
// has no use for the resume ordinal or the completed-ordinal array, and sending
// them per course would put the whole player payload in a list response.
type courseCatalogProgressDTO struct {
	Status         string `json:"status"` // "in-progress" | "completed"
	CompletedSteps int    `json:"completedSteps"`
	UpdatedAt      string `json:"updatedAt"`
}

type courseSummaryDTO struct {
	Slug      string   `json:"slug"`
	Branch    string   `json:"branch"`
	Title     string   `json:"title"`
	Blurb     string   `json:"blurb"`
	TimeLabel string   `json:"time_label"`
	CardIDs   []string `json:"card_ids"`
	StepCount int      `json:"step_count"`
	CoverURL  string   `json:"coverUrl"`

	Category     *string         `json:"category"`
	Introduction json.RawMessage `json:"introduction"`
	FeaturedRank *int32          `json:"featuredRank"`

	// null for a course this student has never opened — the catalog reads that
	// absence as 未开始, which a zero-valued object could not express.
	Progress *courseCatalogProgressDTO `json:"progress"`
}

// toCourseSummaryDTO is an *API method (not a free function) solely so it can
// reach a.resolveCoverURL — the same project-cover resolver projects.go uses
// for CoverURL, signing the row's raw Cover value ("img:<n>") into a
// short-lived GET URL. An empty/unresolvable cover resolves to "" (no cover),
// mirroring resolveCoverURL's own contract.
func (a *API) toCourseSummaryDTO(r agent.CourseSummaryRow) courseSummaryDTO {
	cardIDs := r.CardIDs
	if cardIDs == nil {
		cardIDs = []string{}
	}
	return courseSummaryDTO{
		Slug: r.Slug, Branch: r.Branch, Title: r.Title, Blurb: r.Blurb,
		TimeLabel: r.TimeLabel, CardIDs: cardIDs, StepCount: r.StepCount,
		CoverURL:     a.resolveCourseCoverURL(r.Slug, r.Cover),
		Category:     r.Category,
		Introduction: json.RawMessage(r.Introduction),
		FeaturedRank: r.FeaturedRank,
	}
}

// withCatalogProgress attaches the student's own state to a summary DTO. A slug
// missing from the map is untouched, and its Progress stays nil.
func withCatalogProgress(dto courseSummaryDTO, progress map[string]agent.CourseCatalogProgress) courseSummaryDTO {
	p, ok := progress[dto.Slug]
	if !ok {
		return dto
	}
	dto.Progress = &courseCatalogProgressDTO{
		Status:         p.Status,
		CompletedSteps: p.CompletedSteps,
		UpdatedAt:      p.UpdatedAt.Format(tsLayout),
	}
	return dto
}

// coursePayloadDTO is CoursePlayerPayload (contract): structure/renderCache
// are emitted verbatim as raw JSON — the store never parses them beyond its
// own report-only needs, and neither does this DTO.
type coursePayloadDTO struct {
	Slug        string            `json:"slug"`
	Title       string            `json:"title"`
	Branch      string            `json:"branch"`
	CardIDs     []string          `json:"cardIds"`
	Structure   json.RawMessage   `json:"structure"`
	RenderCache json.RawMessage   `json:"renderCache"`
	AudioKeys   map[string]string `json:"audioKeys"`
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
	// AudioKeys rides along verbatim (pieceId→object key) — read-path resolves
	// URLs on demand (api.resolveUrl), so getCourse never signs or touches OSS
	// here; nil → {} so the JSON is always an object, never null.
	audioKeys := p.AudioManifest
	if audioKeys == nil {
		audioKeys = map[string]string{}
	}
	return coursePayloadDTO{
		Slug: p.Slug, Title: p.Title, Branch: p.Branch,
		CardIDs: cardIDs, Structure: structure, RenderCache: renderCache,
		AudioKeys: audioKeys,
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

// courseAnswerItemDTO / courseAnswerSliceDTO / courseAnswerReportDTO mirror the
// @mind-imprint/contracts CourseAnswerReport DTO. `correct` is *bool so an
// ungraded item marshals to JSON null (the contract's nullable), not false.
type courseAnswerItemDTO struct {
	BlockID    string `json:"blockId"`
	Type       string `json:"type"`
	Prompt     string `json:"prompt"`
	Answered   bool   `json:"answered"`
	YourAnswer string `json:"yourAnswer"`
	Correct    *bool  `json:"correct"`
	Attempts   int    `json:"attempts"`
}

type courseAnswerSliceDTO struct {
	SliceID          string                `json:"sliceId"`
	Title            string                `json:"title"`
	TimeSpentSeconds int                   `json:"timeSpentSeconds"`
	Items            []courseAnswerItemDTO `json:"items"`
}

type courseAnswerReportDTO struct {
	Slices []courseAnswerSliceDTO `json:"slices"`
}

func toCourseAnswerReportDTO(rep agent.CourseAnswerReportData) courseAnswerReportDTO {
	slices := make([]courseAnswerSliceDTO, 0, len(rep.Slices))
	for _, sl := range rep.Slices {
		items := make([]courseAnswerItemDTO, 0, len(sl.Items))
		for _, it := range sl.Items {
			items = append(items, courseAnswerItemDTO{
				BlockID: it.BlockID, Type: it.Type, Prompt: it.Prompt,
				Answered: it.Answered, YourAnswer: it.YourAnswer, Correct: it.Correct, Attempts: it.Attempts,
			})
		}
		slices = append(slices, courseAnswerSliceDTO{
			SliceID: sl.SliceID, Title: sl.Title, TimeSpentSeconds: sl.TimeSpentSeconds, Items: items,
		})
	}
	return courseAnswerReportDTO{Slices: slices}
}
