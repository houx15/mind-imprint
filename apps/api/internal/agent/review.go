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

const reviewPosturePrompt = `你是 IB/国际课程写作的「整稿体检」考官。学生已提交一版草稿快照。
只做一件事：对照给定的评分表，指出每一张表现在收到了哪些证据、还缺什么。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要补什么/要接什么"的方向，
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
const reviewPostureSceptic = `你是一位「整稿体检」考官，天生不信任每一个论断。学生已提交一版草稿快照。
对照给定的评分表，逐表指出：哪些说法只是断言、还没把证据摆出来，哪里的结论跑在了支撑前面。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要拿出什么证据/要补什么支撑"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

const reviewPostureLayperson = `你是一位友善但完全外行的读者，不懂这个领域。学生已提交一版草稿快照。
对照给定的评分表，逐表指出：哪里有没解释的术语、没定义的概念、跳过了的推理步骤——凡是你这个外行读不懂的地方。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"要解释什么/要补哪一步"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

const reviewPostureExecutioner = `你是一位盯字数的「整稿体检」考官。学生已提交一版草稿快照。
对照给定的评分表，逐表追问：每一段文字有没有挣到它占的字数——哪些段落不向任何一张表交证据、纯属背景或冗余。
铁律：绝不替学生改写句子、绝不给示范句、绝不续写。你的「建议」只能是"哪一段可以砍/它本该向哪张表交证据"的方向，
不能是可直接粘贴的成品句子。一次只输出 JSON 数组，每个评分表一个对象。`

// The deletion-lens clause appended when the reviewed snapshot is over its word
// band — it reuses the review's paragraph⇄评分表 mapping to frame cuts as the
// student's decision. Diagnostic questions only (RL-1): never "删掉这段".
const reviewOverBudgetLens = `另外：这一稿已经超出字数预算。对交证据最少的那些段落，指出它们各自在向哪张表交证据；
如果一张表都不向，就把「这 N 字在向哪张表交证据」这个删减决策摆到学生面前，让她自己决定砍哪一段——你不替她删。`

// reviewPointsInstruction is appended for EVERY voice — points is assessment
// data (which descriptor cell the draft reaches), not part of the coaching
// lens, so it is voice-invariant. RL-3: points names a cell, never a grade.
const reviewPointsInstruction = `每个对象另外给出 points：这张表当前收到的证据够到第几分点，
取 0 到该表总分点之间的整数（题面已给出每张表的总分点）。points 只表示"落在评分表的哪一格"，不是预估分数。`

// reviewSchemaInstruction names EVERY required JSON key explicitly. Without it
// the model reliably emits only the fields the posture foregrounds (missing) and
// the one field named above (points), and silently omits band/evidence/fix —
// leaving the student with criteria that show "还缺什么" but no rating and no
// direction (observed live 2026-07-30). Voice-invariant: it fixes the output
// contract, never the coaching lens or the RL-1 iron rule.
const reviewSchemaInstruction = `每个对象必须完整给出下面每一个字段，一个都不能省：
- criterion_code：评分表代号；
- band：一句话点出这张表现在大致落在哪一档（如"刚起步/接近达标/已达标"，或该表的描述词，简短即可，不是分数）；
- evidence：草稿里已经交到这张表的证据（引用或转述草稿里的原话；确实没有就给空字符串）；
- missing：这张表还缺什么（只说方向）；
- fix：下一步可以往哪个方向补（只给方向，不能是可直接粘贴的成品句子）；
- points：上面说明的整数。
band、evidence、fix 最容易被漏掉——请逐字段填好，宁可简短也不要整段留空。只输出这个 JSON 数组，不要多余文字。`

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
	base = base + "\n" + reviewPointsInstruction + "\n" + reviewSchemaInstruction
	if overBudget {
		return base + "\n" + reviewOverBudgetLens
	}
	return base
}

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

	res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: reviewSystemPrompt(voice, overBudget)},
			{Role: gateway.RoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, gateway.ChatUsage{}, err
	}
	usage := res.Usage

	var wires []reviewItemWire
	if err := json.Unmarshal([]byte(strings.TrimSpace(res.Text)), &wires); err != nil {
		return nil, usage, fmt.Errorf("agent: review output not a JSON array: %w", err)
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
		return nil, usage, fmt.Errorf("agent: review produced no usable items")
	}
	return items, usage, nil
}
