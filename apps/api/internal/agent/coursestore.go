package agent

// coursestore.go — course v2 content store (Task 4): course.go (Task 3) owns
// the sqlc queries this adapts; every method here lives on *sqlcAgentStore
// (agentstore.go), not a separate course-only store type — course v2 has no
// session/skill/card runtime left to isolate FROM (course_step.go's
// CourseStore seam is the OLD phase-gated runtime, retired by migration 0050
// and Task 5's follow-up deletion; this file shares nothing with it).
//
// Four jobs: (1) content CRUD (ListCourses/GetCoursePayload/UpsertCourse) —
// structure/render_cache are stored and returned as raw []byte, never
// unmarshalled here; (2) progress (GetProgress/SaveProgress), where
// SaveProgress does the completed_ordinals UNION in Go because
// UpsertCourseProgress's SQL only ever SETs the column (no array-union
// operator in the query — see internal/store/queries/course.sql); (3) event
// logging (LogCourseEvent), a course-scoped AppendEvent; (4) the report
// (CourseReport), the only place JSON is actually unmarshalled — structure
// for step titles, render_cache for the authored quiz-question total, and
// course-scoped events for the quiz-correct tally.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// CourseSummaryRow is one ListCourses row — the catalog card's fields, no
// structure/render_cache (those are GetCoursePayload's job, fetched only
// once a student opens a specific course).
type CourseSummaryRow struct {
	Slug      string
	Branch    string
	Title     string
	Blurb     string
	TimeLabel string
	CardIDs   []string
	StepCount int
	Status    string
	Cover     string

	Category     *string
	Introduction []byte
	FeaturedRank *int32
}

// CoursePlayerPayload is what the player needs to render one course:
// structure (the authored script) and render_cache (the published per-step
// content) travel as json.RawMessage — this store never parses them, only
// CourseReport does, and only the two narrow fields it needs. AudioManifest
// maps pieceId ("<stepId>#<segIdx>") to the OSS object key holding that
// teaching segment's narration (migration 0052); nil/empty column decodes
// to an empty (non-nil) map, never nil, so callers can range over it freely.
type CoursePlayerPayload struct {
	Slug          string
	Title         string
	Branch        string
	CardIDs       []string
	Structure     json.RawMessage
	RenderCache   json.RawMessage
	AudioManifest map[string]string
	Status        string
	Cover         string
}

// UpsertCourseInput is UpsertCourse's argument — StepCount is caller-supplied
// (the seed loader / admin publish path computes it once from Structure via
// encoding/json, per the same rule CourseReport uses for its own tallies)
// rather than recomputed here, so this store stays a pure adapter with no
// opinion on how the caller derived it. AudioManifest is the pieceId→object-key
// map a later task's GenerateCourseAudio produces; nil is stored as the
// column's own default ('{}'), never a SQL NULL.
type UpsertCourseInput struct {
	Slug          string
	Branch        string
	Title         string
	Blurb         string
	TimeLabel     string
	CardIDs       []string
	Structure     []byte
	RenderCache   []byte
	StepCount     int
	AudioManifest map[string]string
}

// CourseProgressRow is one student's position in one course. StartedAt/
// CompletedAt are nil until set — course_progress.started_at/completed_at
// are nullable timestamptz columns (migration 0050).
type CourseProgressRow struct {
	CourseID          uuid.UUID
	CurrentOrdinal    int
	CompletedOrdinals []int
	StartedAt         *time.Time
	CompletedAt       *time.Time
	UpdatedAt         time.Time
	ActiveSeconds     int
}

// CourseQuizTally is CourseReportData.Quiz: Total is the AUTHORED question
// count (sum of len(content.interactions) over every render_cache step —
// how many questions this course has, not how many any one student
// attempted); Correct counts distinct interactionIds with correct=true,
// deduped last-attempt-wins.
type CourseQuizTally struct {
	Total   int
	Correct int
}

// CourseReportData is CourseReport's return value — the four facts a
// finished-course summary needs, computed fresh from structure/render_cache/
// events every call (no stored report row; this is cheap enough to recompute
// per view at this scale — one course, one small JSON blob, one small event
// slice).
type CourseReportData struct {
	CompletedStepTitles []string
	CardIDs             []string
	SecondsSpent        int
	Quiz                CourseQuizTally
}

