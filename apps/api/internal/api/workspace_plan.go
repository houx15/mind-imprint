package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// workspace_plan.go — Slice 2 (Project Management room) cheap-CRUD handlers:
// the four kick-off proposal dimensions, the plan board / gantt items, and the
// activity log. None of these make a model call (that is /coach's job, in
// coach.go), so none gate on HasEntitlement — they are plain owned-project REST.

// -- Proposal ---------------------------------------------------------------

// putProposal upserts the four kick-off dimensions. First save also drops a
// light auto-log line so the working-phase timeline shows the project taking
// shape.
func (a *API) putProposal(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Objective     string `json:"objective"`
		Reason        string `json:"reason"`
		Activities    string `json:"activities"`
		Resources     string `json:"resources"`
		Counterpoints string `json:"counterpoints"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// "First save" = no proposal row yet. Read before the upsert so the auto-log
	// fires exactly once, on the transition from absent to present.
	_, getErr := a.d.Queries.GetProjectProposal(r.Context(), projectID)
	firstSave := errors.Is(getErr, pgx.ErrNoRows)

	row, err := a.d.Queries.UpsertProjectProposal(r.Context(), sqlc.UpsertProjectProposalParams{
		ProjectID:     projectID,
		Objective:     body.Objective,
		Reason:        body.Reason,
		Activities:    body.Activities,
		Resources:     body.Resources,
		Counterpoints: body.Counterpoints,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if firstSave {
		if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "开题四问初次落定"); err != nil {
			slog.Warn("proposal: append auto-log failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
		// S1 · lever 1 (compaction): the proposal has solidified, so the 立题
		// shaping dialogue is now durable structure — fold those raw turns out
		// of the coach's active window. Best-effort; a fold failure must never
		// fail the save. The turns stay in the thread; the spine carries the
		// result.
		store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
		if err := store.FoldCoachSurfaces(r.Context(), projectID, solidifyFoldSurfaces); err != nil {
			slog.Warn("proposal: fold shaping turns failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
		}
	}

	// Server-side funnel: if this save just completed the four required dims,
	// generate the plan and advance the funnel NOW — so the plan is ready the
	// instant the student confirms the last dim (印记's 记进), not on their next
	// chat turn. Best-effort; a failure never fails the save. Only runs once the
	// journey has started (studio_state present + Started).
	if raw, gerr := a.d.Queries.GetStudioState(r.Context(), projectID); gerr == nil && len(raw) > 0 {
		var st agent.StudioState
		if json.Unmarshal(raw, &st) == nil && st.Started {
			reconciled, autoPlan := a.reconcileStudioFunnel(r.Context(), projectID, st)
			if autoPlan || reconciled.Stage != st.Stage {
				if b, merr := json.Marshal(reconciled); merr == nil {
					if serr := a.d.Queries.SetStudioState(r.Context(), sqlc.SetStudioStateParams{ID: projectID, StudioState: b}); serr != nil {
						slog.Warn("proposal: persist reconciled studio_state failed", "err", serr, "request_id", httpx.RequestIDFromContext(r.Context()))
					}
				}
			}
		}
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{"proposal": workspaceProposal{
		Objective:     row.Objective,
		Reason:        row.Reason,
		Activities:    row.Activities,
		Resources:     row.Resources,
		Counterpoints: row.Counterpoints,
	}})
}

// -- Plan items -------------------------------------------------------------

// planItemDTO is the wire shape (contracts.PlanItem): DB `col` -> `column`,
// `ref_material_id` -> `refMaterialId`, `start_day` -> `start`.
type planItemDTO struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Tag           string  `json:"tag"`
	Column        string  `json:"column"`
	Stage         string  `json:"stage"`
	RefMaterialID *string `json:"refMaterialId"`
	Start         int32   `json:"start"`
	Days          int32   `json:"days"`
	Position      int32   `json:"position"`
	// CreatedAt anchors the Gantt timeline to when the plan was actually made:
	// regeneratePlan recreates every item wholesale, so the earliest item's
	// created_at IS the plan's start date (= "today" when freshly generated).
	CreatedAt string `json:"createdAt"`
}

func toPlanItemDTO(row sqlc.PlanItem) planItemDTO {
	return planItemDTO{
		ID:            row.ID.String(),
		Title:         row.Title,
		Tag:           row.Tag,
		Column:        row.Col,
		Stage:         row.Stage,
		RefMaterialID: pgUUIDToStringPtr(row.RefMaterialID),
		Start:         row.StartDay,
		Days:          row.Days,
		Position:      row.Position,
		CreatedAt:     row.CreatedAt.UTC().Format(time.RFC3339),
	}
}

var validPlanTags = map[string]bool{"read": true, "write": true, "review": true}
var validPlanColumns = map[string]bool{"todo": true, "doing": true, "done": true}

// planItemToUpdateParams copies an existing row into an UpdatePlanItemParams so
// callers can override just the fields they change (mirrors patchPlanItem's
// read-modify-write).
func planItemToUpdateParams(projectID uuid.UUID, it sqlc.PlanItem) sqlc.UpdatePlanItemParams {
	return sqlc.UpdatePlanItemParams{
		ID: it.ID, ProjectID: projectID, Title: it.Title, Tag: it.Tag, Col: it.Col,
		Stage: it.Stage, RefMaterialID: it.RefMaterialID, StartDay: it.StartDay,
		Days: it.Days, Position: it.Position,
	}
}

// findPlanItem resolves an update_plan target: by exact id first (scoped to the
// project), else the first item whose title contains `match` (case-insensitive).
func (a *API) findPlanItem(ctx context.Context, projectID uuid.UUID, id, match string) (sqlc.PlanItem, bool) {
	if id = strings.TrimSpace(id); id != "" {
		if iid, err := uuid.Parse(id); err == nil {
			if row, err := a.d.Queries.GetPlanItem(ctx, sqlc.GetPlanItemParams{ID: iid, ProjectID: projectID}); err == nil {
				return row, true
			}
		}
	}
	if match = strings.TrimSpace(match); match != "" {
		if rows, err := a.d.Queries.ListPlanItems(ctx, projectID); err == nil {
			lm := strings.ToLower(match)
			for _, r := range rows {
				if strings.Contains(strings.ToLower(r.Title), lm) {
					return r, true
				}
			}
		}
	}
	return sqlc.PlanItem{}, false
}

// applyPlanUpdate executes one update_plan op (see agent.UpdatePlanArgsT) and
// returns an auto-log line + ok. A missing target or a failed write is a silent
// no-op (ok=false) — the coach's narration lands regardless, like generate_plan.
func (a *API) applyPlanUpdate(ctx context.Context, projectID uuid.UUID, args agent.UpdatePlanArgsT) (string, bool) {
	if args.Op == "add" {
		title := strings.TrimSpace(args.Title)
		if title == "" {
			return "", false
		}
		tag := args.Tag
		if !validPlanTags[tag] {
			tag = "write"
		}
		stage := strings.TrimSpace(args.Stage)
		if stage == "" {
			stage = "阶段一 · 研究"
		}
		days := args.Days
		if days < 1 {
			days = 3
		}
		if _, err := a.d.Queries.CreatePlanItem(ctx, sqlc.CreatePlanItemParams{
			ProjectID: projectID, Title: title, Tag: tag, Col: "todo", Stage: stage,
			StartDay: 0, Days: days, Position: 0,
		}); err != nil {
			return "", false
		}
		return "印记新增计划任务：" + title, true
	}

	item, ok := a.findPlanItem(ctx, projectID, args.ID, args.Match)
	if !ok {
		return "", false
	}
	switch args.Op {
	case "complete", "start", "reopen":
		col := map[string]string{"complete": "done", "start": "doing", "reopen": "todo"}[args.Op]
		next := planItemToUpdateParams(projectID, item)
		next.Col = col
		if _, err := a.d.Queries.UpdatePlanItem(ctx, next); err != nil {
			return "", false
		}
		verb := map[string]string{"done": "完成", "doing": "进行中", "todo": "待办"}[col]
		return "印记把「" + item.Title + "」标为" + verb, true
	case "edit":
		next := planItemToUpdateParams(projectID, item)
		if t := strings.TrimSpace(args.Title); t != "" {
			next.Title = t
		}
		if s := strings.TrimSpace(args.Stage); s != "" {
			next.Stage = s
		}
		if validPlanTags[args.Tag] {
			next.Tag = args.Tag
		}
		if args.Days >= 1 {
			next.Days = args.Days
		}
		if _, err := a.d.Queries.UpdatePlanItem(ctx, next); err != nil {
			return "", false
		}
		return "印记调整了计划任务：" + next.Title, true
	case "remove":
		if err := a.d.Queries.DeletePlanItem(ctx, sqlc.DeletePlanItemParams{ID: item.ID, ProjectID: projectID}); err != nil {
			return "", false
		}
		return "印记移除了计划任务：" + item.Title, true
	}
	return "", false
}

// resourceNeedExists reports whether a keyword is already in the box (case-
// insensitive substring), so note_resource_need never adds a duplicate.
func resourceNeedExists(needs []agent.ResourceNeed, text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	for _, n := range needs {
		if strings.Contains(strings.ToLower(n.Text), t) {
			return true
		}
	}
	return false
}

// listPlan returns the project's plan items ordered (stage, position, start).
func (a *API) listPlan(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListPlanItems(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]planItemDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, toPlanItemDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// createPlanItem inserts one plan item. Also drops a light auto-log line.
func (a *API) createPlanItem(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Title         string  `json:"title"`
		Tag           string  `json:"tag"`
		Column        string  `json:"column"`
		Stage         string  `json:"stage"`
		RefMaterialID *string `json:"refMaterialId"`
		Start         int32   `json:"start"`
		Days          int32   `json:"days"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !validPlanTags[body.Tag] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "tag 只能是 read/write/review", nil))
		return
	}
	if !validPlanColumns[body.Column] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "column 只能是 todo/doing/done", nil))
		return
	}
	refID, err := stringPtrToPgUUID(body.RefMaterialID)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "refMaterialId 不是有效的 id", nil))
		return
	}
	if body.Days < 1 {
		body.Days = 1
	}

	row, err := a.d.Queries.CreatePlanItem(r.Context(), sqlc.CreatePlanItemParams{
		ProjectID:     projectID,
		Title:         body.Title,
		Tag:           body.Tag,
		Col:           body.Column,
		Stage:         body.Stage,
		RefMaterialID: refID,
		StartDay:      body.Start,
		Days:          body.Days,
		Position:      0,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := a.appendAutoLog(r.Context(), a.d.Queries, projectID, "新增计划任务："+strings.TrimSpace(body.Title)); err != nil {
		slog.Warn("plan item: append auto-log failed", "err", err, "request_id", httpx.RequestIDFromContext(r.Context()))
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"item": toPlanItemDTO(row)})
}

