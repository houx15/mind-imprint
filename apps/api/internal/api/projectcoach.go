package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mindimprint/api/internal/agent"
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
func (a *API) buildSpineProjection(ctx context.Context, projectID uuid.UUID) (string, error) {
	var b strings.Builder

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
				var tk agent.ReadingTakeaway
				_ = json.Unmarshal(ref.Takeaway, &tk)
				fmt.Fprintf(&b, "- %s%s｜印记：%s\n", truncateRunes(ref.Title, 32), phase, truncateRunes(tk.ProposalImpact, 40))
			case ref.MaterialID.Valid:
				n := len(a.readingOutcomesByMaterialCtx(ctx, projectID, uuid.UUID(ref.MaterialID.Bytes)).Findings)
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
