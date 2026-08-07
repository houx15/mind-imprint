package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// projectcoach.go — S1 support for the ONE continuous per-project coach
// (coach.go): the surface labels the room `scope` maps to, the fold-on-solidify
// surface sets, and the compact spine projection the coach sees every turn
// (D2 · generous always-on projection, deep-fetch deferred to a later skill).

// coachSurfaceLabels maps a room scope (the wire `scope`, also the stored
// surface tag) to a human room name for the projection ("学生现在在「…」").
var coachSurfaceLabels = map[string]string{
	"forming":         "计划 · 立题",
	"proposal_review": "计划 · 开题检视",
	"find_sources":    "文献库",
	"writing":         "写作",
	"reading":         "阅读",
	"reflection":      "回顾",
}

func coachSurfaceLabel(scope string) string {
	if s, ok := coachSurfaceLabels[scope]; ok {
		return s
	}
	return "项目"
}

// solidifyFoldSurfaces are the surfaces whose shaping turns get folded into the
// spine once 立题/计划 solidifies (proposal first-save, plan generate). The
// plan room reuses the forming/proposal_review scopes, so both events fold the
// same set; FoldCoachSurfaces is idempotent (only non-folded rows), so a second
// event just catches any turns added since the first.
var solidifyFoldSurfaces = []string{"forming", "proposal_review"}

// coachProposalDTO is the /coach response's optional proposal (camelCase,
// matching packages/contracts's CardProposal). Present only on the summon rung.
type coachProposalDTO struct {
	CardID    string `json:"cardId"`
	Reason    string `json:"reason"`
	NudgeText string `json:"nudgeText"`
}

// coachProposeSurfaces are the room scopes where the coach may OFFER an
// argument-moment card. The gate MUST equal the set of surfaces whose CLIENT
// renders the proposal chip — otherwise the server spends a classify call and
// records a coach_proposed event for an offer no student ever sees (whole-branch
// review IMPORTANT 1). Today only WritingBlock renders it, so the gate is
// writing-only; forming/proposal_review/reflection can be added the moment their
// rooms render CoachProposal + wire persist. The classifier's vocabulary
// (fact-opinion-value / concession) is writing-native anyway.
// #18: forming, proposal_review and 文献库(find_sources) join writing — the
// coach may offer a thinking-card in those rooms too. The gate MUST stay equal
// to the set of surfaces whose CLIENT renders the CoachProposal chip (PlanBlock
// forming + ReadingBlock library + WritingBlock), else the server would spend a
// classify call + record a coach_proposed event for an offer no student sees.
// Slice 5 (#21): reflection joins the set — while the student fills her OWN
// reflection, the coach may offer a review/reflection card (ReviewBlock now
// renders the CoachProposal chip + a REFLECTION_DECK shelf, keeping the gate
// equal to the client-renders-chip set).
var coachProposeSurfaces = map[string]bool{
	"writing":         true,
	"forming":         true,
	"proposal_review": true,
	"find_sources":    true,
	"reflection":      true,
}

// coachCardProposal decides whether to OFFER a student card on this coach turn.
// Mirrors semanticCardCandidate's discipline (loop.go): surface gate → in-flight
// guard (never offer over a card already proposed/active) → rune floor + eligible
// set → shared classifier cap → one metered classify call. Returns nil (and, on
// the gated paths, spends nothing) unless a still-eligible moment fires. The
// classify call is metered as Purpose="classify" so it shares the per-project
// MaxClassifyCallsPerProject backstop with the studio loop; the coach-propose
// semantic is recorded separately as a coach_proposed event by the caller.
func (a *API) coachCardProposal(ctx context.Context, projectID uuid.UUID, scope, studentText string, resolved gateway.Resolved) *agent.CardProposal {
	if !coachProposeSurfaces[scope] {
		return nil
	}
	rows, err := a.d.Queries.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		return nil
	}
	views := make([]agent.CardInstanceView, 0, len(rows))
	for _, ci := range rows {
		if ci.Status == "proposed" || ci.Status == "active" {
			return nil // never offer over an in-flight card (no pile-up; no spend)
		}
		views = append(views, agent.CardInstanceView{ID: ci.ID.String(), CardID: ci.CardID, Status: ci.Status})
	}
	eligible := agent.EligibleMoments(views)
	if len(eligible) == 0 {
		return nil
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if n, cerr := store.CountClassifierCalls(ctx, projectID); cerr == nil && n >= agent.MaxClassifyCallsPerProject {
		return nil // shared classifier spend backstop reached
	}
	proposal, usage, perr := agent.ProposeCoachCard(ctx, a.d.Provider, resolved, studentText, eligible)
	// Meter only a completed classify call (perr == nil): ProposeCoachCard zeroes
	// usage on error today, but guard explicitly so a future partial-usage error
	// contract can't record a phantom row (parity with maybeCompactBackstop).
	if perr == nil && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		if rerr := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "classify",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("coach propose: record classify call failed", "err", rerr)
		}
	}
	if perr != nil {
		return nil
	}
	return proposal
}

