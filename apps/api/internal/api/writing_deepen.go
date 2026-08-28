package api

// writing_deepen.go — 深入一层 — the same 印记, opened onto ONE block.
//
// The student meets no second character: the drawer is headed 「印记 · 关于这一块」,
// with no new name and no new avatar. The reading room spent a whole session
// collapsing two AIs into one; this must not quietly undo that.
//
// Under the hood it is a context-isolated sub-agent, spawned server-side and
// briefed with: the title, the whole map (so it can see where this block sits),
// this block's role/heading/text, and the guide questions already shown so it
// does not re-ask them. It is NOT given the planning transcript — that exclusion
// is what keeps it focused and cheap, and TestBuildDeepenBrief pins it.
//
// WHAT IS GUARANTEED, HONESTLY: this endpoint has no write path to
// writing_outline or writing_snippet, so it CANNOT author her outline or her
// prose — that part is structural. But it is a free-form chat, so the guide
// box's "output must be questions" type guarantee does not apply here; that
// rests on the prompt, exactly as the main 印记's chat already does. Giving it
// the vocab library instead of letting it compose illustrations is what narrows
// the surface. Known limit, accepted deliberately.

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// deepenSystem — the same four-part 怎么说话 doctrine every other 印记 voice
// in this room speaks (writingGuideTeachingRules, copied verbatim rather than
// re-derived so an engineer reading these files out of order never finds two
// versions of how 印记 talks), plus the Socratic charter specific to this
// sub-agent: it asks, it never writes her sentence, and its examples come
// from the borrowed material in the vocab library rather than her own topic.
const deepenSystem = writingGuideTeachingRules + `

你是在帮她想这一块，用苏格拉底式的追问：问出她已经知道但还没说出来的东西。绝不替她写句子。你可以从【可用的方法】里举例子——那些例子讲的都是别的题目，不是她的。`

// liteDeepenTurnReq is postWritingBlockDeepen's request body — one line of
// text, the same shape as liteWritingTurnReq (writing_turn.go): this sub-agent
// has no article, no focused spans, nothing beyond what she just typed.
type liteDeepenTurnReq struct {
	Text string `json:"text"`
}

// deepenTurnDTO is the wire shape of one deepen turn's reply.
type deepenTurnDTO struct {
	Reply string `json:"reply"`
}

// buildDeepenBrief assembles the WHOLE context this sub-agent gets, and
// nothing else: the paper's title and language, the whole outline map (so it
// can see where this block sits relative to every other block), this block's
// role/heading/text and what she has already drafted in it, and the guide
// questions already shown to her (so it does not re-ask them). It does NOT
// take the planning transcript (atom_message, block_id IS NULL) as a
// parameter — that omission from the signature IS the guarantee, not just an
// unused argument that could be added later. TestBuildDeepenBrief pins it.
func buildDeepenBrief(wr sqlc.Writing, outline []sqlc.WritingOutline, block sqlc.WritingOutline, snippetText string, guideQuestions []string) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目：" + t + "\n")
	}
	b.WriteString("写作语言：" + wr.Lang + "\n")

	b.WriteString("\n【整篇的结构】\n")
	for _, s := range outline {
		indent := strings.Repeat("  ", int(s.Depth))
		role := s.Role
		if role == "" {
			role = "（未命名的块）"
		}
		line := indent + "- " + role
		if t := strings.TrimSpace(s.Text); t != "" {
			line += "：" + t
		} else {
			line += "：（还没写）"
		}
		if s.ID == block.ID {
			line += "   ← **她现在停在这一块**"
		}
		b.WriteString(line + "\n")
	}

	b.WriteString("\n【她现在停住的这一块】\n")
	b.WriteString("这一块的作用：" + block.Role + "\n")
	if t := strings.TrimSpace(block.Text); t != "" {
		b.WriteString("她给这一块定的要点：" + t + "\n")
	} else {
		b.WriteString("她还没给这一块定要点。\n")
	}
	if t := strings.TrimSpace(snippetText); t != "" {
		b.WriteString("她已经写下的段落内容：\n" + t + "\n")
	} else {
		b.WriteString("这一段还是空的。\n")
	}

	if len(guideQuestions) > 0 {
		b.WriteString("\n【已经给她看过的引导问题——不要在这个对话里重复问这些】\n")
		for _, q := range guideQuestions {
			b.WriteString("- " + q + "\n")
		}
	}

	// Position AND language (vocab.For): this sub-agent is the one that actually
	// shows examples, so a wrong-language entry here would be read out loud.
	b.WriteString("\n【可用的方法】（举例子只能用这里的，别自己编，例子讲的是别的题目，不是她的）\n")
	for _, m := range vocab.For(writingGuideAppliesTo(block.Role), wr.Lang) {
		b.WriteString("- id=" + m.ID + " · " + m.Label() + "：" + m.Definition + "\n")
	}
	return b.String()
}