// ListCourses returns the catalog: every course's summary row, branch/title
// ordered (the underlying query's ORDER BY — a stable, human-legible
// listing, not insertion order).
func (s *sqlcAgentStore) ListCourses(ctx context.Context, includePreview bool) ([]CourseSummaryRow, error) {
	rows, err := s.q.ListCourseRows(ctx, includePreview)
	if err != nil {
		return nil, err
	}
	out := make([]CourseSummaryRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, CourseSummaryRow{
			Slug: r.Slug, Branch: r.Branch, Title: r.Title, Blurb: r.Blurb,
			TimeLabel: r.TimeLabel, CardIDs: r.CardIds, StepCount: int(r.StepCount),
			Status: r.Status, Cover: r.Cover,
			Category: r.Category, Introduction: r.Introduction, FeaturedRank: r.FeaturedRank,
		})
	}
	return out, nil
}

// CourseCatalogProgress is one student's state on one course, as the CATALOG
// needs it — a completed-step count and when they last worked on it, not the
// full CourseProgressRow the player resumes from. Status is already normalized
// to "completed" / "in-progress" by the query.
type CourseCatalogProgress struct {
	Status         string
	CompletedSteps int
	UpdatedAt      time.Time
}

// ListProgressForUser returns the student's progress on every course they have
// TOUCHED, keyed by slug — the catalog's whole progress picture in one round
// trip. Untouched courses are simply absent from the map (never a zero entry),
// which is what lets the caller distinguish 未开始 from 0%.
//
// This is the batch form of GetProgress and shares its storage preference: a
// course with both a 2.0 session and a legacy progress row reports the session.
func (s *sqlcAgentStore) ListProgressForUser(ctx context.Context, userID uuid.UUID) (map[string]CourseCatalogProgress, error) {
	rows, err := s.q.ListCourseProgressForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]CourseCatalogProgress, len(rows))
	for _, r := range rows {
		n := int(r.CompletedCount)
		if n < 0 {
			n = 0
		}
		out[r.Slug] = CourseCatalogProgress{Status: r.Status, CompletedSteps: n, UpdatedAt: r.UpdatedAt}
	}
	return out, nil
}

// GetCoursePayload loads one course by its external slug, returning both the
// player payload and the internal uuid — callers need the uuid for every
// subsequent progress/event call (course_progress and event.course_id are
// both keyed by it, not the slug).
func (s *sqlcAgentStore) GetCoursePayload(ctx context.Context, slug string) (CoursePlayerPayload, uuid.UUID, error) {
	row, err := s.q.GetCourseBySlug(ctx, slug)
	if err != nil {
		return CoursePlayerPayload{}, uuid.Nil, err
	}
	audioManifest := map[string]string{}
	if len(row.AudioManifest) > 0 {
		if err := json.Unmarshal(row.AudioManifest, &audioManifest); err != nil {
			return CoursePlayerPayload{}, uuid.Nil, fmt.Errorf("course payload: parse audio_manifest: %w", err)
		}
	}
	payload := CoursePlayerPayload{
		Slug: row.Slug, Title: row.Title, Branch: row.Branch,
		CardIDs:       row.CardIds,
		Structure:     json.RawMessage(row.Structure),
		RenderCache:   json.RawMessage(row.RenderCache),
		AudioManifest: audioManifest,
		Status:        row.Status,
		Cover:         row.Cover,
	}
	return payload, row.ID, nil
}

// UpsertCourse publishes (creates or replaces) one course's content, keyed
// by slug. structure/render_cache are stored verbatim — this store never
// interprets them beyond the caller-supplied StepCount.
func (s *sqlcAgentStore) UpsertCourse(ctx context.Context, in UpsertCourseInput) error {
	audioManifest := in.AudioManifest
	if audioManifest == nil {
		audioManifest = map[string]string{}
	}
	audioManifestJSON, err := json.Marshal(audioManifest)
	if err != nil {
		return fmt.Errorf("upsert course: marshal audio_manifest: %w", err)
	}
	_, err = s.q.UpsertCourse(ctx, sqlc.UpsertCourseParams{
		Slug: in.Slug, Branch: in.Branch, Title: in.Title, Blurb: in.Blurb,
		TimeLabel:     in.TimeLabel,
		CardIds:       in.CardIDs,
		StepCount:     int32(in.StepCount),
		Structure:     in.Structure,
		RenderCache:   in.RenderCache,
		AudioManifest: audioManifestJSON,
	})
	return err
}

