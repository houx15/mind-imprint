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

// cannedDigCandidates — three real, on-topic sources for the demo research
// question ("中国是否让地球变得更可持续"), so the read-only 探索 tray has
// something worth practising 采纳/丢弃 on. Real, checkable sources (NASA-fed
// Nature Sustainability study, IEA renewables report, Global Carbon Project
// budget) — never a live OpenAlex call, never model output.
func cannedDigCandidates() []digCandidateDTO {
	return []digCandidateDTO{
		{
			DOI:      "10.1038/s41893-019-0220-7",
			Title:    "China and India Lead in Greening of the World Through Land-Use Management",
			Authors:  "Chen, C., Park, T., Wang, X. et al.",
			Year:     "2019",
			Journal:  "Nature Sustainability",
			Abstract: "基于 NASA MODIS 卫星数据的研究发现，2000–2017 年间地球新增绿化面积中，中国和印度贡献最大；中国的贡献主要来自植树造林工程，其次是集约农业。",
			URL:      "https://doi.org/10.1038/s41893-019-0220-7",
		},
		{
			DOI:      "",
			Title:    "Renewables 2023: Analysis and Forecast to 2028",
			Authors:  "International Energy Agency (IEA)",
			Year:     "2023",
			Journal:  "IEA Renewables Market Report",
			Abstract: "IEA 报告指出，中国 2023 年新增可再生能源装机容量占全球增量的一半以上，光伏和风电新增规模均为世界第一，是全球可再生能源增长最主要的驱动力。",
			URL:      "https://www.iea.org/reports/renewables-2023",
		},
		{
			DOI:      "10.5194/essd-15-5301-2023",
			Title:    "Global Carbon Budget 2023",
			Authors:  "Friedlingstein, P., O'Sullivan, M., Jones, M. W. et al.",
			Year:     "2023",
			Journal:  "Earth System Science Data",
			Abstract: "全球碳计划年度报告显示，中国仍是全球最大的化石燃料二氧化碳排放国，2022 年排放量约占全球总量的三成——是评估「中国是否让地球更可持续」时必须正视的反例数据。",
			URL:      "https://doi.org/10.5194/essd-15-5301-2023",
		},
	}
}

func cannedDig() map[string]any {
	return map[string]any{"candidates": cannedDigCandidates()}
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
