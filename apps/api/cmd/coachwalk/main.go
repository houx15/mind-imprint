// coachwalk —— 多轮陪练走查台。
//
// routebench 回答「这个模型这一轮答得好不好」。这件工具回答另一个问题：
// **换了模型，陪练连起来还能用吗，以及每一轮她要等多久。** 它让另一家的模型演
// 学生，和真的 prompt / 真的解析器来回若干轮（重试照生产的样子做），然后检查
// 状态和语言观察项，量每一轮陪练的延迟，最后请一个旗舰判官读整条对话。
//
// 走查四个陪练，每个有自己的 Driver（住在各自的包里，因为 prompt 和解析器都是
// 未导出的）：阅读室、写作室、pro、PBL。
//
// 和 routebench 一样：它是一件**与运行系统分开**的工具。不连数据库，不被
// cmd/api 引用，永远不在请求路径上。改 models.json 的是人，不是它。
//
//	cd apps/api
//	DASHSCOPE_API_KEY=… DEEPSEEK_API_KEY=… go run ./cmd/coachwalk \
//	  -models dashscope/deepseek-v4-pro,deepseek/deepseek-flash -turns 8 -out walk.md
//
//	# 只走某几个陪练：
//	go run ./cmd/coachwalk -coaches reading,writing
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/api"
	"mindimprint/api/internal/coachwalk"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/pbl"
)

// coach 是一个可走查的陪练：怎么新建它的 driver，以及用哪张判官标准。
type coach struct {
	name, suite, id string
	version         int
	judge           string
	make            func() coachwalk.Driver
	script          []string
}

// coaches 按「线上有多重要」排序：阅读室和写作室是 lite 的两个活面，
// pro 和 PBL 在后面。
var coaches = allCoaches()

func allCoaches() []coach {
	out := []coach{
		{name: "writing", judge: agent.WritingWalkJudge, make: func() coachwalk.Driver { return agent.NewWritingWalkDriver() }},
		{name: "pro", judge: agent.ProWalkJudge, make: func() coachwalk.Driver { return agent.NewProWalkDriver() }},
		{name: "pbl", judge: pbl.WalkJudge, make: func() coachwalk.Driver { return pbl.NewWalkDriver() }},
	}
	for _, s := range api.ReadingWalkScenarios() {
		name := "reading"
		if s.ID != "lite-reading-coach/hint-ladder" {
			name = "reading-" + strings.TrimPrefix(s.ID, "lite-reading-coach/")
		}
		out = append(out, coach{name: name, suite: s.Suite, id: s.ID, version: s.Version, judge: s.Judge, make: s.Make, script: s.Script})
	}
	// 长文走查：同一篇库里的真文章跑两遍，只差「导读里有没有 parts」——
	// 也就是渐进披露开没开。原来那条阅读走查的文章只有 286 字，线上一轮平均
	// 11,035 个输入 token，凡是跟文章大小有关的事在那条上都测不出来。
	for _, s := range api.LongReadWalkScenarios() {
		out = append(out, coach{
			name:  "reading-" + strings.TrimPrefix(s.ID, "lite-reading-coach/"),
			suite: s.Suite, id: s.ID, version: s.Version,
			judge: s.Judge, make: s.Make, script: s.Script,
		})
	}
	return out
}

var hardKinds = []string{"run-error", "parse", "advanced-too-early", "answer-unprompted", "banned-phrasing", "expected-terminal"}
var observationKinds = []string{"question-marks", "question+hook", "repeat", "ungrounded"}

// row 是一条走查的结果。
type row struct {
	coach, suite, id, model string
	version                 int
	rep                     int
	expected                int
	log                     *coachwalk.Log
	err                     error
	score                   float64
	why                     string
}

