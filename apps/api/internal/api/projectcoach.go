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
// (fact-opinion-value / certainty-spectrum / steelman) is writing-native anyway.
var coachProposeSurfaces = map[string]bool{
	"writing": true,
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
	}
	// EC · on the forming surfaces, steer the coach toward the kick-off
	// dimensions the student hasn't touched yet — so 立题 actually drives
	// coverage instead of only passively seeing （未填）markers. Guides one at a
	// time, never fills the panel (克制 · AI 绝不替学生写开题).
	if surface == "forming" || surface == "proposal_review" {
		b.WriteString(formingCoverageNudge(prop.Objective, prop.Reason, prop.Activities, prop.Resources))
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
	// blocks), 未读 is unchanged from the pre-S2 line.
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
				meta := ""
				if ref.Decision != nil && *ref.Decision != "" {
					meta = "｜" + *ref.Decision
				}
				if ref.Credibility != nil && *ref.Credibility != "" {
					meta += "｜可信度 " + *ref.Credibility
				}
				fmt.Fprintf(&b, "- %s%s\n", truncateRunes(ref.Title, 40), meta)
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

// formingCoverageNudge (EC) names the kick-off dimensions the student hasn't
// touched, as a one-line steer for the forming coach: guide toward them, one at
// a time, never fill them (克制). Empty when all four are covered.
func formingCoverageNudge(objective, reason, activities, resources string) string {
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
		return ""
	}
	return "（开题还没触及：" + strings.Join(missing, "、") + "——顺着学生的话，把话题往其中一个维度带一步，一次只带一个，别替他写。）\n"
}
