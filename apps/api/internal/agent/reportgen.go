package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/rubric"
)

// reportgen.go — the evaluation-report GENERATION subagents. The report splits
// into a deterministic FACT half (assembled in api/evaluation_generate.go) and
// this LLM half: four flagship (reasoning-on, never-downgraded) calls,
// partitioned by the data-context each judgment needs so the big trajectory
// digest is not re-sent to a call that only needs a slice:
//
//	A · GeneratePromptLens — lean: only the student's prompts.
//	B · GenerateRisks      — lean: only the risk-signal slice.
//	C · GenerateRubric     — full trajectory digest (depth D1-D6 + autonomy A1-A6).
//	D · GenerateAbstract   — full digest + C's axis results (synthesis needs them).
//
// Every call is grounded in an evidence-candidate list (cite ONLY bracketed
// ids); api-side evalreport.ValidateRefs drops any id that slips through.
// Each returns (result, usage, error) so the caller meters even a failed call.

const reportGenMaxTokens = 16000

// ReportGenContext carries the pre-rendered digests the four calls draw on. The
// api layer builds these deterministically from recorded data; this package
// only prompts + parses.
type ReportGenContext struct {
	Title         string // the research question / project title
	Qualification string // e.g. 拓展论文 EE
	// Candidates is the evidence-candidate list, one "[id] kind: label" line
	// each. Every cited evidence/ref id MUST be one of these ids.
	Candidates string
	// Trajectory is the full process digest (proposal, timeline, references,
	// cards, draft excerpt) — the shared context for the rubric + abstract.
	Trajectory string
	// Prompts is the lean digest of the student's own prompts to AI (chat user
	// turns + reading focus). Feeds promptLens.
	Prompts string
	// RiskSignals is the lean digest for the risk scan (draft + citations +
	// sourcing/ghostwrite/rabbit-hole signals). Feeds risks.
	RiskSignals string
	// Counters is a one-line factual summary (turns/materials/words/comments/edits)
	// for the abstract.
	Counters string
}

// collectReport runs one flagship completion and returns the salvaged JSON
// object text + usage, retrying once on an unparseable reply.
func collectReport(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, system, user string) (string, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: user},
		},
		MaxTokens: reportGenMaxTokens,
	}
	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		obj := extractJSONObject(stripFences(res.Text))
		if obj != "" {
			return obj, lastUsage, nil
		}
		lastErr = fmt.Errorf("reportgen: no JSON object in reply")
	}
	return "", lastUsage, lastErr
}

// ---- C · Rubric (depth D1-D6 + autonomy A1-A6) ----

type rubricReply struct {
	Depth []struct {
		ID       string `json:"id"`
		Level    int    `json:"level"`
		Summary  string `json:"summary"`
		Evidence []struct {
			ID          string `json:"id"`
			Quote       string `json:"quote"`
			Observation string `json:"observation"`
			Boundary    string `json:"boundary"`
		} `json:"evidence"`
		Suggestion string `json:"suggestion"`
	} `json:"depth"`
	Autonomy []struct {
		ID       string `json:"id"`
		Band     int    `json:"band"`
		Summary  string `json:"summary"`
		Evidence []struct {
			ID          string `json:"id"`
			Quote       string `json:"quote"`
			Observation string `json:"observation"`
			Boundary    string `json:"boundary"`
		} `json:"evidence"`
		Suggestion string `json:"suggestion"`
	} `json:"autonomy"`
}

// RubricResult is the parsed depth + autonomy judgment.
type RubricResult struct {
	Depth    []evalreport.DepthDimResult
	Autonomy []evalreport.AutonomyDimResult
}