func main() {
	models := flag.String("models", "dashscope/deepseek-v4-pro,deepseek/deepseek-flash",
		"逗号分隔的候选，按这个顺序跑；第一个是在位的那个")
	studentModel := flag.String("student", "dashscope/qwen3.8-max",
		"演学生的模型。必须和候选不同家，否则是同一个脑子自问自答")
	judgeModel := flag.String("judge", "dashscope/qwen3.7-max", "判官，必须是旗舰且不在候选里")
	coachFilter := flag.String("coaches", "", "只走这几个陪练（逗号分隔），留空跑全部")
	suite := flag.String("suite", "", "只运行指定评测套件")
	scripted := flag.Bool("scripted", false, "对带脚本的 scenario 使用确定性学生")
	bound := flag.Bool("bound", false, "只测试当前 dialogue 绑定")
	turns := flag.Int("turns", 8, "每条走查跑几轮")
	repeats := flag.Int("repeats", 1, "每个组合跑几条走查。学生每次都会走岔，1 条噪声很大")
	out := flag.String("out", "", "把报告写到这个文件，留空则只打到 stdout")
	jsonOut := flag.String("json-out", "", "把机器可读 artifact 写到这个文件")
	compare := flag.String("compare", "", "与一次较早的 artifact 比较")
	check := flag.Bool("check", false, "调用或最终解析失败时以非零退出")
	list := flag.Bool("list", false, "只列出要跑什么，一个字也不花")
	flag.Parse()

	picked := pick(*coachFilter)
	if *suite != "" {
		picked = pickSuite(*suite)
		*scripted = true
	}
	cands := split(*models)

	if len(cands) == 0 {
		die("至少要给一个候选")
	}
	for _, m := range cands {
		if m == *studentModel && !*scripted {
			die("%s 既是候选又在演学生 —— 同一个脑子自问自答，不是测量", m)
		}
		if m == *judgeModel {
			die("%s 既是候选又是判官 —— 自己给自己判卷，不是测量", m)
		}
	}

	cat, err := gateway.DefaultCatalog()
	if err != nil {
		die("目录加载失败: %v", err)
	}
	if *bound {
		boundModel := cat.Lanes[gateway.ClassDialogue].Model
		if boundModel == "" {
			die("dialogue 没有当前绑定")
		}
		cands = []string{boundModel}
	}
	if *list {
		if *suite != "" {
			fmt.Printf("suite：%s\n", *suite)
		}
		fmt.Printf("陪练：%s\n", strings.Join(names(picked), ", "))
		fmt.Printf("候选：%s\n", strings.Join(cands, ", "))
		studentLabel := *studentModel
		if *scripted {
			studentLabel = "deterministic-script"
		}
		fmt.Printf("学生：%s\n判官：%s\n", studentLabel, *judgeModel)
		coachCalls := 0
		for _, c := range picked {
			turnN := *turns
			multiplier := 2
			if *scripted && len(c.script) > 0 {
				turnN, multiplier = len(c.script), 1
			}
			coachCalls += turnN * multiplier
		}
		fmt.Printf("调用量：%d 候选 × %d 条 × %d 重复 ≈ %d 次（不含重试与判官）\n", len(cands), len(picked), *repeats, len(cands)*(*repeats)*coachCalls)
		return
	}
	// 一个 client 走完全程，超时给得宽：旗舰模型在一条长对话上答几分钟是正常的，
	// 而客户端超时会被记成「模型失败」——那是报告会一路带下去的一句假话。
	httpc := &http.Client{Timeout: 10 * time.Minute}
	prov := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(httpc),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(httpc),
	})
	ctx := context.Background()

	// 学生走 dialogue 档：关思考。她只是说一句学生会说的话，不需要推理预算。
	var student gateway.Resolved
	if !*scripted {
		student, err = cat.Resolve(gateway.ClassDialogue, *studentModel, gateway.OSEnvKeyLookup)
		if err != nil {
			die("学生模型: %v", err)
		}
	}
	// 判官走 assess 档：旗舰、满额推理。判卷是这里唯一不能省的一件事。
	judge, err := cat.Resolve(gateway.ClassAssess, *judgeModel, gateway.OSEnvKeyLookup)
	if err != nil {
		die("判官模型: %v", err)
	}

	var rows []row
	for _, c := range picked {
		for _, m := range cands {
			// 候选走 dialogue 档，拿到的就是这一档在生产里真正的约束（关思考）。
			coachR, rerr := cat.Resolve(gateway.ClassDialogue, m, gateway.OSEnvKeyLookup)
			if rerr != nil {
				die("候选 %s: %v", m, rerr)
			}
			for rep := 1; rep <= *repeats; rep++ {
				fmt.Fprintf(os.Stderr, "%-8s %-30s 第 %d 条…", c.name, m, rep)
				var lg *coachwalk.Log
				var werr error
				if *scripted && len(c.script) > 0 {
					lg, werr = coachwalk.RunScripted(ctx, c.make(), callFn(prov, coachR), c.script)
				} else {
					lg, werr = coachwalk.Run(ctx, c.make(), callFn(prov, coachR), callFn(prov, student), *turns)
				}
				expected := *turns
				if *scripted && len(c.script) > 0 {
					expected = len(c.script)
				}
				r := row{coach: c.name, suite: c.suite, id: c.id, version: c.version, model: m, rep: rep, expected: expected, log: lg, err: werr}
				if lg != nil && len(lg.Turns) > 0 {
					r.score, r.why = judgeWalk(ctx, prov, judge, c.judge, lg.Transcript())
				}
				rows = append(rows, r)
				lat := coachwalk.LatencyOf([]*coachwalk.Log{lg})
				violationCount, observationCount, turnCount := 0, 0, 0
				if lg != nil {
					violationCount = len(lg.Violations())
					observationCount = len(lg.Observations())
					turnCount = len(lg.Turns)
				}
				if werr != nil {
					violationCount++
				}
				fmt.Fprintf(os.Stderr, " %d 轮，%d 个状态发现、%d 条观察，每轮 p50 %.1fs，判官 %.0f\n",
					turnCount, violationCount, observationCount, float64(lat.P50)/1000, r.score)
				if werr != nil {
					fmt.Fprintf(os.Stderr, "  🚨 %v\n", werr)
				}
			}
		}
	}

	studentLabel := *studentModel
	if *scripted {
		studentLabel = "deterministic-script"
	}
	scenarioHashes, err := hashCoaches(picked)
	if err != nil {
		die("计算 scenario hash: %v", err)
	}
	artifact := makeWalkArtifact(rows, *suite, *judgeModel, *turns, *repeats, scenarioHashes)
	md := report(rows, *turns, *repeats, studentLabel, *judgeModel, picked, cands)
	if *compare != "" {
		base, err := readWalkArtifact(*compare)
		if err != nil {
			die("读改前结果: %v", err)
		}
		md += compareWalkArtifacts(base, artifact)
	}
	fmt.Print(md)
	if *out != "" {
		if werr := os.WriteFile(*out, []byte(md), 0o644); werr != nil {
			die("写报告失败: %v", werr)
		}
		fmt.Fprintf(os.Stderr, "\n写到 %s\n", *out)
	}
	if *jsonOut != "" {
		if err := writeWalkArtifact(*jsonOut, artifact); err != nil {
			die("写 artifact: %v", err)
		}
	}
	if failures := walkTechnicalFailures(artifact); len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "coachwalk technical failures:\n- "+strings.Join(failures, "\n- "))
		if *check {
			os.Exit(1)
		}
	}
}

