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

// cardEligibleForSummon is the in-flight guard for the orchestrator's summon_card
// tool: the model already chose the card, so we only decide whether re-offering
// it is allowed. It mirrors the retired EligibleMoments rule EXACTLY (moment.go:
// "ineligible as soon as its target card has a card_instance in ANY status —
// including skipped"): a card_id with ANY existing instance is ineligible. That
// covers proposed/active (no pile-up over a live offer), skipped (once she has
// dismissed it we do not ask again — 铁律 2 · 不操纵), and completed (already done).
// A list error fails closed (no offer). Card CHOICE is the orchestrator's own
// posture, not a classifier.
func (a *API) cardEligibleForSummon(ctx context.Context, projectID uuid.UUID, cardID string) bool {
	rows, err := a.d.Queries.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
	if err != nil {
		return false
	}
	for _, ci := range rows {
		if ci.CardID == cardID {
			return false // any existing instance (proposed/active/skipped/completed) ⇒ do not re-offer
		}
	}
	return true
}

// filterKnownReferences (P3) drops curate_reference items whose id does not
// match a real reference (material) or snippet id for this project.
// filterCurateReferenceCall (orchestrator.go) already validates `kind` against
// the closed enum, but the model can still HALLUCINATE an id that was never in
// the projection — a made-up id would reach studio_state.reference, where a
// later task's frontend resolves ids into rich content, so it must be real.
// Best-effort: a query failure degrades to an empty allowed set (drop every
// item) rather than letting an unchecked id through. `kind` and `label` are
// left as-is for surviving items — this only filters on `id`.
func (a *API) filterKnownReferences(ctx context.Context, projectID uuid.UUID, items []agent.ReferenceRef) []agent.ReferenceRef {
	allowed := make(map[string]bool)
	if refs, err := a.d.Queries.ListReferences(ctx, projectID); err == nil {
		for _, ref := range refs {
			allowed[ref.ID.String()] = true
		}
	}
	if snippets, err := a.d.Queries.ListSnippets(ctx, projectID); err == nil {
		for _, s := range snippets {
			allowed[s.ID.String()] = true
		}
	}
	// Task 2 (annotation entity): review_item interventions (整稿体检 results)
	// are the third curate_reference kind ("annotation") — same allow-by-real-id
	// posture, degrade-to-skip on a query error like the two calls above.
	if reviewItems, err := a.d.Queries.ListReviewItemsByProject(ctx, projectID); err == nil {
		for _, row := range reviewItems {
			allowed[row.ID.String()] = true
		}
	}
	kept := make([]agent.ReferenceRef, 0, len(items))
	for _, it := range items {
		if allowed[it.ID] {
			kept = append(kept, it)
		}
	}
	return kept
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
// of a coach turn. Best-effort: a failure never disturbs the reply. Returns
// true ONLY when it actually composed AND persisted a digest this call (so a
// caller building an OrchestratorReply can surface reply.Compacted) — false
// on every early return, including a fold-under-budget no-op or any failure.
// Discipline (克制 / 过程即数据):
//   - under budget → ZERO provider calls, ZERO spend;
//   - meter ONLY a completed compact call (resolved.Provider != "" && cerr == nil);
//   - the digest write PRECEDES the fold — a turn is never folded out of the
//     window before its content is durable in conversation_digest.
func (a *API) maybeCompactBackstop(ctx context.Context, projectID uuid.UUID) bool {
	pid := pgtype.UUID{Bytes: projectID, Valid: true}
	active, err := a.d.Queries.ListActiveChatMessagesByProject(ctx, pid)
	if err != nil {
		return false
	}
	total := 0
	for _, m := range active {
		total += len([]rune(m.Content))
	}
	// Compact when the window overflows by EITHER measure: too many runes (long
	// turns) OR more turns than the coach shows verbatim (short turns that would
	// otherwise slide out of context undigested).
	if total <= digestRuneBudget && len(active) <= digestKeepLastN {
		return false // within the verbatim window and under budget → no spend
	}

	overflow, err := a.d.Queries.SelectOldestActiveChatMessages(ctx, sqlc.SelectOldestActiveChatMessagesParams{
		SeededProjectID: pid, Limit: digestKeepLastN,
	})
	if err != nil || len(overflow) == 0 {
		return false
	}

	prior, _ := a.d.Queries.GetConversationDigest(ctx, projectID) // zero value if no row yet
	turns := make([]agent.DigestTurn, 0, len(overflow))
	ids := make([]uuid.UUID, 0, len(overflow))
	for _, m := range overflow {
		turns = append(turns, agent.DigestTurn{Role: m.Role, Content: m.Content})
		ids = append(ids, m.ID)
	}

	resolved, rerr := a.routeE(ctx, gateway.ClassDigest)
	if rerr != nil {
		return false
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
		return false // compose failed / empty → fold NOTHING (digest-before-fold)
	}

	// Digest write PRECEDES fold: only after the overflow content is durable do we
	// remove those turns from the active window.
	model, tier := resolved.Model, resolved.Tier
	if uerr := a.d.Queries.UpsertConversationDigest(ctx, sqlc.UpsertConversationDigestParams{
		ProjectID: projectID, Prose: prose, TurnsFolded: prior.TurnsFolded + int32(len(overflow)),
		Model: &model, Tier: &tier,
	}); uerr != nil {
		slog.Warn("coach compact: upsert digest failed; not folding", "err", uerr)
		return false // could not persist digest → do NOT fold
	}
	if ferr := a.d.Queries.FoldChatMessagesByID(ctx, ids); ferr != nil {
		slog.Warn("coach compact: fold failed", "err", ferr)
	}
	return true
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
	// Prefer the FULL assignment brief over project.Title, which is only a 60-char
	// truncation (titleFromPrompt) — the coach otherwise sees a title cut mid-word
	// and treats it as the student's unfinished title (it once opened by asking
	// her to complete "…for Western"). Capped so a long multi-part prompt can't
	// bloat the projection. Falls back to the title when the brief is absent.
	if raw, berr := a.d.Queries.GetAssignmentBriefNode(ctx, projectID); berr == nil {
		var brief struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &brief) == nil && strings.TrimSpace(brief.Text) != "" {
			title = truncateRunes(strings.TrimSpace(brief.Text), 400)
		}
	}
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
		b.WriteString("（学生在找资料：根据研究问题建议几个检索关键词，提醒中文和英文期刊都值得查，可以先从中国知网、Google Scholar 入手；读过几篇后再根据阅读内容补充关键词和相关学者。给方向和关键词，由学生自行检索、别直接下可信与否的结论。）\n")
	}
	// Studio batch-5 followup: 回顾 is where the student fills in HER OWN
	// reflection (AI 使用声明 + 学习报告/元认知/认知者视角 这类回顾卡). This is NOT a
	// defense rehearsal where the coach interrogates her — it's supportive: help
	// her find the words for whichever part she's stuck on (目标有没有达成、方法与
	// 数据用得怎么样、过程中卡在哪、局限在哪、收获与接下来想怎么做不一样），一次只问一
	// 个开放问题，帮她想起具体细节和例子。绝不替她下结论、绝不替她把话写出来——那些话必
	// 须是她自己的（铁律①）。
	if surface == "reflection" {
		b.WriteString("（学生正在撰写回顾，请帮助学生梳理真实经历与自己的理解。根据当前内容，选择目标完成情况、方法与数据、遇到的问题、研究局限、后续改进或 AI 使用情况中的一个相关方面，提出一个具体的开放问题。已说明的内容直接确认，不重复询问。回顾结论及正文由学生自己表达。）\n")
	}

	// 计划 status. Once a plan EXISTS the coach must (a) STOP offering to generate
	// one — the stage code `plan_generation` reads to the model as "generate now",
	// so a bare count line let it narrate "要我帮你生成计划吗？" over an existing
	// 8-item plan — and (b) actively lead the student to the next plan step (印记
	// 是 agent，房间是它的工具). We surface the concrete 下一步 (the first not-done
	// task) + the room it lives in, and instruct the coach to propose it and wait
	// for confirmation before opening that room (铁律② 打开由学生确认 · one-tap).
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
			fmt.Fprintf(&b, "计划：已生成 %d 项（待办 %d · 进行 %d · 完成 %d）——不要再提议「生成计划」，计划已经有了。\n", len(plan), todo, doing, done)
			// List the items with their real [id] so update_plan can target one
			// (mark done / edit / remove). Ids belong in tool args only, never in
			// narrate. Compact — title + stage + current column.
			b.WriteString("计划项（[id] 传给 update_plan 标记完成/修改，别在给学生的话里出现 id）：\n")
			for _, p := range plan {
				col := "待办"
				switch p.Col {
				case "doing":
					col = "进行中"
				case "done":
					col = "已完成"
				}
				fmt.Fprintf(&b, "- [%s] %s · %s（%s）\n", p.ID.String(), strings.TrimSpace(p.Stage), strings.TrimSpace(p.Title), col)
			}
			if step, ok := nextPlanStep(plan); ok {
				fmt.Fprintf(&b, "按计划下一步是：%s（属于「%s」，在「%s」房间做）。先在 narrate 里说明下一步任务，询问学生是否现在开始，得到肯定后再 open_tool 打开「%s」并用 set_status 推进阶段；学生若有其他安排，尊重这一选择。任务由学生完成，每次只说明一个步骤。\n",
					step.title, step.stageLabel, step.roomLabel, step.tool)
			}
		}
	}

	// 文献库 index — state-aware (S2): 已归纳 carries the durable
	// proposal_impact takeaway, 在读 shows a gentle in-progress count (never
	// blocks), 未读/pending is marked "尚未读取正文" so the coach never implies it
	// read a source whose body was never fetched (honesty 铁律).
	if refs, err := a.d.Queries.ListReferences(ctx, projectID); err == nil && len(refs) > 0 {
		// P3: each line carries the reference's REAL id in `[id]` — the orchestrator
		// may pass it back verbatim to curate_reference (kind="material") to put
		// that source in the left panel. Ids not present in this projection are
		// hallucinated and get dropped server-side (coach.go's curate_reference
		// apply case validates against the actual DB rows), so the model must
		// copy one it sees here rather than invent one.
		b.WriteString("文献库（[id] 可原样传给 curate_reference 把来源摆进侧栏，别编造不存在的 id）：\n")
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
				fmt.Fprintf(&b, "- [%s] %s%s｜%s\n", ref.ID, truncateRunes(ref.Title, 32), phase, line)
			case ref.MaterialID.Valid:
				if !cardsLoaded {
					cards, _ = a.d.Queries.ListCardInstancesByProject(ctx, pgtype.UUID{Bytes: projectID, Valid: true})
					cardsLoaded = true
				}
				n := len(readingOutcomesFromCards(cards, uuid.UUID(ref.MaterialID.Bytes)).Findings)
				fmt.Fprintf(&b, "- [%s] %s%s｜在读·已确认 %d 条发现\n", ref.ID, truncateRunes(ref.Title, 32), phase, n)
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
				fmt.Fprintf(&b, "- [%s] %s%s%s\n", ref.ID, truncateRunes(ref.Title, 40), phase, meta)
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
			// 问题节点 + 未归类 · a student asking "帮我把文献理一理" is asking about
			// exactly this pairing, and the projection never carried either
			// side (bug report 2026-08-28 §2): the coach saw a flat 文献库 with
			// no idea which sources hang under no question, and no idea what
			// the questions even were. Both are cheap (already-loaded rows)
			// and capped. Advisory: the coach SAYS where a source belongs, the
			// student taps 归位 in 未归类 to place it (铁律②).
			questions, unfiled := explorationQuestionsAndUnfiled(refs, leads)
			if len(questions) > 0 {
				b.WriteString("兔子洞地图上的问题：" + strings.Join(cappedTexts(questions, 6), " / ") + "\n")
			}
			if len(unfiled) > 0 {
				fmt.Fprintf(&b, "未归类的来源（%d 篇，还没挂到任何问题下）：%s\n", len(unfiled), strings.Join(cappedTexts(unfiled, 6), " / "))
				if len(questions) == 0 {
					b.WriteString("（学生已有材料，尚未确定研究问题。可以根据材料构思 2–3 个研究方向，用 propose_question 每次提出一个候选问题，学生确认后加入地图。）\n")
				} else {
					b.WriteString("（学生要求整理文献时，逐篇说明建议关联的研究问题及理由，再请学生在「未归类」中确认。只有学生完成确认后，文献归类才生效。）\n")
				}
			}
		}
	}

	// 片段 (P3): saved excerpts weren't in the projection before — 印记 had no
	// real id to cite for curate_reference kind="note". Same [id]-tag posture as
	// 文献库 above: token-lean, capped, ids are the real snippet ids so a curated
	// item survives coach.go's id-validation.
	if snippets, err := a.d.Queries.ListSnippets(ctx, projectID); err == nil && len(snippets) > 0 {
		b.WriteString("片段（[id] 同样可传给 curate_reference，kind=\"note\"）：\n")
		const snippetCap = 6
		for i, s := range snippets {
			if i >= snippetCap {
				fmt.Fprintf(&b, "- …另有 %d 条\n", len(snippets)-snippetCap)
				break
			}
			section := ""
			if s.Section != nil && strings.TrimSpace(*s.Section) != "" {
				section = "｜" + *s.Section
			}
			fmt.Fprintf(&b, "- [%s] %s%s\n", s.ID, truncateRunes(s.Text, 30), section)
		}
	}

	// 批注 (Task 2, annotation entity): persisted review_item interventions —
	// the results of an 整稿体检 (whole-draft review) — become addressable once
	// a student has actually ordered one. Same [id]-tag posture as 文献库/片段
	// above: ids are real intervention ids so a curated kind="annotation" item
	// survives filterKnownReferences' id-validation. Only shown when review
	// items exist, so the block never implies a 体检 that hasn't happened.
	if reviewItems, err := a.d.Queries.ListReviewItemsByProject(ctx, projectID); err == nil && len(reviewItems) > 0 {
		var lines []string
		const annotationCap = 6
		for i, row := range reviewItems {
			if i >= annotationCap {
				lines = append(lines, fmt.Sprintf("- …另有 %d 条", len(reviewItems)-annotationCap))
				break
			}
			var item agent.ReviewItem
			if err := json.Unmarshal([]byte(row.Body), &item); err != nil {
				continue
			}
			detail := strings.TrimSpace(item.Fix)
			if detail == "" {
				detail = strings.TrimSpace(item.Missing)
			}
			if detail == "" {
				continue // nothing actionable to cite
			}
			lines = append(lines, fmt.Sprintf("- [%s] %s·%s｜%s", row.ID, item.CriterionName, item.Band, truncateRunes(detail, 40)))
		}
		if len(lines) > 0 {
			b.WriteString("批注（[id] 可传给 curate_reference，kind=\"annotation\"；只有体检过才有）：\n")
			b.WriteString(strings.Join(lines, "\n"))
			b.WriteString("\n")
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

// planStep is the projection's view of the next actionable plan task: the task
// title, its stage label, and the room (open_tool code + human label) the coach
// should offer to open for it.
type planStep struct {
	title      string
	stageLabel string
	tool       string // OpenTool code: reading | writing | reflection
	roomLabel  string
}

// planRoomForTag maps a plan item's tag to the room its work happens in, plus a
// human label. read → 阅读室; write → 写作台; review (整稿体检/复盘) → 写作台.
func planRoomForTag(tag string) (tool, label string) {
	switch tag {
	case "read":
		return "reading", "阅读"
	default: // write, review, and anything else → the writing surface
		return "writing", "写作"
	}
}

// nextPlanStep returns the first not-done plan item (lowest Position, ties by
// creation order as ListPlanItems already returns) so the coach can lead the
// student to it. ok=false when every task is done (nothing left to steer to).
func nextPlanStep(items []sqlc.PlanItem) (planStep, bool) {
	var best *sqlc.PlanItem
	for i := range items {
		if items[i].Col == "done" {
			continue
		}
		if best == nil || items[i].Position < best.Position {
			best = &items[i]
		}
	}
	if best == nil {
		return planStep{}, false
	}
	tool, roomLabel := planRoomForTag(best.Tag)
	stageLabel := strings.TrimSpace(best.Stage)
	if stageLabel == "" {
		stageLabel = "计划"
	}
	return planStep{
		title:      truncateRunes(best.Title, 40),
		stageLabel: stageLabel,
		tool:       tool,
		roomLabel:  roomLabel,
	}, true
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
			return "（开题四问都落定了——明确告诉学生开题已经成形，随时可以点『生成项目计划』。可以顺带（一次、不强求）邀请学生思考：是否有需要考虑的反例或不同解释？愿意的话记进「可能的反例/张力」。别反复追问同一件事。）\n"
		}
		return "（开题四问都落定了，反例/张力也记了——明确告诉学生开题已经成形，随时可以点『生成项目计划』，不要再反复追问同一件事。）\n"
	}
	var b strings.Builder
	b.WriteString("（开题还没落定：" + strings.Join(missing, "、") +
		"。先依据相应标准，回应学生刚谈到的内容；需要补充时说明具体缺少什么。当前内容已充分时，再讨论一个尚未涉及的维度。各维的标准：\n")
	b.WriteString("- 目标：一句清晰、完整、可研究的研究问题（要是完整的句子，不是一个话题词）。\n")
	b.WriteString("- 缘由：学生自己的经历，以及这段经历和这个主题的具体联系。\n")
	b.WriteString("- 活动与时间：要覆盖四个阶段——澄清问题 → 收集素材/搭故事线 → 写作 → 回顾。\n")
	b.WriteString("- 资源：每个阶段都有对应的资源支撑（资源要和活动对得上）。\n")
	b.WriteString("若学生已在对话里说清某一维，请确认是否记录到右侧开题栏，不重复追问。提供反馈和方向，开题内容由学生撰写。")
	// While still shaping 目标/缘由, nudge him to START collecting possible
	// literature/materials in passing — 顺手收集，不替他搜。
	if strings.TrimSpace(objective) == "" || strings.TrimSpace(reason) == "" {
		b.WriteString("在聊目标/缘由时，可以建议学生留意、收集与研究问题相关的文献或素材，由学生自行检索。")
	}
	b.WriteString("）\n")
	return b.String()
}