// formingDimProposal (#13) offers a 克制 confirm chip to record a still-empty
// kick-off dimension the student just articulated — so the 开题四问 panel fills
// as they talk, WITHOUT the AI writing her proposal (she taps to confirm; 打开
// 由学生确认). Forming surfaces only; gated (uncovered dims + rune floor inside
// ProposeFormingDim) + metered like coachCardProposal, sharing its classify cap.
func (a *API) formingDimProposal(ctx context.Context, projectID uuid.UUID, scope, studentText string, resolved gateway.Resolved) *agent.FormingDimSuggestion {
	if scope != "forming" && scope != "proposal_review" {
		return nil
	}
	var uncovered []string
	if prop, err := a.d.Queries.GetProjectProposal(ctx, projectID); err == nil {
		if strings.TrimSpace(prop.Objective) == "" {
			uncovered = append(uncovered, "objective")
		}
		if strings.TrimSpace(prop.Reason) == "" {
			uncovered = append(uncovered, "reason")
		}
		if strings.TrimSpace(prop.Activities) == "" {
			uncovered = append(uncovered, "activities")
		}
		if strings.TrimSpace(prop.Resources) == "" {
			uncovered = append(uncovered, "resources")
		}
		if strings.TrimSpace(prop.Counterpoints) == "" {
			uncovered = append(uncovered, "counterpoints")
		}
	} else {
		uncovered = []string{"objective", "reason", "activities", "resources", "counterpoints"} // no row → all empty
	}
	if len(uncovered) == 0 {
		return nil
	}
	store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
	if n, cerr := store.CountClassifierCalls(ctx, projectID); cerr == nil && n >= agent.MaxClassifyCallsPerProject {
		return nil // shared classifier spend backstop reached
	}
	sug, usage, perr := agent.ProposeFormingDim(ctx, a.d.Provider, resolved, studentText, uncovered)
	if perr == nil && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		if rerr := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "classify",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); rerr != nil {
			slog.Warn("forming dim propose: record classify call failed", "err", rerr)
		}
	}
	if perr != nil {
		return nil
	}
	return sug
}

// digestRuneBudget / digestKeepLastN — S4 compaction backstop thresholds.
// digestKeepLastN is aligned to coachHistoryWindow (the coach's verbatim
// context window): we never fold a turn the coach still shows, and — the audit
// fix — a turn that FALLS OUT of that window is ALWAYS digested, whether it left
// by turn-count or rune budget. Before, the backstop triggered on runes only
// (6000), so many short turns slid out of the last-N window with their gist in
// neither the verbatim window nor the digest (a continuity gap).
const (
	digestRuneBudget = 6000
	digestKeepLastN  = coachHistoryWindow
)