// patchPlanItem loads the current item (scoped to the owned project -> 404 if it
// belongs to another project or does not exist), merges whatever partial fields
// the body supplied over it, then writes ALL columns back.
func (a *API) patchPlanItem(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	cur, err := a.d.Queries.GetPlanItem(r.Context(), sqlc.GetPlanItemParams{ID: iid, ProjectID: projectID})
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在")) // pgx.ErrNoRows or wrong project -> 404-no-leak
		return
	}

	// Every field a pointer: absent (nil) means "keep current", present means
	// "replace". refMaterialId is doubly optional — a present-but-null value
	// clears the link — so it carries its own "provided" flag via json.RawMessage.
	var body struct {
		Title         *string `json:"title"`
		Tag           *string `json:"tag"`
		Column        *string `json:"column"`
		Stage         *string `json:"stage"`
		Start         *int32  `json:"start"`
		Days          *int32  `json:"days"`
		Position      *int32  `json:"position"`
		RefMaterialID *string `json:"refMaterialId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	next := sqlc.UpdatePlanItemParams{
		ID:            iid,
		ProjectID:     projectID,
		Title:         cur.Title,
		Tag:           cur.Tag,
		Col:           cur.Col,
		Stage:         cur.Stage,
		RefMaterialID: cur.RefMaterialID,
		StartDay:      cur.StartDay,
		Days:          cur.Days,
		Position:      cur.Position,
	}
	if body.Title != nil {
		next.Title = *body.Title
	}
	if body.Tag != nil {
		if !validPlanTags[*body.Tag] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "tag 只能是 read/write/review", nil))
			return
		}
		next.Tag = *body.Tag
	}
	if body.Column != nil {
		if !validPlanColumns[*body.Column] {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "column 只能是 todo/doing/done", nil))
			return
		}
		next.Col = *body.Column
	}
	if body.Stage != nil {
		next.Stage = *body.Stage
	}
	if body.Start != nil {
		next.StartDay = *body.Start
	}
	if body.Days != nil {
		d := *body.Days
		if d < 1 {
			d = 1
		}
		next.Days = d
	}
	if body.Position != nil {
		next.Position = *body.Position
	}
	if body.RefMaterialID != nil {
		refID, err := stringPtrToPgUUID(body.RefMaterialID)
		if err != nil {
			httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "refMaterialId 不是有效的 id", nil))
			return
		}
		next.RefMaterialID = refID
	}

	row, err := a.d.Queries.UpdatePlanItem(r.Context(), next)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"item": toPlanItemDTO(row)})
}

// deletePlanItem removes one item (idempotent — a miss still 204s, since the
// item is gone either way; ownership is enforced by the project_id scope).
func (a *API) deletePlanItem(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	iid, err := uuid.Parse(r.PathValue("iid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	if err := a.d.Queries.DeletePlanItem(r.Context(), sqlc.DeletePlanItemParams{ID: iid, ProjectID: projectID}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// -- Activity log -----------------------------------------------------------

// logEntryDTO is the wire shape (contracts.LogEntry): entry_date -> "MM-DD".
type logEntryDTO struct {
	ID     string `json:"id"`
	Date   string `json:"date"`
	Text   string `json:"text"`
	Source string `json:"source"`
}

func toLogEntryDTO(row sqlc.ActivityLogEntry) logEntryDTO {
	date := ""
	if row.EntryDate.Valid {
		date = row.EntryDate.Time.Format("01-02")
	}
	return logEntryDTO{ID: row.ID.String(), Date: date, Text: row.Text, Source: row.Source}
}

// listLog returns the project's activity log, chronological (newest last).
func (a *API) listLog(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListActivityLog(r.Context(), projectID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	entries := make([]logEntryDTO, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, toLogEntryDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

// postLog appends one manual log line (source="me", entry_date=today).
func (a *API) postLog(w http.ResponseWriter, r *http.Request) {
	projectID, ok := a.loadOwnedProject(w, r)
	if !ok {
		return
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &body); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("validation_failed", "记一笔不能是空的", nil))
		return
	}
	row, err := a.d.Queries.CreateActivityLogEntry(r.Context(), sqlc.CreateActivityLogEntryParams{
		ProjectID: projectID,
		EntryDate: todayDate(),
		Text:      body.Text,
		Source:    "me",
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"entry": toLogEntryDTO(row)})
}

// -- Shared helpers ---------------------------------------------------------

// appendAutoLog inserts one source='auto', entry_date=today activity-log line.
// Exported (package-internal) for later slices to drop a timeline breadcrumb on
// their own platform actions (source opened, snapshot committed, ...). Takes a
// *sqlc.Queries so it works with both a.d.Queries and a transaction's querier.
func (a *API) appendAutoLog(ctx context.Context, q *sqlc.Queries, projectID uuid.UUID, text string) error {
	_, err := q.CreateActivityLogEntry(ctx, sqlc.CreateActivityLogEntryParams{
		ProjectID: projectID,
		EntryDate: todayDate(),
		Text:      text,
		Source:    "auto",
	})
	return err
}

// todayDate is the pgtype.Date for the current day.
func todayDate() pgtype.Date {
	return pgtype.Date{Time: time.Now(), Valid: true}
}

// pgUUIDToStringPtr renders a nullable ref material id as *string (null -> nil).
func pgUUIDToStringPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

// stringPtrToPgUUID parses a nullable id string into pgtype.UUID. A nil pointer
// or empty string is a valid NULL; a malformed non-empty string is an error.
func stringPtrToPgUUID(s *string) (pgtype.UUID, error) {
	if s == nil || strings.TrimSpace(*s) == "" {
		return pgtype.UUID{}, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