// GenerateRubric produces the 12 dual-axis judgments grounded in the full
// trajectory. Levels 1-4 (depth), bands 0-5 (autonomy); D6 reflection is NA
// (level 0 → clamped to 1 with an NA note by the model) when the student wrote
// no reflection. Missing dims are backfilled to a conservative floor by the
// caller so the report always has 6+6.
func GenerateRubric(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in ReportGenContext) (RubricResult, gateway.ChatUsage, error) {
	system := rubricSystemPrompt()
	var b strings.Builder
	fmt.Fprintf(&b, "题目：%s（%s）\n\n", in.Title, in.Qualification)
	fmt.Fprintf(&b, "【可引用的证据（evidence.id 只能取下面方括号里的 id，取不到就留空字符串）】\n%s\n\n", in.Candidates)
	fmt.Fprintf(&b, "【学生的全过程记录】\n%s\n", in.Trajectory)
	obj, usage, err := collectReport(ctx, prov, resolved, system, b.String())
	if err != nil {
		return RubricResult{}, usage, err
	}
	var parsed rubricReply
	if uerr := json.Unmarshal([]byte(obj), &parsed); uerr != nil {
		return RubricResult{}, usage, fmt.Errorf("reportgen: rubric unmarshal: %w", uerr)
	}
	out := RubricResult{}
	for _, d := range parsed.Depth {
		ev := make([]evalreport.EvidenceItem, 0, len(d.Evidence))
		for _, e := range d.Evidence {
			ev = append(ev, evalreport.EvidenceItem{ID: e.ID, Quote: e.Quote, Observation: e.Observation, Boundary: e.Boundary})
		}
		out.Depth = append(out.Depth, evalreport.DepthDimResult{
			ID: d.ID, Level: clampInt(d.Level, 1, 4), Summary: d.Summary, Evidence: ev, Suggestion: d.Suggestion,
		})
	}
	for _, a := range parsed.Autonomy {
		ev := make([]evalreport.EvidenceItem, 0, len(a.Evidence))
		for _, e := range a.Evidence {
			ev = append(ev, evalreport.EvidenceItem{ID: e.ID, Quote: e.Quote, Observation: e.Observation, Boundary: e.Boundary})
		}
		out.Autonomy = append(out.Autonomy, evalreport.AutonomyDimResult{
			ID: a.ID, Band: clampInt(a.Band, 0, 5), Summary: a.Summary, Evidence: ev, Suggestion: a.Suggestion,
		})
	}
	return out, usage, nil
}

func rubricSystemPrompt() string {
	var b strings.Builder
	m := rubric.Model()
	b.WriteString("你是「印记」的过程评估分析器。基于学生的真实全过程记录，对两条轴逐维给出判定。铁律：\n")
	b.WriteString("· 公理：" + m.Axiom + "\n")
	b.WriteString("· 两轴永不合成总分。认知深度按 L1-L4 档；智识自主按 0-5 行为计数带（不是质量分）。\n")
	b.WriteString("· 只依据记录里真实出现的证据判定；证据不足就给保守档并说明「暂无足够证据」，绝不拔高。\n")
	b.WriteString("· evidence 里每条的 id 只能取用户消息中【可引用的证据】里方括号内的 id；找不到贴切的就把 id 留空字符串，绝不编造 id。\n")
	b.WriteString("· D6 反思维度只认学生亲手写的反思；若无，summary 注明「学生未留下自写反思（NA）」并给最低档 1，不惩罚性判低。\n\n")
	b.WriteString("【认知深度 D1-D6（id · 名称 · 看的是 · L1/L2/L3/L4 锚点）】\n")
	for _, d := range m.Depth {
		fmt.Fprintf(&b, "%s %s — %s\n  L1:%s\n  L2:%s\n  L3:%s\n  L4:%s\n", d.ID, d.Name, d.Means, d.Anchors["L1"], d.Anchors["L2"], d.Anchors["L3"], d.Anchors["L4"])
	}
	b.WriteString("\n【智识自主 A1-A6（id · 名称 · 看的是 · 可计入事件）】\n")
	for _, a := range m.Autonomy {
		fmt.Fprintf(&b, "%s %s — %s（事件：%s）\n", a.ID, a.Name, a.Means, a.Event)
	}
	fmt.Fprintf(&b, "\n【自主 0-5 带的判定口径】%s\n\n", m.AutonomyBand)
	b.WriteString("只输出一个 JSON 对象，形如：\n")
	b.WriteString(`{"depth":[{"id":"D1","level":1,"summary":"一句话判定，含证据依据","evidence":[{"id":"<证据id或空>","quote":"记录里的原话片段","observation":"这条证据说明什么","boundary":"可选·边界或局限"}],"suggestion":"一句可操作的建议"}],"autonomy":[{"id":"A1","band":0,"summary":"...","evidence":[{"id":"...","quote":"...","observation":"..."}],"suggestion":"..."}]}`)
	b.WriteString("\ndepth 必须含 D1-D6 六项，autonomy 必须含 A1-A6 六项，顺序不限。不要输出对象以外的任何文字或代码块标记。")
	return b.String()
}