// maybeCompactBackstop is S4 lever-1's size-threshold backstop, run at the tail
// of a coach turn. Best-effort, no return: a failure never disturbs the reply.
// Discipline (克制 / 过程即数据):
//   - under budget → ZERO provider calls, ZERO spend;
//   - meter ONLY a completed compact call (resolved.Provider != "" && cerr == nil);
//   - the digest write PRECEDES the fold — a turn is never folded out of the
//     window before its content is durable in conversation_digest.
func (a *API) maybeCompactBackstop(ctx context.Context, projectID uuid.UUID) {
	pid := pgtype.UUID{Bytes: projectID, Valid: true}
	active, err := a.d.Queries.ListActiveChatMessagesByProject(ctx, pid)
	if err != nil {
		return
	}
	total := 0
	for _, m := range active {
		total += len([]rune(m.Content))
	}
	// Compact when the window overflows by EITHER measure: too many runes (long
	// turns) OR more turns than the coach shows verbatim (short turns that would
	// otherwise slide out of context undigested).
	if total <= digestRuneBudget && len(active) <= digestKeepLastN {
		return // within the verbatim window and under budget → no spend
	}

	overflow, err := a.d.Queries.SelectOldestActiveChatMessages(ctx, sqlc.SelectOldestActiveChatMessagesParams{
		SeededProjectID: pid, Limit: digestKeepLastN,
	})
	if err != nil || len(overflow) == 0 {
		return
	}

	prior, _ := a.d.Queries.GetConversationDigest(ctx, projectID) // zero value if no row yet
	turns := make([]agent.DigestTurn, 0, len(overflow))
	ids := make([]uuid.UUID, 0, len(overflow))
	for _, m := range overflow {
		turns = append(turns, agent.DigestTurn{Role: m.Role, Content: m.Content})
		ids = append(ids, m.ID)
	}

	resolved, rerr := a.d.ChatResolver(ctx)
	if rerr != nil {
		return
	}
	prose, usage, cerr := agent.ComposeDigestMerge(ctx, a.d.Provider, resolved, prior.Prose, turns)

	// Meter ONLY a completed call that actually spent tokens (matches coach.go's
	// sibling guard): a compose error records nothing (S3's phantom-row fix), and
	// an empty-but-successful completion records no 0-token row either.
	if cerr == nil && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
		store := agent.NewSqlcAgentStore(a.d.Queries, a.d.Pool)
		if mrerr := store.RecordLLMCall(ctx, agent.LLMCallRow{
			ProjectID: projectID, Surface: "studio", Purpose: "coach_compact",
			Resolved: resolved, PromptTokens: int32(usage.InputTokens), CompletionTokens: int32(usage.OutputTokens),
		}); mrerr != nil {
			slog.Warn("coach compact: record llm call failed", "err", mrerr)
		}
	}
	if cerr != nil || strings.TrimSpace(prose) == "" {
		return // compose failed / empty → fold NOTHING (digest-before-fold)
	}

	// Digest write PRECEDES fold: only after the overflow content is durable do we
	// remove those turns from the active window.
	model, tier := resolved.Model, resolved.Tier
	if uerr := a.d.Queries.UpsertConversationDigest(ctx, sqlc.UpsertConversationDigestParams{
		ProjectID: projectID, Prose: prose, TurnsFolded: prior.TurnsFolded + int32(len(overflow)),
		Model: &model, Tier: &tier,
	}); uerr != nil {
		slog.Warn("coach compact: upsert digest failed; not folding", "err", uerr)
		return // could not persist digest → do NOT fold
	}
	if ferr := a.d.Queries.FoldChatMessagesByID(ctx, ids); ferr != nil {
		slog.Warn("coach compact: fold failed", "err", ferr)
	}
}

