package evalbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
)

func TestRenderMarkdownReportGoldenFixture(t *testing.T) {
	c, manifest, summary, states := markdownFixture()
	got := renderMarkdownReport(c, manifest, summary, states)
	if again := renderMarkdownReport(c, manifest, summary, states); got != again {
		t.Fatal("Markdown renderer is not deterministic")
	}
	for _, want := range []string{
		"# EvalBench 实验结果摘要",
		"## 实验范围",
		"## 核心比较",
		"## 人工 Gold 对齐与错误画像",
		"## 性能与成本",
		"## 可靠性与解释边界",
		"JSON 无法解析 × 1",
		"不可评估（每个 case 少于 2 个完整成功 run）",
		"[详细报告](report-details.md)",
		"[summary.json](summary.json)",
		"$0.000022",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"Candidate EvaluationReport",
		"完整 46 项矩阵",
		"全过程综述",
		"下一步",
		"推荐课程",
		"<script>",
	} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("compact report must not contain %q:\n%s", forbidden, got)
		}
	}
	if lines := strings.Count(got, "\n"); lines > 150 {
		t.Fatalf("compact report has %d lines, want <= 150", lines)
	}
	if strings.Contains(got, "建议") {
		t.Fatalf("compact report must not emit suggestions:\n%s", got)
	}
}

func TestRenderDetailedMarkdownReportGoldenFixture(t *testing.T) {
	c, manifest, summary, states := markdownFixture()
	got := renderDetailedMarkdownReport(c, manifest, summary, states)
	if again := renderDetailedMarkdownReport(c, manifest, summary, states); got != again {
		t.Fatal("detailed Markdown renderer is not deterministic")
	}
	for _, want := range []string{
		"# EvalBench 实验详细报告",
		"[返回实验结果摘要](report.md)",
		"Candidate 中出现的建议或下一步属于模型原始输出",
		"##### Candidate EvaluationReport",
		"代表 run**：attempt-002",
		"D1 · 任务理解与问题表述 — L4",
		"A1 · 方向自主 — 4/5",
		"###### 完整 46 项矩阵",
		"###### Comparator verdict 稳定性",
		"##### 调用成本明细",
		"## 可复现信息",
		"`cases/case-a/production-current/attempt-002/comparison.json`",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("detailed report missing %q", want)
		}
	}
	if gotCount := strings.Count(got, "| 认知深度 | D1 | judgement |"); gotCount != 4 {
		t.Fatalf("D1 comparison rows = %d, want each variant in highlighted differences and complete matrix", gotCount)
	}
	if !strings.Contains(got, "$0.000022") || !strings.Contains(got, "eval\\_report\\_rubric") || !strings.Contains(got, "single\\_prompt\\_evalreport\\_v1") {
		t.Fatalf("cost comparison is missing priced call details:\n%s", got)
	}
	if strings.Contains(got, "<script>") || !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("HTML was not escaped:\n%s", got)
	}
	if !strings.Contains(got, "\\# heading") || !strings.Contains(got, "\\|") || !strings.Contains(got, "\\\\path") {
		t.Fatalf("unsafe Markdown was not escaped:\n%s", got)
	}
	if !strings.Contains(got, "> first line\n> \\# second line") {
		t.Fatalf("multiline source quote was not safely prefixed:\n%s", got)
	}
	for n, part := range strings.Split(got, "###### 完整 46 项矩阵\n\n")[1:] {
		matrix := strings.SplitN(part, "\n原始 comparison：", 2)[0]
		if rows := strings.Count(matrix, "\n|") - 1; rows != 46 {
			t.Fatalf("matrix %d has %d item rows, want 46", n+1, rows)
		}
	}
}

func TestRenderDetailedMarkdownReportUsesDiagnosticPreviewWhenNoSuccessfulRun(t *testing.T) {
	c, manifest, summary, states := markdownFixture()
	state := states["case-a"][0]
	state.attempts = []attemptResult{{
		status: AttemptStatus{Attempt: 1, Status: "partial", Error: "comparator: provider stream failed"},
		report: &evalreport.Report{Basics: evalreport.Basics{Title: "诊断任务"}},
	}}
	summary = buildSummary("exp-report", states, c)
	got := renderDetailedMarkdownReport(c, manifest, summary, states)
	for _, want := range []string{
		"**诊断预览**",
		"未提供：该 attempt 没有通过 comparator。",
		"Provider 流失败",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diagnostic report missing %q", want)
		}
	}
}

