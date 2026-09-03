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

// qualityFirst names the classes where being right matters more than being
// cheap, so the winner is the highest scorer rather than the cheapest one
// inside a band.
//
// 🚨 This distinction was missing from the first version and it produced a
// visibly wrong answer on 2026-09-03: on `review`, every candidate scored 2,
// so the band admitted all of them and the cheapest won — which was glm-5.3,
// answering in 4.6s with ZERO reasoning tokens on a class whose entire purpose
// is to reason. "Cheapest among equally mediocre" is the right rule for a
// classifier and the wrong rule for a reviewer.
func qualityFirst(class string) bool {
	return class == gateway.ClassReview || class == gateway.ClassAssess
}

// Recommend picks a model per class under one rule, applied in order:
//
//  1. anything that failed a call, or whose output the production parser
//     rejected even once, is out. Structural failure is not a tradeoff — it is
//     a student seeing an error.
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
// Cost is ranked by TOKENS, not money, because every DashScope model in the
// catalog is UNPRICED. Recording an invented price would defeat the one thing
// the catalog exists for. Fill in priceUsd and the same run converts to money
// without being re-run.
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
			out, think    int
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
			a.out += r.MedOut()
			a.think += r.MedReasoning()
			a.total += r.P50Total()
			if r.ErrRate() > 0 {
				a.hardFail = append(a.hardFail, fmt.Sprintf("%s: %.0f%% of calls failed", r.CaseID, r.ErrRate()*100))
			}
			if v := r.ValidRate(); v >= 0 && v < 1 {
				a.hardFail = append(a.hardFail, fmt.Sprintf("%s: production parser rejected %.0f%% of outputs", r.CaseID, (1-v)*100))
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
			return ok[i].a.out+ok[i].a.think < ok[j].a.out+ok[j].a.think
		})
		w := ok[0]
		// On a class where being right dominates, a TIE is not evidence to move.
		// Every candidate scoring the same 2 on `review` means the prompt is
		// broken, not that the cheapest one earned the binding — and switching on
		// cost there would hand a reasoning class to whichever model reasons
		// least, which is how glm-5.3 (zero reasoning tokens) kept winning it.
		if qualityFirst(class) {
			for _, c := range ok {
				if c.id == cat.Lanes[class].Model && c.min == w.min && c.judge == w.judge {
					w = c
					break
				}
			}
		}
		why := fmt.Sprintf("结构 100%%，质量 %.1f（最差一项 %.0f），输出 %d tokens（其中推理 %d），p50 %s",
			w.judge, w.min, w.a.out, w.a.think, w.a.total.Round(100*time.Millisecond))
		if qualityFirst(class) && w.id == cat.Lanes[class].Model && len(ok) > 1 {
			why += "  · 与其他候选质量并列，没有换绑的依据，保持不变"
		}
		if w.a.judged == 0 {
			why = fmt.Sprintf("结构 100%%（此档无判官用例），输出 %d tokens（其中推理 %d），p50 %s",
				w.a.out, w.a.think, w.a.total.Round(100*time.Millisecond))
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
	fmt.Fprintf(&b, "# routebench · 分级路由实测\n\n")
	fmt.Fprintf(&b, "%s · 每格 n=%d，取中位数 · 判官 `%s`\n\n", started.Format("2006-01-02 15:04"), cfg.Samples, cfg.JudgeModel)

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

	fmt.Fprintf(&b, "## 逐格结果\n\n")
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
		fmt.Fprintf(&b, "| 模型 | 首字 | 总时长 | 入 tokens | 出 tokens | 其中推理 | 结构 | 质量 | 备注 |\n")
		fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|---:|---|\n")
		for _, r := range rs {
			if r.Skipped != "" {
				fmt.Fprintf(&b, "| `%s` | — | — | — | — | — | — | — | 目录拒绝：%s |\n", r.ModelID, r.Skipped)
				continue
			}
			judge := "—"
			if r.Judge > 0 {
				judge = fmt.Sprintf("%.0f", r.Judge)
			}
			note := r.JudgeWhy
			if e := firstError(r); e != "" {
				note = e + "  " + note
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %d | %d | %s | %s | %s |\n",
				r.ModelID,
				r.P50TTFT().Round(100*time.Millisecond),
				r.P50Total().Round(100*time.Millisecond),
				r.MedIn(), r.MedOut(), r.MedReasoning(), pct(r.ValidRate()), judge, note)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// firstError surfaces the first thing that went wrong in a cell — a failed call
// or a parser rejection — because "valid 67%" without the reason is a number
// nobody can act on.
//
// The call-failure budget is deliberately generous. It used to be 70 characters,
// which cut a DashScope error off inside the word "message" and left a cell that
// said only "http 400" — the reader is then back to guessing, which is the exact
// failure this function exists to prevent.
func firstError(r Result) string {
	for _, s := range r.Samples {
		if s.Err != "" {
			return "调用失败：" + truncate(s.Err, 400)
		}
	}
	for _, s := range r.Samples {
		if s.ValidErr != "" && s.ValidErr != "n/a" {
			return "结构不合格：" + truncate(s.ValidErr, 200)
		}
	}
	return ""
}

// judgeAttempted reports whether this class has judged cases at all — inferred
// from a judge verdict having been recorded, successful or not.
func judgeAttempted(rs []Result) bool {
	for _, r := range rs {
		if r.Judge > 0 || strings.HasPrefix(r.JudgeWhy, "judge call failed") {
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
		if r.Judge == 0 && strings.HasPrefix(r.JudgeWhy, "judge call failed") && !seen[r.JudgeWhy] {
			seen[r.JudgeWhy] = true
			out = append(out, r.JudgeWhy)
		}
	}
	return out
}
