package routebench

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
)

// judgeSystem keeps each case rubric authoritative while applying product
// boundaries only to the tasks where they are relevant.
const judgeSystem = `你是 routebench 的质量判官。你会收到【本题评分标准】和【模型输出】。

评分方法：
1. 【本题评分标准】定义这次任务的目标、约束和失败条件，是本题评分的唯一依据。
2. 先核对模型是否完成标准要求的判断或任务，再检查理由、建议或行动是否有具体依据并与判断一致。
3. 不要因为文字流畅就给高分，也不要加入评分标准没有要求的任务目标。

产品边界与适用范围：
- 学生保留要提交的作业正文和最终判断的作者身份。AI 可以解释知识、示范不同题目的方法、从已表达的信息派生计划；审阅中的问题诊断、修改方向和待补信息属于反馈，不是代写正文。
- 「最多问一个实际问题」是对话陪练约束，不是审阅、分类或结构化输出的通用约束；仅在本题评分标准要求时据此扣分。
- 「未完成却被标记完成」或「已完成、已求助仍被反复盘问」只在本题涉及完成度、门槛或对话推进时构成失败，以本题评分标准为准。
- 同时按本题评分标准检查准确性、是否提供当前需要的帮助、是否存在无依据的能力或动机评价。

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

// Judge scores one usable sample per result whose case carries a rubric.
// The score is a review aid, not an automatic prompt-regression threshold.
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
		if idx := firstGoodSample(r.Samples); idx >= 0 {
			s := &r.Samples[idx]
			s.Judge, s.JudgeWhy = rn.judgeOne(ctx, jr, c.Judge, s.Text)
			r.Judge, r.JudgeWhy = s.Judge, s.JudgeWhy
		}
	}
}

func firstGoodSample(samples []Sample) int {
	for i, s := range samples {
		if s.Err == "" && strings.TrimSpace(s.Text) != "" && (s.Valid || s.ValidErr == "n/a") {
			return i
		}
	}
	return -1
}
func (rn *Runner) judgeOne(ctx context.Context, jr gateway.Resolved, rubric, output string) (float64, string) {
	var last string
	for attempt := 1; attempt <= 2; attempt++ {
		cctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
		res, err := gateway.Collect(cctx, rn.Provider, jr, gateway.ChatRequest{
			MaxTokens: 4000,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: judgeSystem},
				{Role: gateway.RoleUser, Content: "【评分标准】\n" + rubric + "\n\n【模型输出】\n" + output},
			},
		})
		cancel()
		if err != nil {
			last = "call failed: " + err.Error()
			continue
		}
		score, why, jerr := parseJudgeReply(res.Text)
		if jerr == nil {
			return score, why
		}
		last = jerr.Error()
	}
	return 0, "judge failed after 2 attempts: " + last
}

func parseJudgeReply(raw string) (float64, string, error) {
	var reply judgeReply
	body := stripFences(raw)
	if jerr := json.Unmarshal([]byte(body), &reply); jerr != nil {
		return 0, "", fmt.Errorf("reply unparseable: %s", truncate(body, 80))
	}
	if reply.Score < 1 || reply.Score > 5 {
		return 0, "", fmt.Errorf("returned out-of-range score %d", reply.Score)
	}
	if strings.TrimSpace(reply.Why) == "" {
		return 0, "", fmt.Errorf("returned an empty why")
	}
	return float64(reply.Score), strings.TrimSpace(reply.Why), nil
}

func isJudgeFailure(why string) bool { return strings.HasPrefix(why, "judge failed after ") }

func truncate(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n]) + "…"
}