func pickSuite(suite string) []coach {
	var out []coach
	for _, c := range coaches {
		if c.suite == suite {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		die("没有叫 %q 的评测套件", suite)
	}
	return out
}

func makeWalkArtifact(rows []row, suite, judge string, turns, repeats int, scenarioHashes map[string]string) walkArtifact {
	a := walkArtifact{Schema: 3, Suite: suite, CapturedAt: time.Now(), JudgeModel: judge, Turns: turns, Repeats: repeats, ScenarioHashes: scenarioHashes}
	for _, r := range rows {
		lat := coachwalk.LatencyOf([]*coachwalk.Log{r.log})
		usage := coachwalk.UsageOf([]*coachwalk.Log{r.log})
		v := map[string]int{}
		observations := map[string]int{}
		if r.log != nil {
			for _, x := range r.log.Violations() {
				v[x.Kind]++
			}
			for _, x := range r.log.Observations() {
				observations[x.Kind]++
			}
		}
		id := r.id
		if id == "" {
			id = r.coach
		}
		rowOut := walkArtifactRow{ID: id, Version: r.version, Model: r.model, Run: r.rep, Score: r.score, JudgeWhy: r.why,
			P50MS: lat.P50, P50InputTokens: usage.InputP50, P50OutputTokens: usage.OutputP50, P50ReasoningTokens: usage.ReasoningP50,
			Retries: lat.Retries, Violations: v, Observations: observations}
		if r.err != nil {
			rowOut.Error = r.err.Error()
			rowOut.Violations["run-error"]++
			rowOut.ViolationDetails = append(rowOut.ViolationDetails, walkArtifactViolation{Kind: "run-error", Note: r.err.Error()})
		}
		rowOut.Completed = r.err == nil && r.log != nil && len(r.log.Turns) == r.expected && r.log.Count("parse") == 0 && r.log.Count("expected-terminal") == 0
		if r.log != nil {
			for _, violation := range r.log.Violations() {
				rowOut.ViolationDetails = append(rowOut.ViolationDetails, walkArtifactViolation{
					Turn: violation.Turn, Kind: violation.Kind, Note: violation.Note,
				})
			}
			for _, observation := range r.log.Observations() {
				rowOut.ObservationDetails = append(rowOut.ObservationDetails, walkArtifactViolation{
					Turn: observation.Turn, Kind: observation.Kind, Note: observation.Note,
				})
			}
			for _, t := range r.log.Turns {
				turn := walkArtifactTurn{Number: t.N, Reply: t.Reply, StudentSaid: t.StudentSaid, ParseError: t.ParseErr,
					SelectedAttempt: t.SelectedAttempt, RecoveryReason: t.RecoveryReason, InputTokens: t.InTokens,
					OutputTokens: t.OutTokens, ReasoningTokens: t.ReasoningTokens, LatencyMS: t.CoachMs}
				for _, at := range t.Attempts {
					turn.Attempts = append(turn.Attempts, walkArtifactAttempt{Raw: at.Raw, Error: at.Error,
						InputTokens: at.InTokens, OutputTokens: at.OutTokens, ReasoningTokens: at.ReasoningTokens, LatencyMS: at.Ms})
				}
				rowOut.Turns = append(rowOut.Turns, turn)
			}
		}
		a.Rows = append(a.Rows, rowOut)
	}
	return a
}

// report 渲染整份报告。
func report(rows []row, turns, repeats int, studentModel, judgeModel string,
	picked []coach, cands []string) string {

	var b strings.Builder
	plannedTurns := turns
	if len(rows) > 0 && rows[0].expected > 0 {
		plannedTurns = rows[0].expected
		for _, r := range rows[1:] {
			if r.expected != plannedTurns {
				plannedTurns = 0
				break
			}
		}
	}
	fmt.Fprintf(&b, "# coachwalk · 多轮陪练走查\n\n")
	if plannedTurns > 0 {
		fmt.Fprintf(&b, "%s · 每条 %d 轮 × %d 条 · 学生 `%s` · 判官 `%s`\n\n",
			time.Now().Format("2006-01-02 15:04"), plannedTurns, repeats, studentModel, judgeModel)
	} else {
		fmt.Fprintf(&b, "%s · 混合轮数 × %d 条 · 学生 `%s` · 判官 `%s`\n\n",
			time.Now().Format("2006-01-02 15:04"), repeats, studentModel, judgeModel)
	}
	fmt.Fprintf(&b, "> 仅调用和最终解析故障使命令失败；终态、教学判断和判官分数供人工审阅。\n")
	fmt.Fprintf(&b, "> `question-marks`、`repeat`、`ungrounded` 等文本信号只帮助定位样本。\n\n")

	byCell := func(c, m string) []row {
		var out []row
		for _, r := range rows {
			if r.coach == c && r.model == m && r.log != nil {
				out = append(out, r)
			}
		}
		return out
	}

	fmt.Fprintf(&b, "## 状态与质量观察（供人工审阅）\n\n")
	fmt.Fprintf(&b, "| 陪练 | 模型 | 状态发现 | 最差一条 | 判官（每条） | %s |\n", strings.Join(hardKinds, " | "))
	fmt.Fprintf(&b, "|---|---|---:|---:|---|%s\n", strings.Repeat("---:|", len(hardKinds)))
	for _, c := range picked {
		for _, m := range cands {
			cell := byCell(c.name, m)
			if len(cell) == 0 {
				continue
			}
			var total, worst int
			var scores []string
			perKind := map[string]int{}
			for _, r := range cell {
				v := len(r.log.Violations())
				if r.err != nil {
					v++
					perKind["run-error"]++
				}
				total += v
				if v > worst {
					worst = v
				}
				for _, k := range hardKinds {
					perKind[k] += r.log.Count(k)
				}
				scores = append(scores, fmt.Sprintf("%.0f", r.score))
			}
			cells := make([]string, 0, len(hardKinds))
			for _, k := range hardKinds {
				cells = append(cells, strconv.Itoa(perKind[k]))
			}
			fmt.Fprintf(&b, "| %s | `%s` | **%d** | %d | %s | %s |\n",
				c.name, m, total, worst, strings.Join(scores, ", "), strings.Join(cells, " | "))
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## 观察项（不阻断）\n\n")
	fmt.Fprintf(&b, "| 陪练 | 模型 | 合计 | %s |\n", strings.Join(observationKinds, " | "))
	fmt.Fprintf(&b, "|---|---|---:|%s\n", strings.Repeat("---:|", len(observationKinds)))
	for _, c := range picked {
		for _, m := range cands {
			cell := byCell(c.name, m)
			if len(cell) == 0 {
				continue
			}
			perKind, total := map[string]int{}, 0
			for _, r := range cell {
				for _, k := range observationKinds {
					n := r.log.CountObservation(k)
					perKind[k] += n
					total += n
				}
			}
			cells := make([]string, 0, len(observationKinds))
			for _, k := range observationKinds {
				cells = append(cells, strconv.Itoa(perKind[k]))
			}
			fmt.Fprintf(&b, "| %s | `%s` | %d | %s |\n", c.name, m, total, strings.Join(cells, " | "))
		}
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "## 速度与用量（每轮，含生产会做的重试）\n\n")
	fmt.Fprintf(&b, "| 陪练 | 模型 | 轮数 | 延迟 p50 | p90 | 最慢 | 入 tokens p50 | 出 tokens p50 | 推理 tokens p50 | 重试次数 |\n")
	fmt.Fprintf(&b, "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, c := range picked {
		for _, m := range cands {
			cell := byCell(c.name, m)
			if len(cell) == 0 {
				continue
			}
			logs := make([]*coachwalk.Log, 0, len(cell))
			for _, r := range cell {
				logs = append(logs, r.log)
			}
			lat := coachwalk.LatencyOf(logs)
			usage := coachwalk.UsageOf(logs)
			fmt.Fprintf(&b, "| %s | `%s` | %d | %.1fs | %.1fs | %.1fs | %d | %d | %d | %d |\n",
				c.name, m, lat.N, float64(lat.P50)/1000, float64(lat.P90)/1000, float64(lat.Max)/1000,
				usage.InputP50, usage.OutputP50, usage.ReasoningP50, lat.Retries)
		}
	}
	b.WriteString("\n")

	for _, c := range picked {
		fmt.Fprintf(&b, "## %s\n\n", c.name)
		for _, m := range cands {
			for _, r := range byCell(c.name, m) {
				fmt.Fprintf(&b, "### `%s` · 第 %d 条 —— %s\n\n", m, r.rep, r.log.Site)
				if r.err != nil {
					fmt.Fprintf(&b, "🚨 走查中断：%v\n\n", r.err)
				}
				fmt.Fprintf(&b, "判官 %.0f 分 —— %s\n\n", r.score, r.why)
				vs := r.log.Violations()
				if len(vs) == 0 {
					b.WriteString("状态发现：无。\n\n")
				} else {
					b.WriteString("状态发现：\n\n")
					sort.SliceStable(vs, func(i, j int) bool { return vs[i].Turn < vs[j].Turn })
					for _, v := range vs {
						fmt.Fprintf(&b, "- 第 %d 轮 · `%s` · %s\n", v.Turn, v.Kind, v.Note)
					}
					b.WriteString("\n")
				}
				observations := r.log.Observations()
				if len(observations) > 0 {
					b.WriteString("观察项（不阻断）：\n\n")
					sort.SliceStable(observations, func(i, j int) bool { return observations[i].Turn < observations[j].Turn })
					for _, observation := range observations {
						fmt.Fprintf(&b, "- 第 %d 轮 · `%s` · %s\n", observation.Turn, observation.Kind, observation.Note)
					}
					b.WriteString("\n")
				}
				fmt.Fprintf(&b, "<details><summary>完整对话</summary>\n\n```\n%s```\n\n</details>\n\n",
					r.log.Transcript())
			}
		}
	}
	return b.String()
}

// callFn 把一个 provider + 一个已解析的模型包成 coachwalk.Call，并量这一次的耗时。
func callFn(prov gateway.Provider, r gateway.Resolved) coachwalk.Call {
	return func(ctx context.Context, req gateway.ChatRequest) (coachwalk.CallResult, error) {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		start := time.Now()
		res, err := gateway.Collect(cctx, prov, r, req)
		ms := time.Since(start).Milliseconds()
		if err != nil {
			return coachwalk.CallResult{Ms: ms}, err
		}
		reasoning := 0
		if res.Usage.ReasoningTokens != nil {
			reasoning = *res.Usage.ReasoningTokens
		}
		return coachwalk.CallResult{Text: res.Text, Ms: ms, In: res.Usage.InputTokens, Out: res.Usage.OutputTokens, Reason: reasoning}, nil
	}
}

func judgeWalk(ctx context.Context, prov gateway.Provider, jr gateway.Resolved, rubric, transcript string) (float64, string) {
	var last string
	for attempt := 1; attempt <= 2; attempt++ {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		res, err := gateway.Collect(cctx, prov, jr, gateway.ChatRequest{
			MaxTokens: 4000,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: walkJudgeSystem},
				{Role: gateway.RoleUser, Content: rubric + "\n\n【对话】\n" + transcript},
			},
		})
		cancel()
		if err != nil {
			last = "调用失败: " + err.Error()
			continue
		}
		score, why, jerr := parseWalkJudgeReply(res.Text)
		if jerr == nil {
			return score, why
		}
		last = jerr.Error()
	}
	return 0, "判官重试 2 次后仍失败：" + last
}

const walkJudgeSystem = `你是教育产品质量判官。严格按用户提供的场景标准给完整对话打分。
判断“多问”时看学生是否被要求完成多个独立回答任务，不按问号数量机械计数；同一问题的选项重述、依赖前一判断的条件或理由追问，以及 AI 在解释中自己继续处理的修辞问题，都不算新增任务。
只输出一个合法 JSON 对象，字段名必须恰好为 score 和 why，不得使用 reasoning、reason、analysis 等替代字段，不得输出 Markdown 或其他文字：
{"score": <1-5 的整数>, "why": "<一句具体理由>"}`

func parseWalkJudgeReply(raw string) (float64, string, error) {
	body := strings.TrimSpace(raw)
	if i := strings.Index(body, "{"); i >= 0 {
		if j := strings.LastIndex(body, "}"); j > i {
			body = body[i : j+1]
		}
	}
	var reply struct {
		Score float64 `json:"score"`
		Why   string  `json:"why"`
	}
	if jerr := json.Unmarshal([]byte(body), &reply); jerr != nil {
		return 0, "", fmt.Errorf("回复无法解析：%s", coachwalk.TruncateRunes(body, 120))
	}
	if reply.Score < 1 || reply.Score > 5 || reply.Score != float64(int(reply.Score)) {
		return 0, "", fmt.Errorf("分数不在 1–5 的整数范围内：%v", reply.Score)
	}
	if strings.TrimSpace(reply.Why) == "" {
		return 0, "", fmt.Errorf("why 为空")
	}
	return reply.Score, strings.TrimSpace(reply.Why), nil
}

func pick(filter string) []coach {
	want := split(filter)
	if len(want) == 0 {
		return coaches
	}
	var out []coach
	for _, w := range want {
		found := false
		for _, c := range coaches {
			if c.name == w {
				out = append(out, c)
				found = true
			}
		}
		if !found {
			die("没有叫 %q 的陪练（有：%s）", w, strings.Join(names(coaches), ", "))
		}
	}
	return out
}

func names(cs []coach) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.name)
	}
	return out
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "coachwalk: "+format+"\n", args...)
	os.Exit(1)
}
