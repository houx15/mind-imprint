package api

// evaluation_benchmark_test.go — the report-generator benchmark (Task 110).
// Re-mocks 10 varied personas (weak→strong), runs the REAL four-call generator
// against a real DeepSeek flagship model, and records per-call token + wall-time
// + result. Gated: runs only when RUN_REPORT_BENCHMARK=1 and DEEPSEEK_API_KEY is
// set (source .deploy-local/build-env.sh), so the normal suite never spends.
//
// Output: a JSON metrics file (REPORT_BENCHMARK_OUT, default /tmp/report-benchmark.json)
// plus a t.Log summary line per persona.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/config"
	"mindimprint/api/internal/evalreport"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// personaSpec drives one seeded persona. level 1..3 scales richness/quality and
// prompt style (1 = leans on AI for answers, few sources, no reflection;
// 3 = sets boundaries, evaluates sources, writes reflection).
type personaSpec struct {
	Name    string
	Qual    string
	Topic   string
	Level   int
	NRefs   int
	NCards  int
	NChat   int // user+assistant messages (even)
	Paras   int // draft paragraphs
	Reflect bool
}

func benchmarkPersonas() []personaSpec {
	return []personaSpec{
		{"林安", "拓展论文 EE", "中国的可再生能源投资是否让全球更可持续？", 3, 9, 5, 40, 12, true},
		{"周晓", "拓展论文 EE", "社交媒体使用与青少年注意力持续时间的关系", 2, 7, 4, 30, 9, true},
		{"陈默", "TOK 论文", "我们能否只在理解语境的范围内理解事物？", 1, 4, 2, 16, 5, false},
		{"李泽", "内部评估 IA", "本地河流氮污染的季节性变化", 3, 8, 5, 36, 11, true},
		{"王予", "EPQ", "AI 生成内容对独立音乐人收入的影响", 2, 6, 3, 26, 8, false},
		{"张宁", "拓展论文 EE", "碳定价政策在欧盟制造业中的实际减排效应", 3, 9, 5, 42, 13, true},
		{"刘一", "TOK 论文", "数学知识是发现的还是发明的？", 1, 3, 2, 14, 4, false},
		{"赵敏", "内部评估 IA", "不同 pH 对酶活性的影响", 2, 7, 4, 28, 9, true},
		{"孙可", "个人项目", "校园食物浪费的行为干预设计", 2, 6, 3, 24, 7, false},
		{"何萱", "拓展论文 EE", "气候叙事框架如何影响青年环保行为意向", 3, 8, 5, 38, 12, true},
	}
}

type personaResult struct {
	Name        string           `json:"name"`
	Qual        string           `json:"qual"`
	Topic       string           `json:"topic"`
	Level       int              `json:"level"`
	ProjectID   string           `json:"projectId"`
	TotalMillis int64            `json:"totalMillis"`
	TotalIn     int              `json:"totalInputTokens"`
	TotalOut    int              `json:"totalOutputTokens"`
	Candidates  int              `json:"candidates"`
	RefsDropped int              `json:"refsDropped"`
	RefsTotal   int              `json:"refsTotal"`
	Calls       []reportCallStat `json:"calls"`
	Result      reportSnapshot   `json:"result"`
	Err         string           `json:"err,omitempty"`
}

type reportSnapshot struct {
	WordsWritten int    `json:"wordsWritten"`
	AICommentCnt int    `json:"aiCommentCount"`
	DepthLevels  []int  `json:"depthLevels"`
	AutonomyBand []int  `json:"autonomyBands"`
	NRisks       int    `json:"nRisks"`
	NPromptLens  int    `json:"nPromptLens"`
	NMaterials   int    `json:"nMaterials"`
	Overview     string `json:"overview"`
}

func snapshotReport(rep evalreport.Report) reportSnapshot {
	s := reportSnapshot{
		WordsWritten: rep.Basics.Counters.WordsWritten,
		AICommentCnt: rep.Basics.Counters.AICommentCount,
		NRisks:       len(rep.Risks),
		NPromptLens:  len(rep.PromptLens.Prompts),
		NMaterials:   len(rep.Materials),
		Overview:     rep.Abstract.Overview,
	}
	for _, d := range rep.Depth {
		s.DepthLevels = append(s.DepthLevels, d.Level)
	}
	for _, a := range rep.Autonomy {
		s.AutonomyBand = append(s.AutonomyBand, a.Band)
	}
	return s
}

