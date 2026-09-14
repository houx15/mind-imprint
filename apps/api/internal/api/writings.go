package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writings.go — the lite edition's 写作 atom: "type one sentence into a box
// and you're started." Like reading (readings.go), it owns its storage
// outright (atom + writing + the shared atom_* machinery) and never touches
// any project-scoped table.

type writingDTO struct {
	ID          string `json:"id"` // the ATOM id — every writing endpoint is keyed by it
	Title       string `json:"title"`
	Lang        string `json:"lang"`
	Stage       string `json:"stage"`
	TargetWords *int32 `json:"targetWords"`
	// StructureKey names the skeleton she picked out of the fixed structure
	// library (writing_structures.go); "" = not chosen yet. SetupAt is when
	// the entry 设定 dialog was completed; null = never, which is exactly
	// what the frontend gates that dialog on. Both are 0100 columns.
	StructureKey string  `json:"structureKey"`
	SetupAt      *string `json:"setupAt"`
	// Origin: "here" = 在这个房间里写的；"brought" = 她带进来的成稿。
	// 界面据此说明结构和段落两步没有发生过，报告也据此说真话
	// （见 0146_writing_origin.sql）。
	Origin    string `json:"origin"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	// UpdatedAt is writing.updated_at: rename / stage change / target-words
	// only. It is NOT "when she last worked on this" — see LastActivityAt.
	UpdatedAt string `json:"updatedAt"`
	// LastActivityAt is atom.last_activity_at (0098): the last time she wrote
	// ANYTHING into this writing — a turn, an outline edit, a snippet, a draft
	// save. This is what 上次改到 means and what 「你有 N 篇还没写完」 orders
	// by. Mirrors readingDTO.LastActivityAt exactly (readings.go); before this
	// field existed the writing shelf had nothing honest to sort by, since an
	// hour spent drafting moved neither updated_at nor created_at.
	LastActivityAt string  `json:"lastActivityAt"`
	FinishedAt     *string `json:"finishedAt"`
}

func writingDTOOf(wr sqlc.Writing, createdAt, lastActivityAt time.Time) writingDTO {
	out := writingDTO{
		ID: wr.AtomID.String(), Title: wr.Title, Lang: wr.Lang, Stage: wr.Stage,
		TargetWords: wr.TargetWords, StructureKey: wr.StructureKey, Status: wr.Status,
		Origin:         wr.Origin,
		CreatedAt:      createdAt.Format(time.RFC3339),
		UpdatedAt:      wr.UpdatedAt.Format(time.RFC3339),
		LastActivityAt: lastActivityAt.Format(time.RFC3339),
	}
	if wr.SetupAt.Valid {
		s := wr.SetupAt.Time.Format(time.RFC3339)
		out.SetupAt = &s
	}
	if wr.FinishedAt.Valid {
		s := wr.FinishedAt.Time.Format(time.RFC3339)
		out.FinishedAt = &s
	}
	return out
}

// writingListDefaultLimit / writingListMaxLimit bound 我的写作. Mirrors
// readingListDefaultLimit / readingListMaxLimit (readings.go) exactly — same
// reasoning, same numbers: generous enough for one student, capped so a
// hand-crafted `?limit=100000` cannot ask the API to serialize her whole
// writing history.
const (
	writingListDefaultLimit = 50
	writingListMaxLimit     = 200
)

// writingBroughtMaxRunes 是她能带进来的一篇稿子的长度上限。
//
// 按 rune 数。两万字比任何一篇中学作文都宽（IB 的 EE 四千词，中文四千字上下），
// 但它是有界的：这篇稿子会整份进通篇审阅那一次调用的上文，而 lite 没有压缩层。
// 超了就明确报错，不做静默截断——截掉一半再去评，评的是另一篇文章。
const writingBroughtMaxRunes = 20000

// writingListLimit reads `?limit=`, mirroring readingListLimit. Anything
// missing, unparseable, or ≤0 takes the default.
func writingListLimit(r *http.Request) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return writingListDefaultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return writingListDefaultLimit
	}
	if n > writingListMaxLimit {
		return writingListMaxLimit
	}
	return n
}

// writingRecency is when this writing last mattered: the moment she finished
// it, or — while still open — the last time she wrote anything into it.
// Mirrors readingRecency (readings.go) exactly; RFC3339 sorts correctly as a
// string, which is why these stay formatted.
func writingRecency(d writingDTO) string {
	if d.FinishedAt != nil && *d.FinishedAt != "" {
		return *d.FinishedAt
	}
	return d.LastActivityAt
}

// loadOwnedWritingAtom is loadOwnedAtom (readings.go) curried to "writing" —
// the exact mirror of loadOwnedReadingAtom. Every W3-W7 handler funnels
// through this one chokepoint for existence/ownership/kind checks, the
// finished-write gate, and the last_activity_at touch.
func (a *API) loadOwnedWritingAtom(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	return a.loadOwnedAtom(w, r, "writing")
}

// loadOwnedWritingAtomRow is loadOwnedWritingAtom's ungated sibling, curried
// to "writing" for the same reason loadOwnedReadingAtomRow is (readings.go):
// finishWritingAtom (writing_compose.go) uses this directly so a second
// POST /finish stays callable after the first one already succeeded.
func (a *API) loadOwnedWritingAtomRow(w http.ResponseWriter, r *http.Request) (sqlc.Atom, bool) {
	return a.loadOwnedAtomRow(w, r, "writing")
}

// createWriting is the "type one sentence into a box" entry point. The
// sentence she types — idea — does double duty: truncated to 200 runes it
// becomes the writing's initial title, and verbatim (untruncated) it becomes
// the first atom_message (role='student') — because 先聊's first line really
// is the one she just said, and it must not vanish from the transcript.
//
// Unlike reading, an empty idea is refused outright (400 missing_idea):
// reading can be created first and have its article pasted in later, but a
// writing has nothing to talk about without one.
//
// atom + writing + the first atom_message are inserted in ONE transaction:
// a writing whose title exists but whose opening line does not (or vice
// versa) is a half-created writing, and the point of the transaction is that
// no caller can ever observe that state — either all three rows exist or
// none do.
func (a *API) createWriting(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}
	var req struct {
		Idea string `json:"idea"`
		Lang string `json:"lang"`
		// Body 是她**已经写完**带进来的那篇稿子（2026-09-11）。
		//
		// 产品负责人的原话：
		//   > we also make students available to upload a written one to seek
		//   > for advice
		//
		// 给了 Body，这一篇就直接落在 draft、来源记成 brought，
		// 走成稿那一侧的通篇审阅——她要的是意见，不是从零开始。
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	idea := strings.TrimSpace(req.Idea)
	if idea == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_idea", "先说说你想写点什么。", nil))
		return
	}
	body := strings.TrimSpace(req.Body)
	if len([]rune(body)) > writingBroughtMaxRunes {
		httpx.WriteError(w, r, httpx.ErrBadRequest("body_too_long",
			"这篇太长了，超出了一次能处理的长度。", nil))
		return
	}
	// atom + writing + seq-1 message go through createWritingInTx, in this
	// handler's own transaction, so the brought body below joins it.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	atID, err := createWritingInTx(r.Context(), qtx, u.ID, idea, req.Lang)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// 她带了一篇写完的进来。
	if body != "" {
		if _, err := qtx.UpsertWritingDraft(r.Context(), sqlc.UpsertWritingDraftParams{
			AtomID: atID, Body: body,
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if err := qtx.MarkWritingBrought(r.Context(), atID); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		// 🚨 正文**不进转录**。它只落在 writing_draft 里。
		//
		// 两个理由。一，转录是每一条下游提示词的共同材料（开场、结构、每一块
		// 引导都读它），把一整篇作文塞进去会把每一次调用的上文顶爆，而 lite
		// 没有压缩层。二，也是更要紧的：转录里的 student 行是「她在这个房间
		// 里说过的话」，而这篇稿子是她在别处写完带进来的——混在一起，过程
		// 评估就再也分不清哪些是这里发生的。
		//
		// 落一条 system 行记下这件事，和 stage 变更用的是同一种记法
		// （writing_stage.go 的 "stage: a → b"）：它是一条结构性记录，
		// 不是谁「说」的话。
		if _, err := qtx.AppendAtomMessage(r.Context(), sqlc.AppendAtomMessageParams{
			AtomID: atID, Seq: 2, Role: "system", Content: "origin: brought",
		}); err != nil {
			httpx.WriteError(w, r, err)
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": atID.String()})
}

func (a *API) listWritings(w http.ResponseWriter, r *http.Request) {
	u, _ := UserFromContext(r.Context())
	rows, err := a.d.Queries.ListWritingsByUser(r.Context(), u.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]writingDTO, 0, len(rows))
	for _, row := range rows {
		out = append(out, writingDTOOf(sqlc.Writing{
			AtomID: row.AtomID, Title: row.Title, Lang: row.Lang, Stage: row.Stage,
			TargetWords: row.TargetWords, StructureKey: row.StructureKey, SetupAt: row.SetupAt,
			Status:    row.Status,
			UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
		}, row.AtomCreatedAt, row.LastActivityAt))
	}
	// 我的写作 is a shelf, not an archive — same ordering + cut as listReadings
	// (readings.go): most recent by when each writing last mattered (finished
	// → when she finished it; still open → when she last touched it), capped
	// to `limit`. See listReadings's comment for why the sort lives here
	// rather than in the query's ORDER BY (which is atom.created_at).
	sort.SliceStable(out, func(i, j int) bool { return writingRecency(out[i]) > writingRecency(out[j]) })
	total := len(out)
	if n := writingListLimit(r); n < len(out) {
		out = out[:n]
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"writings": out, "total": total})
}

func (a *API) getWriting(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
}

// renameWriting is PATCH /writings/{id}: the only field this task's PATCH
// touches is title, mirroring renameReading. Stage/targetWords get their own
// endpoints in later tasks (W3-W7) — this one does not reach into them.
func (a *API) renameWriting(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_title", "标题不能为空。", nil))
		return
	}
	if len([]rune(title)) > 200 {
		title = string([]rune(title)[:200])
	}
	if err := a.d.Queries.RenameWriting(r.Context(), sqlc.RenameWritingParams{AtomID: at.ID, Title: title}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	wr, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
}