func TestRenderDetailedMarkdownReportChoosesLowestSuccessfulAttemptNumber(t *testing.T) {
	c, manifest, summary, states := markdownFixture()
	state := states["case-a"][0]
	report2, report3 := markdownReportFixture(), markdownReportFixture()
	report2.Basics.Title = "attempt two"
	report3.Basics.Title = "attempt three"
	comparison := markdownComparisonFixture()
	state.attempts = []attemptResult{
		{status: AttemptStatus{Attempt: 3, Status: "success", Comparison: "success"}, report: &report3, comparison: &comparison, complete: true},
		{status: AttemptStatus{Attempt: 2, Status: "success", Comparison: "success"}, report: &report2, comparison: &comparison, complete: true},
	}
	summary = buildSummary("exp-report", states, c)
	got := renderDetailedMarkdownReport(c, manifest, summary, states)
	if !strings.Contains(got, "代表 run**：attempt-002") || !strings.Contains(got, "attempt two") || strings.Contains(got, "attempt three") {
		t.Fatalf("representative run was not the lowest successful attempt number")
	}
	if strings.Index(got, "| 2 | success") > strings.Index(got, "| 3 | success") {
		t.Fatalf("attempt diagnostics are not ordered by attempt number")
	}
}

func TestRenderDetailedMarkdownReportHandlesFailedRunAndMissingUsage(t *testing.T) {
	c, manifest, summary, states := markdownFixture()
	states["case-a"][0].attempts = []attemptResult{{status: AttemptStatus{Attempt: 1, Status: "failed", Error: "timeout"}}}
	states["case-a"][1].attempts[0].candidate = []CallRecord{{TotalMs: 100}}
	summary = buildSummary("exp-report", states, c)
	got := renderDetailedMarkdownReport(c, manifest, summary, states)
	for _, want := range []string{"没有可展示的 candidate report", "调用超时", "未提供"} {
		if !strings.Contains(got, want) {
			t.Errorf("failure/missing-data report missing %q", want)
		}
	}
	compact := renderMarkdownReport(c, manifest, summary, states)
	for _, want := range []string{"usage 缺失 1", "Candidate / 完整成功"} {
		if !strings.Contains(compact, want) {
			t.Errorf("compact failure/missing-data report missing %q", want)
		}
	}
}

func TestRenderMarkdownReportRendersCaseCoverageInConfigOrder(t *testing.T) {
	c, manifest, _, states := markdownFixture()
	c.Cases = append(c.Cases, CaseConfig{ID: "case-b"})
	states["case-b"] = []*variantState{
		{config: c.Variants[0], attempts: []attemptResult{{status: AttemptStatus{Attempt: 1, Status: "failed", Error: "timeout"}}}},
		{config: c.Variants[1], attempts: []attemptResult{{status: AttemptStatus{Attempt: 1, Status: "failed", Error: "timeout"}}}},
	}
	summary := buildSummary("exp-report", states, c)
	got := renderMarkdownReport(c, manifest, summary, states)
	if !strings.Contains(got, "### Case 覆盖") {
		t.Fatal("compact report missing case coverage")
	}
	first, second := strings.Index(got, "| case-a | production-current"), strings.Index(got, "| case-b | production-current")
	if first < 0 || second < 0 || first > second {
		t.Fatalf("case coverage is not in config order:\n%s", got)
	}
}

func TestRenderMarkdownReportSummarizesComparatorStability(t *testing.T) {
	c, manifest, _, states := markdownFixture()
	state := states["case-a"][0]
	report := markdownReportFixture()
	comparison := markdownComparisonFixture()
	state.attempts = append(state.attempts, attemptResult{
		status:     AttemptStatus{Attempt: 3, Status: "success", Comparison: "success"},
		report:     &report,
		comparison: &comparison,
		complete:   true,
	})
	summary := buildSummary("exp-report", states, c)
	got := renderMarkdownReport(c, manifest, summary, states)
	if !strings.Contains(got, "46 项；平均 100.0%；最低 100.0%") {
		t.Fatalf("compact report missing stability summary:\n%s", got)
	}
}

