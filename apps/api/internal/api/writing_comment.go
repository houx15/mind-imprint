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

	"log/slog"

	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/quotematch"
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
// 「把第5段移到第3段前面」这种落在已有单位上的操作。
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
	ID    string `json:"id"`
	Scope string `json:"scope"`
	// Verdict 是这一段现在算什么：pass ／ polish ／ revise（writing_verdict.go）。
	//
	// 🚨 空串 = 0183 之前存的老行，那一版还没有分级 —— 前端据此**不渲染**
	// 那一行标签。不回填一个等级进去：那是替当初那条意见做一个它没做过的
	// 判断，而她会读到一个凭空出现的「需修改」。
	Verdict   string         `json:"verdict"`
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
		Verdict:    row.Verdict,
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
//
// 🚨 **丢掉的理由要记下来。** 2026-09-21 真学生走查：她那一段拿回来的是
// 一句「要紧的问题只有一处 —— 读者读到这里会把它和上一段当成同一件事」，
// 底下却只有一条夸她的话。模型**是**给了 issue 的，是这里丢掉的；而
// `collectWritingComment` 那道「说了有问题就得指出一处」的闸门查的是
// **模型原样回的** points，站在筛子的上游，于是这一整类它一次都看不见。
// 上游看不见、下游没人管 —— 中间掉下去的那条 issue 就这么没了。
//
// 所以现在多回一份「掉下去的都是为什么」：闸门拿它决定要不要再问一次，
// 日志拿它说清楚到底是哪条规矩在丢东西。
func validateCommentPoints(points []CommentPoint, source, lang string, maxIssues int) []CommentPoint {
	out, _ := validateCommentPointsVerbose(points, source, lang, maxIssues)
	return out
}

// commentPointDrop 一条没能留下来的意见，和它没留下来的理由。
//
// Reason 是个闭表（下面那几个常量），因为它要进日志、也要进重问那一轮的
// 提示语 —— 两处都不该出现一句临时拼的话。
type commentPointDrop struct {
	Kind   string
	Reason string
	Quote  string
}

// 丢掉的理由。闭表。
const (
	dropNoQuote          = "no_quote"           // 一条引文都没给
	dropQuoteNotVerbatim = "quote_not_verbatim" // 引文不在她这一段里（编的，或者引的是别段）
	dropNoText           = "no_text"            // 没说是什么问题
	dropPersonDirected   = "person_directed"    // 对着人说，不是对着文字说
	dropUnknownSymptom   = "unknown_symptom"    // symptom 不在这门语言的闭表里
	dropNoAction         = "no_action"          // 只下了诊断，没给下一步
	dropLowerLayer       = "lower_layer"        // 上面还有更要紧的一层
	dropOverMax          = "over_max"           // 这一轮说不了这么多条
)

func validateCommentPointsVerbose(points []CommentPoint, source, lang string, maxIssues int) ([]CommentPoint, []commentPointDrop) {
	var good []CommentPoint
	var issues []CommentPoint
	var drops []commentPointDrop
	drop := func(p CommentPoint, reason string) {
		drops = append(drops, commentPointDrop{Kind: p.Kind, Reason: reason, Quote: strings.TrimSpace(p.Quote)})
	}

	for _, p := range points {
		q := strings.TrimSpace(p.Quote)
		// 老规矩，不动：引文必须逐字在她写的东西里。
		// 只差标点、空格、大小写的，从原文里取出那一段逐字的换上
		// （quotematch.Locate）——锚点仍然逐字是她写的。
		if q == "" {
			// 🚨 模型常常把她那句话写进 text 里用「」括着，而 quote 那一格空着。
			// 实测三趟三趟都是这个死法（writing_dropped_issue_live_test.go）。
			// 放错格子不等于编了一句话 —— 捞出来的候选仍然要逐字对得上，
			// 对不上照旧丢掉。见 writing_quote_salvage.go。
			q = salvageQuoteFromText(p.Text, source)
		}
		if q == "" {
			drop(p, dropNoQuote)
			continue
		}
		if !strings.Contains(source, q) {
			span, ok := quotematch.Locate(source, q)
			if !ok {
				drop(p, dropQuoteNotVerbatim)
				continue
			}
			q = span
		}
		p.Quote = q
		p.Text = strings.TrimSpace(p.Text)
		p.Action = strings.TrimSpace(p.Action)
		if p.Text == "" {
			drop(p, dropNoText)
			continue
		}
		// 对着文字说，别对着人说。
		if personDirectedVerdict(p.Text) || personDirectedVerdict(p.Action) {
			drop(p, dropPersonDirected)
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
			drop(p, dropUnknownSymptom)
			continue
		}
		// 🚨 层由表查出来，不采信模型报的值。
		p.Symptom, p.Layer, p.Method = sym.ID, sym.Layer, ""
		// "Diagnostic Without Return"：没有下一步的意见，整条丢掉。
		if p.Action == "" {
			drop(p, dropNoAction)
			continue
		}
		issues = append(issues, p)
	}

	// 🚨 这里是那条优先级真正生效的地方：**只留最上面那一层**。
	//
	// 四份互不相干的材料都写了同一句话（见 writing_symptoms.go 顶上的引文）。
	// 一篇中心意思还不明确的文章，收到的第一条意见不该是某个词不准——
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
			continue
		}
		drop(p, dropLowerLayer)
	}
	if maxIssues > 0 && len(kept) > maxIssues {
		for _, p := range kept[maxIssues:] {
			drop(p, dropOverMax)
		}
		kept = kept[:maxIssues]
	}

	// 先好后坏，而且肯定只留一条——「很有灵气」说多了，肯定就不值钱了。
	out := make([]CommentPoint, 0, len(kept)+1)
	if len(good) > 0 {
		out = append(out, good[0])
	}
	return append(out, kept...), drops
}

