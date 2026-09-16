package api

// lite_student_assignments.go — a student's side of a lite assignment: the
// inbox, marking one seen, 开始 (which creates her item on first press) and
// looking up the assignment an item belongs to.
//
// Nothing here returns atom_message content.

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/liteassign"
	"mindimprint/api/internal/store/sqlc"
)

// InboxItemDTO is one assignment in the student's inbox (Type "assignment");
// sent gradings are InboxGradingDTO (lite_student_gradings.go). Every key is
// always present: an assignment with no instructions still has
// "instructions", and one she has not started has "atomId": null.
type InboxItemDTO struct {
	Type         string  `json:"type"`
	ID           string  `json:"id"`
	Kind         string  `json:"kind"`
	Title        string  `json:"title"`
	Instructions string  `json:"instructions"`
	ClassName    string  `json:"className"`
	DueAt        string  `json:"dueAt"`
	Status       string  `json:"status"`
	StatusLabel  string  `json:"statusLabel"`
	AtomID       *string `json:"atomId"`
	Unread       bool    `json:"unread"`
	// Set when the teacher returned this writing homework (0153).
	ReturnDueAt *string `json:"returnDueAt"`
	ReturnNote  *string `json:"returnNote"`
}

// getLiteInbox handles GET /api/v1/lite/inbox. Archived assignments, and those
// of a class she is no longer enrolled in, are left out by the query, which
// also orders the list: unread first, then by due date.
func (a *API) getLiteInbox(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, _ := UserFromContext(ctx)
	rows, err := a.d.Queries.ListLiteInboxAssignments(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	items := make([]any, 0, len(rows))
	unread := 0
	for _, row := range rows {
		status := liteassign.StatusWithReturn(row.StartedAt.Valid, tsPtr(row.FinishedAt), row.DueAt, now,
			returnOf(row.ReturnedAt, row.ReturnDueAt, row.Resubmitted))
		if !row.SeenAt.Valid {
			unread++
		}
		items = append(items, InboxItemDTO{
			Type: "assignment", ID: row.ID.String(), Kind: row.Kind, Title: row.Title,
			Instructions: row.Instructions, ClassName: row.ClassName, DueAt: row.DueAt.Format(time.RFC3339),
			Status: status, StatusLabel: liteassign.StatusLabel(status),
			AtomID: uuidStringPtr(row.AtomID), Unread: !row.SeenAt.Valid,
			ReturnDueAt: tsStringPtr(row.ReturnDueAt), ReturnNote: row.ReturnNote,
		})
	}

	// Sent 批改 of her writings follow the assignments, newest first.
	gradingRows, err := a.d.Queries.ListLiteInboxGradings(ctx, u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	gradingItems, gradingUnread := inboxGradingItems(gradingRows)
	for _, it := range gradingItems {
		items = append(items, it)
	}
	unread += gradingUnread

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "unread": unread})
}

