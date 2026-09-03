package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
)

// writing_turn.go — 陪练一轮 for the WRITING room (Task 4).
//
// Step 1 finding, recorded here as the brief asked: agent.RouteReading (the
// producer postLiteReadingTurn reuses, reading_turn.go) is shaped around an
// Article plus a reading-card catalog — neither of which writing has. What
// writing has instead — the idea, the outline she has confirmed, the
// fragments she has already written, and which of the four stages
// (构思/大纲/段落/成稿) she is in — has no dedicated router of its own, and
// none is needed: §6.2.2 of the spec asks for "talk first" — an always-reply
// conversational coach, not a card-routing decision. That producer already
// exists in internal/agent, already proven in production: pro's own
// project-wide coach (coach.go's postCoach / postCoachSubagentTurn,
// card_reflect.go) is built on exactly agent.ProposeProjectCoachReply — a
// tool-less, room-aware, always-reply producer that takes (history, a
// free-text state projection, an active-surface label) and returns one
// restrained reply. That is precisely the three things writing can supply:
// the windowed transcript, a projection of title/target-words/outline/
// snippets, and the current stage as the surface label. internal/agent
// required ZERO changes to serve this room — the same reuse thesis as
// reading, on a different existing producer because the shape of what
// writing has is different from what reading has.
//
// Card summoning is deliberately OUT of this turn: Task 1.5 left
// liteSummonCard reading-only ("a writing room summons over an outline or
// snippet — a different shape entirely... Writing gets its own summon
// endpoint"), and no such endpoint exists yet anywhere in this phase's plan.
// So decision/card/nudge/hintCardId below hold liteTurnDTO's shape — the
// exact shape the frontend already speaks for reading — at their zero value
// (decision "respond", card nil) until that door is built.
//
// The three jobs this handler does, mirroring postLiteReadingTurn exactly:
//  1. assemble — agent.ChatTurn history (windowed) + a projection built from
//     the lite writing tables
//  2. persist  — student turn + AI turn in ONE transaction, consecutive seq
//  3. meter    — one llm_call row (surface='lite', purpose='writing_turn')

// writingTurnsWindow bounds how many atom_message rows feed the coach's
// context. A SEPARATE constant from reading_turn.go's recentTurnsWindow,
// deliberately not reused, even though both currently read 12: they guard two
// different producers whose prompts are built out of different material (an
// article that must keep dominating the prompt, versus a stage/outline/
// snippet projection that already carries its own separate size discipline —
// see buildWritingCoachProjection's truncateRunes calls). Collapsing them into
// one shared constant would couple two rooms' prompt budgets to a single
// number for no reason other than that the numbers happen to match today.
// Load-bearing for the same reason as reading's: lite has NO compaction
// layer, so this window is the only thing bounding prompt growth on a long
// writing thread.
const writingTurnsWindow = 12

// writingStageLabels names each stage for the coach's "you are here" framing
// (mirrors coachSurfaceLabel's pro-side room names, projectcoach.go, but
// keyed on writing.stage rather than a room scope).
//
// Two entries here are READ-ONLY mappings for values this API no longer
// accepts (validWritingStages, writing_stage.go): 'finished' cannot reach
// this handler at all (loadOwnedWritingAtom's finished-write gate refuses the
// POST first), and 'ideate' was retired when the map collapsed to three steps
// — 0100 migrated the rows, but an un-migrated replica or an old fixture
// could still hand one over, and a label lookup must degrade to the right
// step rather than to the generic "写作".
var writingStageLabels = map[string]string{
	"ideate":   "结构",
	"outline":  "结构",
	"snippets": "段落",
	"draft":    "成稿",
	"finished": "成稿",
}

func writingStageLabel(stage string) string {
	if s, ok := writingStageLabels[stage]; ok {
		return s
	}
	return "写作"
}

// liteWritingTurnReq is postLiteWritingTurn's request body. Writing has no
// analogue of reading's focusedSpans (there is no article to select spans
// in), so this is deliberately smaller than reading_turn.go's liteTurnReq.
type liteWritingTurnReq struct {
	Text string `json:"text"`
}