func TestReportGenerationBenchmark(t *testing.T) {
	if os.Getenv("RUN_REPORT_BENCHMARK") != "1" {
		t.Skip("set RUN_REPORT_BENCHMARK=1 (and DEEPSEEK_API_KEY) to run the report-generator benchmark")
	}
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		t.Skip("DEEPSEEK_API_KEY not set")
	}

	pool := newLifecycleTestPool(t)
	q := sqlc.New(pool)
	provider := gateway.NewMuxProvider(map[string]gateway.Provider{
		"deepseek": gateway.NewDeepSeekProvider(&http.Client{}),
	})
	resolver := gateway.NewEvalKeyResolver(config.Config{DeepSeekKey: key})
	a := &API{d: Deps{Queries: q, Pool: pool, Provider: provider, EvalResolver: resolver}}
	ctx := context.Background()

	personas := benchmarkPersonas()
	if k, err := strconv.Atoi(os.Getenv("REPORT_BENCHMARK_LIMIT")); err == nil && k > 0 && k < len(personas) {
		personas = personas[:k]
	}
	results := make([]personaResult, 0, len(personas))

	for i, p := range personas {
		pid := seedPersona(t, ctx, q, pool, i, p)

		t0 := time.Now()
		rep, stats, err := a.runReportGeneration(ctx, pid, uuid.NewString(), p.Name)
		total := time.Since(t0)

		res := personaResult{
			Name: p.Name, Qual: p.Qual, Topic: p.Topic, Level: p.Level,
			ProjectID: pid.String(), TotalMillis: total.Milliseconds(),
			Candidates: stats.Candidates, RefsDropped: stats.RefsDropped, RefsTotal: stats.RefsTotal,
			Calls: stats.Calls,
		}
		for _, c := range stats.Calls {
			res.TotalIn += c.InputTokens
			res.TotalOut += c.OutputTokens
		}
		if err != nil {
			res.Err = err.Error()
		} else {
			res.Result = snapshotReport(rep)
		}
		results = append(results, res)
		t.Logf("[%d/%d] %s (L%d) — %dms in=%d out=%d cand=%d dropped=%d/%d depth=%v autonomy=%v risks=%d",
			i+1, len(personas), p.Name, p.Level, res.TotalMillis, res.TotalIn, res.TotalOut,
			res.Candidates, res.RefsDropped, res.RefsTotal, res.Result.DepthLevels, res.Result.AutonomyBand, res.Result.NRisks)
	}

	writeBenchmarkOutput(t, results)
}