// truncateRunes clamps s to at most n runes, appending … when clipped. Keeps
// the projection compact without splitting a multibyte rune.
func truncateRunes(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// buildSpineProjection assembles the compact, always-on projection of the
// project's spine for the coach's context: kickoff four-questions, plan status,
// reading-list index, outline skeleton, recent activity. Best-effort per slice
// — a missing proposal (fresh project) or a failed sub-read degrades that line
// to a placeholder rather than failing the turn. Deep detail is a later
// on-demand skill; this is the thin projection that rides every turn.
func (a *API) buildSpineProjection(ctx context.Context, projectID uuid.UUID, surface string) (string, error) {
	var b strings.Builder

	// S4 · folded history rides the projection as a compact 会话记忆 block, so the
	// coach never forgets turns the backstop folded out of the active window.
	if d, derr := a.d.Queries.GetConversationDigest(ctx, projectID); derr == nil && strings.TrimSpace(d.Prose) != "" {
		fmt.Fprintf(&b, "会话记忆：%s\n", truncateRunes(d.Prose, 600))
	}

	proj, err := a.d.Queries.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(proj.Title)
	if title == "" {
		title = "（未命名）"
	}
	fmt.Fprintf(&b, "主题：%s（%s）\n", title, proj.Qualification)

	// #4: the essay's target writing language, so the coach knows the final
	// product's language (it may chaperone in Chinese but should remember the
	// deliverable's language and remind the student at the right moment).
	if wlBody, werr := a.d.Queries.GetWritingLanguageNode(ctx, projectID); werr == nil {
		var wl struct {
			Lang string `json:"lang"`
		}
		if json.Unmarshal(wlBody, &wl) == nil {
			if label := writingLangLabel(wl.Lang); label != "" {
				fmt.Fprintf(&b, "写作语言：%s（成品最终用这门语言写）\n", label)
			}
		}
	}

	// 开题四问.
	prop, perr := a.d.Queries.GetProjectProposal(ctx, projectID)
	if perr != nil && !errors.Is(perr, pgx.ErrNoRows) {
		return "", perr
	}
	if errors.Is(perr, pgx.ErrNoRows) || !anyProposalDim(prop) {
		b.WriteString("开题四问：（还没落定）\n")
	} else {
		b.WriteString("开题四问：\n")
		fmt.Fprintf(&b, "- 目标：%s\n", proposalDimOrBlank(prop.Objective))
		fmt.Fprintf(&b, "- 缘由：%s\n", proposalDimOrBlank(prop.Reason))
		fmt.Fprintf(&b, "- 活动：%s\n", proposalDimOrBlank(prop.Activities))
		fmt.Fprintf(&b, "- 资源：%s\n", proposalDimOrBlank(prop.Resources))
		// The 5th section (可能的反例/张力) is optional — only surface it once she's
		// noted something, so an empty one never reads as "still missing" (it does
		// not gate plan generation).
		if strings.TrimSpace(prop.Counterpoints) != "" {
			fmt.Fprintf(&b, "- 可能的反例/张力：%s\n", prop.Counterpoints)
		}
	}
	// EC · on the forming surfaces, steer the coach toward the kick-off
	// dimensions the student hasn't touched yet — so 立题 actually drives
	// coverage instead of only passively seeing （未填）markers. Guides one at a
	// time, never fills the panel (克制 · AI 绝不替学生写开题).
	if surface == "forming" || surface == "proposal_review" {
		b.WriteString(formingCoverageNudge(prop.Objective, prop.Reason, prop.Activities, prop.Resources, prop.Counterpoints))
	}
	// #17 · in 文献库 the coach should proactively help the student generate
	// search keywords and point at databases — but never search for her or hand
	// her conclusions (克制).
	if surface == "find_sources" {
		b.WriteString("（学生在找资料：主动帮他想几个检索关键词，提醒中文和英文期刊都值得查，可以先从中国知网、Google Scholar 入手；读过几篇后再从里面滚出新的关键词和关键学者。给方向和关键词，别替他去搜、别直接下可信与否的结论。）\n")
	}
	// Studio batch-5 followup: 回顾 is where the student fills in HER OWN
	// reflection (AI 使用声明 + 学习报告/元认知/认知者视角 这类回顾卡). This is NOT a
	// defense rehearsal where the coach interrogates her — it's supportive: help
	// her find the words for whichever part she's stuck on (目标有没有达成、方法与
	// 数据用得怎么样、过程中卡在哪、局限在哪、收获与接下来想怎么做不一样），一次只问一
	// 个开放问题，帮她想起具体细节和例子。绝不替她下结论、绝不替她把话写出来——那些话必
	// 须是她自己的（铁律①）。
	if surface == "reflection" {
		b.WriteString("（学生在写回顾：这不是答辩，别追问、别考她——是陪她把自己的思考和感受说清楚。她可能卡在目标有没有达成、方法和数据用得怎么样、过程中遇到的问题、局限在哪，或者收获与接下来想怎么不一样地做；也可能是在写和 AI 互动的使用声明。看她卡在哪一部分，就顺着那部分一次问一个具体的开放问题，帮她想起细节、举个例子、找到自己的措辞；如果她已经写得不错，明确认可她、再问下一处还没写的。绝不替她下结论、绝不替她把话写出来——那些话必须是她自己的。）\n")
	}

	// 计划 status.
	if plan, err := a.d.Queries.ListPlanItems(ctx, projectID); err == nil {
		if len(plan) == 0 {
			b.WriteString("计划：（未生成）\n")
		} else {
			var todo, doing, done int
			for _, p := range plan {
				switch p.Col {
				case "doing":
					doing++
				case "done":
					done++
				default:
					todo++
				}
			}
			fmt.Fprintf(&b, "计划：%d 项（待办 %d · 进行 %d · 完成 %d）\n", len(plan), todo, doing, done)
		}
	}

	// 文献库 index — state-aware (S2): 已归纳 carries the durable
	// proposal_impact takeaway, 在读 shows a gentle in-progress count (never
	// blocks), 未读/pending is marked "尚未读取正文" so the coach never implies it
	// read a source whose body was never fetched (honesty 铁律).
	if refs, err := a.d.Queries.ListReferences(ctx, projectID); err == nil && len(refs) > 0 {
		b.WriteString("文献库：\n")
		// Cards are fetched AT MOST ONCE for the whole loop (lazily, only if a
		// 在读 ref actually needs them) — readingOutcomesByMaterialCtx would
		// otherwise re-run a full ListCardInstancesByProject scan per 在读 ref
		// (up to 6 full scans per projection, built every coach turn).
		var cards []sqlc.CardInstance
		var cardsLoaded bool
		for i, ref := range refs {
			if i >= 6 {
				fmt.Fprintf(&b, "- …另有 %d 条\n", len(refs)-6)
				break
			}
			phase := ""
			if ref.PhaseTag != nil && *ref.PhaseTag != "" {
				phase = "｜" + *ref.PhaseTag
			}
			switch {
			case ref.TakeawayFinalizedAt.Valid && len(ref.Takeaway) > 0:
				// EA: surface the substance the student confirmed — not just the
				// one-line proposal impact — so a later writing-room coach turn
				// actually remembers what she read (findings/credibility were a
				// write-only dead-end before). Kept compact (one line/source).
				var tk agent.ReadingTakeaway
				_ = json.Unmarshal(ref.Takeaway, &tk)
				line := "印记：" + truncateRunes(tk.ProposalImpact, 40)
				if len(tk.Findings) > 0 {
					line += " · 发现：" + truncateRunes(tk.Findings[0], 34)
				}
				if v := strings.TrimSpace(tk.Credibility.Verdict); v != "" {
					line += " · 可信度：" + v
				}
				fmt.Fprintf(&b, "- %s%s｜%s\n", truncateRunes(ref.Title, 32), phase, line)
			case ref.MaterialID.Valid:
				if !cardsLoaded {
					cards, _ = a.d.Queries.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
					cardsLoaded = true
				}
				n := len(readingOutcomesFromCards(cards, uuid.UUID(ref.MaterialID.Bytes)).Findings)
				fmt.Fprintf(&b, "- %s%s｜在读·已确认 %d 条发现\n", truncateRunes(ref.Title, 32), phase, n)
			default:
				// 未读: URL saved (or a lead) but the body was never fetched —
				// material_id is null, so the coach has NO content for this source.
				// It MUST be told so it never implies knowledge of a source it
				// cannot see (honesty 铁律: AI 克制, 绝不替学生定论). A pending lead
				// has no URL yet; both surface as contentless.
				meta := "｜尚未读取正文，我还看不到内容"
				if ref.Pending {
					meta = "｜待补充，还没有内容"
				}
				if ref.Decision != nil && *ref.Decision != "" {
					meta += "｜" + *ref.Decision
				}
				if ref.Credibility != nil && *ref.Credibility != "" {
					meta += "｜可信度 " + *ref.Credibility
				}
				fmt.Fprintf(&b, "- %s%s%s\n", truncateRunes(ref.Title, 40), phase, meta)
			}
		}

		// 探索 branch state (S3, Task 7): open leads still to follow + sources
		// read but not yet connected or pruned — a nudge to keep the coach
		// aware of the rabbit-hole graph without deep-fetching it every turn.
		// Best-effort: a failed ListExplorationLeads just skips this line,
		// matching the block's existing degrade-don't-fail posture.
		if leads, err := a.d.Queries.ListExplorationLeads(ctx, projectID); err == nil {
			var open int
			for _, l := range leads {
				if l.Status == "open" {
					open++
				}
			}
			dangling := len(computeDanglingSourceIds(refs, leads))
			if open > 0 || dangling > 0 {
				fmt.Fprintf(&b, "探索：待追 %d 条线索 · %d 个悬空来源\n", open, dangling)
			}
		}
	}

	// 提纲 skeleton (top-level nodes only).
	if nodes, err := a.d.Queries.ListOutlineNodes(ctx, projectID); err == nil {
		var tops []string
		for _, n := range nodes {
			if n.Depth == 0 {
				tops = append(tops, truncateRunes(n.Text, 30))
			}
		}
		if len(tops) > 0 {
			if len(tops) > 6 {
				tops = tops[:6]
			}
			fmt.Fprintf(&b, "提纲（顶层）：%s\n", strings.Join(tops, " / "))
		}
	}

	// Recent activity (most-recent few).
	if logs, err := a.d.Queries.ListActivityLog(ctx, projectID); err == nil && len(logs) > 0 {
		b.WriteString("最近活动：\n")
		start := 0
		if len(logs) > 3 {
			start = len(logs) - 3
		}
		for _, e := range logs[start:] {
			fmt.Fprintf(&b, "- %s\n", truncateRunes(e.Text, 40))
		}
	}

	return strings.TrimSpace(b.String()), nil
}

// proposalDimOrBlank renders one proposal dimension, or a placeholder when empty.
// (anyProposalDim lives in projects.go — reused here.)
func proposalDimOrBlank(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（未填）"
	}
	return truncateRunes(s, 60)
}