// buildWritingCoachProjection assembles the free-text state projection
// agent.ProposeProjectCoachReply's BuildProjectCoachContext renders verbatim
// under "项目当前状态（供你参考，别照搬复述）". Kept pure and separate from
// the handler so the "what does writing have instead of an article" assembly
// can be reasoned about (and tested) on its own, mirroring
// buildReadingRouteInput's split in reading_turn.go.
//
// target_words is reported as either a number or "还没定" (never omitted,
// never invented) — 2026-08-27 product ruling (W-R7): the coach must be able
// to SEE that length is still unsettled while she is in 构思 so she can raise
// it, but nothing here treats it as a precondition.
func buildWritingCoachProjection(wr sqlc.Writing, outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("题目/想法：" + t + "\n")
	}
	// 🚨 The language rule reaches the MAIN coach chat here. It was missing
	// entirely, which is why 印记 kept discussing an English piece as though
	// every artifact it produced should be Chinese. See writing_lang.go.
	b.WriteString(writingLangLine(wr))
	// 🚨 「约 %d 字」 was hard-coded here too. On an English piece this told the
	// coach a 500-word essay was 500 Chinese characters — see writing_lang.go
	// for the advice that came out the other end.
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "目标篇幅"))
	} else {
		b.WriteString("目标篇幅：还没定\n")
	}
	if len(outline) > 0 {
		b.WriteString("已确定的提纲：\n")
		for _, o := range outline {
			indent := strings.Repeat("  ", int(o.Depth))
			fmt.Fprintf(&b, "%s- %s\n", indent, truncateRunes(o.Text, 120))
		}
	}
	nonEmpty := 0
	for _, s := range snippets {
		if strings.TrimSpace(s.Text) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 0 {
		b.WriteString("已经写好的片段：\n")
		for _, s := range snippets {
			text := strings.TrimSpace(s.Text)
			if text == "" {
				continue
			}
			fmt.Fprintf(&b, "  [%d] %s\n", s.Position+1, truncateRunes(text, 300))
		}
	}
	return b.String()
}

// buildWritingTurnHistory windows the raw transcript to writingTurnsWindow
// (never the whole thread — see the constant's comment) and converts it to
// agent.ChatTurn, then appends the CURRENT student turn — mirroring how
// coach.go's postCoachSubagentTurn builds ProposeProjectCoachReply's history
// (load prior, then append the turn under way, so the producer's context
// always ends with what she just said regardless of whether the persist
// below succeeds).
//
// role='system' rows (writing_stage.go's stage-transition trace, e.g. "stage:
// ideate → outline") are deliberately excluded from the conversation itself —
// they are a structural record, not something either side "said", and
// BuildProjectCoachContext has no third bucket for them; folding one in under
// "学生：" would misattribute it to her. They still count against the window
// (it slices the raw tail first, then filters), so a writing thread with many
// stage skips cannot inflate the prompt past the same bound reading enjoys.
func buildWritingTurnHistory(msgs []sqlc.AtomMessage, studentText string) []agent.ChatTurn {
	tail := msgs
	if len(tail) > writingTurnsWindow {
		tail = tail[len(tail)-writingTurnsWindow:]
	}
	history := make([]agent.ChatTurn, 0, len(tail)+1)
	for _, m := range tail {
		switch m.Role {
		case "student":
			history = append(history, agent.ChatTurn{Role: "user", Content: m.Content})
		case "ai":
			history = append(history, agent.ChatTurn{Role: "assistant", Content: m.Content})
		}
	}
	return append(history, agent.ChatTurn{Role: "user", Content: studentText})
}

