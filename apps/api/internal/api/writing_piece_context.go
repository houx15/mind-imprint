package api

// writing_piece_context.go —— 段落级的每一次调用都该看得见的那份**整篇**上下文。
//
// # 为什么加这个（2026-09-20，同事的意见 9 和 10）
//
// 在这之前，`buildWritingCommentPrompt(wr, label, text)` 拿到的全部上下文是：
// 题目、语言、目标字数、方法表、**这一段的正文**。就这些。
//
// 于是同事截到的那一幕：她的开头段写「手机可以帮助我们联系家长，也能用来学习」，
// 印记判「没有一件具体的事，这两个好处就只是说法」—— 而那两件具体的事
// **就写在第二段和第三段里**。它说得没错，只是它看不见。他的原话：
//
//	「对每一段的分析需要明确各个段落各自的功能，而且也要知道其他的段落写了
//	  什么。……不知道开头段只是一个引子的作用，给出了错误的分析结果。」
//
// 两件事都要给：**这一块是什么**（开头的活不是正文的活）和**别的块写了什么**。
//
// # 三处调用点共用一份
//
// 「请印记看看这一段」（writing_comment.go）、「深入一层」（writing_deepen.go）、
// 「写作引导」（writing_guide.go）—— 三处都只看得见一段，而且各自手写了一小段
// 「整篇的结构」，都只列标题不带正文。合成一份，它们的上下文才不会再分岔。
//
// # 🚨 块的顺序是成本契约
//
// 2026-09-20 记下的那条：DashScope 的隐式缓存关不掉，命中按输入价 20% 计费，
// 而命中要求**前缀逐字一样**。她的正文每一轮都在变，所以稳定的块（题目、语言、
// 篇幅、老师的要求）必须排在前面，易变的块（各段正文、她停在哪一块）排在后面。
// 倒过来写，每一轮都是全价。

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// writingPieceBlockRunes 是别的块在上下文里最多占多少字。
//
// 🚨 **不能整篇原样塞。** 一篇 3000 字的稿子会让每一次「看看这一段」都按整篇
// 计费，而这个按钮她一段能按好几次。
//
// 300：够看出那一段在做什么（有没有具体的事、有没有解释），而不是够读完它。
// 判断「后面的段里有没有例子」不需要读完那一段。
const writingPieceBlockRunes = 300

// writingPieceMaxBlocks 是最多列几块。一篇中学作文到不了这个数；
// 到得了的那种，多出来的几块对「这一段缺什么」也不再带信息。
const writingPieceMaxBlocks = 12

// buildWritingPieceContext 组装整篇上下文。
//
// focus 是她现在停在的那个结构图节点，可以是 nil（自由段落没有节点）。
// comments 是这一篇的**全部**历史意见，函数自己挑出落在 focus 这一段上的。
func buildWritingPieceContext(
	wr sqlc.Writing,
	outline []sqlc.WritingOutline,
	snippets []sqlc.WritingSnippet,
	comments []sqlc.WritingComment,
	focus *sqlc.WritingOutline,
) string {
	var b strings.Builder

	// —— 稳定的那几块，排在前面（见文件顶上的成本契约）——
	b.WriteString("\n【这一篇】\n")
	b.WriteString(writingTopicLine(wr, "题目："))
	b.WriteString(writingLangLine(wr))
	b.WriteString(writingLengthLine(wr, "目标篇幅"))

	// —— 易变的那几块 ——
	b.WriteString("\n【整篇的结构，以及她在每一块写下的字】\n")
	cards := writingCards(outline, snippets)
	if len(cards) == 0 {
		b.WriteString("（还没有结构。）\n")
	}
	shown := 0
	for i, c := range cards {
		if shown >= writingPieceMaxBlocks {
			b.WriteString("（后面还有几块，没有全部列出。）\n")
			break
		}
		shown++
		b.WriteString("- 第 " + itoa(i+1) + " 张 · " + writingPieceCardName(c))
		if c.Node != nil {
			if t := strings.TrimSpace(c.Node.Text); t != "" {
				b.WriteString("：" + t)
			}
		}
		if focus != nil && c.Node != nil && c.Node.ID == focus.ID {
			b.WriteString("   ← **她现在停在这一块**")
		}
		b.WriteString("\n")
		b.WriteString("    " + writingPieceBlockBody(c) + "\n")
	}

	// —— 她在这一块收到过什么意见，以及她做到了没有 ——
	if focus != nil {
		if line := writingPriorCommentLines(comments, snippets, *focus); line != "" {
			b.WriteString("\n【她这一块之前收到过的意见】\n" + line)
			b.WriteString("🚨 她已经改过的那几条**不要再说一遍**；还没动的那几条，" +
				"这一轮优先接着说那一条，而不是另起一个新问题。\n")
		}
	}

	return b.String()
}

