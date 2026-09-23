package agent

import (
	"context"
	"fmt"
	"strings"

	"mindimprint/api/internal/gateway"
)

// exploration_review.go — §5 (user follow-up) · once the student has collected +
// tagged several papers, they can ask 印记 to review the WHOLE exploration (across
// all sub-questions) and suggest which materials are most correlated / important,
// which look weakly related (trim candidates), and what's still missing. Flagship
// reviewer (reasoning on — it's a judgment call). Advisory only (铁律②): a prose
// read, never an auto-archive.

// ExplorationReviewPaper is one collected paper's facets, for the review.
type ExplorationReviewPaper struct {
	Title    string
	Nature   string // support | challenge | ""
	Argument string
	Finding  string
}

// ExplorationReviewSubQuestion is one sub-question + its gathered papers.
type ExplorationReviewSubQuestion struct {
	Text   string
	Papers []ExplorationReviewPaper
}

// ExplorationReviewInput is the whole evidence map for the review.
type ExplorationReviewInput struct {
	Question     string
	SubQuestions []ExplorationReviewSubQuestion
	// Unfiled · sources the student has collected that hang under NO question
	// yet (the 未归类 bucket). They used to be invisible here — the projection
	// only walked 子问题-edge targets — so "帮我理一理文献" silently skipped the
	// very papers the student was asking for help organizing (bug report
	// 2026-08-28 §2). Advisory only: 印记 says where each one seems to belong,
	// the student taps to place it (铁律②).
	Unfiled []ExplorationReviewPaper
}

const explorationReviewSystem = `你是一位严谨而务实的 IB 研究导师。输入包含学生已搜集的文献及现有标注，部分材料可能尚未归类或标注完整。请根据提供的标题和标注整理材料关系，不假装读过未提供的全文。回复直接展示给学生，用“你”称呼学生。

给出一段简洁、可操作的评述，覆盖：
1. 哪些材料最贴近核心问题、最重要（点名，说明为什么）；
2. 哪些材料关联较弱、或与已有的重复，可以考虑归档/降低优先级（点名）；
3. 现有材料如何帮助回答各个问题，是否还有需要了解的反例或适用条件；不以正反材料数量相等作为要求；
4. 还缺什么方向的材料（给 1-2 个具体建议）。

如果给了「未归类的材料」：先处理它们——逐篇点名，说明它可能对应哪个问题及依据；若与现有问题均无明显关联，说明原因，并依据现有标注建议它可能对应的问题。

如果上面一个问题都还没有：依据这批材料，直接提议 2-3 个值得立的研究问题（每个一句话），并说明哪几篇归到哪一个下面。

要求：具体说明所指的篇目；用中文；像老师给学生的口头点评，可用简短段落或编号列表组织意见；只评述、只建议，不替学生删改、不替学生归位，归类与归档由学生确认。若材料太少（<2 篇），直接说"目前材料不足以比较研究方向，建议先补充与研究问题相关的材料"。`

// reviewPaperLine renders one paper the same way in every bucket (a
// sub-question's, or 未归类's) so the model reads a uniform list.
func reviewPaperLine(p ExplorationReviewPaper) string {
	nature := "未标注"
	switch p.Nature {
	case "support":
		nature = "支持"
	case "challenge":
		nature = "反驳/张力"
	}
	return fmt.Sprintf("《%s》[%s] 论点：%s 证据：%s", strings.TrimSpace(p.Title), nature,
		strings.TrimSpace(p.Argument), strings.TrimSpace(p.Finding))
}

const maxExplorationReviewAttempts = 2

// ReviewExploration runs the flagship holistic review. Best-effort: any error →
// the caller degrades (empty review). Usage returned so the caller meters.
func ReviewExploration(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in ExplorationReviewInput) (string, gateway.ChatUsage, error) {
	var b strings.Builder
	if q := strings.TrimSpace(in.Question); q != "" {
		fmt.Fprintf(&b, "核心研究问题：%s\n\n", q)
	} else {
		b.WriteString("核心研究问题：（学生还没立下问题）\n\n")
	}
	total := 0
	for i, sq := range in.SubQuestions {
		fmt.Fprintf(&b, "子问题 %d：%s\n", i+1, strings.TrimSpace(sq.Text))
		if len(sq.Papers) == 0 {
			b.WriteString("  （还没有材料）\n")
		}
		for _, p := range sq.Papers {
			b.WriteString("  - " + reviewPaperLine(p) + "\n")
			total++
		}
	}
	if len(in.SubQuestions) == 0 {
		b.WriteString("（还没有任何问题节点——所有材料都还没归位）\n")
	}
	if len(in.Unfiled) > 0 {
		b.WriteString("\n未归类的材料（还没挂到任何问题下）：\n")
		for _, p := range in.Unfiled {
			b.WriteString("  - " + reviewPaperLine(p) + "\n")
			total++
		}
	}
	fmt.Fprintf(&b, "\n（共 %d 篇材料）", total)

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: explorationReviewSystem},
			{Role: gateway.RoleUser, Content: b.String()},
		},
		MaxTokens: 3000,
	}

	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxExplorationReviewAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		if text := strings.TrimSpace(stripFences(res.Text)); text != "" {
			return text, lastUsage, nil
		}
		lastErr = fmt.Errorf("exploration review: empty reply")
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("exploration review: no reply")
	}
	return "", lastUsage, lastErr
}