// ---- A · PromptLens ----

type promptLensReply struct {
	Summary string `json:"summary"`
	Prompts []struct {
		Stage          string   `json:"stage"`
		Quote          string   `json:"quote"`
		Ref            struct{ ID, Label string } `json:"ref"`
		Observation    string   `json:"observation"`
		RelatedDomains []string `json:"relatedDomains"`
		Attention      bool     `json:"attention"`
	} `json:"prompts"`
}

// GeneratePromptLens reads only the student's own prompts to AI and produces the
// 提问透镜 summary + a small set of notable prompts.
func GeneratePromptLens(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in ReportGenContext) (evalreport.PromptLens, gateway.ChatUsage, error) {
	system := "你是「印记」的提问透镜分析器。只看学生对 AI 的提问/指令本身，判断她是把 AI 当搜索引擎/代写，还是当思考的对手。挑出最有代表性的 3-6 条提问，指出每条体现的提问成熟度。ref.id 只能取【可引用的证据】里的方括号 id，取不到留空。只输出一个 JSON 对象：" +
		`{"summary":"一段话·整体提问画像","prompts":[{"stage":"阶段","quote":"提问原话","ref":{"id":"<id或空>","label":"简短标签"},"observation":"这条提问体现了什么","relatedDomains":["相关学科/主题"],"attention":false}]}` +
		"\nattention=true 表示这条提问值得提醒（如求代写/求直接答案）。不要输出对象以外的文字或代码块标记。"
	user := fmt.Sprintf("题目：%s\n\n【可引用的证据】\n%s\n\n【学生对 AI 的提问记录】\n%s\n", in.Title, in.Candidates, in.Prompts)
	obj, usage, err := collectReport(ctx, prov, resolved, system, user)
	if err != nil {
		return evalreport.PromptLens{}, usage, err
	}
	var parsed promptLensReply
	if uerr := json.Unmarshal([]byte(obj), &parsed); uerr != nil {
		return evalreport.PromptLens{}, usage, fmt.Errorf("reportgen: promptlens unmarshal: %w", uerr)
	}
	out := evalreport.PromptLens{Summary: parsed.Summary}
	for _, p := range parsed.Prompts {
		out.Prompts = append(out.Prompts, evalreport.PromptItem{
			Stage: p.Stage, Quote: p.Quote,
			Ref:         evalreport.Ref{ID: p.Ref.ID, Label: p.Ref.Label},
			Observation: p.Observation, RelatedDomains: p.RelatedDomains, Attention: p.Attention,
		})
	}
	return out, usage, nil
}

// ---- B · Risks ----

type risksReply struct {
	Risks []struct {
		Type      string `json:"type"`
		Behaviour string `json:"behaviour"`
		Ref       struct{ ID, Label string } `json:"ref"`
		Suggestion string `json:"suggestion"`
	} `json:"risks"`
}

var validRiskTypes = map[string]bool{
	"ai-ghostwrite": true, "missing-source": true, "argument-logic": true,
	"data-scope": true, "rabbit-hole-offtopic": true,
}