// findWritingOutlineBlock locates oid inside an already-loaded outline
// slice, scoped to THIS atom — mirrors guideWritingBlock's inline lookup
// (writing_guide.go): a valid uuid belonging to someone else's writing is a
// 404 here, not a leak.
func findWritingOutlineBlock(outline []sqlc.WritingOutline, oid uuid.UUID) (sqlc.WritingOutline, bool) {
	for _, s := range outline {
		if s.ID == oid {
			return s, true
		}
	}
	return sqlc.WritingOutline{}, false
}

// deepenBlockGuideQuestions decodes the questions already shown for a block
// out of its stored `guide` column (writing_guide.go's writingGuideDTO) — the
// same stored shape guideWritingBlock/guideWritingBlocks persist. A block
// with no guide yet (or an unreadable one) simply hands over no questions;
// that is a legitimate state, not a failure worth surfacing here.
func deepenBlockGuideQuestions(block sqlc.WritingOutline) []string {
	if len(block.Guide) == 0 {
		return nil
	}
	var g writingGuideDTO
	if err := json.Unmarshal(block.Guide, &g); err != nil {
		return nil
	}
	return g.Questions
}

// deepenBlockSnippetText finds what she has already drafted in this block,
// if anything — same lookup guideWritingBlock does against ListWritingSnippets.
func deepenBlockSnippetText(snippets []sqlc.WritingSnippet, oid uuid.UUID) string {
	for _, s := range snippets {
		if s.OutlineID.Valid && s.OutlineID.Bytes == oid {
			return s.Text
		}
	}
	return ""
}