// explorationQuestionsAndUnfiled splits the exploration graph into the two
// lists the coach needs to help a student organize: the live QUESTION nodes'
// texts, and the titles of references that hang under none of them (未归类).
// Pure — mirrors the client's own unfiledReferences so both ends agree on what
// "未归类" means.
func explorationQuestionsAndUnfiled(refs []sqlc.Reference, leads []sqlc.ExplorationLead) (questions []string, unfiled []string) {
	attached := map[string]bool{}
	for _, l := range leads {
		if l.Status == "pruned" {
			continue
		}
		if l.ConnectedReferenceID.Valid {
			attached[uuid.UUID(l.ConnectedReferenceID.Bytes).String()] = true
			continue
		}
		if t := strings.TrimSpace(l.Text); t != "" {
			questions = append(questions, t)
		}
	}
	for _, r := range refs {
		if r.Archived || attached[r.ID.String()] {
			continue
		}
		unfiled = append(unfiled, strings.TrimSpace(r.Title))
	}
	return questions, unfiled
}

// cappedTexts truncates each entry and caps the list, appending a "…另有 N 条"
// tail so the coach knows the list is partial without paying for the rest.
func cappedTexts(items []string, limit int) []string {
	out := make([]string, 0, limit+1)
	for i, s := range items {
		if i >= limit {
			out = append(out, fmt.Sprintf("…另有 %d 条", len(items)-limit))
			break
		}
		out = append(out, truncateRunes(s, 30))
	}
	return out
}
