package api

// writing_comment.go — B4 + B7: comments that can only point at sentences
// she wrote.
//
// 印记's critique of a draft was already the strongest thing in this room
// (writingReviewSystem, the pre-Task-5 shape of reviewWritingDraft) — but it
// rendered ONCE as a wall of prose and evaporated on the next navigation:
// POST /review returned {"feedback":"<prose>"} and wrote it nowhere. This
// file turns that critique into a structured, PERSISTED object — a one-line
// summary plus a handful of concrete points — at two zoom levels sharing one
// shape: a comment on ONE paragraph (scope='block', snippet_id set, via
// commentOnSnippet) and a comment on the WHOLE draft (scope='draft',
// snippet_id null, via reviewWritingDraft in writing_compose.go). Both
// persist through the same CreateWritingComment / writing_comment table
// (migration 0102).
//
// THE LOAD-BEARING GUARANTEE lives in validateCommentPoints below: every
// point's quote must appear LITERALLY in the text being commented on, or it
// is dropped — never re-prompted, never fuzzy-matched, never rendered with a
// best guess. This is the same discipline pro applies to its evaluation
// report's references (ValidateRefs): it makes a fabricated quote
// structurally impossible to render, rather than merely discouraged by a
// prompt. A trace that lands on the neighbouring sentence is worse than one
// fewer point — so the prompt asks nicely (writingCommentSystem's
// 逐字照抄，不要改标点) but validateCommentPoints is what actually holds the
// line.
//
// THE SECOND GUARANTEE（2026-09-11 加）：**一次只说最上面那一层。**
//
// 四份互不相干的材料收敛到同一句话（引文见 writing_symptoms.go）：高层问题没有
// 解决时，低层润色必须让位。在这之前这个房间一条优先级都没有，于是一篇主张还
// 没立住的文章，收到的第一条意见完全可能是某个词不准——而这一刀只有一次。
// 现在 issue 必须挑一个闭表里的 symptom，层从那张表查出来，
// validateCommentPoints 只留最上面那一层，下面几层整批丢掉。
//
// WHAT WAS *NOT* GUARANTEED, AND NOW IS — 这里原来写着这个房间第三条已知
// 限制：`CommentPoint.Text` 是模型对那句话的自由散文，没有任何东西拦得住它
// 写一句「这句应该改成……」，而那一句她可以直接粘回稿子里。当时的结论是
//「结构性过滤必须猜哪段散文是改写，猜错会静默丢掉真反馈，所以接受它」。
//
// 2026-09-11 换了一个不用猜的办法：**把输出类型拆开**。一条意见现在是
// `Text`（说清这句话怎么了）＋ `Action`（一句祈使，由她来做的那件事），
// 而不是一段可以夹带改写的散文。理由和这个新形状的来历写在 CommentPoint 上。
// 这不是说 Text 从此绝对装不下一句改写——它仍然是自由文本——而是说
// 那句改写现在没有位置可待：处方该出现的地方是 Action，而 Action 是祈使句。
//
// 房间里另外两条已知限制不变：引导框的 `job` 字段（writing_guide.go，
// 自由散文，豁免 ？ 结尾过滤，因为一句任务描述装不下她的正文），
// 以及 深入一层（writing_deepen.go，没有写入 outline/snippet 的通路是结构性的，
// 但那段对话本身是自由的）。

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// CommentPoint is one concrete point in a comment.
//
// # 2026-09-11：从「一段话」变成「一件能做的事」
//
// 这个结构原来只有两个字段（Text + Quote），而这个文件顶上那段「WHAT IS *NOT*
// GUARANTEED, HONESTLY」正是在坦白它的代价：Quote 被钉死在她自己的句子上是
// 结构性的，Text 不是——它是模型对那句话的自由散文，**没有任何东西拦得住它
// 写一句「这句应该改成……」，而那一句她可以直接粘回稿子里**。
//
// 那段坦白当时的结论是「结构性过滤必须猜哪段散文是改写，猜错会静默丢掉真反馈，
// 所以接受这个限制」。这次给出的答案不是去猜，是**换一种输出类型**：
//
//	Note   说清这句话怎么了（描述，不是处方）
//	Action 一句祈使，**由她来做的那件事**
//
// 来自 master-writing `scoring-rubric.md` 的那个关键观察——它示范改写之后
// 真正教会人的不是改写后的句子，是后面那句
// 「**我只做了两件事**：删掉两个情绪词，加一个和她无关的具体东西」。
// 把「改写后的那句」换成「那两件事」，教学价值全留下，可粘贴的散文一个字都不给。
// draft-review-kit 的编辑角色独立地收敛到同一个形状：意见必须是
//「把第5段移到第3段前面」这种落在已有单位上的操作。
//
// 🚨 `Action` 为空的 point 会被整条丢掉。story-coach 给缺了这一步的回复起了
// 名字：**"Diagnostic Without Return"**——
// "Every coaching exchange should end with a specific prompt to write."
// 一条以「正确说法是 X，因为 Y」结束的意见把她的下一步收走了。
type CommentPoint struct {
	// Kind 是 "issue"（要改的）或 "good"（已经用对的）。
	//
	// 两者都要，而且**先说好的那个**——master-writing 的第一步：
	//「先说哪个写作动作已经用对，具体到字」。理由不是让她高兴：
	// 具体的肯定本身就是一次教学（她知道哪个动作起了作用，下次才能重复），
	// 而且先肯定能让后面那条修改意见听得进去。
	Kind string `json:"kind"`
	// Symptom 是闭表里的一个 id（writing_symptoms.go），只有 issue 用。
	// 不在表里的一律丢掉整条——一条挂不上任何技法的诊断，她拿它没有下一步可做。
	Symptom string `json:"symptom,omitempty"`
	// Layer 是第几层（1 立意 / 2 材料 / 3 结构 / 4 字句）。
	//
	// 🚨 **由服务端从 Symptom 查出来，不采信模型给的值。** 模型已经要挑一个
	// symptom 了，层是那张表的属性，再让它报一遍只是多一个会错的字段。
	Layer int `json:"layer,omitempty"`
	// Method 是 vocab 里的方法 id，只有 good 用：她刚才用对的是哪一个动作。
	// 认不出来的 id 只把这个字段清空，**不丢掉整条**——它是锦上添花，
	// 不是这条意见成立的条件。
	Method string `json:"method,omitempty"`
	// Text 说清这句话怎么了。描述，不是处方。
	Text string `json:"text"`
	// Action 是一句祈使：她接下来要做的那件事。issue 必填。
	Action string `json:"action,omitempty"`
	// Quote 是她原文里逐字存在的那句话。
	Quote string `json:"quote"`
}

