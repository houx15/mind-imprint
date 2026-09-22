package api

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// writing_stall.go —— 同一处说了两遍还没动，就换一种帮法。
//
// # 为什么（产品负责人 2026-09-20，general-suggestions.md）
//
//	交互策略：缺信息时问一个具体问题；**连续两轮无新增信息时，改用选项、
//	句式或简短示范**。
//
// 验收标准里也单独列了一条：「连续两轮卡住后改变帮助方式」。
//
// # 🚨 这条在 R4 之前只做了一半
//
// `writingPlanStalled` 只接在**立题**那条路上（writing_plan.go）。
// 段落陪练（writing_guide.go）和 AI审阅（writing_comment.go）两条路上
// 一个都没有 —— 也就是她在写段落时卡住，产品会用同一句话问她第三遍。
// R4 把它补齐。
//
// # 判据从**存下来的行**里数，不问模型
//
// 同一块上连着几条意见指着同一个症状、而她这一段的字一个都没改过。
// 两样都是库里的事实：`writing_comment.points[].symptom` 是闭表里的值，
// 「字没改过」是 `source_text` 逐字比。
//
// 🚨 **逐字比，不准拿长度当代理**（2026-09-12 的教训：screenKey 放了
// `.value.length`，于是「把一句话挪个位置」被判成「屏幕没有变化」，
// 连着十步告诉她没变）。这里直接用 R2 的 writingSheActedOn。

// 帮法的三档。
type writingHelpMode int

const (
	// helpAsk：问一个具体的问题。默认，也是绝大多数轮次该做的事。
	helpAsk writingHelpMode = iota
	// helpOffer：给两个选项让她挑。她答不上来那个问题，往往不是不想答，
	// 是不知道可以往哪儿想。
	helpOffer
	// helpShow：给一句句式，或者半句示范。
	//
	// 🚨 给的是**句式**（「假如……，那么……？」），不是替她写好的正文 ——
	// 铁律①。讲义的分析句三法正好每条都带着这样一句。
	helpShow
)

// writingStallToOffer / writingStallToShow —— 第几轮换档。
//
// 2 来自产品负责人的原话（「连续两轮无新增信息时」）。3 是它的下一档：
// 换了一次帮法她还是没动，就再往前递一步。
const (
	writingStallToOffer = 2
	writingStallToShow  = 3
)

// writingHelpModeFor 数这一块上连着几轮指着同一个症状、而她一个字都没改。
//
// prior 是 ListWritingComments 的结果（按 created_at DESC，最新的在前）。
// now 是她这一段**现在**的字。
func writingHelpModeFor(prior []sqlc.WritingComment, snippetID uuid.UUID, now string) writingHelpMode {
	var rounds int
	var symptoms map[string]bool

	for _, c := range prior {
		if c.Scope != "block" || !c.SnippetID.Valid || uuid.UUID(c.SnippetID.Bytes) != snippetID {
			continue
		}
		// 她在这条意见之后动过这一段 —— 那她没卡住，从这里往前不必再数。
		if writingSheActedOn(c.SourceText, now) {
			break
		}
		got := map[string]bool{}
		var old []CommentPoint
		if len(c.Points) > 0 {
			_ = json.Unmarshal(c.Points, &old)
		}
		for _, p := range old {
			if p.Kind == "issue" && strings.TrimSpace(p.Symptom) != "" {
				got[p.Symptom] = true
			}
		}
		if len(got) == 0 {
			// 那一轮是 pass，没提毛病 —— 不算卡住。
			break
		}
		if symptoms == nil {
			symptoms = got
			rounds = 1
			continue
		}
		// 和前面那几轮至少共用一个症状，才算「说的是同一件事」。
		var shares bool
		for s := range got {
			if symptoms[s] {
				shares = true
				break
			}
		}
		if !shares {
			break
		}
		rounds++
	}

	switch {
	case rounds >= writingStallToShow:
		return helpShow
	case rounds >= writingStallToOffer:
		return helpOffer
	}
	return helpAsk
}

// writingGuideHelpMode —— 段落陪练那条路上的同一把尺。
//
// 陪练不存 symptom，它存的是**上一组问题**（writingGuideDTO.Previous，
// 只留一层）。所以这里数的是「这一块的引导重新生成到第几代了」：
//
//	第 1 代：prior == nil                      → 问问题
//	第 2 代：prior != nil, prior.Previous == nil → 她读过一组、按了「换一组」→ 给选项
//	第 3 代：prior.Previous != nil               → 换了角度还是没动 → 给句式
//
// 🚨 writingGuideWithPrevious 里那句 `trimmed.Previous = nil` 是这条判据
// 成立的前提：存下来的 Previous 永远不带自己的 Previous，所以
// `prior.Previous != nil` 唯一的意思就是「至少第三代」。那句话是为了
// 「不要叠成一份她读不完的历史」写的，这里搭了个便车 —— 它要是被改掉，
// 这条判据会跟着失效，TestGuideHelpModeLadder 会响。
func writingGuideHelpMode(prior *writingGuideDTO) writingHelpMode {
	if prior == nil || len(prior.Questions) == 0 {
		return helpAsk
	}
	if prior.Previous != nil {
		return helpShow
	}
	return helpOffer
}