func TestWriteMarkdownReportsWritesBothArtifactsAndReturnsWriteErrors(t *testing.T) {
	c, manifest, summary, states := markdownFixture()
	root := t.TempDir()
	if err := writeMarkdownReports(root, c, manifest, summary, states); err != nil {
		t.Fatalf("writeMarkdownReports() error = %v", err)
	}
	for _, name := range []string{"report.md", "report-details.md"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if len(content) == 0 {
			t.Fatalf("%s is empty", name)
		}
	}

	notDir := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(notDir, []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeMarkdownReports(notDir, c, manifest, summary, states); err == nil || !strings.Contains(err.Error(), "write report-details.md") {
		t.Fatalf("writeMarkdownReports() error = %v, want report-details.md write error", err)
	}
}

func markdownFixture() (Config, Manifest, Summary, map[string][]*variantState) {
	c := Config{
		Name:           "markdown-fixture",
		SuccessfulRuns: 1,
		MaxAttempts:    2,
		Cases:          []CaseConfig{{ID: "case-a"}},
		Models:         map[string]ModelProfile{"flagship": {Provider: "deepseek", Model: "deepseek-v4-pro"}},
		Variants: []VariantConfig{
			{ID: "production-current", Evaluator: "production-evalreport-v1", Model: "flagship"},
			{ID: "single-prompt-v1", Evaluator: singlePromptEvaluatorID, Model: "flagship"},
		},
		Comparator: ModelUse{Model: "flagship", PromptVersion: ComparatorPromptVersion},
	}
	now := time.Date(2026, 8, 18, 4, 5, 6, 0, time.UTC)
	manifest := Manifest{
		ExperimentID: "exp-report", Name: c.Name, ConfigHash: "config-sha", RubricHash: "rubric-sha", GitCommit: "abc123", Dirty: true,
		StartedAt: now, CompletedAt: &now, Status: "completed", Inputs: map[string]string{"case-a": "input-sha"}, Gold: map[string]string{"case-a": "gold-sha"},
		Models: c.Models, Pricing: buildPricingSnapshot(c.Models), Versions: map[string]string{"adapter": "persona-export-v1", "pricing": gateway.PricingVersion}, Evaluators: map[string]EvaluatorDescriptor{
			"production-current": {ImplementationVersion: "evalbench-production-baseline-v1", PromptVersion: "agent-reportgen-v1"},
			"single-prompt-v1":   {ImplementationVersion: "single-prompt-v1", PromptVersion: "single-prompt-v1", PromptSHA256: "prompt-sha"},
		},
		Execution: []string{"case-a/production-current/attempt-001", "case-a/production-current/attempt-002", "case-a/single-prompt-v1/attempt-001"},
	}
	firstIn, firstOut := 10, 20
	rep := markdownReportFixture()
	comparison := markdownComparisonFixture()
	states := map[string][]*variantState{
		"case-a": {
			{config: c.Variants[0], attempts: []attemptResult{
				{status: AttemptStatus{Attempt: 1, Status: "failed", WallMs: 11, Error: "agent: report output not JSON: unexpected end of JSON input", InputHash: "input-sha"}},
				{status: AttemptStatus{Attempt: 2, Status: "success", WallMs: 1250, CallCount: 4, Comparison: "success", InputHash: "input-sha"}, report: &rep, comparison: &comparison, complete: true, candidate: []CallRecord{{Purpose: "eval_report_rubric", Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &firstIn, OutputTokens: &firstOut, TotalMs: 100, FirstOutputMs: intPtr(10)}}, comparator: []CallRecord{{Purpose: "comparator", Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &firstIn, OutputTokens: &firstOut, TotalMs: 100}}},
			}},
			{config: c.Variants[1], attempts: []attemptResult{
				{status: AttemptStatus{Attempt: 1, Status: "success", WallMs: 1000, CallCount: 1, Comparison: "success", InputHash: "input-sha"}, report: &rep, comparison: &comparison, complete: true, candidate: []CallRecord{{Purpose: "single_prompt_evalreport_v1", Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &firstIn, OutputTokens: &firstOut, TotalMs: 100, FirstOutputMs: intPtr(8)}}, comparator: []CallRecord{{Purpose: "comparator", Provider: "deepseek", Model: "deepseek-v4-pro", InputTokens: &firstIn, OutputTokens: &firstOut, TotalMs: 100}}},
			}},
		},
	}
	return c, manifest, buildSummary("exp-report", states, c), states
}

func markdownReportFixture() evalreport.Report {
	end := "2026-08-02T00:00:00Z"
	r := evalreport.Report{Version: 1, ReportID: "report", ProjectID: "project", GeneratedAt: "2026-08-03T00:00:00Z"}
	r.Student.ID, r.Student.Name = "student", "学生 A"
	r.Basics = evalreport.Basics{Title: "任务 | <script>", Type: "essay", StartDate: "2026-08-01T00:00:00Z", EndDate: &end, Counters: evalreport.Counters{AITurns: 2, MaterialsRead: 3, WordsWritten: 400, AICommentCount: 1, EditCount: 2}}
	r.Abstract = evalreport.Abstract{Overview: "总览", MaterialSentence: "材料", WritingSentence: "写作", AISentence: "AI 边界", SuggestionParagraph: "建议", SuggestionSentences: []string{"下一步"}, RecommendedCourses: []evalreport.RecommendedCourse{{CourseID: "course-a", Reason: "原因"}}}
	r.Events = []evalreport.EventEntry{{TS: "2026-08-01T08:00:00+08:00", Kind: "chat", Summary: "# heading | <script> \\path\nsecond", AITurns: 1}}
	materialURL := "https://example.com/a|b"
	r.Materials = []evalreport.MaterialEntry{{MaterialID: "material-a", AddedAt: "2026-08-01T08:00:00+08:00", Source: "来源", URL: &materialURL, FinalStatus: "used", Comment: "备注", CannotSupport: "不能支持"}}
	for _, id := range []string{"D1", "D2", "D3", "D4", "D5", "D6"} {
		quote := "quote"
		if id == "D1" {
			quote = "first line\n# second line"
		}
		r.Depth = append(r.Depth, evalreport.DepthDimResult{ID: id, Level: 4, Summary: id + " 摘要", Evidence: []evalreport.EvidenceItem{{ID: "chat:1", TS: "2026-08-01T08:00:00+08:00", Stage: "研究", Quote: quote, Observation: "观察"}}, Suggestion: "建议"})
	}
	for _, id := range []string{"A1", "A2", "A3", "A4", "A5", "A6"} {
		r.Autonomy = append(r.Autonomy, evalreport.AutonomyDimResult{ID: id, Band: 4, Summary: id + " 摘要", Evidence: []evalreport.EvidenceItem{{ID: "chat:1", Quote: "quote", Observation: "观察"}}, Suggestion: "建议"})
	}
	r.PromptLens = evalreport.PromptLens{Summary: "透镜", Prompts: []evalreport.PromptItem{{Stage: "研究", Quote: "问题", Ref: evalreport.Ref{ID: "chat:1"}, Observation: "观察", RelatedDomains: []string{"D1"}}}}
	r.ToolUsage = []evalreport.ToolUsageEntry{{ToolID: "craap", Name: "CRAAP", Stage: "阅读", Purpose: "目的", Summary: "摘要"}}
	r.Risks = []evalreport.RiskEntry{{Type: "data-scope", Behaviour: "风险", Suggestion: "建议"}}
	return r
}

func markdownComparisonFixture() Comparison {
	items := ExpectedComparisonItems()
	for i := range items {
		items[i].Comparison = "aligned"
		items[i].Confidence = "high"
		items[i].GoldExcerpt = "gold"
		items[i].CandidateExcerpt = "candidate"
		items[i].Reason = "对齐"
	}
	items[0].Comparison = "understates"
	items[0].Reason = "候选较保守"
	return Comparison{Items: items}
}

func intPtr(v int64) *int64 { return &v }
