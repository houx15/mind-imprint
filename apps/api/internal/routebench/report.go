package routebench

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
)

// Recommendation is the workbench's answer for one class.
type Recommendation struct {
	Class   string
	ModelID string
	Why     string
	// Rejected lists the candidates that were disqualified and why, so the
	// recommendation can be argued with instead of merely believed.
	Rejected []string
}

// Recommend picks a model per class under one rule, applied in order:
//
//  1. anything that failed a call, whose output the production parser rejected,
//     or that missed a deterministic fixture expectation even once, is out.
//     These are separate measurements in the report: malformed wire output is
//     not the same defect as a readable reply with the wrong advance/tool.
//
//  2. anything whose WORST case is more than one point below the best model's
//     worst case is out.
//
//     The worst case, not the mean, because a mean hides the failure that
//     matters. On 2026-09-03 qwen3.7-plus tied deepseek-v4-pro on the dialogue
//     mean (3.75 each) while scoring 1 on the lite reading coach — it waved a
//     student past a step she had not finished. Averaged against three good
//     turns that reads as "as good as the incumbent". It is not: three of four
//     student-facing surfaces working is a broken product, not a 75% one.
//
//  3. of what remains, ranked in the owner's stated order (2026-09-03):
//     PERFORMANCE > SPEED > COST. Worst-case quality first, then mean quality,
//     then p50 latency, and only then token count. Cost is the last word, never
//     the first: it may break a tie and may not override a difference.
//
// Cost is ranked by MONEY since 2026-09-20, when the DashScope rates were
// filled in (priceCny). Before that it could only rank by token count, and that
// is a rule with a known failure mode: qwen3.7-flash and deepseek-v4-pro differ
// by 60x on input price, so a model that emits slightly more tokens can still
// be an order of magnitude cheaper. Ranking those two on tokens gives the
// opposite answer to ranking them on the bill.
//
// A model the catalog cannot price still falls back to token count: a missing
// measurement has to read as missing, never as a tie.
func Recommend(results []Result, cat *gateway.Catalog) []Recommendation {
	byClass := map[string][]Result{}
	for _, r := range results {
		byClass[r.Class] = append(byClass[r.Class], r)
	}

	var out []Recommendation
	for _, class := range gateway.Classes {
		rs := byClass[class]
		if len(rs) == 0 {
			continue
		}
		// Aggregate per model across that class's cases.
		type agg struct {
			in, out, think int
			// money is the USD this model would spend on one pass of this
			// class's cases, at catalog rates. 0 when the model is unpriced.
			money         float64
			priced        bool
			total         time.Duration
			judgeSum      float64
			judgeMin      float64
			judgeN        int
			hardFail      []string
			cases, judged int
		}
		am := map[string]*agg{}
		for _, r := range rs {
			a := am[r.ModelID]
			if a == nil {
				a = &agg{}
				am[r.ModelID] = a
			}
			if r.Skipped != "" {
				a.hardFail = append(a.hardFail, "catalog refused this binding: "+r.Skipped)
				continue
			}
			a.cases++
			a.in += r.MedIn()
			a.out += r.MedOut()
			a.think += r.MedReasoning()
			a.total += r.P50Total()
			// Reasoning tokens are billed as output, and the vendors that charge
			// most for thinking are exactly the ones a token count flatters.
			if spec, known := cat.Models[r.ModelID]; known {
				if c, priced := gateway.EstimateCost(spec.Provider, spec.Model,
					r.MedIn(), r.MedOut()+r.MedReasoning()); priced {
					a.money += c
					a.priced = true
				}
			}
			if r.ErrRate() > 0 {
				a.hardFail = append(a.hardFail, fmt.Sprintf("%s: %.0f%% of calls failed", r.CaseID, r.ErrRate()*100))
			}
			if v := r.ParseRate(); v >= 0 && v < 1 {
				a.hardFail = append(a.hardFail, fmt.Sprintf("%s: production parser rejected %.0f%% of outputs", r.CaseID, (1-v)*100))
			}
			if g := r.GoldRate(); g >= 0 && g < 1 {
				a.hardFail = append(a.hardFail, fmt.Sprintf("%s: gold expectation missed on %.0f%% of outputs", r.CaseID, (1-g)*100))
			}
			if v := r.ExpectedRate(); v >= 0 && v < 1 {
				a.hardFail = append(a.hardFail, fmt.Sprintf("%s: deterministic expectations rejected %.0f%% of outputs", r.CaseID, (1-v)*100))
			}
			for _, failure := range resultJudgeFailures(r) {
				a.hardFail = append(a.hardFail, r.CaseID+": "+failure)
			}
			if r.Judge > 0 {
				a.judgeSum += r.Judge
				if a.judged == 0 || r.Judge < a.judgeMin {
					a.judgeMin = r.Judge
				}
				a.judged++
			}
		}

		// The bar is the best model's WORST case — see Recommend's note on why
		// a mean is the wrong summary for a product with several surfaces.
		bestWorst := 0.0
		for _, a := range am {
			if a.judged > 0 && len(a.hardFail) == 0 && a.judgeMin > bestWorst {
				bestWorst = a.judgeMin
			}
		}

		type cand struct {
			id         string
			a          *agg
			judge, min float64
		}
		var ok []cand
		var rejected []string
		for _, id := range sortedModelIDs(am) {
			a := am[id]
			if len(a.hardFail) > 0 {
				rejected = append(rejected, fmt.Sprintf("%s — %s", id, strings.Join(a.hardFail, "; ")))
				continue
			}
			j := 0.0
			if a.judged > 0 {
				j = a.judgeSum / float64(a.judged)
			}
			if bestWorst > 0 && a.judged > 0 && a.judgeMin < bestWorst-1.0 {
				rejected = append(rejected, fmt.Sprintf("%s — 最差一项 %.0f 分，比最好的最差项 %.0f 低超过一分（均分 %.1f 把这一项藏起来了）",
					id, a.judgeMin, bestWorst, j))
				continue
			}
			ok = append(ok, cand{id, a, j, a.judgeMin})
		}
		if len(ok) == 0 {
			out = append(out, Recommendation{Class: class, ModelID: "", Why: "没有候选通过", Rejected: rejected})
			continue
		}
		// 🚨 A judged class with no surviving quality score must not produce a
		// recommendation. On 2026-09-03 the judge model 400'd on every single
		// call (it was qwen3.7-max, and the assess class sends reasoning_effort
		// "max", which that model rejects). The run still emitted a full table —
		// ranking on token count alone and crowning the model that thought least,
		// on classes whose entire purpose is thinking. A missing measurement has
		// to read as missing, never as a tie.
		anyJudged := false
		for _, c := range ok {
			if c.a.judged > 0 {
				anyJudged = true
			}
		}
		if judgeAttempted(rs) && !anyJudged {
			out = append(out, Recommendation{
				Class:    class,
				ModelID:  "",
				Why:      "**判官在这一档全部失败，没有质量数据——不做推荐。** 先修判官再重跑；按 token 排出来的名次在这里没有意义。",
				Rejected: append(rejected, judgeFailures(rs)...),
			})
			continue
		}
		// Cost breaks a TIE in quality. It never overrides a difference in it.
		//
		// 🚨 Third time this rule needed sharpening, each time after it produced
		// a visibly wrong answer. The band above filters out candidates more than
		// a point below the best worst case — but a filter is not a preference,
		// and on 2026-09-03 that gap let `dialogue` recommend qwen3.8-max (worst
		// case 2) over deepseek-v4-pro (worst case 3) purely on token count. The
		// worse model was inside the band, so cost decided. That is precisely the
		// failure the worst-case rule was introduced to prevent.
		sort.Slice(ok, func(i, j int) bool {
			if ok[i].min != ok[j].min {
				return ok[i].min > ok[j].min
			}
			if ok[i].judge != ok[j].judge {
				return ok[i].judge > ok[j].judge
			}
			// Owner's ordering, 2026-09-03: performance > speed > cost. Speed
			// outranks cost because the student waits through it and never sees
			// the token bill; a class only reaches this line when quality is
			// already tied, so nothing is being bought with her experience.
			if ok[i].a.total != ok[j].a.total {
				return ok[i].a.total < ok[j].a.total
			}
			// 🚨 2026-09-20 · 这一行原来比的是 token 数。价格填上之前那是唯一能比
			// 的东西，但它在价差一个数量级的时候会给出**反的**结论：
			// qwen3.7-flash 0.2/0.8 元 与 deepseek-v4-pro 12/24 元 相差 60 倍，
			// 少吐几个 token 补不回来。现在按钱比。
			// 两边都没有价格时退回 token —— 缺测量要读成缺测量，不能读成打平。
			if ok[i].a.priced && ok[j].a.priced && ok[i].a.money != ok[j].a.money {
				return ok[i].a.money < ok[j].a.money
			}
			return ok[i].a.out+ok[i].a.think < ok[j].a.out+ok[j].a.think
		})
		w := ok[0]
		keptIncumbentOnExactTie := false
		// The incumbent is retained only when every deciding metric is exactly
		// tied. Equal quality alone is not a tie: speed and then token count are
		// explicit parts of the recommendation rule.
		if spec, bound := cat.Lanes[class]; bound {
			for _, c := range ok {
				if c.id == spec.Model &&
					c.min == w.min && c.judge == w.judge &&
					c.a.total == w.a.total &&
					c.a.out+c.a.think == w.a.out+w.a.think {
					keptIncumbentOnExactTie = c.id != w.id
					w = c
					break
				}
			}
		}
		why := fmt.Sprintf("解析/预期 100%%，质量 %.1f（最差一项 %.0f），输出 %d tokens（其中推理 %d），p50 %s",
			w.judge, w.min, w.a.out, w.a.think, w.a.total.Round(100*time.Millisecond))
		if keptIncumbentOnExactTie {
			why += "  · 所有排序指标完全相同，保持现有绑定"
		}
		if w.a.judged == 0 {
			why = fmt.Sprintf("解析/预期 100%%（此档无判官用例），输出 %d tokens（其中推理 %d），p50 %s",
				w.a.out, w.a.think, w.a.total.Round(100*time.Millisecond))
		}
		// The bill for one pass of this class's cases, so a reader can see what
		// the last tiebreak was actually comparing instead of inferring it.
		if w.a.priced {
			why += fmt.Sprintf("，本档一遍 $%.4f", w.a.money)
		}
		if spec, bound := cat.Lanes[class]; bound && spec.LatencyBudgetMs > 0 {
			if w.a.cases > 0 {
				avg := w.a.total / time.Duration(w.a.cases)
				if avg > time.Duration(spec.LatencyBudgetMs)*time.Millisecond {
					why += fmt.Sprintf("  🚨 超出该档 %.1fs 的延迟预算", float64(spec.LatencyBudgetMs)/1000)
				}
			}
		}
		out = append(out, Recommendation{Class: class, ModelID: w.id, Why: why, Rejected: rejected})
	}
	return out
}