// markLiteAssignmentSeen handles POST /api/v1/lite/assignments/{aid}/seen.
// Only a recipient of a live assignment may mark it; anything else is 404.
func (a *API) markLiteAssignmentSeen(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, _ := UserFromContext(ctx)
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if _, err := a.d.Queries.GetLiteAssignmentRecipient(ctx, sqlc.GetLiteAssignmentRecipientParams{AssignmentID: aid, UserID: u.ID}); err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	as, err := a.d.Queries.GetLiteAssignment(ctx, aid)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	if as.ArchivedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err := a.d.Queries.MarkLiteAssignmentSeen(ctx, sqlc.MarkLiteAssignmentSeenParams{AssignmentID: aid, UserID: u.ID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeNotFoundOr(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	httpx.WriteError(w, r, err)
}

// errAssignmentGone: the assignment is archived or deleted, or she is not (or
// no longer) a recipient. Every one of these answers 404.
var errAssignmentGone = errors.New("lite assignment archived, gone, or not hers")

// startAfterPreflightHook runs between the unlocked preflight and the locked re-read; tests set it, production leaves it nil.
var startAfterPreflightHook func()

// lockAssignmentStart takes a start's locks: the assignment FOR SHARE, then her
// recipient row FOR UPDATE, and returns both locked rows.
//
// The order is shared with patchLiteAssignment (assignment first, then
// recipient rows). The reverse order deadlocks against a PATCH that removes
// this recipient.
//
// FOR SHARE: a PATCH takes FOR UPDATE on the assignment before it reads
// settings or counts started recipients, so it waits until this start has
// linked her item (or rolled back) and cannot change the payload the item is
// built from.
//
// A missing or archived assignment, or a missing recipient row, is
// errAssignmentGone.
func lockAssignmentStart(ctx context.Context, qtx *sqlc.Queries, assignmentID, userID uuid.UUID) (sqlc.LiteAssignment, sqlc.LiteAssignmentRecipient, error) {
	as, err := qtx.GetLiteAssignmentForShare(ctx, assignmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.LiteAssignment{}, sqlc.LiteAssignmentRecipient{}, errAssignmentGone
	}
	if err != nil {
		return sqlc.LiteAssignment{}, sqlc.LiteAssignmentRecipient{}, err
	}
	if as.ArchivedAt.Valid {
		return sqlc.LiteAssignment{}, sqlc.LiteAssignmentRecipient{}, errAssignmentGone
	}
	rec, err := qtx.GetLiteAssignmentRecipientForUpdate(ctx, sqlc.GetLiteAssignmentRecipientForUpdateParams{AssignmentID: assignmentID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.LiteAssignment{}, sqlc.LiteAssignmentRecipient{}, errAssignmentGone
	}
	if err != nil {
		return sqlc.LiteAssignment{}, sqlc.LiteAssignmentRecipient{}, err
	}
	return as, rec, nil
}

// startLiteAssignment handles POST /api/v1/lite/assignments/{aid}/start.
//
// Idempotent, and it holds at most one pooled connection at a time:
//
//  1. Preflight without a transaction or locks: 404 unless she is a current
//     recipient of a live assignment in a class she is still enrolled in; an
//     item she already started is returned as is.
//  2. Network work (a link reading's fetch) before any transaction opens, so
//     no lock is held while a page downloads.
//  3. One transaction: lockAssignmentStart, then re-check. A concurrent start
//     that already linked an item wins and its item is returned; a teacher
//     edit that changed the settings since the preflight is 409.
//  4. Her item is created, linked and started through that same transaction.
//     A failure anywhere rolls all of it back: no orphan item.
//
// An assigned project skips the homepage gate.
func (a *API) startLiteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, _ := UserFromContext(ctx)
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	// 1. Preflight.
	as, err := a.d.Queries.GetLiteAssignment(ctx, aid)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	if as.ArchivedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	rec, err := a.d.Queries.GetLiteAssignmentRecipient(ctx, sqlc.GetLiteAssignmentRecipientParams{AssignmentID: aid, UserID: u.ID})
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	enrolled, err := a.d.Queries.IsEnrolledStudent(ctx, sqlc.IsEnrolledStudentParams{ClassID: as.ClassID, UserID: u.ID})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !enrolled {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if rec.AtomID.Valid {
		writeStartResponse(w, as.Kind, uuid.UUID(rec.AtomID.Bytes))
		return
	}
	entitled, err := HasEntitlement(ctx, u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	// 2. Network work.
	article, err := a.fetchAssignedArticle(ctx, as)
	if err != nil {
		httpx.WriteError(w, r, startErrorResponse(err, as.ID))
		return
	}
	if startAfterPreflightHook != nil {
		startAfterPreflightHook()
	}

	// 3. Lock and re-check.
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	locked, lockedRec, err := lockAssignmentStart(ctx, qtx, aid, u.ID)
	if errors.Is(err, errAssignmentGone) {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if lockedRec.AtomID.Valid {
		writeStartResponse(w, locked.Kind, uuid.UUID(lockedRec.AtomID.Bytes))
		return
	}
	if locked.Kind != as.Kind || !samePayload(locked.Payload, as.Payload) {
		httpx.WriteError(w, r, &httpx.APIError{
			Status: http.StatusConflict, Code: "assignment_changed",
			Message: "开始失败：老师刚修改了这份作业，请重新开始",
		})
		return
	}

	// 4. Create, link, commit.
	atomID, err := a.createAssignedItemInTx(ctx, tx, qtx, u.ID, locked, article)
	if err != nil {
		httpx.WriteError(w, r, startErrorResponse(err, as.ID))
		return
	}
	if err := qtx.SetLiteAssignmentStarted(ctx, sqlc.SetLiteAssignmentStartedParams{
		AssignmentID: aid, UserID: u.ID, AtomID: pgtype.UUID{Bytes: atomID, Valid: true},
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Warn("lite assignment start: commit failed",
			"err", err, "assignment_id", aid.String())
		httpx.WriteError(w, r, err)
		return
	}
	writeStartResponse(w, locked.Kind, atomID)
}

// writeStartResponse: for a project, projectId is the atom id (pbl_project's
// primary key is atom_id); for the other kinds it is null.
func writeStartResponse(w http.ResponseWriter, kind string, atomID uuid.UUID) {
	var projectID *string
	if kind == "project" {
		s := atomID.String()
		projectID = &s
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"kind": kind, "atomId": atomID.String(), "projectId": projectID})
}

// assignedArticle is a link reading's article, fetched before the start
// transaction opens.
type assignedArticle struct {
	title, lang, body, url string
}

// fetchAssignedArticle fetches a link reading's page. Every other assignment
// needs no network work and returns nil.
//
// The title is left blank so the page's own title is used, and the language
// comes from the fetched body: the assignment's title is the teacher's name for
// the practice, not the article's.
func (a *API) fetchAssignedArticle(ctx context.Context, as sqlc.LiteAssignment) (*assignedArticle, error) {
	if as.Kind != "reading" {
		return nil, nil
	}
	var p liteassign.ReadingPayload
	if err := json.Unmarshal(as.Payload, &p); err != nil {
		return nil, err
	}
	if p.Source != "url" {
		return nil, nil
	}
	title, body, err := a.resolveReadingSource(ctx, "", p.URL, "")
	if err != nil {
		return nil, err
	}
	return &assignedArticle{title: title, lang: readingLangOf(body), body: body, url: p.URL}, nil
}

// createAssignedItemInTx creates her reading, writing or project from the
// locked assignment, through the start transaction. article is the fetched
// page for a link reading and nil otherwise.
func (a *API) createAssignedItemInTx(ctx context.Context, tx pgx.Tx, qtx *sqlc.Queries, userID uuid.UUID, as sqlc.LiteAssignment, article *assignedArticle) (uuid.UUID, error) {
	switch as.Kind {
	case "reading":
		var p liteassign.ReadingPayload
		if err := json.Unmarshal(as.Payload, &p); err != nil {
			return uuid.Nil, err
		}
		switch p.Source {
		case "library":
			return startAssignedLibraryReading(ctx, tx, qtx, userID, p)
		case "personalized":
			// The recommendation is computed through qtx, so it happens
			// under the recipient row lock this transaction holds.
			target, err := personalizedTargetIn(ctx, qtx, userID, p)
			if err != nil {
				return uuid.Nil, err
			}
			return startAssignedLibraryReading(ctx, tx, qtx, userID, target)
		case "url":
			if article == nil {
				return uuid.Nil, errors.New("lite assignment: link reading started without its fetched article")
			}
			return createReadingWithSourceInTx(ctx, qtx, userID, article.title, article.lang, article.url, article.body)
		case "text":
			// A non-blank text is never fetched, so this makes no network call.
			title, body, err := a.resolveReadingSource(ctx, as.Title, "", p.Text)
			if err != nil {
				return uuid.Nil, err
			}
			return createReadingWithSourceInTx(ctx, qtx, userID, title, langOf(body), "", body)
		}
		return uuid.Nil, errors.New("lite assignment: unknown reading source " + p.Source)
	case "writing":
		var p liteassign.WritingPayload
		if err := json.Unmarshal(as.Payload, &p); err != nil {
			return uuid.Nil, err
		}
		return createAssignedWritingInTx(ctx, qtx, userID, as.Title, p.Prompt, p.Lang, int32(p.TargetWords))
	case "project":
		var p liteassign.ProjectPayload
		if err := json.Unmarshal(as.Payload, &p); err != nil {
			return uuid.Nil, err
		}
		return createAssignedPblProjectInTx(ctx, qtx, userID, as.Title, p.DrivingQuestion, combinePblAssignmentBrief(p.Description, as.Instructions))
	}
	return uuid.Nil, errors.New("lite assignment: unknown kind " + as.Kind)
}

// startAssignedLibraryReading opens the library article at the payload's
// tier, or at her suggested tier when the teacher left it open. An unfinished
// reading of the same article and tier is reused, unless another assignment
// already links it: recipient.atom_id is UNIQUE, and one item answers one
// assignment.
func startAssignedLibraryReading(ctx context.Context, tx pgx.Tx, qtx *sqlc.Queries, userID uuid.UUID, p liteassign.ReadingPayload) (uuid.UUID, error) {
	var tier int
	if p.Tier != nil {
		tier = *p.Tier
	} else {
		suggested, err := suggestLibraryTierInTx(ctx, qtx, userID)
		if err != nil {
			return uuid.Nil, err
		}
		tier = suggested
	}
	id, resumed, err := createLibraryReadingInTx(ctx, qtx, userID, p.Slug, tier)
	if err != nil || !resumed {
		return id, err
	}
	var linked bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM lite_assignment_recipient WHERE atom_id = $1)`, id).Scan(&linked); err != nil {
		return uuid.Nil, err
	}
	if !linked {
		return id, nil
	}
	return createLibraryReadingFreshInTx(ctx, qtx, userID, p.Slug, tier)
}

// langOf is en when s has no Han character, zh otherwise.
func langOf(s string) string {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return "zh"
		}
	}
	return "en"
}

// startErrorResponse maps a creation helper's error to the response. A fetch
// failure's cause carries fetcher internals: it is logged, never returned.
func startErrorResponse(err error, assignmentID uuid.UUID) error {
	switch {
	case errors.Is(err, errLibraryArticleNotFound):
		return httpx.ErrNotFound("这篇文章不在阅读库里")
	case errors.Is(err, errLibraryTierInvalid):
		return httpx.ErrBadRequest("bad_tier", "这篇文章没有这一档", nil)
	case errors.Is(err, errInvalidSourceURL):
		return httpx.ErrBadRequest("invalid_url", "请输入以 http 或 https 开头的链接", nil)
	case errors.Is(err, errFetchUnavailable):
		return httpx.ErrBadRequest("fetch_unavailable", "开始失败：暂时无法抓取链接", nil)
	case errors.Is(err, errFetchFailed):
		slog.Warn("lite assignment start: fetch failed", "err", err, "assignment_id", assignmentID.String())
		return httpx.ErrBadRequest("fetch_failed", "开始失败：链接无法读取正文，请告知老师更换阅读材料", nil)
	case errors.Is(err, errMissingSourceText):
		return httpx.ErrBadRequest("missing_text", "开始失败：文章没有正文", nil)
	case errors.Is(err, errEmptyIdea):
		return httpx.ErrBadRequest("empty_idea", "开始失败：作业题目为空", nil)
	}
	return err
}

// getLiteAssignmentForAtom handles GET /api/v1/lite/assignments/for-atom/{atomId}.
// An atom that is not hers, or answers no live assignment, gets
// {"assignment": null}: the answer is the same either way, so it says nothing
// about another student's items.
func (a *API) getLiteAssignmentForAtom(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, _ := UserFromContext(ctx)
	atomID, err := uuid.Parse(r.PathValue("atomId"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	row, err := a.d.Queries.GetLiteAssignmentForAtom(ctx, sqlc.GetLiteAssignmentForAtomParams{
		AtomID: pgtype.UUID{Bytes: atomID, Valid: true}, UserID: u.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"assignment": nil})
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"assignment": map[string]any{
		// 🚨 两条线的并集（2026-09-16）：退回修改那几个字段来自已经上线的教师端，
		// instructions 来自作业材料那条线。少任何一边都会让对面那个功能在这一个
		// 端点上静默失效 —— 前端读不到的字段就是不存在。
		"id": row.ID.String(), "kind": row.Kind, "title": row.Title, "dueAt": row.DueAt.Format(time.RFC3339),
		"returnedAt": tsStringPtr(row.ReturnedAt), "returnDueAt": tsStringPtr(row.ReturnDueAt),
		"returnNote": row.ReturnNote, "resubmitted": row.Resubmitted,
		"instructions": row.Instructions,
	}})
}
