package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
)

// demo.go — guided-tour P2, Task 2. A shared demo project (project.is_demo)
// must NEVER call the model: every token-consuming endpoint short-circuits to a
// canned, on-topic-neutral fixture here instead of resolving a provider or
// writing to the DB. Task 1 built the read-only seam — loadOwnedProjectRow is
// world-readable and applies NO write-guard 403, so a demo POST reaches these
// fixtures rather than being pre-empted by loadOwnedProject's demo_readonly.
// The fixtures return the SAME response shape each real handler emits, but with
// fixed placeholder content that is clearly a walkthrough (never a real model
// output, never student-specific).

// Demo copy — fixed, restrained, clearly a read-only walkthrough placeholder.
const (
	demoCoachReply        = "这是一个只读的演示项目——你看到的是别人走过的完整流程。想亲自试试的话，回到「项目」新建一个自己的项目吧。"
	demoStudioTurnReply   = "这是演示项目的只读回放。要和印记真正对话，请新建你自己的项目。"
	demoChatReply         = "这是演示对话的只读回放。想和印记真正聊聊，请新建你自己的项目再来。"
	demoExplorationReview = "这是演示项目里的探索地图回放——真实项目里，印记会在这里点出哪些来源最相关、哪些偏题、还差哪一块。"
)

// isDemoProject reports whether the project id is flagged is_demo — for a
// handler that holds only the id, not the row loadOwnedProjectRow returns.
func (a *API) isDemoProject(ctx context.Context, id uuid.UUID) (bool, error) {
	p, err := a.d.Queries.GetProject(ctx, id)
	if err != nil {
		return false, err
	}
	return p.IsDemo, nil
}

// cannedCoachReply is the demo fixture for every /coach* endpoint — they all
// emit orchestratorReplyDTO. Fixed narrate + a default directive; no tool
// proposals, no next-step: a read-only demo never advances anything.
func cannedCoachReply() orchestratorReplyDTO {
	return orchestratorReplyDTO{Narrate: demoCoachReply, Directive: agent.DefaultStudioState()}
}

// cannedExplorationGuide / cannedDig / cannedQuestionEdges / cannedSearchGuidance /
// cannedPlacement / cannedExplorationReview / cannedAnnotationReview each return
// the SAME shape their real handler emits, empty/neutral — a demo surfaces no
// live results (no OpenAlex, no LLM, no persistence).
func cannedExplorationGuide() map[string]any {
	return map[string]any{"directions": toGuideDirectionDTOs(nil)}
}

func cannedDig() map[string]any {
	return map[string]any{"candidates": toDigCandidateDTOs(nil)}
}

func cannedQuestionEdges() map[string]any {
	return map[string]any{"edges": []questionEdgeDTO{}}
}

func cannedExplorationReview() map[string]any {
	return map[string]any{"review": demoExplorationReview}
}

func cannedSearchGuidance() map[string]any {
	return map[string]any{"suggestions": []map[string]string{
		{"keyword": "示例检索方向（演示）", "why": "演示项目不联网检索——真实项目里这里会是印记给你的检索建议。"},
	}}
}

func cannedPlacement() map[string]any {
	// leadId serialises as null (未归类) — the advisory neutral default.
	return map[string]any{"leadId": nil, "reason": ""}
}

func cannedAnnotationReview() map[string]any {
	return map[string]any{"annotations": []draftAnnotationDTO{}}
}

// streamDemoTurn emits a minimal canned SSE reply (one assistant text delta +
// done) for a demo project's STREAMING turn endpoints (postProjectTurn,
// postChatTurn) — no provider, no persistence, so the frontend still sees a
// well-formed stream for a read-only demo. On an SSE-setup failure it falls
// back to a JSON error, matching the real handlers' pre-stream error posture.
func (a *API) streamDemoTurn(w http.ResponseWriter, r *http.Request, reply string) {
	sse, err := gateway.NewSSEWriter(w)
	if err != nil {
		httpx.WriteError(w, r, httpx.ErrInternal())
		return
	}
	em := &studioEmitter{sse: sse}
	_ = em.Text(reply)
	_ = em.Done()
}