// personDirectedVerdict 认出「对着人说」的那种句子。
//
// master-writing `scoring-rubric.md`：
//
//	**对着文字说，别对着人说。**
//
// 🚨 这里**只认字面前缀**，不去判断一句话整体的语气——判断语气要猜，猜错会
// 静默丢掉真反馈（这个文件顶上那段坦白说的就是这件事）。所以表很短，
// 每一条都是「你」紧跟着一个对人的评价词，几乎不会误伤：
// 「把你这句里的两个形容词删掉」里的「你」后面跟的是「这句」，不在表里。
//
// 注意 Action 也要过这一关：一句祈使可以、而且应该对着她说
// （「把这句里的两个情绪词删掉」），被禁的是评价她这个人。
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
// Review uses evidence-focused feedback rather than the dialogue's invitations
// to choose a method. It keeps the same exact-quote validation below.
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

// writingCommentResult is the model's expected JSON reply shape, decoded
// before validateCommentPoints ever runs — this parser only checks that the
// reply is well-formed JSON with a non-empty summary; it does not (cannot)
// validate quotes, since it has no access to the source text they must
// appear in.
type writingCommentResult struct {
	// Verdict 是这一段算什么。模型给的字符串在 parseWritingComment 里过一次
	// normalizeWritingVerdict —— 认不出来的退到 polish，不是 revise。
	Verdict string         `json:"verdict"`
	Summary string         `json:"summary"`
	Points  []CommentPoint `json:"points"`
}

// parseWritingComment decodes and lightly sanity-checks the model's reply.
// Reuses extractWritingJSONObject (writing_snippets.go) — same "strip
// fences, clamp to the outermost {..}" extraction every JSON-replying prompt
// in this package already shares.
func parseWritingComment(text string) (writingCommentResult, bool) {
	// 先走大家共用的那一条（去围栏 + 夹到最外层的 {..}）。绝大多数轮次到这里
	// 就结束了，这一条的行为一个字都没变。
	if got, ok := decodeWritingComment(extractWritingJSONObject(text)); ok {
		return got, true
	}
	// 🚨 它回了**不止一个** JSON 对象。
	//
	// 2026-09-21 实测抓到的样子：模型先写了一份，接着用大白话跟自己商量
	//（「补一句好的话也可以说……最终输出加一条 good」），然后又写了一份
	// 改好的。夹到「第一个 { 到最后一个 }」得到的是
	// `{对象一} 大白话 {对象二}` —— 不是合法 JSON，于是整轮作废，
	// 她那边是一个转不动的终端。
	//
	// 这和 [[model-json-half-arrived-2026-09-08]] 是同一类（「写完了但写坏了」），
	// 而 prompt 越长模型越爱这样自言自语 —— R4 把检查表加长了，正好撞上。
	//
	// **从后往前取**：它自己说的是「最终输出」，改好的那一份在后面。
	spans := writingJSONObjectSpans(text)
	for i := len(spans) - 1; i >= 0; i-- {
		if got, ok := decodeWritingComment(spans[i]); ok {
			return got, true
		}
	}
	return writingCommentResult{}, false
}

func decodeWritingComment(c string) (writingCommentResult, bool) {
	if strings.TrimSpace(c) == "" {
		return writingCommentResult{}, false
	}
	var got writingCommentResult
	if err := json.Unmarshal([]byte(c), &got); err != nil {
		return writingCommentResult{}, false
	}
	if strings.TrimSpace(got.Summary) == "" {
		return writingCommentResult{}, false
	}
	got.Verdict = normalizeWritingVerdict(got.Verdict)
	return got, true
}

