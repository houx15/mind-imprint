package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_stage.go — Task 3 of the lite writing phase: record which of the
// four steps (构思 ideate → 大纲 outline → 段落 snippets → 成稿 draft) she's
// on, and how long the piece is meant to be. Both are recorded facts, never
// gates:
//
//   - Stage is a MAP shown to the student, not a checkpoint the server
//     enforces (铁律②: never manipulate — no gating). Jumping ideate→snippets
//     (skipping outline) is 200, and going backward (snippets→outline) is
//     200 too — wanting to go back and fill in the outline is normal. The
//     writing.stage CHECK constraint (0099) is the only real guard; the same
//     five values are validated here so an invalid one is a clean 400
//     instead of a 500 surfaced from Postgres.
//   - Every stage change — especially a skip — writes ONE atom_message
//     role='system' recording from→to (铁律④: process is data; skipping is
//     allowed AND recorded, never silently prevented). See setWritingStage's
//     comment on why the update and the trace are one atomic transaction.
//   - targetWords is NEVER a precondition for anything (2026-08-27 product
//     ruling, overrides any older reading of the spec that implied length
//     had to be settled before leaving 构思). It may be set at ANY stage and
//     may stay NULL forever; no endpoint may 400/403 because it's NULL. It
//     gets its own PUT with no interaction with stage at all —
//     SetWritingStage and SetWritingTargetWords are deliberately independent
//     sqlc methods (like SetWritingFinished, which also never touches
//     stage), and this handler does not couple them either.

// validWritingStages is the vocabulary this API ACCEPTS, and it is
// deliberately NARROWER than the writing.stage CHECK constraint (0099),
// which still permits 'ideate'.
//
// The four-step map collapsed to three on 2026-08-27 — 结构 / 段落 / 成稿 —
// when 构思 stopped being a page of its own: what little lived there (目标
// 篇幅) moved into the entry 设定 dialog, and the thinking it was supposed to
// host is now the coach's opening line plus the per-block guiding questions.
// 0100 migrated every existing 'ideate' row to 'outline'.
//
// The CHECK constraint was left alone on purpose: tightening it means
// rebuilding it, and a database that still tolerates a value nothing writes
// costs nothing. THIS map is the enforcement point, so a stale client posting
// 'ideate' gets a clean 400 rather than parking a writing on a stage with no
// page behind it.
var validWritingStages = map[string]bool{
	"outline": true, "snippets": true, "draft": true, "finished": true,
}

const (
	minTargetWords = 1
	maxTargetWords = 100000
)

// setWritingStage is POST /api/v1/writings/{id}/stage.
func (a *API) setWritingStage(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	var req struct {
		Stage string `json:"stage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if !validWritingStages[req.Stage] {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_stage", "无效的写作阶段。", nil))
		return
	}

	// The stage update and its atom_message trace are written in ONE
	// transaction, exactly like createWriting's atom+writing+first-message
	// insert and postLiteReadingTurn's student+ai append (reading_turn.go):
	// either both the new stage and the record of how she got there commit,
	// or (on any failure — including a failed Commit itself) neither does.
	// A skip applied but not recorded would make the skip invisible, which
	// is the one outcome 铁律④ exists to prevent; a skip recorded but not
	// applied would desync the trace from the truth. Atomicity rules out
	// both.
	tx, err := a.d.Pool.Begin(r.Context())
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	qtx := a.d.Queries.WithTx(tx)

	before, err := qtx.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	wr, err := qtx.SetWritingStage(r.Context(), sqlc.SetWritingStageParams{AtomID: at.ID, Stage: req.Stage})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	// 串行化这颗原子的 seq 分配。NextAtomMessageSeq 是先读后插，
	// 并发下两笔事务会读到同一个 MAX——唯一索引保住的是数据，代价是
	// 其中一轮直接失败，而那一轮的模型钱已经花掉了。见 queries/atom.sql。
	if _, err := qtx.LockAtom(r.Context(), at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next, err := qtx.NextAtomMessageSeq(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(r.Context(), sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: next, Role: "system",
		Content: fmt.Sprintf("stage: %s → %s", before.Stage, req.Stage),
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, writingDTOOf(wr, at.CreatedAt, at.LastActivityAt))
}

// setWritingTargetWords is PUT /api/v1/writings/{id}/target-words. No stage
// check, no stage side effect — see the file comment: length is never a
// precondition for anything, at any stage.
//
// An assigned writing's target is the teacher's (owner, 2026-09-15), so it is
// 409 assigned_target_locked there, whatever the body says.
func (a *API) setWritingTargetWords(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	cur, err := a.d.Queries.GetWriting(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if writingIsAssigned(cur) {
		httpx.WriteError(w, r, &httpx.APIError{Status: http.StatusConflict, Code: "assigned_target_locked", Message: "作业的字数要求由老师设定"})
		return
	}
	var req struct {
		TargetWords int `json:"targetWords"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, r, httpx.ErrBadJSON(err))
		return
	}
	if req.TargetWords < minTargetWords || req.TargetWords > maxTargetWords {
		httpx.WriteError(w, r, httpx.ErrBadRequest("invalid_target_words", "目标字数需在 1 到 100000 之间。", nil))
		return
	}
	tw := int32(req.TargetWords)
	if err := a.d.Queries.SetWritingTargetWords(r.Context(), sqlc.SetWritingTargetWordsParams{
		AtomID: at.ID, TargetWords: &tw,
	}); err != nil {
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