// GenerateRisks scans the risk-signal slice and returns any flagged behaviours
// (possibly none — an empty list is the healthy case, reported as such).
func GenerateRisks(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in ReportGenContext) ([]evalreport.RiskEntry, gateway.ChatUsage, error) {
	system := "你是「印记」的风险审阅器。只标记记录里真实出现的、值得提醒的行为，绝不臆测。type 只能取：ai-ghostwrite（疑似让 AI 代写正文）、missing-source（引用未溯源）、argument-logic（论证逻辑跳跃）、data-scope（数据/结论范围过宽）、rabbit-hole-offtopic（跑题的兔子洞）。没有风险就返回空数组。ref.id 只能取【可引用的证据】里的方括号 id，取不到留空。只输出一个 JSON 对象：" +
		`{"risks":[{"type":"missing-source","behaviour":"具体行为描述","ref":{"id":"<id或空>","label":"标签"},"suggestion":"一句改进建议"}]}` +
		"\n不要输出对象以外的文字或代码块标记。"
	user := fmt.Sprintf("题目：%s\n\n【可引用的证据】\n%s\n\n【风险信号记录（正文、引用、来源、跑题等）】\n%s\n", in.Title, in.Candidates, in.RiskSignals)
	obj, usage, err := collectReport(ctx, prov, resolved, system, user)
	if err != nil {
		return nil, usage, err
	}
	var parsed risksReply
	if uerr := json.Unmarshal([]byte(obj), &parsed); uerr != nil {
		return nil, usage, fmt.Errorf("reportgen: risks unmarshal: %w", uerr)
	}
	out := make([]evalreport.RiskEntry, 0, len(parsed.Risks))
	for _, r := range parsed.Risks {
		if !validRiskTypes[r.Type] {
			continue // drop an out-of-enum type rather than fail the report
		}
		var ref *evalreport.Ref
		if r.Ref.ID != "" || r.Ref.Label != "" {
			ref = &evalreport.Ref{ID: r.Ref.ID, Label: r.Ref.Label}
		}
		out = append(out, evalreport.RiskEntry{Type: r.Type, Behaviour: r.Behaviour, Ref: ref, Suggestion: r.Suggestion})
	}
	return out, usage, nil
}

// ---- D · Abstract ----

type abstractReply struct {
	Overview            string   `json:"overview"`
	MaterialSentence    string   `json:"materialSentence"`
	WritingSentence     string   `json:"writingSentence"`
	AISentence          string   `json:"aiSentence"`
	SuggestionParagraph string   `json:"suggestionParagraph"`
	SuggestionSentences []string `json:"suggestionSentences"`
	RecommendedCourses  []struct {
		CourseID string `json:"courseId"`
		Reason   string `json:"reason"`
	} `json:"recommendedCourses"`
}

// GenerateAbstract synthesises the 综述 · 学生画像 from the full digest plus the
// axis results already produced (it summarises what depth/autonomy found).
func GenerateAbstract(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in ReportGenContext, axes RubricResult) (evalreport.Abstract, gateway.ChatUsage, error) {
	system := "你是「印记」的过程评估综述器。用一段克制、诚实、对学生说话的画像总结这个项目的思考过程：材料一句、写作一句、与 AI 协作一句，再给一段建议 + 2-3 条可操作的下一步。绝不吹捧、绝不替学生定论。只输出一个 JSON 对象：" +
		`{"overview":"一段总画像","materialSentence":"材料一句","writingSentence":"写作一句","aiSentence":"与AI协作一句","suggestionParagraph":"一段建议","suggestionSentences":["下一步1","下一步2"],"recommendedCourses":[]}` +
		"\nrecommendedCourses 没有把握就留空数组。不要输出对象以外的文字或代码块标记。"
	var ax strings.Builder
	for _, d := range axes.Depth {
		fmt.Fprintf(&ax, "%s L%d：%s\n", d.ID, d.Level, d.Summary)
	}
	for _, a := range axes.Autonomy {
		fmt.Fprintf(&ax, "%s 带%d：%s\n", a.ID, a.Band, a.Summary)
	}
	user := fmt.Sprintf("题目：%s（%s）\n\n【客观计数】%s\n\n【两轴判定结果】\n%s\n\n【全过程记录】\n%s\n", in.Title, in.Qualification, in.Counters, ax.String(), in.Trajectory)
	obj, usage, err := collectReport(ctx, prov, resolved, system, user)
	if err != nil {
		return evalreport.Abstract{}, usage, err
	}
	var parsed abstractReply
	if uerr := json.Unmarshal([]byte(obj), &parsed); uerr != nil {
		return evalreport.Abstract{}, usage, fmt.Errorf("reportgen: abstract unmarshal: %w", uerr)
	}
	out := evalreport.Abstract{
		Overview: parsed.Overview, MaterialSentence: parsed.MaterialSentence,
		WritingSentence: parsed.WritingSentence, AISentence: parsed.AISentence,
		SuggestionParagraph: parsed.SuggestionParagraph, SuggestionSentences: parsed.SuggestionSentences,
	}
	for _, c := range parsed.RecommendedCourses {
		out.RecommendedCourses = append(out.RecommendedCourses, evalreport.RecommendedCourse{CourseID: c.CourseID, Reason: c.Reason})
	}
	return out, usage, nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