// writingJSONObjectSpans 交出文本里每一段**括号配平**的 {...}。
//
// 🚨 字符串里的括号不算数。她的正文里有一个 `}`（或者模型引了一句带括号的
// 话），按裸括号数就会在半路上「配平」，切出一段断掉的 JSON —— 那正是
// 这个函数要避免的事，所以这里跟着引号和反斜杠走。
func writingJSONObjectSpans(text string) []string {
	var spans []string
	var depth, start int
	var inStr, esc bool
	for i, r := range text {
		switch {
		case esc:
			esc = false
		case inStr && r == '\\':
			esc = true
		case r == '"':
			inStr = !inStr
		case inStr:
			// 字符串里的括号不参与配平。
		case r == '{':
			if depth == 0 {
				start = i
			}
			depth++
		case r == '}':
			if depth > 0 {
				depth--
				if depth == 0 {
					spans = append(spans, text[start:i+1])
				}
			}
		}
	}
	return spans
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
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "无法审阅：请先输入段落内容。", nil))
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

	// 整篇上下文：这一块是什么、别的块写了什么、她这一块收到过什么意见。
	// 见 writing_piece_context.go（同事 2026-09-20 的意见 9 和 10）。
	//
	// 这三次查询**失败就降级为空上下文，不报错**。上下文是让意见更准的
	// 东西，不是它成立的条件；为了少一份上下文让她按下按钮拿到一个 502，
	// 是更糟的交换。
	piece := ""
	// 两项减法要用到的三样：这一块是什么、后面的段写了什么、她收到过什么意见。
	focusKind := ""
	laterText := ""
	// 读不到图的时候按题目推 —— 那条路默认议论文，见 writing_genre.go。
	genre := writingGenreOf(wr, nil)
	var priorComments []sqlc.WritingComment
	if outline, oerr := a.d.Queries.ListWritingOutline(turnCtx, at.ID); oerr == nil {
		genre = writingGenreOf(wr, outline)
		snippets, _ := a.d.Queries.ListWritingSnippets(turnCtx, at.ID)
		prior, _ := a.d.Queries.ListWritingComments(turnCtx, at.ID)
		focus := writingBlockOfSnippet(outline, snippet)
		piece = buildWritingPieceContext(wr, outline, snippets, prior, focus)
		if focus != nil {
			focusKind = writingKindOf(*focus)
		}
		laterText = writingLaterBlocksText(outline, snippets, focus)
		priorComments = prior
	} else {
		slog.Warn("writing block comment: outline unavailable, commenting without the whole piece",
			"err", oerr, "atom_id", at.ID)
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
	// 同一处说过两轮她还没动 → 换一种帮法（选项 / 句式）。
	// general-suggestions.md 交互策略那一条，R4 之前只接在立题那条路上。
	help := writingHelpModeFor(priorComments, snippet.ID, source)
	parsed, okParse := a.collectWritingComment(turnCtx, u.ID, at.ID, "block_comment", resolved,
		buildWritingCommentSystem(wr.Lang, writingBlockCommentMaxIssues, focusKind, help, genre),
		buildWritingCommentPrompt(wr, "她写的这一段", source, piece, genre),
		// 她真的会看到的那几条 —— 校验加两道减法之后剩下的。闸门查的就是这个。
		func(pts []CommentPoint) []CommentPoint {
			out := validateCommentPoints(pts, source, wr.Lang, writingBlockCommentMaxIssues)
			out = dropIssuesLaterBlocksAnswer(out, focusKind, laterText)
			return dropIssuesSheAlreadyFixed(out, priorComments, snippet.ID, source)
		},
		"scope", "block", "atom_id", at.ID, "snippet_id", snippet.ID,
		"request_id", httpx.RequestIDFromContext(r.Context()))
	if !okParse {
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	points := validateCommentPoints(parsed.Points, source, wr.Lang, writingBlockCommentMaxIssues)

	// 两项减法，**渲染之前**就丢掉 —— 写在这里而不是提示词里，理由同这个房间
	// 的老规矩：提示词里的「不要说」是模型可以推翻的。见 writing_verdict.go。
	before := len(points)
	points = dropIssuesLaterBlocksAnswer(points, focusKind, laterText)
	points = dropIssuesSheAlreadyFixed(points, priorComments, snippet.ID, source)
	if dropped := before - len(points); dropped > 0 {
		slog.Info("writing block comment: dropped issues the rest of the piece already answers",
			"atom_id", at.ID, "snippet_id", snippet.ID, "dropped", dropped, "focus_kind", focusKind)
	}

	// 一条 issue 都不剩，这一段就是 pass —— 不要把一个「本来要改、
	// 但那件事后面已经做了」的判断留在需修改上，她会对着一段没有任何意见的
	// 卡片读到「需修改」。
	//
	// 2026-09-21 真学生走查：这里原来**只收 revise**，而实测撞到的两次
	// 都是 `polish`（总评说「要紧的问题只有一处……」，底下只有一条夸她的话）。
	// 对她来说两者是同一件事：被告知这儿还不行，却没有一个字可以照着改。
	// 一个没有任何可做之事的档位，不该挂在她那张卡片上。
	verdict := parsed.Verdict
	if verdict != writingVerdictPass && !writingHasIssue(points) {
		slog.Info("writing block comment: no issue survived, verdict falls back to pass",
			"atom_id", at.ID, "snippet_id", snippet.ID, "was", parsed.Verdict)
		verdict = writingVerdictPass
	}

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
		Scope:     "block",
		Summary:   strings.TrimSpace(parsed.Summary),
		Points:    payload,
		// 存的是**它真的读过的那一版**（服务端手上这一份），不是她此刻框里
		// 的字：意见是对着这一版说的，比对也只能对着这一版。
		SourceText: snippet.Text,
		Verdict:    verdict,
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