// GetProgress reads one student's progress in one course by slug. No row
// yet (first-time visitor) is not an error: it returns a zero progress row
// still carrying the resolved CourseID, mirroring the pre-v2 handler's own
// zero-progress default (internal/api/course.go's getCourseProgress).
//
// A 2.0 (runtime) course keeps its progress in course_session, not
// course_progress, so its completion is derived from the session's completed
// slice states (GetCourseSessionProgressBySlug) and takes precedence when a
// session exists — that is what lets the catalog ring reflect a 2.0 course's
// real progress. CompletedOrdinals is synthesized as the prefix [0..n) of the
// completed-slice count: the catalog consumers use only its length, and a course
// is completed slice-by-slice in order, so a prefix is a faithful record.
func (s *sqlcAgentStore) GetProgress(ctx context.Context, userID uuid.UUID, slug string) (CourseProgressRow, error) {
	if sp, serr := s.q.GetCourseSessionProgressBySlug(ctx, sqlc.GetCourseSessionProgressBySlugParams{UserID: userID, Slug: slug}); serr == nil {
		n := int(sp.CompletedSlices)
		if n < 0 {
			n = 0
		}
		ords := make([]int, n)
		for i := range ords {
			ords[i] = i
		}
		var completedAt *time.Time
		if sp.Status == "completed" {
			t := sp.UpdatedAt
			completedAt = &t
		}
		return CourseProgressRow{
			CourseID:          sp.CourseID,
			CompletedOrdinals: ords,
			CompletedAt:       completedAt,
			UpdatedAt:         sp.UpdatedAt,
		}, nil
	} else if !errors.Is(serr, pgx.ErrNoRows) {
		return CourseProgressRow{}, serr
	}

	row, err := s.q.GetCourseProgressBySlug(ctx, sqlc.GetCourseProgressBySlugParams{UserID: userID, Slug: slug})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c, cerr := s.q.GetCourseBySlug(ctx, slug)
			if cerr != nil {
				return CourseProgressRow{}, cerr
			}
			return CourseProgressRow{CourseID: c.ID}, nil
		}
		return CourseProgressRow{}, err
	}
	return CourseProgressRow{
		CourseID:          row.CourseID,
		CurrentOrdinal:    int(row.CurrentOrdinal),
		CompletedOrdinals: int32sToInts(row.CompletedOrdinals),
		StartedAt:         pgTimeToPtr(row.StartedAt),
		CompletedAt:       pgTimeToPtr(row.CompletedAt),
		UpdatedAt:         row.UpdatedAt,
		ActiveSeconds:     int(row.ActiveSeconds),
	}, nil
}