// appendBlockMessage allocates the next seq (shared with the room's own
// thread — ONE seq space per atom, per Task 1) and appends one block-scoped
// turn, inside its own short transaction: NextAtomMessageSeq's doc comment
// asks for "the same transaction as this read" as the actual guard against a
// concurrent double-append, not the read itself.
func (a *API) appendBlockMessage(ctx context.Context, atomID uuid.UUID, blockID *string, role, content string) (sqlc.AtomMessage, error) {
	tx, err := a.d.Pool.Begin(ctx)
	if err != nil {
		return sqlc.AtomMessage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := a.d.Queries.WithTx(tx)

	next, err := qtx.NextAtomMessageSeq(ctx, atomID)
	if err != nil {
		return sqlc.AtomMessage{}, err
	}
	row, err := qtx.AppendAtomBlockMessage(ctx, sqlc.AppendAtomBlockMessageParams{
		AtomID: atomID, Seq: next, Role: role, Content: content, BlockID: blockID,
	})
	if err != nil {
		return sqlc.AtomMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.AtomMessage{}, err
	}
	return row, nil
}

// blockThreadToChatMessages converts a block's stored thread into gateway
// chat turns — the same student/ai → user/assistant mapping
// buildWritingTurnHistory uses for the room's own thread, just scoped to one
// block's rows instead of the whole atom.
func blockThreadToChatMessages(msgs []sqlc.AtomMessage) []gateway.ChatMessage {
	out := make([]gateway.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		switch m.Role {
		case "student":
			out = append(out, gateway.ChatMessage{Role: gateway.RoleUser, Content: m.Content})
		case "ai":
			out = append(out, gateway.ChatMessage{Role: gateway.RoleAssistant, Content: m.Content})
		}
	}
	return out
}

// deepenWritingBlock is POST /api/v1/writings/{id}/outline/{oid}/deepen — one
// turn of the block-scoped Socratic sub-agent. A spend endpoint (one model
// call), metered as purpose="block_deepen".
//
// Order of operations, deliberately: read the prior block thread → allocate a
// seq and persist HER turn immediately, before the model is ever called → ask
// the model → meter → persist the reply. Her turn lands in the thread even if
// the model call then fails, the same "what she said is never lost" posture
// postLiteWritingTurn's transcript already has — a network hiccup must not
// also erase her half of the conversation.
func (a *API) deepenWritingBlock(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	oid, err := uuid.Parse(r.PathValue("oid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req liteDeepenTurnReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)
	if studentText == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先说点什么，我在听。", nil))
		return
	}

	// Run to completion even if she navigates away mid-call — same reasoning
	// and same 150s cap as every other spend endpoint in this room.
	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	outline, err := a.d.Queries.ListWritingOutline(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	block, found := findWritingOutlineBlock(outline, oid)
	if !found {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	snippets, err := a.d.Queries.ListWritingSnippets(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	snippetText := deepenBlockSnippetText(snippets, oid)
	guideQuestions := deepenBlockGuideQuestions(block)

	blockID := oid.String()
	prior, err := a.d.Queries.ListAtomBlockMessages(turnCtx, sqlc.ListAtomBlockMessagesParams{
		AtomID: at.ID, BlockID: &blockID,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	if _, err := a.appendBlockMessage(turnCtx, at.ID, &blockID, "student", studentText); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	brief := buildDeepenBrief(wr, outline, block, snippetText, guideQuestions)
	messages := make([]gateway.ChatMessage, 0, len(prior)+2)
	messages = append(messages, gateway.ChatMessage{Role: gateway.RoleSystem, Content: deepenSystem + "\n\n" + brief})
	messages = append(messages, blockThreadToChatMessages(prior)...)
	messages = append(messages, gateway.ChatMessage{Role: gateway.RoleUser, Content: studentText})

	// §model-routing: a good Socratic follow-up on a half-formed argument is
	// the same hard reasoning guideWritingBlock's flagship call does — resolves
	// EvalResolver, not writing_turn.go's chaperone ChatResolver.
	resolved, ok2 := a.resolveEval(turnCtx)
	if !ok2 {
		slog.Warn("writing block deepen: no provider resolved",
			"atom_id", at.ID, "outline_id", oid, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{Messages: messages})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "block_deepen", resolved, res.Usage)
	reply := strings.TrimSpace(res.Text)
	if cerr != nil || reply == "" {
		slog.Warn("writing block deepen: provider call failed", "err", cerr,
			"atom_id", at.ID, "outline_id", oid, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	if _, err := a.appendBlockMessage(turnCtx, at.ID, &blockID, "ai", reply); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, deepenTurnDTO{Reply: reply})
}

// getWritingBlockThread is GET /api/v1/writings/{id}/outline/{oid}/deepen —
// this block's whole deepen thread, oldest first, so reopening the drawer
// returns to the conversation rather than a blank slate. No entitlement
// gate — no model call, no spend, same reasoning as every other GET in this
// room (listWritingComments, getWritingOutline).
func (a *API) getWritingBlockThread(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	oid, err := uuid.Parse(r.PathValue("oid"))
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}
	outline, err := a.d.Queries.ListWritingOutline(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, found := findWritingOutlineBlock(outline, oid); !found {
		httpx.WriteError(w, r, httpx.ErrNotFound("资源不存在"))
		return
	}

	blockID := oid.String()
	rows, err := a.d.Queries.ListAtomBlockMessages(r.Context(), sqlc.ListAtomBlockMessagesParams{
		AtomID: at.ID, BlockID: &blockID,
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]liteMessageDTO, 0, len(rows))
	for _, m := range rows {
		out = append(out, liteMessageDTO{
			Seq: m.Seq, Role: m.Role, Content: m.Content,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"messages": out})
}