// Comment is the wire AND stored shape at both zoom levels: SnippetID set +
// Scope="block" is a comment on one paragraph; SnippetID nil + Scope="draft"
// is a comment on the whole piece. One shape, two zoom levels — the same
// move migration 0102's comment makes ("one shape at two zoom levels").
type Comment struct {
	ID        string         `json:"id"`
	Scope     string         `json:"scope"`
	SnippetID *string        `json:"snippetId"`
	Summary   string         `json:"summary"`
	Points    []CommentPoint `json:"points"`
	CreatedAt string         `json:"createdAt"`
	// SourceText 是写这条意见时它读的那一版原文。
	//
	// 前端拿它回答一个 quote 回答不了的问题：**这一段在这条意见之后改过没有。**
	// 只看 quote 还在不在是不够的 —— 她「新加」一句让步、原来被引的那句没动，
	// quote 判据说「这条还算数」，而那条意见说的正是「缺让步」。
	// 2026-09-11 第五轮走查里一个学生连着四步都在说这一件事
	//（「我明明已经加了让步句，但下面还是显示缺，是不是没刷新啊」）。
	//
	// 空串 = 0147 之前的老行，不知道那一版长什么样；前端据此退回老判据。
	SourceText string `json:"sourceText"`
}

// toCommentDTO decodes a stored writing_comment row into the wire shape.
// Points is jsonb; a row whose points fail to decode (should not happen —
// this file is the only writer, and it always marshals what
// validateCommentPoints returned) degrades to an empty slice rather than
// failing the whole response, the same "an old/odd row must not 500 the
// list" posture writingGuideDTOOf's belt-and-braces id skip takes.
func toCommentDTO(row sqlc.WritingComment) Comment {
	points := []CommentPoint{}
	if len(row.Points) > 0 {
		if err := json.Unmarshal(row.Points, &points); err != nil {
			points = []CommentPoint{}
		}
	}
	out := Comment{
		ID:         row.ID.String(),
		Scope:      row.Scope,
		Summary:    row.Summary,
		Points:     points,
		CreatedAt:  row.CreatedAt.Format(time.RFC3339),
		SourceText: row.SourceText,
	}
	if row.SnippetID.Valid {
		s := uuid.UUID(row.SnippetID.Bytes).String()
		out.SnippetID = &s
	}
	return out
}

