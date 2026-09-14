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

// InboxItemDTO is one entry in the student's inbox. Type is "assignment";
// other item types share the list later.
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
}

// getLiteInbox handles GET /api/v1/lite/inbox. Archived assignments are left
// out by the query.
func (a *API) getLiteInbox(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListLiteInboxAssignments(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	now := time.Now()
	items := make([]InboxItemDTO, 0, len(rows))
	unread := 0
	for _, row := range rows {
		status := liteassign.Status(row.StartedAt.Valid, tsPtr(row.FinishedAt), row.DueAt, now)
		if !row.SeenAt.Valid {
			unread++
		}
		items = append(items, InboxItemDTO{
			Type: "assignment", ID: row.ID.String(), Kind: row.Kind, Title: row.Title,
			Instructions: row.Instructions, ClassName: row.ClassName, DueAt: row.DueAt.Format(time.RFC3339),
			Status: status, StatusLabel: liteassign.StatusLabel(status),
			AtomID: uuidStringPtr(row.AtomID), Unread: !row.SeenAt.Valid,
		})
	}
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

// startLiteAssignment handles POST /api/v1/lite/assignments/{aid}/start.
//
// Idempotent. The recipient row is locked FOR UPDATE first and the assignment
// is read through the same transaction, so a second concurrent press waits
// for the first to link its item and then returns that item. The item is
// created through the helpers the student's own create buttons use; they
// commit their own transactions while the lock is held.
//
// An assigned project skips the homepage gate.
func (a *API) startLiteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u, _ := UserFromContext(ctx)
	entitled, err := HasEntitlement(ctx, u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	aid, err := uuid.Parse(r.PathValue("aid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	rec, err := qtx.GetLiteAssignmentRecipientForUpdate(ctx, sqlc.GetLiteAssignmentRecipientForUpdateParams{AssignmentID: aid, UserID: u.ID})
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	as, err := qtx.GetLiteAssignment(ctx, aid)
	if err != nil {
		writeNotFoundOr(w, r, err)
		return
	}
	if as.ArchivedAt.Valid {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	if rec.AtomID.Valid {
		if err := tx.Commit(ctx); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		writeStartResponse(w, as.Kind, uuid.UUID(rec.AtomID.Bytes))
		return
	}

	atomID, err := a.createAssignedItem(ctx, tx, u.ID, as)
	if err != nil {
		httpx.WriteError(w, r, startErrorResponse(err, as.ID))
		return
	}
	if err := qtx.SetLiteAssignmentStarted(ctx, sqlc.SetLiteAssignmentStartedParams{
		AssignmentID: aid, UserID: u.ID, AtomID: pgtype.UUID{Bytes: atomID, Valid: true},
	}); err != nil {
		slog.Warn("lite assignment start: item created but not linked",
			"err", err, "assignment_id", aid.String(), "atom_id", atomID.String())
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		slog.Warn("lite assignment start: item created but link not committed",
			"err", err, "assignment_id", aid.String(), "atom_id", atomID.String())
		httpx.WriteError(w, r, err)
		return
	}
	writeStartResponse(w, as.Kind, atomID)
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

// createAssignedItem creates her reading, writing or project from the stored
// payload. tx is the start transaction holding the recipient lock; it is only
// read from here.
func (a *API) createAssignedItem(ctx context.Context, tx pgx.Tx, userID uuid.UUID, as sqlc.LiteAssignment) (uuid.UUID, error) {
	switch as.Kind {
	case "reading":
		var p liteassign.ReadingPayload
		if err := json.Unmarshal(as.Payload, &p); err != nil {
			return uuid.Nil, err
		}
		switch p.Source {
		case "library":
			return a.startAssignedLibraryReading(ctx, tx, userID, p)
		case "url":
			return a.createReadingWithSourceFor(ctx, userID, as.Title, langOf(as.Title), p.URL, "")
		case "text":
			return a.createReadingWithSourceFor(ctx, userID, as.Title, langOf(p.Text), "", p.Text)
		}
		return uuid.Nil, errors.New("lite assignment: unknown reading source " + p.Source)
	case "writing":
		var p liteassign.WritingPayload
		if err := json.Unmarshal(as.Payload, &p); err != nil {
			return uuid.Nil, err
		}
		tw := int32(p.TargetWords)
		return a.createWritingFor(ctx, userID, p.Prompt, p.Lang, &tw)
	case "project":
		var p liteassign.ProjectPayload
		if err := json.Unmarshal(as.Payload, &p); err != nil {
			return uuid.Nil, err
		}
		pr, err := a.createPblProjectFor(ctx, userID, p.DrivingQuestion)
		if err != nil {
			return uuid.Nil, err
		}
		return pr.AtomID, nil
	}
	return uuid.Nil, errors.New("lite assignment: unknown kind " + as.Kind)
}

// startAssignedLibraryReading opens the library article at the payload's
// tier, or at her suggested tier when the teacher left it open. An unfinished
// reading of the same article and tier is reused, unless another assignment
// already links it: recipient.atom_id is UNIQUE, and one item answers one
// assignment.
func (a *API) startAssignedLibraryReading(ctx context.Context, tx pgx.Tx, userID uuid.UUID, p liteassign.ReadingPayload) (uuid.UUID, error) {
	var tier int
	if p.Tier != nil {
		tier = *p.Tier
	} else {
		suggested, err := a.suggestLibraryTierFor(ctx, userID)
		if err != nil {
			return uuid.Nil, err
		}
		tier = suggested
	}
	id, resumed, err := a.createLibraryReadingFor(ctx, userID, p.Slug, tier)
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
	return a.createLibraryReadingFreshFor(ctx, userID, p.Slug, tier)
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
		return httpx.ErrBadRequest("fetch_failed", "开始失败：这个链接抓不到正文，请直接粘贴。", nil)
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
		"id": row.ID.String(), "title": row.Title, "dueAt": row.DueAt.Format(time.RFC3339),
	}})
}
