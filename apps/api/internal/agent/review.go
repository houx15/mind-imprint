package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"mindimprint/api/internal/agent/enforcement"
	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/skills"
	"mindimprint/api/internal/teachingvoice"
)

// ReviewItem is one work-order row: which criterion, the band the draft sits
// in, the evidence it submits, what is missing, and an OPTIONAL fix — advice
// only, never a rewritten sentence (RL-1).
type ReviewItem struct {
	CriterionCode string `json:"criterion_code"`
	CriterionName string `json:"criterion_name"`
	Band          string `json:"band"`
	Evidence      string `json:"evidence"`
	Missing       string `json:"missing"`
	Fix           string `json:"fix"`
	Points        int    `json:"points"` // descriptor points evidenced, 0..criterion total (RL-3: which cell, not a grade)
}

// reviewItemWire is the model's per-item JSON contract (name is resolved
// server-side from the criterion code — the model never invents labels).
type reviewItemWire struct {
	CriterionCode string     `json:"criterion_code"`
	Band          string     `json:"band"`
	Evidence      flexString `json:"evidence"`
	Missing       flexString `json:"missing"`
	Fix           flexString `json:"fix"`
	Points        int        `json:"points"`
}

const reviewPosturePrompt = `你为 IB/国际课程学生审阅当前草稿，依据给定评分标准提供具体的修改建议。
只做一件事：对照给定的评分表，指出草稿满足哪些评分要求、还缺什么。
要求：不替学生改写句子、不给示范句、绝不续写。建议应说明需要补充的内容或需要解释的逻辑关系，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

// Voice selects the examiner posture the whole-draft review performs. board is
// the default (the existing reviewPosturePrompt); the three generic voices are
// board-agnostic postures. Voices change tone and lens only — never the JSON
// output shape and never the RL-1 iron rule.
type Voice string

const (
	VoiceBoard       Voice = "board"
	VoiceSceptic     Voice = "sceptic"
	VoiceLayperson   Voice = "layperson"
	VoiceExecutioner Voice = "executioner"
)

// ParseVoice maps an untrusted ?voice= value to a Voice; anything unrecognized
// (including "") falls back to the default board voice, so every existing call
// stays valid.
func ParseVoice(s string) Voice {
	switch Voice(s) {
	case VoiceSceptic, VoiceLayperson, VoiceExecutioner:
		return Voice(s)
	default:
		return VoiceBoard
	}
}

// The three generic postures keep the SAME iron rule and JSON-array output as
// the board voice; only the stance differs.
const reviewPostureSceptic = `你负责「论证审阅」，依据原文检查证据与推理，不预设论断有错，不评价学生的能力、态度或动机。学生已提交一版草稿快照。
对照给定的评分表，逐表说明已有证据能说明什么、还不能据此判断什么。讨论学生提出的方案时，直接指明具体内容，例如“延长图书馆开放时间的建议”，避免抽象地给学生的表达贴标签。给学生的文字中，将文中的判断称为“观点”，将提出的行动方案称为“建议”；说明学生的意见时直接写“你认为”或“你建议”。修改方向使用自然、完整的句子，说明需要核实的事实或需要补充的依据。
要求：不替学生改写句子、不给示范句、绝不续写。你的「建议」只能是"需要核实哪些事实/需要补充哪些依据"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

const reviewPostureLayperson = `请从不熟悉该主题的读者角度审阅学生当前草稿，判断说明是否足够清楚。
对照给定的评分表，逐表指出：哪些术语需要解释、哪些概念需要定义、哪些推理步骤需要展开，并说明这些信息如何帮助读者理解。
要求：不替学生改写句子、不给示范句、绝不续写。你的「建议」只能是"要解释什么/要补哪一步"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

const reviewPostureExecutioner = `你负责「精简审阅」，检查重复内容、与主题的关联及字数分配。学生已提交一版草稿快照。
对照给定的评分表，说明各段对论证的作用，指出重复或偏离主题的内容及可精简的理由。必要的背景可以保留，不评价学生的能力、态度或动机。
要求：不替学生改写句子、不给示范句、绝不续写。你的「建议」只能是"哪一段可精简/它与哪项评分要求相关"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

// The deletion-lens clause appended when the reviewed snapshot is over its word
// band — it reuses the review's paragraph⇄评分表 mapping to frame cuts as the
// student's decision. Diagnostic questions only (RL-1): never "删掉这段".
const reviewOverBudgetLens = `补充审阅任务：请考虑哪些内容可以精简。系统已按本次作业目标确认超出字数预算；该信息仅用于选择审阅视角，本轮不需要计算或评价字数。请在现有对象字段中说明重复、偏离主题的具体内容及精简理由，同时说明删减会损失什么信息，由学生决定如何修改。必要背景可以保留，没有明确可精简内容时如实说明。数组外不补充文字。`

