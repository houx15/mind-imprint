package agent

import (
	"context"
	"encoding/json"
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

type SelectionCheck struct {
	Key, Label, Status, Evidence, Explanation string
}

type SelectionEval struct {
	Verdict, VerdictLabel, VerdictReason         string
	Checks                                       []SelectionCheck
	Finding, Judgment, Support, Caveat, NextStep string
	SpanIDs                                      []string
	// Degraded marks a review NO MODEL PRODUCED — fallbackEval's canned,
	// deliberately generic text, returned when the resolver failed, the
	// provider failed, or the reply did not parse (the known MaxTokens
	// truncation makes that a common path, not a rare one).
	//
	// It matters because this struct is PERSISTED (framework_fill) and read
	// back as the student's `finding`. Unmarked, a report generated from the
	// process record would count "你选了这句作为证据。" as her own reading of
	// the sentence — a canned sentence laundered into evidence. 铁律④ makes
	// the record evidence; evidence has to say when it is not real.
	Degraded bool
}

var checkLabels = map[string]string{"target": "找对对象", "evidence": "看得到线索", "centrality": "线索足够关键"}

var verdictLabels = map[string]string{"strong": "高度匹配", "partial": "部分匹配", "rethink": "暂不匹配"}

// verdictFromChecks derives the grade from the three checks — PROGRAM-owned, the
// model's own "verdict" field is ignored. target miss → rethink; all pass →
// strong; else partial (spec §11).
func verdictFromChecks(checks []SelectionCheck) (string, string) {
	byKey := map[string]string{}
	for _, c := range checks {
		byKey[c.Key] = c.Status
	}
	if byKey["target"] == "miss" {
		return "rethink", verdictLabels["rethink"]
	}
	if byKey["target"] == "pass" && byKey["evidence"] == "pass" && byKey["centrality"] == "pass" {
		return "strong", verdictLabels["strong"]
	}
	return "partial", verdictLabels["partial"]
}

type evalReply struct {
	Checks []struct {
		Key         string `json:"key"`
		Status      string `json:"status"`
		Evidence    string `json:"evidence"`
		Explanation string `json:"explanation"`
	} `json:"checks"`
	Finding  string `json:"finding"`
	Judgment string `json:"judgment"`
	Support  string `json:"support"`
	Caveat   string `json:"caveat"`
	NextStep string `json:"next_step"`
}

// normalizeStatus clamps the model's status onto the closed set.
func normalizeStatus(s string) string {
	switch s {
	case "pass", "partial", "miss":
		return s
	default:
		return "partial"
	}
}

// EvaluateSelection judges the student's picked sentence against the active
// card's lens using the uniform 3-check rubric. The verdict is recomputed from
// the checks (never trusted from the model); evidence snippets that are not
// verbatim from the student's span are dropped; the finding may cite ONLY the
// student's span. On any model/parse failure it degrades to a deterministic
// evaluator so the loop stays alive.
func EvaluateSelection(ctx context.Context, p gateway.Provider, resolver gateway.KeyResolver, spec cards.Spec, dimension string, studentSpan Anchor) (SelectionEval, gateway.Resolved, gateway.ChatUsage, error) {
	resolved, err := resolver(ctx)
	if err != nil {
		return fallbackEval(studentSpan), gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildEvalPrompt(spec, dimension)},
			{Role: gateway.RoleUser, Content: "学生从文章里选的句子：「" + studentSpan.Quote + "」"},
		},
		// 3000, not 700: reasoning-model headroom (see reading_router.go) — a
		// truncated eval returns an empty verdict and the 3-check silently fails.
		MaxTokens: 3000,
	}
	res, err := gateway.Collect(ctx, p, resolved, req)
	if err != nil {
		return fallbackEval(studentSpan), gateway.Resolved{}, gateway.ChatUsage{}, nil
	}
	var reply evalReply
	if perr := json.Unmarshal([]byte(stripFences(res.Text)), &reply); perr != nil || len(reply.Checks) == 0 {
		return fallbackEval(studentSpan), resolved, res.Usage, nil
	}
	checks := make([]SelectionCheck, 0, 3)
	for _, c := range reply.Checks {
		ev := ""
		if c.Evidence != "" && strings.Contains(studentSpan.Quote, c.Evidence) { // integrity: verbatim only
			ev = c.Evidence
		}
		checks = append(checks, SelectionCheck{
			Key: c.Key, Label: checkLabels[c.Key], Status: normalizeStatus(c.Status),
			Evidence: ev, Explanation: c.Explanation,
		})
	}
	verdict, label := verdictFromChecks(checks)
	return SelectionEval{
		Verdict: verdict, VerdictLabel: label, VerdictReason: reply.Finding,
		Checks:  checks,
		Finding: reply.Finding, Judgment: reply.Judgment, Support: reply.Support,
		Caveat: reply.Caveat, NextStep: reply.NextStep,
		SpanIDs: []string{studentSpan.ID}, // integrity: the student's span only
	}, resolved, res.Usage, nil
}

// fallbackEval keeps the loop alive with a neutral, honest partial verdict when
// the model is unavailable. It never fabricates specifics — evidence snippets
// stay empty and the finding is generic.
func fallbackEval(studentSpan Anchor) SelectionEval {
	checks := []SelectionCheck{
		{Key: "target", Label: checkLabels["target"], Status: "partial", Explanation: "先记下你的选择，我们一起再看这句和任务的关系。"},
		{Key: "evidence", Label: checkLabels["evidence"], Status: "partial", Explanation: "看看这句里最关键的词是哪一个。"},
		{Key: "centrality", Label: checkLabels["centrality"], Status: "partial", Explanation: "这条线索足以支撑你的判断吗？"},
	}
	verdict, label := verdictFromChecks(checks)
	return SelectionEval{
		Verdict: verdict, VerdictLabel: label,
		VerdictReason: "先把你的发现记下来。", Checks: checks,
		Finding: "你选了这句作为证据。", NextStep: "回到文章，标出这句里最关键的一处线索。",
		SpanIDs: []string{studentSpan.ID},
		// Set HERE, at the single place the canned text is minted, so no
		// caller can forget it and no future degrade path can slip through
		// unmarked.
		Degraded: true,
	}
}