func sortedModelIDs[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Markdown renders the whole run: the recommendation first, because that is
// what a reader came for, then the table it rests on.
func Markdown(results []Result, cat *gateway.Catalog, cfg Config, started time.Time) string {
	var b strings.Builder
	if cfg.Suite != "" {
		fmt.Fprintf(&b, "# prompt gate · 单轮体检\n\n")
		fmt.Fprintf(&b, "%s · suite `%s` · 每格 n=%d · 判官 `%s`\n\n",
			started.Format("2006-01-02 15:04"), cfg.Suite, cfg.Samples, cfg.JudgeModel)
		fmt.Fprintf(&b, "> 仅模型调用与最终解析失败使命令失败；预期、判官、token 和延迟供人工审阅。\n\n")
	} else {
		fmt.Fprintf(&b, "# routebench · 分级路由实测\n\n")
		fmt.Fprintf(&b, "%s · 每格 n=%d，取中位数 · 判官 `%s`\n\n", started.Format("2006-01-02 15:04"), cfg.Samples, cfg.JudgeModel)
	}

	if cfg.Suite == "" {
		unpriced := 0
		for _, id := range cat.ModelIDs() {
			if cat.Models[id].Price == nil {
				unpriced++
			}
		}
		if unpriced > 0 {
			fmt.Fprintf(&b, "> **成本按 token 量排序，不是按钱。** 目录里有 %d 个模型 `priceUsd` 为空——\n"+
				"> 宁可记成本为空，也不能编一个数字，否则这份目录存在的意义（比成本）当场就废了。\n"+
				"> 把百炼控制台的费率填进 `models.json` 之后，同一份结果不用重跑就能换算成钱。\n\n", unpriced)
		}

		fmt.Fprintf(&b, "## 推荐绑定\n\n")
		fmt.Fprintf(&b, "| 档 | 推荐模型 | 依据 |\n|---|---|---|\n")
		recs := Recommend(results, cat)
		for _, r := range recs {
			model := r.ModelID
			if model == "" {
				model = "**无**"
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", r.Class, model, r.Why)
		}
		b.WriteString("\n")
		for _, r := range recs {
			if len(r.Rejected) == 0 {
				continue
			}
			fmt.Fprintf(&b, "**`%s` 排除了：**\n", r.Class)
			for _, x := range r.Rejected {
				fmt.Fprintf(&b, "- %s\n", x)
			}
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "## 单轮结果\n\n")
	byCase := map[string][]Result{}
	var order []string
	for _, r := range results {
		if _, seen := byCase[r.CaseID]; !seen {
			order = append(order, r.CaseID)
		}
		byCase[r.CaseID] = append(byCase[r.CaseID], r)
	}
	for _, id := range order {
		rs := byCase[id]
		fmt.Fprintf(&b, "### `%s`\n\n档：`%s` · 调用点：`%s`\n\n", id, rs[0].Class, rs[0].Site)
		// 入 tokens is here because cost is (in x in_price + out x out_price), and a
		// report that prints only the output half cannot be converted to money no
		// matter what prices you later fill in.
		fmt.Fprintf(&b, "| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 解析 | 预期 | 任务命中 | 质量 | 备注 |\n")
		fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|\n")
		for _, r := range rs {
			if r.Skipped != "" {
				fmt.Fprintf(&b, "| `%s` | — | — | — | — | — | — | — | — | — | 目录拒绝：%s |\n", r.ModelID, r.Skipped)
				continue
			}
			judge := "—"
			if r.Judge > 0 {
				judge = fmt.Sprintf("%.1f（最低 %.0f / 中位 %.0f）", r.Judge, r.JudgeMin(), r.JudgeMedian())
			}
			note := r.JudgeWhy
			if e := firstError(r); e != "" {
				note = e + "  " + note
			}
			if r.RetryRate() > 0 {
				note = fmt.Sprintf("重试 %.0f%%；%s", r.RetryRate()*100, note)
			}
			if failures := resultJudgeFailures(r); len(failures) > 0 {
				note = "判官失败：" + strings.Join(failures, "；") + "；" + note
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %d | %d | %s | %s | %s | %s | %s |\n",
				r.ModelID,
				r.P50TTFT().Round(100*time.Millisecond),
				r.P50Total().Round(100*time.Millisecond),
				r.MedIn(), r.MedOut(), r.MedReasoning(), pct(r.ParseRate()), pct(r.ExpectedRate()), pct(r.GoldRate()), judge, note)
		}
		b.WriteString("\n")
	}
	if cfg.Suite != "" {
		b.WriteString("## 回复速览\n\n每个用例展示一个代表样本，以及其他需要关注的样本；完整样本见 JSON。\n\n")
		for _, id := range order {
			for _, r := range byCase[id] {
				if len(r.Samples) == 0 {
					continue
				}
				fmt.Fprintf(&b, "### `%s` · `%s`\n\n", id, r.ModelID)
				picked := map[int]bool{0: true}
				for i, s := range r.Samples {
					if s.Err != "" || (s.ParseErr != "" && s.ParseErr != "n/a") || (s.ValidErr != "" && s.ValidErr != "n/a") || isJudgeFailure(s.JudgeWhy) || (s.Judge > 0 && s.Judge < 3) {
						picked[i] = true
					}
				}
				for i, s := range r.Samples {
					if !picked[i] {
						continue
					}
					fmt.Fprintf(&b, "样本 %d · 预期：%s · 判官：%.0f · 入/出/推理 tokens：%d/%d/%d\n\n", i+1, sampleExpectation(s), s.Judge, s.In, s.Out, s.Reason)
					if s.ValidErr != "" && s.ValidErr != "n/a" {
						fmt.Fprintf(&b, "需审阅：%s\n\n", s.ValidErr)
					}
					if s.Err != "" || (s.ParseErr != "" && s.ParseErr != "n/a") {
						fmt.Fprintf(&b, "技术故障：%s %s\n\n", s.Err, s.ParseErr)
					}
					if s.JudgeWhy != "" {
						fmt.Fprintf(&b, "判官理由：%s\n\n", s.JudgeWhy)
					}
					reply := s.Text
					if reply == "" {
						reply = s.FirstText
					}
					fmt.Fprintf(&b, "```json\n%s\n```\n\n", reply)
				}
			}
		}
	}
	return b.String()
}

func sampleExpectation(s Sample) string {
	if s.ValidErr == "n/a" {
		return "未设置"
	}
	if s.Valid {
		return "符合"
	}
	return "需审阅"
}

// firstError surfaces the first thing that went wrong in a cell while keeping
// parser failures, provider/finalization failures, and fixture expectations
// visibly distinct.
//
// The call-failure budget is deliberately generous. It used to be 70 characters,
// which cut a DashScope error off inside the word "message" and left a cell that
// said only "http 400" — the reader is then back to guessing, which is the exact
// failure this function exists to prevent.
func firstError(r Result) string {
	for _, s := range r.Samples {
		if s.ParseErr != "" && s.ParseErr != "n/a" {
			return "解析失败：" + truncate(s.ParseErr, 400)
		}
	}
	for _, s := range r.Samples {
		if s.Err != "" {
			return "调用失败：" + truncate(s.Err, 400)
		}
	}
	for _, s := range r.Samples {
		if s.ValidErr != "" && s.ValidErr != "n/a" {
			return "预期不符：" + truncate(s.ValidErr, 200)
		}
	}
	for _, s := range r.Samples {
		if s.GoldErr != "" {
			return "任务未命中：" + truncate(s.GoldErr, 200)
		}
	}
	return ""
}

// judgeAttempted reports whether this class has judged cases at all — inferred
// from a judge verdict having been recorded, successful or not.
func judgeAttempted(rs []Result) bool {
	for _, r := range rs {
		if r.Judge > 0 || len(resultJudgeFailures(r)) > 0 {
			return true
		}
	}
	return false
}

// judgeFailures surfaces the judge's own error so the reader can fix the tool
// rather than wonder why a column is empty.
func judgeFailures(rs []Result) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rs {
		for _, failure := range resultJudgeFailures(r) {
			if !seen[failure] {
				seen[failure] = true
				out = append(out, failure)
			}
		}
	}
	return out
}

func resultJudgeFailures(r Result) []string {
	var out []string
	for _, s := range r.Samples {
		if isJudgeFailure(s.JudgeWhy) {
			out = append(out, s.JudgeWhy)
		}
	}
	return out
}
