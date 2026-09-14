package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// getLiteTeacherItem handles
// GET /api/v1/lite/teacher/classes/{id}/students/{userId}/items/{atomId}:
// one reading/writing/project atom's outputs and moments.
//
// Outputs and moments only (spec DEC-1). This file must never read
// atom_message.content: the chat with 印记 is not shown to teachers. Turn
// counts come from the item row query, which counts rows without content.
//
// authTeacherStudent is the only gate — never loadOwnedAtom/TouchAtom, a
// teacher read must not bump last_activity_at.
func (a *API) getLiteTeacherItem(w http.ResponseWriter, r *http.Request) {
	_, userID, ok := a.authTeacherStudent(w, r)
	if !ok {
		return
	}
	atomID, err := uuid.Parse(r.PathValue("atomId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	ctx := r.Context()
	at, err := a.d.Queries.GetAtom(ctx, atomID)
	if err != nil || at.UserID != userID {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	row, err := a.liteTeacherItemRow(ctx, userID, atomID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	resp := map[string]any{
		"item": row, "reading": nil, "writing": nil, "project": nil,
		"report": nil, "reportError": nil,
	}

	switch at.Kind {
	case "reading":
		v, err := a.liteTeacherReading(ctx, atomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		resp["reading"] = v
	case "writing":
		v, err := a.liteTeacherWriting(ctx, atomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		resp["writing"] = v
	case "project":
		v, err := a.liteTeacherProject(ctx, userID, atomID)
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		resp["project"] = v
	default:
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	// Reading/writing only: a project has no atom_report row (its finish
	// state and its review section are its own thing, not a lite report).
	if at.Kind == "reading" || at.Kind == "writing" {
		rep, rerr := a.liteTeacherReport(r, userID, atomID, at.Kind, r.URL.Query().Get("prose") == "1")
		if rerr != nil {
			resp["reportError"] = "报告生成失败：" + rerr.Error()
		} else if rep != nil {
			resp["report"] = rep
		}
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// liteTeacherReport returns the stored report's teacher-facing slice
// (stats, moments, keep, prosePending), generating it first when the item
// is finished and no report exists yet (ensureAtomReport, atom_report.go).
// Unfinished atom → (nil, nil), no generation, no model call.
//
// The two-phase protocol (atom_report.go's file comment) means a report can
// come back with `prosePending: true` and no `moments` yet: phase 1 stores
// the deterministic half with no model call, and the prose
// (moments/gains/keep-from-summary) is only generated on a LATER request.
//
// 🚨 A plain GET never runs phase 2 (wantProse=false). A stored report that
// still owes its prose would otherwise make the teacher's FIRST open wait on
// the flagship call (up to liteModelWorkTimeout, 150s) before the page shows
// anything. Only the client's one follow-up re-fetch sends `?prose=1`, and
// that request is the one allowed to pay for the prose — after checking the
// STUDENT's entitlement, since the tokens are spent on her report. Not
// entitled → the stored report is returned as it is, with no error.
//
// Takes the request rather than a bare context — see detachedModelCtx
// (reading_lens.go): once the model call is under way it must run to
// completion even if the teacher navigates away mid-request.
func (a *API) liteTeacherReport(r *http.Request, userID, atomID uuid.UUID, kind string, wantProse bool) (map[string]any, error) {
	if wantProse {
		entitled, err := a.studentEntitled(r.Context(), userID)
		if err != nil {
			return nil, err
		}
		wantProse = entitled
	}
	mctx, cancel := detachedModelCtx(r)
	defer cancel()
	row, ok, err := a.ensureAtomReportWith(mctx, userID, atomID, kind, wantProse)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	var rep struct {
		Stats        json.RawMessage `json:"stats"`
		Moments      json.RawMessage `json:"moments"`
		Keep         json.RawMessage `json:"keep"`
		ProsePending bool            `json:"prosePending"`
	}
	if err := json.Unmarshal(row.Report, &rep); err != nil {
		return nil, err
	}
	return map[string]any{
		"stats": rep.Stats, "moments": rep.Moments, "keep": rep.Keep,
		"prosePending": rep.ProsePending,
	}, nil
}

// studentEntitled runs the HasEntitlement seam for the student whose report
// a teacher is viewing. The student path checks the request user; here the
// request user is the teacher, so the student row is loaded the same way
// auth.go builds a User.
func (a *API) studentEntitled(ctx context.Context, userID uuid.UUID) (bool, error) {
	row, err := a.d.Queries.GetUserByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return HasEntitlement(ctx, User{ID: row.ID, SchoolID: row.SchoolID, Role: row.Role, DisplayName: row.DisplayName})
}

// notFoundIsNil turns a :one query's ErrNoRows into a nil pointer, for the
// several tables here that a reading/writing/project may or may not have a
// row in yet (a source never pasted, a takeaway never left, a draft never
// started). Any other error still propagates.
func notFoundIsNil[T any](v T, err error) (*T, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// liteTeacherItemRow fetches the one atom row (GetLiteStudentItem, scoped to
// user_id + atom_id + kind IN (...)) and maps it through the same
// liteItemRowDTO mapper the student-page list uses (lite_teacher_roster.go).
// A miss here (wrong owner, wrong kind, or the atom does not exist) is
// reported as 404 — the same posture authTeacherStudent already takes for
// cross-tenant probing.
func (a *API) liteTeacherItemRow(ctx context.Context, userID, atomID uuid.UUID) (LiteItemRowDTO, error) {
	row, err := a.d.Queries.GetLiteStudentItem(ctx, sqlc.GetLiteStudentItemParams{UserID: userID, AtomID: atomID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return LiteItemRowDTO{}, httpx.ErrNotFound("资源不存在")
		}
		return LiteItemRowDTO{}, err
	}
	return liteItemRowDTO(row.AtomID, row.Kind, row.Title, row.Status, row.Level,
		row.CreatedAt, row.LastActivityAt, row.ActiveSeconds, row.Turns, row.FinishedAt), nil
}

// --- reading ---------------------------------------------------------------

// liteReadingSourceDTO is reading.source on the item-detail response: the
// library placement and the article's URL. Never body — the article text
// is not an output OF her, and 铁律①/DEC-1 both point the same way: a
// teacher reads what she produced, not the material she read.
type liteReadingSourceDTO struct {
	LibrarySlug string `json:"librarySlug"`
	Level       int32  `json:"level"`
	URL         string `json:"url"`
}

type liteHighlightDTO struct {
	Quote string `json:"quote"`
	Note  string `json:"note"`
}

type liteLensDTO struct {
	Title  string          `json:"title"`
	Fields json.RawMessage `json:"fields"`
}

// liteTeacherReading assembles reading's slice of the item-detail response:
// her highlights/notes, her takeaway, and the lens cards she submitted.
// Never reads GetReadingSource.Body and never touches atom_message.
func (a *API) liteTeacherReading(ctx context.Context, atomID uuid.UUID) (map[string]any, error) {
	rd, err := a.d.Queries.GetReading(ctx, atomID)
	if err != nil {
		return nil, err
	}
	src, err := notFoundIsNil(a.d.Queries.GetReadingSource(ctx, atomID))
	if err != nil {
		return nil, err
	}
	url := ""
	if src != nil && src.SourceUrl != nil {
		url = *src.SourceUrl
	}

	notes, err := a.d.Queries.ListAtomAnnotations(ctx, atomID)
	if err != nil {
		return nil, err
	}
	highlights := make([]liteHighlightDTO, 0, len(notes))
	for _, n := range notes {
		highlights = append(highlights, liteHighlightDTO{Quote: n.Quote, Note: n.Note})
	}

	takeaway, err := notFoundIsNil(a.d.Queries.GetReadingTakeaway(ctx, atomID))
	if err != nil {
		return nil, err
	}
	takeawayText := ""
	if takeaway != nil {
		takeawayText = takeaway.Text
	}

	atomCards, err := a.d.Queries.ListAtomCards(ctx, atomID)
	if err != nil {
		return nil, err
	}
	lenses := make([]liteLensDTO, 0, len(atomCards))
	for _, c := range atomCards {
		if c.Status != "submitted" {
			continue
		}
		spec, ok := cards.ByID(c.CardID)
		if !ok {
			continue
		}
		lenses = append(lenses, liteLensDTO{Title: spec.Name, Fields: json.RawMessage(c.FieldValues)})
	}

	return map[string]any{
		"source": liteReadingSourceDTO{
			LibrarySlug: rd.LibrarySlug, Level: int32(rd.LibraryTier), URL: url,
		},
		"highlights": highlights,
		"takeaway":   takeawayText,
		"lenses":     lenses,
	}, nil
}

// --- writing -----------------------------------------------------------

type liteOutlineDTO struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type liteSnippetDTO struct {
	Position int32  `json:"position"`
	Text     string `json:"text"`
}

type liteWritingCommentDTO struct {
	Scope   string          `json:"scope"`
	Summary string          `json:"summary"`
	Points  json.RawMessage `json:"points"`
}

// liteTeacherWriting assembles writing's slice of the item-detail response:
// her outline, her snippets, her draft, and 印记's comments on it. Never
// touches atom_message (comments are the AI-facing output that IS shown —
// see WritingComment, a review of her draft, not the chat that produced it).
func (a *API) liteTeacherWriting(ctx context.Context, atomID uuid.UUID) (map[string]any, error) {
	wr, err := a.d.Queries.GetWriting(ctx, atomID)
	if err != nil {
		return nil, err
	}
	targetWords := int32(0)
	if wr.TargetWords != nil {
		targetWords = *wr.TargetWords
	}

	outlineRows, err := a.d.Queries.ListWritingOutline(ctx, atomID)
	if err != nil {
		return nil, err
	}
	outline := make([]liteOutlineDTO, 0, len(outlineRows))
	for _, o := range outlineRows {
		outline = append(outline, liteOutlineDTO{Role: o.Role, Text: o.Text})
	}

	snippetRows, err := a.d.Queries.ListWritingSnippets(ctx, atomID)
	if err != nil {
		return nil, err
	}
	snippets := make([]liteSnippetDTO, 0, len(snippetRows))
	for _, s := range snippetRows {
		snippets = append(snippets, liteSnippetDTO{Position: s.Position, Text: s.Text})
	}

	draft, err := notFoundIsNil(a.d.Queries.GetWritingDraft(ctx, atomID))
	if err != nil {
		return nil, err
	}
	draftBody := ""
	if draft != nil {
		draftBody = draft.Body
	}

	commentRows, err := a.d.Queries.ListWritingComments(ctx, atomID)
	if err != nil {
		return nil, err
	}
	comments := make([]liteWritingCommentDTO, 0, len(commentRows))
	for _, c := range commentRows {
		comments = append(comments, liteWritingCommentDTO{
			Scope: c.Scope, Summary: c.Summary, Points: json.RawMessage(c.Points),
		})
	}

	return map[string]any{
		"targetWords":  targetWords,
		"lang":         wr.Lang,
		"structureKey": wr.StructureKey,
		"outline":      outline,
		"snippets":     snippets,
		"draft":        draftBody,
		"comments":     comments,
	}, nil
}

// --- project -------------------------------------------------------------

type litePlanStepDTO struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

type liteToolDTO struct {
	Key    string          `json:"key"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
}

type liteArtifactDTO struct {
	Title   string          `json:"title"`
	Payload json.RawMessage `json:"payload"`
}

type liteKeepDTO struct {
	Text string `json:"text"`
}

type liteCourseDTO struct {
	Slug       string  `json:"slug"`
	Why        string  `json:"why"`
	Takeaway   string  `json:"takeaway"`
	FinishedAt *string `json:"finishedAt"`
}

// liteTeacherProject assembles project's slice of the item-detail response:
// idea/status, the live plan's steps, tools, artifacts, keeps, course
// assignments, and — for the student's website project — its share token
// once published. Never touches atom_message.
//
// Takes userID (not in the brief's terse signature) because GetPblSite is
// keyed by user_id, not atom_id — a student has one site row, not one per
// project.
func (a *API) liteTeacherProject(ctx context.Context, userID, atomID uuid.UUID) (map[string]any, error) {
	proj, err := a.d.Queries.GetPblProject(ctx, atomID)
	if err != nil {
		return nil, err
	}

	version, err := notFoundIsNil(a.d.Queries.GetPblLivePlan(ctx, atomID))
	if err != nil {
		return nil, err
	}
	var steps []litePlanStepDTO
	stepsDone, stepsTotal := 0, 0
	if version != nil {
		stepRows, err := a.d.Queries.ListPblPlanSteps(ctx, version.ID)
		if err != nil {
			return nil, err
		}
		steps = make([]litePlanStepDTO, 0, len(stepRows))
		for _, s := range stepRows {
			steps = append(steps, litePlanStepDTO{Title: s.Title, Status: s.Status})
			stepsTotal++
			if s.Status == "done" {
				stepsDone++
			}
		}
	}

	toolRows, err := a.d.Queries.ListPblTools(ctx, atomID)
	if err != nil {
		return nil, err
	}
	tools := make([]liteToolDTO, 0, len(toolRows))
	for _, tl := range toolRows {
		tools = append(tools, liteToolDTO{Key: tl.Tool, Status: tl.Status, Result: json.RawMessage(tl.Result)})
	}

	artifactRows, err := a.d.Queries.ListPblArtifacts(ctx, atomID)
	if err != nil {
		return nil, err
	}
	artifacts := make([]liteArtifactDTO, 0, len(artifactRows))
	for _, ar := range artifactRows {
		artifacts = append(artifacts, liteArtifactDTO{Title: ar.Title, Payload: json.RawMessage(ar.Payload)})
	}

	keepRows, err := a.d.Queries.ListPblKeepEntries(ctx, atomID)
	if err != nil {
		return nil, err
	}
	keeps := make([]liteKeepDTO, 0, len(keepRows))
	for _, k := range keepRows {
		keeps = append(keeps, liteKeepDTO{Text: k.Body})
	}

	courseRows, err := a.d.Queries.ListPblCourseAssignments(ctx, atomID)
	if err != nil {
		return nil, err
	}
	courses := make([]liteCourseDTO, 0, len(courseRows))
	for _, c := range courseRows {
		var finishedAt *string
		if c.FinishedAt.Valid {
			s := c.FinishedAt.Time.Format(time.RFC3339)
			finishedAt = &s
		}
		courses = append(courses, liteCourseDTO{Slug: c.CourseSlug, Why: c.Why, Takeaway: c.Takeaway, FinishedAt: finishedAt})
	}

	var siteToken *string
	if proj.Kind == "website" {
		site, err := notFoundIsNil(a.d.Queries.GetPblSite(ctx, userID))
		if err != nil {
			return nil, err
		}
		if site != nil && site.PublishedAt.Valid && site.ShareToken != nil && *site.ShareToken != "" {
			siteToken = site.ShareToken
		}
	}

	return map[string]any{
		"idea":       proj.Idea,
		"status":     proj.Status,
		"stepsDone":  stepsDone,
		"stepsTotal": stepsTotal,
		"steps":      steps,
		"tools":      tools,
		"artifacts":  artifacts,
		"keeps":      keeps,
		"courses":    courses,
		"siteToken":  siteToken,
	}, nil
}