// reviewPointsInstruction is appended for EVERY voice — points is assessment
// data (which descriptor cell the draft reaches), not part of the coaching
// lens, so it is voice-invariant. RL-3: points names a cell, never a grade.
const reviewPointsInstruction = `每个对象另外给出 points：草稿证据对应该评分表的哪个分点，
取 0 到该表总分点之间的整数（题面已给出每张表的总分点）。points 只表示"落在评分表的哪一格"，不是预估分数。`

// reviewSchemaInstruction names EVERY required JSON key explicitly. Without it
// the model reliably emits only the fields the posture foregrounds (missing) and
// the one field named above (points), and silently omits band/evidence/fix —
// leaving the student with criteria that show "还缺什么" but no rating and no
// direction (observed live 2026-07-30). Voice-invariant: it fixes the output
// contract, never the coaching lens or the RL-1 iron rule.
const reviewSchemaInstruction = `每个对象必须完整给出下面每一个字段，一个都不能省：
- criterion_code：评分表代号；
- band：依据评分表描述当前草稿对应的档位（如"刚起步/接近达标/已达标"，或该表的描述词，简短即可，不是分数）；
- evidence：草稿中与该评分表要求相关的证据（引用或转述草稿里的原话；确实没有就给空字符串）；
- missing：该评分要求下尚需补充或澄清的内容；
- fix：学生可以采取的具体修改行动，不提供可直接粘贴的成品句子；
- points：上面说明的整数。
完整保留各字段。band 说明判断，evidence 写实际依据；没有明确缺口时，missing 和 fix 可用空字符串，不编造不足。评语直接展示给学生，用“你”称呼学生。只输出这个 JSON 数组，不要多余文字。
格式示例：[{"criterion_code":"评分表代号","band":"描述档位","evidence":"原文依据","missing":"需要补充的内容","fix":"修改方向","points":0}]
请使用实际评分表代号和草稿内容替换示例值；输出在数组的右方括号处结束。`

// reviewSystemPrompt builds the system content for a review: the posture for
// the chosen voice, the voice-invariant points instruction, plus the deletion
// lens when overBudget. Pure — no I/O — so posture selection is unit-testable
// without a model.
func reviewSystemPrompt(voice Voice, overBudget bool) string {
	var base string
	switch voice {
	case VoiceSceptic:
		base = reviewPostureSceptic
	case VoiceLayperson:
		base = reviewPostureLayperson
	case VoiceExecutioner:
		base = reviewPostureExecutioner
	default:
		base = reviewPosturePrompt
	}
	base = base + "\n" + reviewPointsInstruction + "\n" + reviewSchemaInstruction + teachingvoice.Rules
	if overBudget {
		return base + "\n" + reviewOverBudgetLens
	}
	return base
}

// maxReviewAttempts bounds retries when the model's reply is UNPARSEABLE JSON
// or resolves to zero usable items after criterion-code matching — the same
// "transient generation hiccup, not a content decision" failure class
// reading_router.go / assess_report.go already guard with a retry (live
// bug-hunts 2026-07-30). A banned-phrasing rejection is a content decision —
// real, banned output was produced — and does NOT retry: re-asking the same
// model for the same content would just look like probing around RL-1.
const maxReviewAttempts = 2