// writingLangLabel maps the stored writing_language code to a human label for
// the coach projection; unknown/empty → "" (the projection omits the line).
func writingLangLabel(code string) string {
	switch strings.TrimSpace(code) {
	case "en":
		return "English"
	case "zh":
		return "中文"
	case "bilingual":
		return "双语"
	default:
		return ""
	}
}

// formingCoverageNudge (EC + Slice 2) steers the forming coach over the kick-off
// dimensions: name the ones the student hasn't touched, one at a time, never
// fill them (克制). Slice 2 adds substance — BEFORE steering to the next
// uncovered dimension the coach gives ONE specific piece of feedback on what the
// student just wrote, judged against that dimension's standard (the old "先认可它"
// was too thin). When all four are covered it emits a FINISH signal (#13) so the
// coach stops re-asking and tells the student the kick-off has taken shape.
func formingCoverageNudge(objective, reason, activities, resources, counterpoints string) string {
	dims := []struct{ name, val string }{
		{"目标", objective}, {"缘由", reason}, {"活动", activities}, {"资源", resources},
	}
	var missing []string
	for _, d := range dims {
		if strings.TrimSpace(d.val) == "" {
			missing = append(missing, d.name)
		}
	}
	if len(missing) == 0 {
		// #13 · finish signal: don't keep interrogating a completed kick-off. The
		// four REQUIRED sections gate plan generation; 反例/张力 is optional, so if
		// she hasn't noted one yet, invite it lightly — never require it.
		if strings.TrimSpace(counterpoints) == "" {
			return "（开题四问都落定了——明确告诉学生开题已经成形，随时可以点『生成项目计划』。可以顺带（一次、不强求）邀请她想想：这个论点最可能撞上的反例或张力是什么？愿意的话记进「可能的反例/张力」。别反复追问同一件事。）\n"
		}
		return "（开题四问都落定了，反例/张力也记了——明确告诉学生开题已经成形，随时可以点『生成项目计划』，不要再反复追问同一件事。）\n"
	}
	var b strings.Builder
	b.WriteString("（开题还没落定：" + strings.Join(missing, "、") +
		"。先针对学生刚说的那一维给一条具体、贴着他内容的反馈——拿它和这一维的标准对一下，指出还差哪一点，再顺势往其中一个还没谈到的维度带一步，一次只带一个。各维的标准：\n")
	b.WriteString("- 目标：一句清晰、完整、可研究的研究问题（要是完整的句子，不是一个话题词）。\n")
	b.WriteString("- 缘由：学生自己的经历，以及这段经历和这个主题的具体联系。\n")
	b.WriteString("- 活动与时间：要覆盖四个阶段——澄清问题 → 收集素材/搭故事线 → 写作 → 回顾。\n")
	b.WriteString("- 资源：每个阶段都有对应的资源支撑（资源要和活动对得上）。\n")
	b.WriteString("若他其实已经在对话里说清了某一维，先认可它、请他确认要不要记进右侧的开题栏，别再重复追问同一维；给反馈和方向，但始终别替他写。")
	// While still shaping 目标/缘由, nudge him to START collecting possible
	// literature/materials in passing — 顺手收集，不替他搜。
	if strings.TrimSpace(objective) == "" || strings.TrimSpace(reason) == "" {
		b.WriteString("在聊目标/缘由时，可以顺带提醒他开始留意、收集一些可能支撑这个想法的文献或素材（顺手收集就好，别替他去搜）。")
	}
	b.WriteString("）\n")
	return b.String()
}
