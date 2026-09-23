package api

// writing_verdict.go —— 一条意见现在先说「这一段算什么」，以及两项服务端减法。
//
// # 为什么（2026-09-20，同事的意见 10）
//
//	「当前反馈忽略段落任务、全文、历史对话和字数，反复要求补材料，
//	  **将可选优化判为必改**。调整为识别实际问题、分级反馈、给出明确下一步，
//	  让学生完成写作。」
//
// 两件事分开做：
//
//   - **分级**（这个文件上半部分）：`pass ｜ polish ｜ revise`。
//     `polish` 的意思是「还可以更好，但不挡着她往下走」—— 产品里以前没有这个
//     档位，于是任何一条意见读起来都像「你得改」。
//   - **减法**（下半部分）：后文已经承接的、她已经照着改过的，**渲染之前就丢掉**。
//     写在代码里而不是提示词里，理由同这个房间的老规矩：提示词里的「不要说」
//     是模型可以推翻的（[[prompt-twice-then-make-it-checkable]]）。

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// 闭表。和 TS 侧 `Comment.verdict` 一致。
const (
	// 这一段站得住了，可以去写下一段。
	writingVerdictPass = "pass"
	// 还可以更好，但**不挡着她往下走**。
	writingVerdictPolish = "polish"
	// 必须改：指出具体文本、它造成的影响、做到什么算完成。
	writingVerdictRevise = "revise"
)

func writingVerdictValid(v string) bool {
	switch v {
	case writingVerdictPass, writingVerdictPolish, writingVerdictRevise:
		return true
	}
	return false
}

// normalizeWritingVerdict 收一收模型给的那个字符串。
//
// 认不出来的**退到 polish**，不是 revise：判错的方向不对称 ——
// 把一处可选的优化说成必改，正是同事指出的那个毛病；反过来，
// 把一处该改的说成可优化，她仍然读得到那条意见和那个动作。
func normalizeWritingVerdict(v string) string {
	t := strings.ToLower(strings.TrimSpace(v))
	if writingVerdictValid(t) {
		return t
	}
	return writingVerdictPolish
}

// writingHasIssue：这几条意见里还有没有「要改的」。
func writingHasIssue(points []CommentPoint) bool {
	for _, p := range points {
		if p.Kind == "issue" {
			return true
		}
	}
	return false
}

// layerVerdictsOf 把一条整体结论拆成四层各自的等级（立意/材料/结构/字句）。
//
// # 为什么（产品负责人：数值分数不给，但可以给等级/颜色反馈）
//
// 一条 `Comment.Verdict` 是这一段的总判断，但一段立意站得住、字句一堆问题的
// 稿子，读到的只有一个笼统的 polish——两件不相干的事被拌在一起说。
// 这个函数只回答「这一层，单独看，算什么」，**只看这条评语自己的 points**，
// 不看别的评语、别的段，也不问模型（Layer 已经是服务端从 Symptom 查出来的，
// 见 CommentPoint.Layer 上的注释，这里在它上面再算一层）。
//
// 判据（和 normalizeWritingVerdict 同一个不对称）：
//
//   - 这一层一个 issue 都没有 → pass。「没提到」和「明确没问题」在这里是
//     同一件事：一层完全没被点名，恰恰说明它没有拖后腿。
//   - 这一层**一条** issue → polish。一处可以更好，不挡着她往下走。
//   - 这一层**两条及以上** issue → revise。同一层连着几处，这一层要回去重来。
//
// 🚨 为什么按**条数**，不按「那条意见带没带 Action」：
// validateCommentPoints 会把没有 Action 的 point 整条丢掉
// （writing_comment.go:304，dropNoAction），所以活下来的 issue **全都**带
// Action。拿「带不带 Action」当判据，三档会塌成两档（pass / revise），
// polish 那一支变成走不到的死代码 —— 而 revise 在她屏幕上是危险色。
// 一处小的用词问题就会让「字句」变红，正好撞上 CommentPanel 里那条
// 「polish 不能长得像错误」。条数是服务端数得出来的事实，不用问模型。
//
// 兜底方向和 normalizeWritingVerdict 一致：拿不准就退到更轻的那一档。
//
// 四层都会有值，哪怕这条评语一个 point 都没提到那一层：学生的卡片上永远是
// 四个格子，不是零散的一两个（未被提到 = 上面第一条 = pass）。
func layerVerdictsOf(c Comment) map[int]string {
	issues := map[int]int{}
	for _, p := range c.Points {
		if p.Kind != "issue" {
			continue
		}
		issues[p.Layer]++
	}
	layers := []int{writingLayerClaim, writingLayerMaterial, writingLayerStructure, writingLayerSentence}
	out := make(map[int]string, len(layers))
	for _, layer := range layers {
		switch n := issues[layer]; {
		case n == 0:
			out[layer] = writingVerdictPass
		case n == 1:
			out[layer] = writingVerdictPolish
		default:
			out[layer] = writingVerdictRevise
		}
	}
	return out
}

// —— 减法一：后面的段已经承接了的，不要在这一段里要 ——