// SaveProgress records a step turn. current_ordinal moves to currentOrdinal
// (the RESUME position — a UX convenience, no longer a completion signal).
// completed_ordinals grows by {*completedOrdinal} ONLY when completedOrdinal is
// non-nil — the caller marks a step complete explicitly, when the student has
// actually finished it (revealed everything + answered its quizzes, per the
// per-step gate), not merely by visiting it. The union happens here, in Go,
// because UpsertCourseProgress's SQL only ever SETs completed_ordinals to
// whatever it's given (no array-union operator in that query); a caller that
// re-sent just [completedOrdinal] would erase every earlier step's mark.
//
// The course is finished (completed_at set to now()) exactly when the resulting
// completed set covers every authored step (len(completed) >= stepCount) — so
// "finished" means all steps done, not "reached the last page". The ON CONFLICT
// COALESCE (existing-completed-at wins over a NULL) means a later call cannot
// un-finish an already finished course.
//
// activeSecondsDelta is the active-focus seconds the client accrued since its
// last flush; UpsertCourseProgress adds it to the stored total (additive,
// across visits). startedAt is always SQL NULL: the query's COALESCE sets it to
// now() on first insert and keeps the original on every later call.
func (s *sqlcAgentStore) SaveProgress(ctx context.Context, userID, courseID uuid.UUID, currentOrdinal int, completedOrdinal *int, activeSecondsDelta, stepCount int) (CourseProgressRow, error) {
	existing, err := s.q.GetCourseProgressByCourseID(ctx, sqlc.GetCourseProgressByCourseIDParams{UserID: userID, CourseID: courseID})
	var completed []int32
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return CourseProgressRow{}, err
		}
		// no row yet: completed starts empty.
	} else {
		completed = existing.CompletedOrdinals
	}
	if completedOrdinal != nil {
		completed = unionOrdinal(completed, int32(*completedOrdinal))
	}
	if completed == nil {
		// completed_ordinals is NOT NULL — a resume-only save (no completed_ordinal,
		// no prior row) must still write an empty array, never NULL.
		completed = []int32{}
	}

	var completedAt pgtype.Timestamptz
	if stepCount > 0 && len(completed) >= stepCount {
		completedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	}
	if activeSecondsDelta < 0 {
		activeSecondsDelta = 0
	}

	row, err := s.q.UpsertCourseProgress(ctx, sqlc.UpsertCourseProgressParams{
		UserID:             userID,
		CourseID:           courseID,
		CurrentOrdinal:     int32(currentOrdinal),
		CompletedOrdinals:  completed,
		StartedAt:          pgtype.Timestamptz{Valid: false},
		CompletedAt:        completedAt,
		ActiveSecondsDelta: int32(activeSecondsDelta),
	})
	if err != nil {
		return CourseProgressRow{}, err
	}
	return CourseProgressRow{
		CourseID:          row.CourseID,
		CurrentOrdinal:    int(row.CurrentOrdinal),
		CompletedOrdinals: int32sToInts(row.CompletedOrdinals),
		StartedAt:         pgTimeToPtr(row.StartedAt),
		CompletedAt:       pgTimeToPtr(row.CompletedAt),
		UpdatedAt:         row.UpdatedAt,
		ActiveSeconds:     int(row.ActiveSeconds),
	}, nil
}

// LogCourseEvent appends one course-scoped event row (surface="course",
// course_id=courseID). userID is a parameter (not in the brief's literal
// signature — see coursestore.go's package doc / Task 4 report) because
// event.user_id is NOT NULL with an FK to users(id) (migration 0016): there
// is no session row left to resolve it from (course v2 dropped
// course_session), and the seeded course events (migration 0034) already
// pair every course-scoped row with a real user_id, confirming attribution
// is per-student even though the scope (course_id) is not.
func (s *sqlcAgentStore) LogCourseEvent(ctx context.Context, userID, courseID uuid.UUID, typ string, payload []byte) error {
	_, err := s.q.AppendEvent(ctx, sqlc.AppendEventParams{
		ProjectID: pgtype.UUID{Valid: false},
		UserID:    userID,
		SessionID: pgtype.UUID{Valid: false},
		ThreadID:  pgtype.UUID{Valid: false},
		CourseID:  pgtype.UUID{Bytes: courseID, Valid: true},
		Surface:   "course",
		Type:      typ,
		Payload:   payload,
	})
	return err
}

// RecordCourseLLMCall meters one course-surface LLM call (Task 6's free-Q&A
// coach turn) — ported verbatim from the pre-migration-0050 sqlcCourseStore
// (see git history: 4c2d8e7~1's coursestore.go) onto the current
// *sqlcAgentStore receiver, since that old course-only store type was
// retired along with the phase runtime it backed. project_id is always NULL
// (course.go's other course-scoped writes use the same pgtype.UUID{Valid:
// false} — a course-surface call is never scoped to a project).
func (s *sqlcAgentStore) RecordCourseLLMCall(ctx context.Context, userID uuid.UUID, purpose string, resolved gateway.Resolved, prompt, completion int32) error {
	cost, priced := gateway.EstimateCost(resolved.Provider, resolved.Model, int(prompt), int(completion))
	if !priced {
		slog.Warn("course llm_call: unpriced model — cost recorded as 0", "provider", resolved.Provider, "model", resolved.Model)
	}
	// llm_call.cost_estimate is NOT NULL, so an unpriced model records an
	// explicit $0.00 via CostNumeric(cost, true) (with the warn above) — NOT
	// the nullable-evaluation NULL. See gateway/pricing.go and agentstore.go.
	_, err := s.q.RecordLLMCall(ctx, sqlc.RecordLLMCallParams{
		UserID: userID, ProjectID: pgtype.UUID{Valid: false},
		Surface: "course", Purpose: purpose,
		Provider: resolved.Provider, Model: resolved.Model, Tier: resolved.Tier,
		PromptTokens: prompt, CompletionTokens: completion, CostEstimate: gateway.CostNumeric(cost, true),
	})
	return err
}