func writeBenchmarkOutput(t *testing.T, results []personaResult) {
	out := os.Getenv("REPORT_BENCHMARK_OUT")
	if out == "" {
		out = "/tmp/report-benchmark.json"
	}
	var sumIn, sumOut int
	var sumMs int64
	for _, r := range results {
		sumIn += r.TotalIn
		sumOut += r.TotalOut
		sumMs += r.TotalMillis
	}
	n := max1(len(results))
	agg := map[string]any{
		"personas":          len(results),
		"totalInputTokens":  sumIn,
		"totalOutputTokens": sumOut,
		"avgInputTokens":    sumIn / n,
		"avgOutputTokens":   sumOut / n,
		"avgMillis":         sumMs / int64(n),
		"model":             "deepseek-v4-pro (flagship)",
		"callsPerReport":    4,
	}
	payload := map[string]any{"aggregate": agg, "personas": results}
	buf, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(out, buf, 0o644); err != nil {
		t.Logf("write benchmark output: %v", err)
	}
	t.Logf("BENCHMARK aggregate: personas=%d avgIn=%d avgOut=%d avgMs=%d — full JSON: %s",
		len(results), sumIn/n, sumOut/n, sumMs/int64(n), out)
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// ---- persona seeder ----

func seedPersona(t *testing.T, ctx context.Context, q *sqlc.Queries, pool *pgxpool.Pool, idx int, p personaSpec) uuid.UUID {
	t.Helper()
	proj, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		UserID: SeedUserID, Qualification: p.Qual, Title: p.Topic, BoardCfgVer: 1,
	})
	if err != nil {
		t.Fatalf("seed %s: create project: %v", p.Name, err)
	}
	pid := proj.ID
	store := agent.NewSqlcAgentStore(q, pool)

	if _, err := q.UpsertProjectProposal(ctx, sqlc.UpsertProjectProposalParams{
		ProjectID: pid, Objective: p.Topic,
		Reason:        fmt.Sprintf("我关心「%s」这个问题，想弄清楚证据到底支持什么。", p.Topic),
		Activities:    "前两周读文献并定义变量，中间三周做证据提取与比较，最后两周写作与复盘。",
		Resources:     "学校数据库、Google Scholar、几份一手数据集与两位老师的意见。",
		Counterpoints: "可能的反例：相关不等于因果；样本可能偏；已有研究结论并不一致。",
	}); err != nil {
		t.Fatalf("seed %s: proposal: %v", p.Name, err)
	}

	refIDs := make([]uuid.UUID, 0, p.NRefs)
	for i := 0; i < p.NRefs; i++ {
		cred := []string{"strong", "mixed", "weak"}[i%3]
		ref, rerr := q.CreateReference(ctx, sqlc.CreateReferenceParams{
			ProjectID: pid, Title: fmt.Sprintf("关于%s的研究文献 #%d", short(p.Topic), i+1),
			Classification: "journal", Author: fmt.Sprintf("Author %d", i+1), Credentials: "peer-reviewed",
			Year: "2023", Url: pick(i%4 != 0, fmt.Sprintf("https://example.org/paper-%d-%d", idx, i), ""),
			Tags: []byte("[]"), Credibility: strptr(cred), Evaluation: "CRAAP 溯源：追到一手出处。",
			Decision: strptr([]string{"use", "maybe", "drop"}[i%3]), Pending: false, SearchHints: []byte("[]"),
			Abstract: fmt.Sprintf("这份来源讨论了%s的关键机制，给出可比较的数据。", short(p.Topic)),
			Journal:  "Journal of Evidence",
		})
		if rerr != nil {
			t.Fatalf("seed %s: reference: %v", p.Name, rerr)
		}
		if _, uerr := pool.Exec(ctx, `UPDATE reference SET takeaway=$2, evidence_finding=$3, reading_note=$4, reading_status='done' WHERE id=$1`,
			ref.ID,
			[]byte(fmt.Sprintf(`"这份来源说明了%s的一个侧面；数据可比但样本有限。"`, short(p.Topic))),
			fmt.Sprintf("支持子问题%d，但只在特定情境成立。", i+1),
			"我核对了它的原始数据来源。"); uerr != nil {
			t.Fatalf("seed %s: ref update: %v", p.Name, uerr)
		}
		refIDs = append(refIDs, ref.ID)
	}

	cardIDs := []string{"craap", "concession", "toulmin", "five-whys", "steelman"}
	cardRows := make([]uuid.UUID, 0, p.NCards)
	for i := 0; i < p.NCards && i < len(cardIDs); i++ {
		c, cerr := q.CreateProjectCardInstance(ctx, sqlc.CreateProjectCardInstanceParams{
			ProjectID: pgtype.UUID{Bytes: pid, Valid: true}, CardID: cardIDs[i], Status: "completed",
		})
		if cerr != nil {
			t.Fatalf("seed %s: card: %v", p.Name, cerr)
		}
		cardRows = append(cardRows, c.ID)
		if _, uerr := pool.Exec(ctx, `UPDATE card_instances SET field_values=$2, completed_at=now() WHERE id=$1`,
			c.ID, []byte(fmt.Sprintf(`{"note":"我用%s梳理了论证，发现证据到结论之间需要补一个warrant。"}`, cardIDs[i]))); uerr != nil {
			t.Fatalf("seed %s: card update: %v", p.Name, uerr)
		}
	}

	for i := 0; i < p.NChat; i++ {
		role, content := "assistant", "我先不给答案。你觉得这份证据能支持到哪一步？还有哪个反例没排除？"
		if i%2 == 0 {
			role, content = "user", studentPrompt(p, i)
		}
		if err := store.CreateChatMessage(ctx, pid, role, content); err != nil {
			t.Fatalf("seed %s: chat: %v", p.Name, err)
		}
	}

	appendEv := func(typ string, payload map[string]any) {
		raw, _ := json.Marshal(payload)
		_ = store.AppendEvent(ctx, agent.EventRow{ProjectID: pid, Surface: "studio", Type: typ, Payload: raw})
	}
	appendEv("milestone:framework_finished", map[string]any{})
	for i := 0; i < p.NRefs; i++ {
		appendEv("source_added", map[string]any{"i": i})
	}
	appendEv("reading_focus", map[string]any{"student_text": "这段的因果推断有点跳，我想确认它排除了混杂因素没有。", "spans": []any{}})
	appendEv("reading_focus", map[string]any{"student_text": "这份来源的样本只有一个城市，能不能外推？", "spans": []any{}})
	for range cardRows {
		appendEv("card_completed", map[string]any{})
	}
	appendEv("annotation_opened", map[string]any{"annotation_id": "a1", "doc": "essay"})
	appendEv("annotation_opened", map[string]any{"annotation_id": "a2", "doc": "essay"})
	if p.Reflect {
		appendEv("reflection_written", map[string]any{})
	}
	appendEv("project_finished", map[string]any{})

	body := draftFor(p)
	if _, err := q.InsertDraftSnapshot(ctx, sqlc.InsertDraftSnapshotParams{
		ProjectID: pid, DocKind: "essay", Seq: 1, Content: body, SpanIndex: []byte("[]"),
	}); err != nil {
		t.Fatalf("seed %s: snapshot: %v", p.Name, err)
	}
	_ = q.SetWritingFinish(ctx, sqlc.SetWritingFinishParams{ProjectID: pid, DocKind: "proposal"})
	_ = q.SetWritingFinish(ctx, sqlc.SetWritingFinishParams{ProjectID: pid, DocKind: "essay"})

	for i := 0; i < len(refIDs) && i < 3; i++ {
		_, _ = q.CreateCitation(ctx, sqlc.CreateCitationParams{ProjectID: pid, ReferenceID: refIDs[i], Section: fmt.Sprintf("claim:%d", i+1)})
	}
	for i := 0; i < 4; i++ {
		anchor, _ := json.Marshal(map[string]string{"level": "paragraph", "nature": "suggest", "quote": "这一段的结论", "locator": fmt.Sprintf("第%d段", i+1)})
		_, _ = q.InsertIntervention(ctx, sqlc.InsertInterventionParams{
			ProjectID: pid, Type: "essay_annotation", Anchor: anchor,
			Body: "这里的证据支持较弱，建议补一个反例或缩小结论范围。",
		})
	}

	return pid
}