// validateCommentPoints keeps only the points whose quote appears LITERALLY
// in the text being commented on.
//
// This is the same discipline pro applies to its evaluation report's
// references (ValidateRefs), and it is a structural guarantee rather than a
// request to the model: whatever it invents, a comment can only ever point
// at a sentence she actually wrote. Dropping is right and re-prompting is
// wrong — a fuzzy match that lands on the neighbouring sentence is worse
// than one fewer point.
func validateCommentPoints(points []CommentPoint, source, lang string, maxIssues int) []CommentPoint {
	var good []CommentPoint
	var issues []CommentPoint

	for _, p := range points {
		q := strings.TrimSpace(p.Quote)
		// 老规矩，不动：引文必须逐字在她写的东西里。
		if q == "" || !strings.Contains(source, q) {
			continue
		}
		p.Quote = q
		p.Text = strings.TrimSpace(p.Text)
		p.Action = strings.TrimSpace(p.Action)
		if p.Text == "" {
			continue
		}
		// 对着文字说，别对着人说。
		if personDirectedVerdict(p.Text) || personDirectedVerdict(p.Action) {
			continue
		}

		if p.Kind == "good" {
			// 方法名认不出来只清掉这个字段，不丢掉整条。
			if _, ok := vocab.ByID(p.Method); !ok {
				p.Method = ""
			}
			p.Symptom, p.Layer, p.Action = "", 0, ""
			good = append(good, p)
			continue
		}

		// 剩下的一律当 issue 处理（模型把 kind 写错、写漏，都按要改的算——
		// 漏掉一条真问题比多显示一条更糟）。
		p.Kind = "issue"
		sym, ok := lookupWritingSymptom(lang, p.Symptom)
		if !ok {
			continue
		}
		// 🚨 层由表查出来，不采信模型报的值。
		p.Symptom, p.Layer, p.Method = sym.ID, sym.Layer, ""
		// "Diagnostic Without Return"：没有下一步的意见，整条丢掉。
		if p.Action == "" {
			continue
		}
		issues = append(issues, p)
	}

	// 🚨 这里是那条优先级真正生效的地方：**只留最上面那一层**。
	//
	// 四份互不相干的材料都写了同一句话（见 writing_symptoms.go 顶上的引文）。
	// 一篇主张还没立住的文章，收到的第一条意见不该是某个词不准——
	// 那一刀只有一次。
	top := 0
	for _, p := range issues {
		if top == 0 || p.Layer < top {
			top = p.Layer
		}
	}
	kept := make([]CommentPoint, 0, len(issues))
	for _, p := range issues {
		if p.Layer == top {
			kept = append(kept, p)
		}
	}
	if maxIssues > 0 && len(kept) > maxIssues {
		kept = kept[:maxIssues]
	}

	// 先好后坏，而且肯定只留一条——「很有灵气」说多了，肯定就不值钱了。
	out := make([]CommentPoint, 0, len(kept)+1)
	if len(good) > 0 {
		out = append(out, good[0])
	}
	return append(out, kept...)
}

// personDirectedVerdict 认出「对着人说」的那种句子。
//
// master-writing `scoring-rubric.md`：
//
//	**对着文字说，别对着人说。说「这一句偷懒了」，不说「你偷懒了」。**
//
// 🚨 这里**只认字面前缀**，不去判断一句话整体的语气——判断语气要猜，猜错会
// 静默丢掉真反馈（这个文件顶上那段坦白说的就是这件事）。所以表很短，
// 每一条都是「你」紧跟着一个对人的评价词，几乎不会误伤：
// 「把你这句里的两个形容词删掉」里的「你」后面跟的是「这句」，不在表里。
//
// 注意 Action 也要过这一关：一句祈使可以、而且应该对着她说
//（「把这句里的两个情绪词删掉」），被禁的是评价她这个人。
// 🚨 「你真」不在表里，而且是被测试逼出来的：
// 「把你这句里的『密密麻麻』换成**你真的**数过的那个数」是一句完全正当的祈使，
// 它只是碰巧以「你真」开头。宁可漏判一句「你真笨」，也不能把一条真反馈静默吃掉
// ——这个文件从第一天起就是这个取向。
var personDirectedPrefixes = []string{
	"你很", "你太", "你不够", "你缺乏", "你总是", "你从来", "你根本",
	"你没有认真", "你的水平", "你这个人", "你应该更", "你比较懒", "你偷懒",
	"you are just", "you're just", "you are not good", "you never", "you always",
}