// courseStructureForReport is the narrow slice of `course.structure` this
// store reads: just enough to title a completed step. The full authored
// structure (purpose/teaching_flow/materials/…) is the course author's
// concern, never the store's.
type courseStructureForReport struct {
	Steps []struct {
		Title string `json:"title"`
	} `json:"steps"`
}

// courseRenderCacheForReport is the narrow slice of `course.render_cache`
// this store reads: render_cache's top level is an object ({version,
// courseId, courseTitle, exportedAt, steps: [...]}, per the published-cache
// writer), not a bare array — Steps here is that "steps" field.
type courseRenderCacheForReport struct {
	Steps []courseRenderStepForReport `json:"steps"`
}

// courseRenderStepForReport is the narrow slice of one `render_cache` step
// this store reads: just the authored interaction count.
type courseRenderStepForReport struct {
	Content struct {
		Interactions []json.RawMessage `json:"interactions"`
	} `json:"content"`
}

// courseQuizAnsweredPayload is the shape LogCourseEvent's caller writes for
// type="course_quiz_answered" (see the test literal / player's quiz submit).
type courseQuizAnsweredPayload struct {
	InteractionID string `json:"interactionId"`
	Correct       bool   `json:"correct"`
}

// CourseReport computes one student's finished-course summary from three
// sources, fresh every call: structure (step titles, indexed by
// completed_ordinals), render_cache (the authored quiz-question total), and
// course-scoped events (the quiz-correct tally). No report row is stored —
// recomputing is cheap at this scale (one course-sized JSON blob, one
// course's event slice).
func (s *sqlcAgentStore) CourseReport(ctx context.Context, userID uuid.UUID, slug string) (CourseReportData, error) {
	course, err := s.q.GetCourseBySlug(ctx, slug)
	if err != nil {
		return CourseReportData{}, err
	}

	var completedOrdinals []int32
	activeSeconds := 0
	progress, err := s.q.GetCourseProgressBySlug(ctx, sqlc.GetCourseProgressBySlugParams{UserID: userID, Slug: slug})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return CourseReportData{}, err
		}
		// no progress row: nothing completed, zero time spent — not an error.
	} else {
		completedOrdinals = progress.CompletedOrdinals
		activeSeconds = int(progress.ActiveSeconds)
	}

	var structure courseStructureForReport
	if err := json.Unmarshal(course.Structure, &structure); err != nil {
		return CourseReportData{}, fmt.Errorf("course report: parse structure: %w", err)
	}
	titles := make([]string, 0, len(completedOrdinals))
	for _, ord := range completedOrdinals {
		if ord >= 0 && int(ord) < len(structure.Steps) {
			titles = append(titles, structure.Steps[ord].Title)
		}
	}

	var renderCache courseRenderCacheForReport
	if err := json.Unmarshal(course.RenderCache, &renderCache); err != nil {
		return CourseReportData{}, fmt.Errorf("course report: parse render cache: %w", err)
	}
	quizTotal := 0
	for _, st := range renderCache.Steps {
		quizTotal += len(st.Content.Interactions)
	}

	events, err := s.q.ListEventsByCourse(ctx, pgtype.UUID{Bytes: course.ID, Valid: true})
	if err != nil {
		return CourseReportData{}, err
	}
	// Dedup by interactionId, last attempt wins: ListEventsByCourse orders
	// ascending by (created_at, id), so a plain overwrite-on-iterate leaves
	// the LAST (latest) attempt's correctness in the map.
	lastAttempt := map[string]bool{}
	for _, ev := range events {
		if ev.Type != "course_quiz_answered" || ev.UserID != userID {
			continue
		}
		var pl courseQuizAnsweredPayload
		if err := json.Unmarshal(ev.Payload, &pl); err != nil || pl.InteractionID == "" {
			continue
		}
		lastAttempt[pl.InteractionID] = pl.Correct
	}
	quizCorrect := 0
	for _, correct := range lastAttempt {
		if correct {
			quizCorrect++
		}
	}

	// secondsSpent is the accumulated ACTIVE-focus time (the client accrues it
	// only while the course page is visible+focused, migration 0051), NOT the
	// wall-clock started→completed span — idle/away time never counts.
	return CourseReportData{
		CompletedStepTitles: titles,
		CardIDs:             course.CardIds,
		SecondsSpent:        activeSeconds,
		Quiz:                CourseQuizTally{Total: quizTotal, Correct: quizCorrect},
	}, nil
}