// writingOpeningExcusedSymptoms 是**开头段不该被要求**的那几种毛病。
//
// 🚨 同事 2026-09-20 的意见 9，原话：
//
//	「这个第一段的分析，这个具体的举例写在了第二段和第三段，但是 ai 在分析
//	  第一段的时候没有进行关联，也不知道开头段只是一个引子的作用，
//	  给出了错误的分析结果。」
//
// 开头的活是「让读者愿意读下去 + 亮出主张」，不是把全文的证据先摆一遍。
// 所以判据是两条一起成立：**这一块是开头**，而且**后面的段里确实有具体的事**。
// 后面也没有，这一条就不丢 —— 那时候它是真的缺，而不是分工。
var writingOpeningExcusedSymptoms = map[string]bool{
	"claim_no_evidence":      true,
	"example_no_detail":      true,
	"no_example":             true,
	"evidence_not_explained": true,
}

// writingConcreteMarkers 是「这一段里有一件具体的事」的痕迹。
//
// 判得宽一点是故意的：这个函数只用来决定**要不要少说一条**，
// 少说一条的代价有界（她下一轮还会拿到别的意见），而误判成「后面没有」
// 会让开头段继续挨那条它不该挨的意见。
var writingConcreteMarkers = []string{
	"有一次", "那天", "上周", "上个月", "去年", "早上", "中午", "晚上", "放学", "当时",
	"我记得", "我看见", "我发现", "我问", "他说", "她说",
	"年", "月", "日", "点", "岁", "次", "个", "%", "％",
}

// laterBlocksAreConcrete：后面的段里有没有一件具体的事。
func laterBlocksAreConcrete(laterText string) bool {
	t := strings.TrimSpace(laterText)
	if t == "" {
		return false
	}
	hits := 0
	for _, m := range writingConcreteMarkers {
		if strings.Contains(t, m) {
			hits++
			if hits >= 2 {
				return true
			}
		}
	}
	return false
}

// dropIssuesLaterBlocksAnswer 丢掉「这一段缺证据」那一类 —— 只在这一块是开头、
// 而且后面的段里确实有具体的事的时候。
func dropIssuesLaterBlocksAnswer(points []CommentPoint, focusKind, laterText string) []CommentPoint {
	if focusKind != writingKindOpening || !laterBlocksAreConcrete(laterText) {
		return points
	}
	kept := make([]CommentPoint, 0, len(points))
	for _, p := range points {
		if p.Kind == "issue" && writingOpeningExcusedSymptoms[p.Symptom] {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// —— 减法二：她已经照着上一条改过了，不要再说一遍 ——

// dropIssuesSheAlreadyFixed 丢掉「和上一条同一个 symptom、而且她已经动过这一段」
// 的那几条。
//
// 「她动过没有」用的是 0147 存下来的 `source_text`：那条意见是对着**那一版**
// 说的，所以只要这一段现在的字和当初不一样，她就动过了。
//
// 🚨 判错的方向不对称（[[feedback-staleness-asymmetry-2026-09-12]]）：
// **她做完了、产品说她没做，是最伤的那一个** —— 那次她连着八步在说
// 「我明明已经加了让步句，但下面还是显示缺」。所以「不一样」就算她动过。
// 反过来，她一个字都没改的时候**必须留着**：那条意见还没被处理。
func dropIssuesSheAlreadyFixed(
	points []CommentPoint,
	prior []sqlc.WritingComment,
	snippetID uuid.UUID,
	now string,
) []CommentPoint {
	addressed := map[string]bool{}
	for _, c := range prior {
		if c.Scope != "block" || !c.SnippetID.Valid || uuid.UUID(c.SnippetID.Bytes) != snippetID {
			continue
		}
		if !writingSheActedOn(c.SourceText, now) {
			continue
		}
		var old []CommentPoint
		if len(c.Points) > 0 {
			_ = json.Unmarshal(c.Points, &old)
		}
		for _, p := range old {
			if p.Kind == "issue" && strings.TrimSpace(p.Symptom) != "" {
				addressed[p.Symptom] = true
			}
		}
	}
	if len(addressed) == 0 {
		return points
	}
	kept := make([]CommentPoint, 0, len(points))
	for _, p := range points {
		if p.Kind == "issue" && addressed[p.Symptom] {
			continue
		}
		kept = append(kept, p)
	}
	return kept
}

// writingLaterBlocksText 把这一块**后面**那几块她写下的字拼起来。
// 用来回答「后面的段里有没有那件具体的事」。
func writingLaterBlocksText(
	outline []sqlc.WritingOutline,
	snippets []sqlc.WritingSnippet,
	focus *sqlc.WritingOutline,
) string {
	if focus == nil {
		return ""
	}
	var b strings.Builder
	for _, o := range outline {
		if o.Position <= focus.Position {
			continue
		}
		for _, s := range snippets {
			if s.OutlineID.Valid && uuid.UUID(s.OutlineID.Bytes) == o.ID {
				b.WriteString(strings.TrimSpace(s.Text))
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}
