// coachwalk —— 多轮陪练走查台。
//
// routebench 回答「这个模型这一轮答得好不好」。这件工具回答另一个问题：
// **换了模型，陪练连起来还能用吗。** 它让另一家的模型演学生，和真的 prompt /
// 真的解析器来回若干轮，然后数可验的犯规，最后请一个旗舰判官读整条对话。
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
//	# 只走一个陪练（名字见 -list）：
//	go run ./cmd/coachwalk -coaches reading,writing
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
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
	name  string
	judge string
	make  func() coachwalk.Driver
}

// coaches 按「线上有多重要」排序：阅读室和写作室是 lite 的两个活面，
// pro 和 PBL 在后面。
var coaches = []coach{
	{"reading", api.ReadingWalkJudge, func() coachwalk.Driver { return api.NewReadingWalkDriver() }},
	{"writing", agent.WritingWalkJudge, func() coachwalk.Driver { return agent.NewWritingWalkDriver() }},
	{"pro", agent.ProWalkJudge, func() coachwalk.Driver { return agent.NewProWalkDriver() }},
	{"pbl", pbl.WalkJudge, func() coachwalk.Driver { return pbl.NewWalkDriver() }},
}

// kinds 是总表里要分列的犯规类型。
var kinds = []string{"parse", "advanced-too-early", "banned-phrasing", "multi-question", "question+hook", "repeat", "ungrounded"}