// ---- Course Runtime 2.0 report (computed from the definition + session) ----

// courseDefForReport is the narrow slice of a CourseDefinition 2.0 document the
// report reads: the course title/objectives (header) and, per Slice, id+title
// (to name the completed steps in definition order).
type courseDefForReport struct {
	SchemaVersion string `json:"schemaVersion"`
	Course        struct {
		Title      string `json:"title"`
		Objectives []struct {
			Text string `json:"text"`
		} `json:"objectives"`
		Parts []struct {
			Slices []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"slices"`
		} `json:"parts"`
	} `json:"course"`
}

// courseSessionForReport is the narrow slice of a persisted CourseSession the
// report reads: per-Slice completion + elapsed time, and the flat event log
// (for the quiz tally). Mirrors @mind-imprint/course-contract's CourseSession.
type courseSessionForReport struct {
	SliceStates map[string]struct {
		Status         string  `json:"status"`
		ElapsedSeconds float64 `json:"elapsedSeconds"`
	} `json:"sliceStates"`
	Events []struct {
		SourceID string `json:"sourceId"`
		Type     string `json:"type"`
	} `json:"events"`
}

// CourseReport20Result carries the 2.0 report: the header fields (title/goal —
// sourced from the definition, since a 2.0 course's legacy `structure` is empty)
// plus the same CourseReportData the legacy path returns.
type CourseReport20Result struct {
	Title string
	Goal  string
	Data  CourseReportData
}

// CourseReport20 computes the finished-course report for a CourseDefinition-2.0
// course from its definition + the student's course_session. Returns found=false
// (not an error) when the course has no 2.0 definition — the caller falls back to
// the legacy CourseReport path. Stats: completed Slice titles (definition order),
// total active time (sum of per-Slice elapsedSeconds), and a quiz tally from the
// session's answer events (dedup by sourceId, last attempt wins — same rule as
// the legacy path). Cards come from course.card_ids (the course↔cards relation).
func (s *sqlcAgentStore) CourseReport20(ctx context.Context, userID uuid.UUID, slug string) (CourseReport20Result, bool, error) {
	def, _, err := s.GetCourseDefinition(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CourseReport20Result{}, false, nil
		}
		return CourseReport20Result{}, false, err
	}
	if len(def) == 0 {
		return CourseReport20Result{}, false, nil // legacy course (no 2.0 definition)
	}
	var d courseDefForReport
	if err := json.Unmarshal(def, &d); err != nil {
		return CourseReport20Result{}, false, fmt.Errorf("course report 2.0: parse definition: %w", err)
	}
	if d.SchemaVersion != "2.0" {
		return CourseReport20Result{}, false, nil
	}

	course, err := s.q.GetCourseBySlug(ctx, slug)
	if err != nil {
		return CourseReport20Result{}, false, err
	}

	var sess courseSessionForReport
	sessBytes, _, found, err := s.GetCourseSession(ctx, userID, slug)
	if err != nil {
		return CourseReport20Result{}, false, err
	}
	if found && len(sessBytes) > 0 {
		if err := json.Unmarshal(sessBytes, &sess); err != nil {
			return CourseReport20Result{}, false, fmt.Errorf("course report 2.0: parse session: %w", err)
		}
	}

	// Completed Slice titles (definition order) + total active time.
	titles := []string{}
	elapsed := 0.0
	for _, part := range d.Course.Parts {
		for _, sl := range part.Slices {
			st, ok := sess.SliceStates[sl.ID]
			if !ok {
				continue
			}
			elapsed += st.ElapsedSeconds
			if st.Status == "completed" {
				titles = append(titles, sl.Title)
			}
		}
	}

	// Quiz tally from the answer events (standalone graded assessments): dedup
	// by sourceId (the block id), last attempt wins.
	lastAttempt := map[string]bool{}
	for _, ev := range sess.Events {
		switch ev.Type {
		case "answer.correct":
			lastAttempt[ev.SourceID] = true
		case "answer.incorrect", "answer.attemptsExhausted":
			lastAttempt[ev.SourceID] = false
		}
	}
	correct := 0
	for _, ok := range lastAttempt {
		if ok {
			correct++
		}
	}

	goal := ""
	if len(d.Course.Objectives) > 0 {
		goal = d.Course.Objectives[0].Text
	}

	return CourseReport20Result{
		Title: d.Course.Title,
		Goal:  goal,
		Data: CourseReportData{
			CompletedStepTitles: titles,
			CardIDs:             course.CardIds,
			SecondsSpent:        int(elapsed + 0.5),
			Quiz:                CourseQuizTally{Total: len(lastAttempt), Correct: correct},
		},
	}, true, nil
}

// GetCourseDefinition returns one course's stored CourseDefinition 2.0 document
// (raw jsonb) by slug, alongside the course's publish status. A legacy course
// with no 2.0 definition returns a nil []byte (SQL NULL), NOT an error; an
// unknown slug returns pgx.ErrNoRows. The caller (Course Runtime Slice 8's
// definition endpoint) treats both as 404 — that course routes to the legacy
// player.
func (s *sqlcAgentStore) GetCourseDefinition(ctx context.Context, slug string) ([]byte, string, error) {
	row, err := s.q.GetCourseDefinition(ctx, slug)
	if err != nil {
		return nil, "", err
	}
	return row.CourseDefinition, row.Status, nil
}

// CourseStatus returns one course's publish status ('preview'/'published') by
// slug, without loading the full player payload — for endpoints that only
// need to gate visibility (e.g. definition/session lookups), not render the
// course itself.
func (s *sqlcAgentStore) CourseStatus(ctx context.Context, slug string) (string, error) {
	return s.q.GetCourseStatusBySlug(ctx, slug)
}

// SetCourseDefinition attaches (or replaces) one course's CourseDefinition 2.0
// document. Separate from UpsertCourse so the legacy content path stays
// untouched — only the golden 2.0 seed writes this column.
func (s *sqlcAgentStore) SetCourseDefinition(ctx context.Context, slug string, def []byte) error {
	return s.q.SetCourseDefinition(ctx, sqlc.SetCourseDefinitionParams{Slug: slug, CourseDefinition: def})
}

// SetCourseStatusAndCover flips a course's publish status (and sets its
// cover) in one write — the ship endpoint's (Task 5) sole use: the ONE
// preview -> published transition, done atomically with the cover so a
// course is never left published-with-no-cover mid-request.
func (s *sqlcAgentStore) SetCourseStatusAndCover(ctx context.Context, slug, status, cover string) error {
	return s.q.SetCourseStatusAndCover(ctx, sqlc.SetCourseStatusAndCoverParams{Slug: slug, Status: status, Cover: cover})
}

// UpsertCourseDefinitionInput is UpsertCourseDefinition's argument — the
// course authoring & publish lifecycle's Task 3 create/modify path.
// Definition travels as []byte (the whole { schemaVersion, course } document,
// stored verbatim as jsonb); structure/render_cache stay the query's own
// empty-object default for 2.0 courses, which use Definition instead.
type UpsertCourseDefinitionInput struct {
	Slug       string
	Branch     string
	Title      string
	Blurb      string
	TimeLabel  string
	CardIDs    []string
	Definition []byte
	// StepCount is the authored slice count (one slice = one step). Stored on
	// the course row's step_count so the catalog's "N 步" label and progress
	// math work for 2.0 courses; the runtime's own in-course progress is
	// session-derived and does not read this.
	StepCount int

	Category     *string
	Introduction []byte
}

// UpsertCourseDefinition creates or modifies one 2.0 course's definition,
// keyed by slug, and returns the row's resulting status ('preview' on a
// fresh insert; unchanged on a re-PUT of an existing course — see the
// UpsertCourseDefinition query's ON CONFLICT contract in
// internal/store/queries/course.sql).
func (s *sqlcAgentStore) UpsertCourseDefinition(ctx context.Context, in UpsertCourseDefinitionInput) (string, error) {
	cardIDs := in.CardIDs
	if cardIDs == nil {
		cardIDs = []string{}
	}
	row, err := s.q.UpsertCourseDefinition(ctx, sqlc.UpsertCourseDefinitionParams{
		Slug: in.Slug, Branch: in.Branch, Title: in.Title, Blurb: in.Blurb,
		TimeLabel:        in.TimeLabel,
		CardIds:          cardIDs,
		StepCount:        int32(in.StepCount),
		CourseDefinition: in.Definition,
		Category:         in.Category,
		Introduction:     in.Introduction,
	})
	if err != nil {
		return "", err
	}
	return row.Status, nil
}

// GetCourseSession reads one student's CourseSession snapshot (§16) for one
// course by slug. found=false (no row yet) is not an error — the caller's
// get-or-create then mints a fresh session. Owner scoping is by userID, so a
// student never reads another's session.
func (s *sqlcAgentStore) GetCourseSession(ctx context.Context, userID uuid.UUID, slug string) (session []byte, status string, found bool, err error) {
	row, err := s.q.GetCourseSessionBySlug(ctx, sqlc.GetCourseSessionBySlugParams{UserID: userID, Slug: slug})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", false, nil
		}
		return nil, "", false, err
	}
	return row.Session, row.Status, true, nil
}

// CreateCourseSession inserts a fresh CourseSession snapshot for (userID,
// courseID) and returns the stored blob. Called only when GetCourseSession found
// none (get-or-create).
func (s *sqlcAgentStore) CreateCourseSession(ctx context.Context, userID, courseID uuid.UUID, session []byte, status string) ([]byte, error) {
	row, err := s.q.CreateCourseSession(ctx, sqlc.CreateCourseSessionParams{
		UserID: userID, CourseID: courseID, Session: session, Status: status,
	})
	if err != nil {
		return nil, err
	}
	return row.Session, nil
}

// SaveCourseSession snapshot-writes the whole CourseSession blob for (userID,
// courseID). Owner-scoped by userID — a student can only overwrite their own
// session. A no-op (no matching row) is not an error: the runtime always
// creates before it saves.
func (s *sqlcAgentStore) SaveCourseSession(ctx context.Context, userID, courseID uuid.UUID, session []byte, status string) error {
	return s.q.SaveCourseSession(ctx, sqlc.SaveCourseSessionParams{
		UserID: userID, CourseID: courseID, Session: session, Status: status,
	})
}

// DeleteCourseSession + DeleteCourseProgress power course RESTART: a finished
// course (or one mid-flight) is wiped back to the start. Both are idempotent
// no-ops when the student never touched the course in that storage model, so a
// restart safely removes both regardless of which player the course uses (2.0
// runtime → course_session; legacy → course_progress). Course events
// (course_event) are intentionally NOT deleted — those are 铁律④ evidence.
func (s *sqlcAgentStore) DeleteCourseSession(ctx context.Context, userID, courseID uuid.UUID) error {
	return s.q.DeleteCourseSession(ctx, sqlc.DeleteCourseSessionParams{UserID: userID, CourseID: courseID})
}

func (s *sqlcAgentStore) DeleteCourseProgress(ctx context.Context, userID, courseID uuid.UUID) error {
	return s.q.DeleteCourseProgress(ctx, sqlc.DeleteCourseProgressParams{UserID: userID, CourseID: courseID})
}

// CourseHistoryItem is one touched course in a student's learning history:
// the slug, a coarse status (the runtime session status verbatim, or
// 'completed'/'in-progress' for a legacy course), the count of completed steps
// (legacy only — 0 for runtime), and the last-activity time. The caller
// enriches title/cover from the course list.
type CourseHistoryItem struct {
	Slug           string
	Status         string
	CompletedCount int
	UpdatedAt      time.Time
}

// ListCourseHistory returns the courses this student has engaged with across
// BOTH storage models (runtime session + legacy progress), newest activity
// first.
func (s *sqlcAgentStore) ListCourseHistory(ctx context.Context, userID uuid.UUID) ([]CourseHistoryItem, error) {
	rows, err := s.q.ListCourseHistory(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]CourseHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, CourseHistoryItem{
			Slug: r.Slug, Status: r.Status, CompletedCount: int(r.CompletedCount), UpdatedAt: r.UpdatedAt,
		})
	}
	return items, nil
}

// unionOrdinal returns xs ∪ {v}, sorted ascending — a no-op copy when v is
// already present, so repeated saves of the same step never grow the slice.
func unionOrdinal(xs []int32, v int32) []int32 {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	out := make([]int32, 0, len(xs)+1)
	out = append(out, xs...)
	out = append(out, v)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func int32sToInts(xs []int32) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = int(x)
	}
	return out
}

func pgTimeToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}
