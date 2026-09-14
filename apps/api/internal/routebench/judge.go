package routebench

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
)

// judgeSystem is deliberately blunt about what a bad answer looks like. A judge
// asked only for "quality" rates fluency, and fluency is the one thing every
// candidate here already has — the differences that matter are pedagogical:
// did it hand her the answer, did it ask three things at once, did it wave her
// past a step she has not finished.
const judgeSystem = `你在给一个教育产品的 AI 输出打分。这个产品的铁律是：
① 学生保留作业正文和最终判断的作者身份；AI 可以解释知识、示范不同题目的方法、从已表达的信息派生计划；
③ 最多问一个实际问题，无需补充信息时可以不问。

你会拿到一份评分标准和一段模型输出。严格按标准打分，1 到 5 的整数。

打分时特别注意三种常见的失败，它们读起来都很流畅：
- 代写学生要提交的作业正文，或未经学生确认替他作出决定（正常概念解释不属于代写）；
- 一口气问了好几个问题；
- 学生尚未完成练习却被标记完成；或已完成、已求助仍被反复盘问。
同时检查回应是否准确、是否提供了当前需要的帮助、是否存在无依据的能力或动机评价。

只输出一个 JSON 对象，不要任何其他文字：
{"score": <1-5 的整数>, "why": "<一句话，说明扣分或满分的具体理由>"}`

type judgeReply struct {
	Score int    `json:"score"`
	Why   string `json:"why"`
}

var fenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

func stripFences(s string) string {
	if m := fenceRe.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return strings.TrimSpace(s)
}

// Judge scores every result whose case carries a rubric. It scores the FIRST
// successful sample of each cell rather than all of them: the judge call costs
// as much as the call it is judging, and the between-sample spread on a fixed
// prompt is far smaller than the between-model spread this exists to measure.
func (rn *Runner) Judge(ctx context.Context, cases []benchcase.Case, results []Result) {
	if rn.Cfg.JudgeModel == "" {
		rn.logf("no judgeModel configured — skipping quality scoring")
		return
	}
	byID := map[string]benchcase.Case{}
	for _, c := range cases {
		byID[c.ID] = c
	}
	// The judge resolves through the assess class: flagship, full reasoning.
	// Grading is the one job here that must not be done cheaply.
	jr, err := rn.Cat.Resolve(gateway.ClassAssess, rn.Cfg.JudgeModel, rn.Keys)
	if err != nil {
		rn.logf("judge unavailable (%v) — skipping quality scoring", err)
		return
	}
	for i := range results {
		r := &results[i]
		c, ok := byID[r.CaseID]
		if !ok || strings.TrimSpace(c.Judge) == "" || r.Skipped != "" {
			continue
		}
		if r.ModelID == rn.Cfg.JudgeModel {
			// A model grading its own homework is not a measurement.
			r.JudgeWhy = "not judged: this model is the judge"
			continue
		}
		out := firstGoodText(*r)
		if out == "" {
			continue
		}
		score, why := rn.judgeOne(ctx, jr, c.Judge, out)
		r.Judge, r.JudgeWhy = score, why
	}
}

func firstGoodText(r Result) string {
	for _, s := range r.Samples {
		if s.Err == "" && strings.TrimSpace(s.Text) != "" {
			return s.Text
		}
	}
	return ""
}

func (rn *Runner) judgeOne(ctx context.Context, jr gateway.Resolved, rubric, output string) (float64, string) {
	cctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	res, err := gateway.Collect(cctx, rn.Provider, jr, gateway.ChatRequest{
		MaxTokens: 4000,
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: judgeSystem},
			{Role: gateway.RoleUser, Content: "【评分标准】\n" + rubric + "\n\n【模型输出】\n" + output},
		},
	})
	if err != nil {
		return 0, "judge call failed: " + err.Error()
	}
	var reply judgeReply
	body := stripFences(res.Text)
	if jerr := json.Unmarshal([]byte(body), &reply); jerr != nil {
		// Salvage a bare digit rather than throwing the call away, but say so —
		// a silently-invented score is worse than a missing one.
		if n := firstDigit(body); n > 0 {
			return float64(n), "score salvaged from unparseable judge reply"
		}
		return 0, "judge reply unparseable: " + truncate(body, 80)
	}
	if reply.Score < 1 || reply.Score > 5 {
		return 0, fmt.Sprintf("judge returned out-of-range score %d", reply.Score)
	}
	return float64(reply.Score), reply.Why
}

func firstDigit(s string) int {
	for _, r := range s {
		if r >= '1' && r <= '5' {
			n, _ := strconv.Atoi(string(r))
			return n
		}
	}
	return 0
}

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}