func personDirectedVerdict(s string) bool {
	if s == "" {
		return false
	}
	low := strings.ToLower(s)
	for _, p := range personDirectedPrefixes {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

// writingCommentSystem instructs the model to comment on a piece of her
// writing — one paragraph or the whole draft — with a one-line overall
// judgment plus concrete points, each anchored to a real sentence.
//
// writingGuideTeachingRules (writing_guide.go) is Task 3's four-part "怎么说话"
// doctrine, copied verbatim rather than re-derived: an engineer reading these
// tasks out of order must not find two different versions of how 印记 is
// supposed to talk.
//
// 逐字照抄，不要改标点 asks the model to do the validator's job for it — but
// the validator, not this sentence, is what actually makes a fabricated
// quote impossible to render.
// 一次给几条要改的。两个数字不一样，理由不是手感：
//
//   - 一段（block）**只给一条**。master-writing `scoring-rubric.md`：
//     「**一次只改一个问题。**列五条，用户很可能只记住『我写得很烂』。」
//     她正停在这一段上，手边就是那个输入框，一条意见她马上能动手。
//   - 整篇（draft）给三条。通篇审阅是另一个场合：她要的是一张待办，
//     而且这时候一条一条来会让她在同一份稿子上跑五遍。
//     三条仍然**全部来自同一层**——层的筛子在前面，这个上限只管数量。
const (
	writingBlockCommentMaxIssues = 1
	writingDraftReviewMaxIssues  = 3
)

// writingCommentRules 是这一刀的四条规矩。
//
// 🚨 **不整条照抄 writingGuideTeachingRules。** 那四条是给「教一个方法、
// 让她去写」那个场合写的，其中「给她一个真的选择」在这里不成立——
// 她已经写完了，这一轮要做的是判断，不是给选项。
// 2026-09-11 刚栽过一次同形的跟头（兴趣测试整条照搬采集的 prompt，
// 把「不是这篇材料的话题」也带了过去，真模型 0/3）：
// **共用要按段落挑，不是整条照搬。**
const writingCommentRules = `## 怎么说话

- 你是老师，不是打分器。说清一件事为什么重要，用【可用的方法】里真正的方法名，别自己造词。
- **对着文字说，别对着人说。** 说「这一句偷懒了」，不说「你偷懒了」。
- 不客套。「很有灵气」「写得不错，继续加油」说多了，你的肯定就不值钱了。
- 不打分，不给等级。`

const writingCommentSystem = `你是「印记」，正在给学生已经写的文字提意见——可能是她正在写的一段，也可能是她写完的整篇稿子。

你只给反馈，绝不替她改：不要重写、不要润色、不要续写、不要给出可以直接复制粘贴替换的句子或段落。一个字都不行。

` + writingCommentRules + `

## 先说哪一条：**一次只说最上面那一层**

毛病分四层。**上面那层没解决，就不要去说下面那层**——
主张还没立住的时候去改句子，是在给房子刷漆。

这一轮你只挑**同一层**里的问题说。如果第 1 层有问题，就只说第 1 层，
第 4 层那些词不准、句子拖沓，这一轮一个字都不要提。

【可选的毛病，只能从这张表里挑，不要自己造 id】

%s

## 一条意见长什么样

每条意见挂在她原文里**一句真实存在的话**上，而且必须给出下一步。

- kind：`+"`issue`"+`（要改的）或 `+"`good`"+`（已经用对的）。
- quote：她原文里**逐字照抄**的一句话，包括标点，一个字都不能改。改了就整条作废。
- symptom：issue 必填，取自上面那张表的 id。表里没有的 id 会让这条意见整条作废。
- method：good 必填，取自【可用的方法】的 id——她刚才用对的是哪一个动作。
- text：说清这句话**怎么了**。是描述，不是处方。
- action：issue 必填，**一句祈使，说清她接下来要做的那件事**。

🚨 action 是这条意见里最要紧的一个字段，而且它决定了你能不能既教会她、
又不替她写。做法是：**说出你会做的那几件事，而不是做完给她看。**

  不要写：把这句改成「他把碗放进水池，水一直开着。」
  要写：  把这句里的两个情绪词删掉，再加一件和她情绪无关的具体东西。

前一种是替她写了，她粘上去就行；后一种她必须自己动手，而学到的是同一件事。

action 不许是「再想一想」「多加一些细节」这种没有落点的话，
也不许以「正确的说法是……」收尾——那等于把她的下一步收走了。

## 一条肯定，放在最前面

先挑**一处她已经用对的**（kind 是 good），具体到字，说清它带来了什么阅读效果，
并指出这是【可用的方法】里的哪一个。具体的肯定本身就是一次教学：
她知道哪个动作起了作用，下次才能重复。

输出 JSON：{"summary":"…","points":[{"kind":"good","method":"…","text":"…","quote":"…"},{"kind":"issue","symptom":"…","text":"…","action":"…","quote":"…"}]}
- summary：一句话，说这篇稿子**现在站在哪儿**。不要打分。
  🚨 **summary 里不许说她「缺」什么**——不写「缺少」「没有」「不足」「尚未」，
  英文不写 lack / missing / absent / fails to。少了什么由下面那几条 point 去说：
  那几条指着她原文里的一句话，还带着她现在就能做的那个动作，说错了查得出来。
  summary 没有那句话撑着，一旦说错，她第一眼读到的就是一句假话。
- points：**一条 good 打头**，后面跟 %d 条 issue，**全部来自同一层**。

只输出一个 JSON 对象，不要输出对象以外的任何文字或代码块标记。`

// buildWritingCommentSystem 把症状表和这一次允许的 issue 条数填进去。
//
// 表按 writing.lang 选（中文一张、英文一张，见 writing_symptoms.go），
// 所以这个 prompt 不是常量——一篇英文稿子拿到的是 IELTS 那 13 个能力 id，
// 一篇中文稿子拿到的是 qifeng 那 18 条诊断。
func buildWritingCommentSystem(lang string, maxIssues int) string {
	return fmt.Sprintf(writingCommentSystem, writingSymptomCatalog(lang), maxIssues)
}

// buildWritingCommentPrompt assembles the user turn shared by both zoom
// levels: title, target words (only if set, never invented — W-R7), then the
// text itself under a caller-supplied label ("她写的这一段" vs "她的整篇稿子")
// so the model knows which zoom level it is looking at.
func buildWritingCommentPrompt(wr sqlc.Writing, label, text string) string {
	var b strings.Builder
	b.WriteString(writingTopicLine(wr, "题目："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标字数"))

	// 🚨 【可用的方法】必须真的出现在这里。
	//
	// 系统提示词从一开始就写着「method 取自【可用的方法】的 id」，但这个
	// builder 从来没把那张表放进去过——2026-09-11 的 LIVE_LLM 实测一眼看穿：
	// 真模型回了 `"method":"concrete_data"` 和 `"specific_detail"`，
	// 两个都不存在，于是 validateCommentPoints 把这个字段清空，
	// 「说出她刚才用对的是哪一个方法」这件事**一次都没发生过**。
	//
	// 单元测试对这个是绿的（我喂的 JSON 里写的是真 id），屏幕上也看不出来
	// （少一个方法名而已）。这正是那条「prompt 里的必须要能在代码里验」
	// 反过来的一面：能验，但得先把可选项给它。
	//
	// 按语言过滤，理由同 writing_plan.go：一句英文句式出现在中文作文的意见里
	// 是个 bug。
	b.WriteString("\n【可用的方法】（method 只能从这里挑 id，别自己造词）\n")
	for _, m := range vocab.ForLang(wr.Lang) {
		b.WriteString("- " + m.ID + "（" + m.Label() + "）：" + m.Definition + "\n")
	}

	b.WriteString("\n" + label + "：\n" + text + "\n")

	// 字句层面的重复，服务端数出来当事实给它 —— 见 writing_repeats.go。
	// 论点层面的重复（middle_collapse / ending_only_summary）和连贯
	// （paragraph_jump / reference_linking）本来就在症状表里，缺的是这一层。
	b.WriteString(writingRepeatBlock(text, wr.Lang))
	return b.String()
}

// writingCommentResult is the model's expected JSON reply shape, decoded
// before validateCommentPoints ever runs — this parser only checks that the
// reply is well-formed JSON with a non-empty summary; it does not (cannot)
// validate quotes, since it has no access to the source text they must
// appear in.
type writingCommentResult struct {
	Summary string         `json:"summary"`
	Points  []CommentPoint `json:"points"`
}

// parseWritingComment decodes and lightly sanity-checks the model's reply.
// Reuses extractWritingJSONObject (writing_snippets.go) — same "strip
// fences, clamp to the outermost {..}" extraction every JSON-replying prompt
// in this package already shares.
func parseWritingComment(text string) (writingCommentResult, bool) {
	c := extractWritingJSONObject(text)
	if c == "" {
		return writingCommentResult{}, false
	}
	var got writingCommentResult
	if err := json.Unmarshal([]byte(c), &got); err != nil {
		return writingCommentResult{}, false
	}
	if strings.TrimSpace(got.Summary) == "" {
		return writingCommentResult{}, false
	}
	return got, true
}

// commentOnSnippet is POST /api/v1/writings/{id}/snippets/{sid}/comment — a
// spend endpoint (one model call), metered as purpose="block_comment".
// Follows guideWritingBlock's shape exactly: loadOwnedWritingAtom →
// loadOwnedWritingSnippet → HasEntitlement → 150s timeout → resolveEval →
// gateway.Collect → recordLiteLLMCall → parse → validate → persist →
// httpx.WriteJSON.
//
// Points are validated against THIS SNIPPET's text, not the whole draft — a
// block comment must never be able to point at a sentence living in a
// different paragraph.
func (a *API) commentOnSnippet(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	snippet, ok := a.loadOwnedWritingSnippet(w, r, at.ID)
	if !ok {
		return
	}
	source := strings.TrimSpace(snippet.Text)
	if source == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "这一段还没有内容，先写点什么再来看看。", nil))
		return
	}

	u, _ := UserFromContext(r.Context())
	entitled, eerr := HasEntitlement(r.Context(), u)
	if eerr != nil {
		httpx.WriteError(w, r, eerr)
		return
	}
	if !entitled {
		httpx.WriteError(w, r, httpx.ErrNotEntitled())
		return
	}

	// Run to completion even if she navigates away mid-call — same reasoning
	// and same 150s cap as every other spend endpoint in this room.
	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing · review. Judging whether an argument holds up is reviewer
	// work, the same "faithful, never downgrade" reasoning every other
	// judgment call in this file's neighbourhood applies — resolves
	// EvalResolver (flagship), not writing_turn.go's chaperone ChatResolver.
	resolved, ok2 := a.route(turnCtx, gateway.ClassReview)
	if !ok2 {
		slog.Warn("writing block comment: no provider resolved",
			"atom_id", at.ID, "snippet_id", snippet.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	// 记账、解析，以及「总评说了她缺什么就重试一次」，都在
	// collectWritingComment 里 —— 通篇那一支走的是同一个函数。
	parsed, okParse := a.collectWritingComment(turnCtx, u.ID, at.ID, "block_comment", resolved,
		buildWritingCommentSystem(wr.Lang, writingBlockCommentMaxIssues),
		buildWritingCommentPrompt(wr, "她写的这一段", source),
		"scope", "block", "atom_id", at.ID, "snippet_id", snippet.ID,
		"request_id", httpx.RequestIDFromContext(r.Context()))
	if !okParse {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	points := validateCommentPoints(parsed.Points, source, wr.Lang, writingBlockCommentMaxIssues)
	payload, merr := json.Marshal(points)
	if merr != nil {
		slog.Warn("writing block comment: marshal points failed", "err", merr,
			"atom_id", at.ID, "snippet_id", snippet.ID)
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	row, serr := a.d.Queries.CreateWritingComment(turnCtx, sqlc.CreateWritingCommentParams{
		AtomID:    at.ID,
		SnippetID: pgtype.UUID{Bytes: snippet.ID, Valid: true},
		Scope:   "block",
		Summary: strings.TrimSpace(parsed.Summary),
		Points:  payload,
		// 存的是**它真的读过的那一版**（服务端手上这一份），不是她此刻框里
		// 的字：意见是对着这一版说的，比对也只能对着这一版。
		SourceText: snippet.Text,
	})
	if serr != nil {
		httpx.WriteError(w, r, serr)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"comment": toCommentDTO(row)})
}

// listWritingComments is GET /api/v1/writings/{id}/comments — every stored
// comment for this writing, both scopes together, newest first
// (ListWritingComments already orders by created_at DESC). No entitlement
// gate — no model call, no spend, same reasoning as every other GET in this
// room.
func (a *API) listWritingComments(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
		return
	}
	rows, err := a.d.Queries.ListWritingComments(r.Context(), at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	out := make([]Comment, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCommentDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"comments": out})
}