// postLiteWritingTurn drives one coach turn for the writing room.
func (a *API) postLiteWritingTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	u, _ := UserFromContext(r.Context())
	// Entitlement is checked BEFORE the model call — this is a token-spending
	// endpoint, exactly where the HasEntitlement seam belongs (mirrors
	// postLiteReadingTurn).
	entitled, err := HasEntitlement(r.Context(), u)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	var req liteWritingTurnReq
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)
	if studentText == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先说点什么，我在听。", nil))
		return
	}

	// Run to completion even if the student navigates away mid-reply — same
	// reasoning and same 150s cap as postLiteReadingTurn / coach.go's turnCtx:
	// a synchronous POST is cancelled by net/http the instant the browser
	// disconnects, and a refresh mid-call would otherwise abort the model
	// call, the metering row AND the transaction below — money spent, nothing
	// recorded, and she loses the answer she already paid for.
	turnCtx, cancelTurn := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancelTurn()

	// 1. 装配 — everything the coach needs to see, entirely from the lite
	// writing tables: her own writing row, the outline, the fragments already
	// written, and the windowed transcript.
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
	snippets, err := a.d.Queries.ListWritingSnippets(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	history := buildWritingTurnHistory(msgs, studentText)
	projection := buildWritingCoachProjection(wr, outline, snippets)
	surfaceLabel := writingStageLabel(wr.Stage)

	resolved, rerr := a.routeE(turnCtx, gateway.ClassDialogue)
	if rerr != nil {
		// No call was ever attempted — nothing to meter, same as coach.go's
		// ChatResolver failure branch — but still surfaced as the honest
		// ai_dialogue_failed 502 rather than ErrInternal: from the student's
		// side this is indistinguishable from "the model didn't answer".
		slog.Warn("lite writing turn: resolve model failed", "err", rerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// 2. 复用 AI 大脑，一行没改 — agent.ProposeProjectCoachReply, the same
	// always-reply room-aware producer pro's own coach.go runs.
	out, usage, cerr := agent.ProposeProjectCoachReply(turnCtx, a.d.Provider, resolved, history, projection, surfaceLabel)

	// Meter BEFORE any bail: a call that reached a provider cost money
	// whatever happens to its reply, including an enforcement rejection.
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "writing_turn", resolved, usage)

	// USER RULE: an AI-dialogue failure is surfaced as a real 502, never
	// masked by a canned stand-in sentence. Unlike agent.RouteReading (see
	// postLiteReadingTurn's long comment on this exact point),
	// ProposeProjectCoachReply does NOT swallow its own failures: a provider
	// error, an empty completion, or an enforcement rejection
	// (enforcement.ValidateOutput / BannedPhrasing) all come back as a
	// genuine non-nil `cerr` — there is no raw-decision-vs-gated-decision
	// distinction to worry about here, because this producer proposes no
	// gate of its own. So `cerr != nil` alone is already the honest signal;
	// the empty-body check below is kept as a second line of defence, not
	// because this producer is known to need it.
	reply := strings.TrimSpace(out.Body)
	if cerr != nil || reply == "" {
		slog.Warn("lite writing turn: model turn failed; surfacing to student",
			"err", cerr, "atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	// 3. 落库 — the student turn and the AI turn in ONE transaction, with
	// consecutive seq allocated inside it via NextAtomMessageSeq (never
	// hardcoded), exactly mirroring postLiteReadingTurn and
	// setWritingStage's atomic stage+trace write.
	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	// 串行化这颗原子的 seq 分配。NextAtomMessageSeq 是先读后插，
	// 并发下两笔事务会读到同一个 MAX——唯一索引保住的是数据，代价是
	// 其中一轮直接失败，而那一轮的模型钱已经花掉了。见 queries/atom.sql。
	if _, err := qtx.LockAtom(turnCtx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	next, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: next, Role: "student", Content: studentText,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: next + 1, Role: "ai", Content: reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// Writing mounts no card-summon door in this task (see the file comment):
	// decision/card/nudge/hintCardId hold liteTurnDTO's shape at their zero
	// value until that endpoint exists.
	httpx.WriteJSON(w, http.StatusOK, liteTurnDTO{
		Reply: reply, Decision: "respond", Card: nil, Nudge: "", HintCardID: nil,
	})
}