// writingPieceCardName 是这一块在上下文里的名字 —— 和她屏幕上看到的那个词一致。
func writingPieceCardName(c writingCard) string {
	switch c.Kind {
	case writingCardOpening:
		return "开头"
	case writingCardClosing:
		return "结尾"
	case writingCardFree:
		return "自由段落"
	}
	if c.Node != nil {
		if n := writingKindLabel(writingKindOf(*c.Node), c.Node.Source); n != "" {
			return n
		}
	}
	return "主体段"
}

// writingPieceBlockBody 是这一块她写下的正文（截断，并且**说出来**截了）。
//
// 🚨 截断不说会出事：模型以为她那一段就写了这么多，然后指责她写得短。
// [[observation-tool-is-the-bug-2026-09-12]] 里她连着三次跟印记说
// 「我的字被截断了」，而那一次切的正是走查自己的眼睛。
func writingPieceBlockBody(c writingCard) string {
	if c.Snippet == nil {
		return "（这一段还没写）"
	}
	t := strings.TrimSpace(c.Snippet.Text)
	if t == "" {
		return "（这一段还没写）"
	}
	r := []rune(t)
	if len(r) <= writingPieceBlockRunes {
		return "她写的：" + t
	}
	return "她写的：" + string(r[:writingPieceBlockRunes]) +
		"……（这一段一共 " + itoa(len(r)) + " 字，后面还有，这里没有全列）"
}

// writingPriorCommentLines 列出落在这一块上的历史意见，并逐条说她做到了没有。
//
// 「做到了没有」用的是 0147 存下来的 `source_text`：那条意见是对着**那一版**
// 说的，所以只要这一段现在的字和当初不一样，她就动过了。
//
// 🚨 判错的方向不对称（[[feedback-staleness-asymmetry-2026-09-12]]）：
// **她做完了、产品说她没做，是最伤的那一个**。所以「不一样」就算动过，
// 不去猜她改得对不对 —— 改得对不对由这一轮重新判。
func writingPriorCommentLines(
	comments []sqlc.WritingComment,
	snippets []sqlc.WritingSnippet,
	focus sqlc.WritingOutline,
) string {
	snippetID, now, ok := writingSnippetOfBlock(snippets, focus.ID)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, c := range comments {
		if c.Scope != "block" || !c.SnippetID.Valid {
			continue
		}
		if uuid.UUID(c.SnippetID.Bytes) != snippetID {
			continue
		}
		var points []CommentPoint
		if len(c.Points) > 0 {
			_ = json.Unmarshal(c.Points, &points)
		}
		acted := writingSheActedOn(c.SourceText, now)
		for _, p := range points {
			if p.Kind != "issue" || strings.TrimSpace(p.Action) == "" {
				continue
			}
			state := "她还没动这一段。"
			if acted {
				state = "她已经改过这一段了。"
			}
			b.WriteString("- 说的是：" + p.Text + "\n  让她做：" + p.Action + "\n  " + state + "\n")
		}
	}
	return b.String()
}

// writingSnippetOfBlock 找这一块的片段 id 和它此刻的正文。
func writingSnippetOfBlock(snippets []sqlc.WritingSnippet, outlineID uuid.UUID) (uuid.UUID, string, bool) {
	for _, s := range snippets {
		if s.OutlineID.Valid && uuid.UUID(s.OutlineID.Bytes) == outlineID {
			return s.ID, s.Text, true
		}
	}
	return uuid.Nil, "", false
}

// writingSheActedOn：这一段和写那条意见时读到的那一版**不一样**了。
//
// 空的 source_text = 0147 之前的老行，不知道那一版长什么样 —— 当她没动，
// 理由同上：宁可多说一次，不要误判她做完了。
func writingSheActedOn(sourceText, now string) bool {
	old := strings.TrimSpace(sourceText)
	if old == "" {
		return false
	}
	return strings.TrimSpace(now) != old
}

// itoa 避免为一个数字引 strconv —— 这个文件里只用到这一处。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
