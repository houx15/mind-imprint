// coachwalk —— 多轮陪练走查台。
//
// routebench 回答「这个模型这一轮答得好不好」。这件工具回答另一个问题：
// **换了模型，陪练连起来还能用吗。** 它让另一家的模型演学生，和真的 coachPrompt
// 来回若干轮，然后数可验的犯规（一轮问两个问题、把前面的话又说一遍、接不住她
// 说过的内容、解析失败），最后请一个旗舰判官读整条对话。
//
// 和 routebench 一样：它是一件**与运行系统分开**的工具。不连数据库，不被
// cmd/api 引用，永远不在请求路径上。改 models.json 的是人，不是它。
//
//	cd apps/api
//	DASHSCOPE_API_KEY=… DEEPSEEK_API_KEY=… go run ./cmd/coachwalk \
//	  -models dashscope/deepseek-v4-pro,deepseek/deepseek-flash -turns 8 -out walk.md
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/pbl"
)

func main() {
	models := flag.String("models", "dashscope/deepseek-v4-pro,deepseek/deepseek-flash",
		"逗号分隔的候选，按这个顺序跑；第一个是在位的那个")
	studentModel := flag.String("student", "dashscope/qwen3.8-max",
		"演学生的模型。必须和候选不同家，否则是同一个脑子自问自答")
	judgeModel := flag.String("judge", "dashscope/qwen3.7-max", "判官，必须是旗舰且不在候选里")
	turns := flag.Int("turns", 8, "每条走查跑几轮")
	out := flag.String("out", "", "把报告写到这个文件，留空则只打到 stdout")
	flag.Parse()

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

	cands := split(*models)
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
		model string
		log   *pbl.WalkLog
		err   error
		ms    int64
		score float64
		why   string
	}
	var runs []run

	for _, m := range cands {
		// 候选走 dialogue 档，拿到的就是这一档真正的约束（关思考）。
		coach, rerr := cat.Resolve(gateway.ClassDialogue, m, gateway.OSEnvKeyLookup)
		if rerr != nil {
			die("候选 %s: %v", m, rerr)
		}
		fmt.Fprintf(os.Stderr, "走查 %s（%d 轮，学生 %s）…\n", m, *turns, *studentModel)
		start := time.Now()
		log, werr := pbl.RunCoachWalk(ctx,
			callFn(prov, coach), callFn(prov, student), *turns)
		r := run{model: m, log: log, err: werr, ms: time.Since(start).Milliseconds()}
		if log != nil && len(log.Turns) > 0 {
			r.score, r.why = judgeWalk(ctx, prov, judge, log.Transcript())
		}
		runs = append(runs, r)
		fmt.Fprintf(os.Stderr, "  %d 轮，%d 处犯规，判官 %.0f 分\n",
			len(log.Turns), len(log.Violations()), r.score)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# coachwalk · 多轮陪练走查\n\n")
	fmt.Fprintf(&b, "%s · 每条 %d 轮 · 学生 `%s` · 判官 `%s`\n\n",
		time.Now().Format("2006-01-02 15:04"), *turns, *studentModel, *judgeModel)
	fmt.Fprintf(&b, "> 一轮的分数看不见「八轮之后她有没有往前走」。这份报告里\n")
	fmt.Fprintf(&b, "> **犯规那一列是数出来的，不是判官的印象**：一轮问两个问题、\n")
	fmt.Fprintf(&b, "> 把前面的话换个说法再说一遍、接不住她上一句、解析失败。\n\n")

	fmt.Fprintf(&b, "## 总表\n\n")
	fmt.Fprintf(&b, "| 模型 | 走完轮数 | 数出来的犯规 | 其中 一轮多问 | 其中 重复自己 | 其中 接不住她 | 解析失败 | 判官 | 总耗时 |\n")
	fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, r := range runs {
		vs := r.log.Violations()
		fmt.Fprintf(&b, "| `%s` | %d | **%d** | %d | %d | %d | %d | %.0f | %.1fs |\n",
			r.model, len(r.log.Turns), len(vs),
			count(vs, "multi-question"), count(vs, "repeat"),
			count(vs, "ungrounded"), count(vs, "parse"),
			r.score, float64(r.ms)/1000)
	}
	b.WriteString("\n")

	for _, r := range runs {
		fmt.Fprintf(&b, "## `%s`\n\n", r.model)
		if r.err != nil {
			fmt.Fprintf(&b, "🚨 走查中断：%v\n\n", r.err)
		}
		fmt.Fprintf(&b, "判官 %.0f 分 —— %s\n\n", r.score, r.why)
		vs := r.log.Violations()
		if len(vs) == 0 {
			fmt.Fprintf(&b, "数出来的犯规：无。\n\n")
		} else {
			fmt.Fprintf(&b, "数出来的犯规：\n\n")
			for _, v := range vs {
				fmt.Fprintf(&b, "- 第 %d 轮 · `%s` · %s\n", v.Turn, v.Kind, v.Note)
			}
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "<details><summary>完整对话</summary>\n\n```\n%s```\n\n</details>\n\n",
			r.log.Transcript())
	}

	fmt.Print(b.String())
	if *out != "" {
		if werr := os.WriteFile(*out, []byte(b.String()), 0o644); werr != nil {
			die("写报告失败: %v", werr)
		}
		fmt.Fprintf(os.Stderr, "\n写到 %s\n", *out)
	}
}

// callFn 把一个 provider + 一个已解析的模型包成 pbl.WalkCall。
func callFn(prov gateway.Provider, r gateway.Resolved) pbl.WalkCall {
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

func judgeWalk(ctx context.Context, prov gateway.Provider, jr gateway.Resolved, transcript string) (float64, string) {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	res, err := gateway.Collect(cctx, prov, jr, gateway.ChatRequest{
		MaxTokens: 4000,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleUser, Content: pbl.WalkJudge + "\n\n【对话】\n" + transcript},
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
		return 0, "判官回复解析失败：" + body
	}
	return reply.Score, reply.Why
}

func count(vs []pbl.Violation, kind string) int {
	n := 0
	for _, v := range vs {
		if v.Kind == kind {
			n++
		}
	}
	return n
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