// ProposeReview asks the flagship model for a whole-draft work-order over the
// snapshot's paragraphs, then runs the full enforcement stack on every field
// before returning. A single banned-phrasing / output-check violation rejects
// the WHOLE review (nothing is returned or persisted) — the same all-or-nothing
// discipline as the coach. Usage is populated whenever Collect succeeded (even
// on a later rejection) so the caller can still record 档位+token+成本.
func ProposeReview(ctx context.Context, prov gateway.Provider, r gateway.Resolved, criteria []skills.ReviewCriterion, paragraphs []string, graphSummary string, voice Voice, overBudget bool) ([]ReviewItem, gateway.ChatUsage, error) {
	name := map[string]string{}
	codes := make([]string, 0, len(criteria))
	// Tolerant lookups: the live model does not always echo the criterion code
	// verbatim (it emits "表D4", "表 D", or the criterion's name instead of the
	// bare "表D"). A strict exact-match dropped every item → "no usable items"
	// → the whole 整稿体检 review rejected → whole_draft_review never set → the
	// student could never finish (and never reach the project 你的思维印记). So
	// resolve codes leniently and only drop what truly matches nothing.
	byNorm := map[string]string{} // normalized code -> canonical code
	byName := map[string]string{} // normalized name -> canonical code
	norm := func(s string) string { return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "")) }
	for _, c := range criteria {
		name[c.Code] = c.Name
		byNorm[norm(c.Code)] = c.Code
		byName[norm(c.Name)] = c.Code
		codes = append(codes, fmt.Sprintf("%s（%s，共 %d 分点）", c.Code, c.Name, c.Points))
	}
	resolveCode := func(raw string) (string, bool) {
		if _, ok := name[raw]; ok {
			return raw, true
		}
		n := norm(raw)
		if c, ok := byNorm[n]; ok {
			return c, true
		}
		for nc, c := range byNorm { // 表D4⇄表D, D⇄表D
			if nc != "" && (strings.HasPrefix(n, nc) || strings.HasPrefix(nc, n)) {
				return c, true
			}
		}
		if c, ok := byName[n]; ok {
			return c, true
		}
		return "", false
	}
	user := fmt.Sprintf("评分表：%s\n\n论证摘要：%s\n\n草稿（分段）：\n%s",
		strings.Join(codes, "、"), graphSummary, strings.Join(paragraphs, "\n\n"))

	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: reviewSystemPrompt(voice, overBudget)},
			{Role: gateway.RoleUser, Content: user},
		},
	}

	var usage gateway.ChatUsage
	var lastErr error
	for attempt := 1; attempt <= maxReviewAttempts; attempt++ {
		res, err := gateway.Collect(ctx, prov, r, req)
		if err != nil {
			return nil, usage, err // transport error: don't retry, don't hammer the API
		}
		// Accumulate across attempts so the caller still records the true
		// 档位+token+成本 even when an earlier attempt was thrown away.
		usage.InputTokens += res.Usage.InputTokens
		usage.OutputTokens += res.Usage.OutputTokens

		var wires []reviewItemWire
		// stripFences: a reasoning model asked to output "只输出 JSON 数组" still
		// occasionally wraps the array in ```json fences (the same failure class
		// already hardened in reading_router.go/anchors.go/course.go/etc.) —
		// review.go had been left on a bare TrimSpace, so a fenced reply failed
		// json.Unmarshal outright and surfaced client-side as the blanket "体检没
		// 跑完，稍后再试一次" (item #10). Most likely to bite on an unusual, tiny
		// scope like a single selected paragraph, where the model has little to
		// work with against the full rubric and is more prone to hedge/wrap its
		// answer instead of emitting the bare array.
		if err := json.Unmarshal([]byte(stripFences(res.Text)), &wires); err != nil {
			lastErr = fmt.Errorf("agent: review output not a JSON array: %w", err)
			continue // truncated/fenced/garbage — retry once before giving up
		}
		items := make([]ReviewItem, 0, len(wires))
		for _, wv := range wires {
			code, known := resolveCode(wv.CriterionCode)
			if !known {
				slog.Warn("review: dropping item with unrecognized criterion code", "code", wv.CriterionCode)
				continue // ignore criteria the skill didn't ask for
			}
			// Enforcement on every free-text field the model produced.
			for _, field := range []string{wv.Evidence.String(), wv.Missing.String(), wv.Fix.String()} {
				if field == "" {
					continue
				}
				if rule := enforcement.BannedPhrasing(field); rule != nil {
					// A content decision, not a transient — never retried.
					return nil, usage, fmt.Errorf("agent: review output rejected by banned-phrasing rule %q", rule.Name)
				}
			}
			// Belt-and-suspenders: even with the schema instruction the reasoning
			// model can still occasionally drop band. Derive a non-empty label from
			// points so the rating pill is never blank (points names the cell, band
			// is just its human label — RL-3: not a grade).
			band := wv.Band
			if strings.TrimSpace(band) == "" {
				if wv.Points <= 0 {
					band = "尚未落点"
				} else {
					band = fmt.Sprintf("到第 %d 分点", wv.Points)
				}
			}
			items = append(items, ReviewItem{
				CriterionCode: code, CriterionName: name[code],
				Band: band, Evidence: wv.Evidence.String(), Missing: wv.Missing.String(), Fix: wv.Fix.String(),
				Points: wv.Points,
			})
		}
		if len(items) == 0 {
			lastErr = fmt.Errorf("agent: review produced no usable items")
			continue // every item's criterion code was unresolvable — retry once
		}
		return items, usage, nil
	}
	return nil, usage, lastErr
}