func main() {
	models := flag.String("models", "dashscope/deepseek-v4-pro,deepseek/deepseek-flash",
		"逗号分隔的候选，按这个顺序跑；第一个是在位的那个")
	studentModel := flag.String("student", "dashscope/qwen3.8-max",
		"演学生的模型。必须和候选不同家，否则是同一个脑子自问自答")
	judgeModel := flag.String("judge", "dashscope/qwen3.7-max", "判官，必须是旗舰且不在候选里")
	coachFilter := flag.String("coaches", "", "只走这几个陪练（逗号分隔），留空跑全部")
	turns := flag.Int("turns", 8, "每条走查跑几轮")
	repeats := flag.Int("repeats", 1, "每个组合跑几条走查。学生每次都会走岔，1 条噪声很大")
	out := flag.String("out", "", "把报告写到这个文件，留空则只打到 stdout")
	list := flag.Bool("list", false, "只列出要跑什么，一个字也不花")
	flag.Parse()

	picked := pick(*coachFilter)
	cands := split(*models)

	if *list {
		fmt.Printf("陪练：%s\n", strings.Join(names(picked), ", "))
		fmt.Printf("候选：%s\n", strings.Join(cands, ", "))
		fmt.Printf("学生：%s\n判官：%s\n", *studentModel, *judgeModel)
		fmt.Printf("调用量：%d 陪练 × %d 候选 × %d 条 × %d 轮 × 2（陪练+学生）= %d 次\n",
			len(picked), len(cands), *repeats, *turns,
			len(picked)*len(cands)*(*repeats)*(*turns)*2)
		return
	}

	if len(cands) == 0 {
		die("至少要给一个候选")
	}
	for _, m := range cands {
		if m == *studentModel {
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
	// 一个 client 走完全程，超时给得宽：旗舰模型在一条长对话上答几分钟是正常的，
	// 而客户端超时会被记成「模型失败」——那是报告会一路带下去的一句假话。
	httpc := &http.Client{Timeout: 10 * time.Minute}
	prov := gateway.NewMuxProvider(map[string]gateway.Provider{
		gateway.KindOpenAICompatible: gateway.NewCatalogProvider(httpc),
		gateway.KindAnthropic:        gateway.NewAnthropicProvider(httpc),
	})
	ctx := context.Background()

	// 学生走 dialogue 档：关思考。她只是说一句学生会说的话，不需要推理预算。
	student, err := cat.Resolve(gateway.ClassDialogue, *studentModel, gateway.OSEnvKeyLookup)
	if err != nil {
		die("学生模型: %v", err)
	}
	// 判官走 assess 档：旗舰、满额推理。判卷是这里唯一不能省的一件事。
	judge, err := cat.Resolve(gateway.ClassAssess, *judgeModel, gateway.OSEnvKeyLookup)
	if err != nil {
		die("判官模型: %v", err)
	}

	type run struct {
		coach, model string
		rep          int
		log          *coachwalk.Log
		err          error
		ms           int64
		score        float64
		why          string
	}
	var runs []run

	for _, c := range picked {
		for _, m := range cands {
			coachR, rerr := cat.Resolve(gateway.ClassDialogue, m, gateway.OSEnvKeyLookup)
			if rerr != nil {
				die("候选 %s: %v", m, rerr)
			}
			for rep := 1; rep <= *repeats; rep++ {
				fmt.Fprintf(os.Stderr, "%-8s %-30s 第 %d 条…", c.name, m, rep)
				start := time.Now()
				lg, werr := coachwalk.Run(ctx, c.make(), callFn(prov, coachR), callFn(prov, student), *turns)
				r := run{coach: c.name, model: m, rep: rep, log: lg, err: werr, ms: time.Since(start).Milliseconds()}
				if lg != nil && len(lg.Turns) > 0 {
					r.score, r.why = judgeWalk(ctx, prov, judge, c.judge, lg.Transcript())
				}
				runs = append(runs, r)
				fmt.Fprintf(os.Stderr, " %d 轮，%d 处犯规，判官 %.0f\n",
					len(lg.Turns), len(lg.Violations()), r.score)
			}
		}
	}

	rows := make([]row, 0, len(runs))
	for _, r := range runs {
		rows = append(rows, row{r.coach, r.model, r.rep, r.log, r.score, r.why, r.ms})
	}
	md := report(rows, *turns, *repeats, *studentModel, *judgeModel, picked, cands)
	fmt.Print(md)
	if *out != "" {
		if werr := os.WriteFile(*out, []byte(md), 0o644); werr != nil {
			die("写报告失败: %v", werr)
		}
		fmt.Fprintf(os.Stderr, "\n写到 %s\n", *out)
	}
}

// row 是报告要用到的那几列。
type row struct {
	coach, model string
	rep          int
	log          *coachwalk.Log
	score        float64
	why          string
	ms           int64
}

// report 渲染整份报告。
func report(runs []row, turns, repeats int, studentModel, judgeModel string,
	picked []coach, cands []string) string {

	var b strings.Builder
	fmt.Fprintf(&b, "# coachwalk · 多轮陪练走查\n\n")
	fmt.Fprintf(&b, "%s · 每条 %d 轮 × %d 条 · 学生 `%s` · 判官 `%s`\n\n",
		time.Now().Format("2006-01-02 15:04"), turns, repeats, studentModel, judgeModel)
	fmt.Fprintf(&b, "> **犯规那几列是数出来的，不是判官的印象。** `parse` = 生产解析器拒收（学生看到一个错）；\n")
	fmt.Fprintf(&b, "> `advanced-too-early` = 她还没答上来就被放过去了；`banned-phrasing` = 生产校验器判定\n")
	fmt.Fprintf(&b, "> 这条回复替她写了；`multi-question` = 一轮两个问号；`repeat` = 和前面某轮高度重合；\n")
	fmt.Fprintf(&b, "> `ungrounded` = 接不住她上一句的任何一段原文。\n\n")

	// 总表：每个陪练 × 每个候选一行，多条走查取合计与最差。
	fmt.Fprintf(&b, "## 总表\n\n")
	fmt.Fprintf(&b, "| 陪练 | 模型 | 犯规合计 | 最差一条 | 判官（每条） | %s | p50 |\n",
		strings.Join(kinds, " | "))
	fmt.Fprintf(&b, "|---|---|---:|---:|---|%s|---:|\n", strings.Repeat("---:|", len(kinds)))

	for _, c := range picked {
		for _, m := range cands {
			var total, worst int
			var scores []string
			var perKind = map[string]int{}
			var msTotal int64
			var n int
			for _, r := range runs {
				cn, mn, lg, sc, ms := r.coach, r.model, r.log, r.score, r.ms
				if cn != c.name || mn != m || lg == nil {
					continue
				}
				v := len(lg.Violations())
				total += v
				if v > worst {
					worst = v
				}
				for _, k := range kinds {
					perKind[k] += lg.Count(k)
				}
				scores = append(scores, fmt.Sprintf("%.0f", sc))
				msTotal += ms
				n++
			}
			if n == 0 {
				continue
			}
			cells := make([]string, 0, len(kinds))
			for _, k := range kinds {
				cells = append(cells, fmt.Sprintf("%d", perKind[k]))
			}
			fmt.Fprintf(&b, "| %s | `%s` | **%d** | %d | %s | %s | %.1fs |\n",
				c.name, m, total, worst, strings.Join(scores, ", "),
				strings.Join(cells, " | "), float64(msTotal)/float64(n)/1000)
		}
	}
	b.WriteString("\n")

	// 逐条明细。
	for _, c := range picked {
		fmt.Fprintf(&b, "## %s\n\n", c.name)
		for _, m := range cands {
			for _, r := range runs {
				cn, mn, rep, lg, sc, why := r.coach, r.model, r.rep, r.log, r.score, r.why
				if cn != c.name || mn != m || lg == nil {
					continue
				}
				fmt.Fprintf(&b, "### `%s` · 第 %d 条 —— %s\n\n", m, rep, lg.Site)
				fmt.Fprintf(&b, "判官 %.0f 分 —— %s\n\n", sc, why)
				vs := lg.Violations()
				if len(vs) == 0 {
					b.WriteString("数出来的犯规：无。\n\n")
				} else {
					b.WriteString("数出来的犯规：\n\n")
					sort.SliceStable(vs, func(i, j int) bool { return vs[i].Turn < vs[j].Turn })
					for _, v := range vs {
						fmt.Fprintf(&b, "- 第 %d 轮 · `%s` · %s\n", v.Turn, v.Kind, v.Note)
					}
					b.WriteString("\n")
				}
				fmt.Fprintf(&b, "<details><summary>完整对话</summary>\n\n```\n%s```\n\n</details>\n\n",
					lg.Transcript())
			}
		}
	}
	return b.String()
}

func callFn(prov gateway.Provider, r gateway.Resolved) coachwalk.Call {
	return func(ctx context.Context, req gateway.ChatRequest) (string, error) {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		res, err := gateway.Collect(cctx, prov, r, req)
		if err != nil {
			return "", err
		}
		return res.Text, nil
	}
}

func judgeWalk(ctx context.Context, prov gateway.Provider, jr gateway.Resolved, rubric, transcript string) (float64, string) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	res, err := gateway.Collect(cctx, prov, jr, gateway.ChatRequest{
		MaxTokens: 4000,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleUser, Content: rubric + "\n\n【对话】\n" + transcript},
		},
	})
	if err != nil {
		return 0, "判官调用失败: " + err.Error()
	}
	body := strings.TrimSpace(res.Text)
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
		// 🚨 抢救一个分数，而不是把这一条记成 0 分。判官经常在 why 里写
		// 「"装机量 vs 效率"」这样的**未转义直双引号**，整份 JSON 就废了 ——
		// 而一条真实得了 4 分的走查会在总表里显示成 0，把在位模型记差。
		// routebench 的 judgeOne 早就有这条抢救路径，这里第一版漏了。
		if m := scoreRe.FindStringSubmatch(body); m != nil {
			n, _ := strconv.ParseFloat(m[1], 64)
			return n, "分数从判官那份读不动的 JSON 里抢救出来（why 字段里有未转义的引号）"
		}
		return 0, "判官回复解析失败：" + coachwalk.TruncateRunes(body, 120)
	}
	return reply.Score, reply.Why
}

// scoreRe 从一份读不动的判官回复里捞出那个分数。
var scoreRe = regexp.MustCompile(`"score"\s*:\s*([1-5])`)

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