func studentPrompt(p personaSpec, i int) string {
	switch p.Level {
	case 1:
		opts := []string{"直接帮我写一段引言吧。", "这个问题的答案是什么？", "帮我总结这篇文章的结论。", "我该写什么论点？你给我三个。"}
		return opts[(i/2)%len(opts)]
	case 2:
		opts := []string{"这个来源可信吗？我该怎么判断？", "我的论点是X，你觉得有什么漏洞？", "帮我看看这段论证的逻辑，但别替我下结论。", "还有哪个反例我没考虑到？"}
		return opts[(i/2)%len(opts)]
	default:
		opts := []string{
			"请不要替我写正文，只帮我挑论证里的问题。", "请你当反方，尽力反驳我的gap。",
			"我先自己改一版，再请你从来源适用范围上挑刺。", "证据不支持我原来的强结论，我把它收窄成有限判断，你看这样是否更稳？",
			"这是我的判断和它的局限，请你只检查warrant是否成立。",
		}
		return opts[(i/2)%len(opts)]
	}
}

func draftFor(p personaSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Topic)
	fmt.Fprintf(&b, "本文探讨「%s」。已有研究结论并不一致，本文尝试把这个宽泛问题收窄成一个可回答的关系，并在有限情境下给出判断。\n\n", p.Topic)
	for i := 0; i < p.Paras; i++ {
		para := fmt.Sprintf("在第%d部分，我先陈述一个claim，再用一手数据作为evidence，并说明推理链条。数据显示了一个趋势，但我注意到样本与测量上的局限，因此不把结果直接当作结论。为处理反方观点，我引入一个让步段：即便存在相反证据X，其适用范围也受限于特定情境，因此不足以推翻本文的有限判断。", i+1)
		if p.Level >= 3 {
			para += "我进一步识别了证据与结论之间缺失的warrant，并据此把结论收窄，明确指出不可回答之处。"
		}
		b.WriteString(para + "\n\n")
	}
	if p.Reflect {
		b.WriteString("## 反思\n\n回头看，我最初把相关当成了因果。随着证据积累，我修正了这个倾向，也更清楚自己的判断是怎么来的、它的边界在哪里。这是我的判断，不是AI替我下的。\n")
	}
	return b.String()
}

func short(s string) string {
	r := []rune(s)
	if len(r) > 10 {
		return string(r[:10])
	}
	return s
}
func strptr(s string) *string { return &s }
func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
