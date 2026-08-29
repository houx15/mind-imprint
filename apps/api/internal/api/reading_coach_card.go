package api

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// reading_coach_card.go — 聊天里那张可点的卡片，以及它的守门人。
//
// 一张卡片值得建，当且仅当它的答案不进文章就产生不出来。choose_span 的选项
// 是文章里真实存在的句子，这才是逼她真的去读的那个东西；模型随手编一句读着
// 很像的话，就把这个保证悄悄拆掉了，而学生永远不会知道。所以这里跟
// validateReadingPicks / validateReadingQuestions 是同一个思路：
// **保证靠输出类型，不靠嘴上叮嘱** —— 不求模型好好引用，而是逐条核对，
// 过不了的整张丢掉。丢掉不是错误：这一轮照常成功，她只是收到一条没有卡片的回复。

// coachCardType 三种：
//   - choose_span     —— 从文章的几句原话里点一句（options 必填）
//   - pick_in_article —— 请她自己去正文里划一句（没有 options）
//   - short_text      —— 请她用自己的话写一小段（没有 options）
const (
	coachCardChooseSpan    = "choose_span"
	coachCardPickInArticle = "pick_in_article"
	coachCardShortText     = "short_text"
)

const (
	// coachCardPromptMaxRunes 按 rune 数，不是 byte 数 —— 正文是中文，
	// 按 byte 算等于只给了三分之一的额度。
	coachCardPromptMaxRunes = 60
	// coachCardMaxOptions 超过 4 个就不是「点一句」而是阅读理解选择题了。
	coachCardMaxOptions = 4
	// coachCardMinOptions 存活不到 2 个，剩下的那一个就没有可选性可言 ——
	// 跟 validateReadingQuestions 的地板同一个理由：一份单薄的东西比没有更糟。
	coachCardMinOptions = 2
	// coachCardMinQuoteRunes 一两个字确实也是文章的子串，但那不是「一句话」，
	// 渲染出来是张废卡。
	coachCardMinQuoteRunes = 4
)

// coachCardOption 是卡片上的一个选项：文章里某一段（BlockID）的某一句原话
// （Quote）。跟 readingPick 同形，也是同一个理由。
type coachCardOption struct {
	BlockID string `json:"blockId"`
	Quote   string `json:"quote"`
}

// coachCard 是 印记 在这一轮回复里附带的一张卡片。**它不带 answer key** ——
// 没有对错、没有分数（铁律②）；卡片只是把「想」这一步交回给她。
type coachCard struct {
	Type    string            `json:"type"`    // choose_span | pick_in_article | short_text
	Prompt  string            `json:"prompt"`  // ≤ 60 字的一句话提问
	Options []coachCardOption `json:"options"` // choose_span 专用
}

// validateCoachCard 校验模型给出的这张卡片，不合格返回 nil —— 静默丢弃，
// 不报错、不渲染残卡。返回的是一张新卡片，调用方手里那张不会被就地改写。
//
// 规则：
//   - Type 必须是三种之一；
//   - Prompt 去空白后非空，且 ≤ 60 runes；
//   - choose_span：每个 Quote 必须是**它自己那个 BlockID** 的字面子串
//     （挂错段落 = 不算）、**落在从句边界上**（见 coachCardQuoteIsClause）、
//     去空、去太短、去重（含**包含式**去重：一个选项是另一个的子串就丢掉短的）、
//     截断到 4 个，存活 < 2 → 整张丢掉；
//   - pick_in_article / short_text：忽略并清空 options
//     （问题本身就是「去文章里找」，给了选项反而把这件事替她做了）。
func validateCoachCard(c *coachCard, blocks []Block) *coachCard {
	if c == nil {
		return nil
	}
	switch c.Type {
	case coachCardChooseSpan, coachCardPickInArticle, coachCardShortText:
	default:
		return nil
	}
	prompt := strings.TrimSpace(c.Prompt)
	if prompt == "" || utf8.RuneCountInString(prompt) > coachCardPromptMaxRunes {
		return nil
	}
	if c.Type != coachCardChooseSpan {
		return &coachCard{Type: c.Type, Prompt: prompt}
	}

	byID := make(map[string]string, len(blocks))
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	seen := make(map[string]bool, len(c.Options))
	kept := make([]coachCardOption, 0, len(c.Options))
	for _, o := range c.Options {
		q := strings.TrimSpace(o.Quote)
		if q == "" || utf8.RuneCountInString(q) < coachCardMinQuoteRunes {
			continue
		}
		body, ok := byID[o.BlockID]
		if !ok || !coachCardQuoteIsClause(body, q) {
			continue
		}
		// 按她**看得见的那句话**去重：同一句从两个段落各来一次，屏幕上就是
		// 两个一模一样的选项，点哪个都没有区别。
		if seen[q] {
			continue
		}
		seen[q] = true
		kept = append(kept, coachCardOption{BlockID: o.BlockID, Quote: q})
	}
	// 包含式去重，在截断到 4 之前跑：两个各自都落在合法边界上的重叠片段
	// （「白天吸热、夜里放热」和「夜里放热」）边界规则合并不掉，但摆在同一张
	// 卡片上就是一组套娃 —— 「挑一句」这件事当场变得莫名其妙。留长的那个：
	// 它信息更完整，短的那半句她在长的里面照样读得到。
	out := make([]coachCardOption, 0, coachCardMaxOptions)
	for i, o := range kept {
		if coachCardIsSwallowed(o.Quote, kept, i) {
			continue
		}
		out = append(out, o)
		if len(out) == coachCardMaxOptions {
			break
		}
	}
	if len(out) < coachCardMinOptions {
		return nil
	}
	return &coachCard{Type: c.Type, Prompt: prompt, Options: out}
}

// coachCardIsSwallowed —— quote 是不是 kept 里**另一个**选项的子串。
// 相等的两条在这之前已经被 seen 去掉了，所以这里只认真子串（更长的那个）。
func coachCardIsSwallowed(quote string, kept []coachCardOption, self int) bool {
	for j := range kept {
		if j == self {
			continue
		}
		if len(kept[j].Quote) > len(quote) && strings.Contains(kept[j].Quote, quote) {
			return true
		}
	}
	return false
}

// coachCardIsBoundary —— 从句边界字符。两套标点都要有：正文可能是中文，也可能
// 是英文（或者中英混排的一段）。
func coachCardIsBoundary(r rune) bool {
	switch r {
	case '，', '。', '！', '？', '；', '：', '、', '\n',
		',', '.', '!', '?', ';', ':':
		return true
	}
	return false
}

// coachCardIsSkippable —— 找边界的路上可以跳过不计的字符：空白，以及成对的
// 引号 / 括号。
//
// 为什么引号必须跳过：`他说：“城市在夜里更热。”` 里那一句的边界标点（`：`）
// 落在引号**外面**，引号本身不是边界字符。不跳过它，这一整类正常引文——中文
// 「“ ”」「‘ ’」「「 」」「『 』」、括号、英文 `" '`——全都会被误杀，而误杀是
// 静默的：选项被丢 → 存活不足 2 → 整张卡片消失，屏幕上看起来就像 印记 这一轮
// 没想出卡片。跳过引号并不放宽「一句话」这件事：引号里从句子中间切的窗口，
// 跳过引号之后撞到的仍然是一个汉字，照样过不了。
func coachCardIsSkippable(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	switch r {
	case '“', '”', '‘', '’', '「', '」', '『', '』', '（', '）', '(', ')', '"', '\'':
		return true
	}
	return false
}

// coachCardQuoteIsClause —— Quote 必须是 body 的字面子串，**并且落在从句边界上**。
//
// 为什么光有 ≥4 runes 的地板不够：地板管的是**长度**，不是「是不是一句话」。
// 「表以沥青和混凝土」从词中间切开，两头都不在标点上，却确确实实是原文的子串 ——
// 这种窗口靠扫几个字就能凑出来，而一句话必须被当作一个整体读过。整个校验器存在
// 的理由就是后面这件事（不进文章就答不出来），所以边界这一层不能省。
//
// 边界的定义（三头都是「一句话」的合法收口）：
//   - 开头：block 的开头，或前面紧挨着一个边界字符；
//   - 结尾：block 的结尾，或后面紧挨着一个边界字符，或它**自己以边界字符收尾**
//     （模型经常把句号一起引进来，那是正常引用，不是毛病）。
//
// 🚨 故意往**松**了收：同一句话在段里出现多次时，只要**有一次**落在边界上就算数；
// 边界字符和引文之间夹着的空白（英文 "…day. They…" 的那个空格）跳过不计，
// 成对的引号 / 括号（`他说：“…”` 的那对引号）同样跳过不计（见 coachCardIsSkippable）。
// 收得过紧是看不见的 —— 误杀的卡片不报错、不打日志，看起来就像模型这一轮
// 没想出卡片；宁可放过一个窗口，也不能让正常的句子静悄悄消失。
func coachCardQuoteIsClause(body, quote string) bool {
	if quote == "" {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(quote)
	endsOnItsOwnBoundary := coachCardIsBoundary(last)
	for off := 0; off <= len(body)-len(quote); {
		i := strings.Index(body[off:], quote)
		if i < 0 {
			return false
		}
		start := off + i
		if coachCardClauseStarts(body, start) &&
			(endsOnItsOwnBoundary || coachCardClauseEnds(body, start+len(quote))) {
			return true
		}
		off = start + 1
	}
	return false
}

// coachCardClauseStarts —— start 这个字节位置是不是一句话的开头。
func coachCardClauseStarts(body string, start int) bool {
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(body[:start])
		if coachCardIsBoundary(r) {
			return true
		}
		if !coachCardIsSkippable(r) {
			return false
		}
		start -= size
	}
	return true // block 的开头
}

// coachCardClauseEnds —— end 这个字节位置是不是一句话的收口。
func coachCardClauseEnds(body string, end int) bool {
	for end < len(body) {
		r, size := utf8.DecodeRuneInString(body[end:])
		if coachCardIsBoundary(r) {
			return true
		}
		if !coachCardIsSkippable(r) {
			return false
		}
		end += size
	}
	return true // block 的结尾
}

// coachMessagePayload is what a chat message carries besides its words
// (`atom_message.payload`, migration 0106). On the AI side that is the card it
// just wrote. It is an envelope rather than the bare card on purpose: a reader
// holding only the JSON has to be able to tell 「这条消息带了一张卡」 apart
// from whatever else a message will carry later (her answer to one).
//
// 🚨 Deliberately NOT atom_card: `atom_card_one_open_idx` (0096) permits one
// open card per atom, so a chat card stored there would deadlock every lens
// summon — and `card_id` there must resolve in cards.ByID / CARD_REGISTRY,
// which a card the model wrote on the spot never will.
type coachMessagePayload struct {
	Card *coachCard `json:"card,omitempty"`
	// Answer is the other half of the envelope: on HER side of the transcript,
	// what she tapped. The words themselves are already in `content` (as `> `
	// lines when they are the article's), but a reader of the content alone
	// cannot tell 「她点了卡片上的第二个选项」 apart from 「她引用了一句然后打字」.
	// Storing the structured answer is what lets the room re-render the card
	// she already answered after a refresh, instead of showing a live card
	// waiting for a tap she has made.
	Answer *coachCardAnswer `json:"answer,omitempty"`
}

// coachCardAnswer is her answer to a chat card: which card it was, what it
// asked, and what she chose. `Choice` is ARTICLE TEXT for choose_span and
// pick_in_article, and HER OWN words for short_text — which is exactly why
// nothing downstream is allowed to trust this field's `Type` to tell the two
// apart. See composeCardAnswerMessage: what goes into the transcript is
// classified by CHECKING the choice against the article, not by believing
// the type the client declared.
type coachCardAnswer struct {
	Type    string `json:"type,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
	Choice  string `json:"choice,omitempty"`
	BlockID string `json:"blockId,omitempty"`
}

// coachCardAnswerPayload renders her answer into the jsonb column's bytes,
// the mirror of coachCardPayload. Nil answer → nil bytes → SQL NULL.
func coachCardAnswerPayload(a *coachCardAnswer) []byte {
	if a == nil {
		return nil
	}
	b, err := json.Marshal(coachMessagePayload{Answer: a})
	if err != nil {
		// Same reasoning as coachCardPayload: a struct of strings cannot fail
		// to marshal, and if it somehow did, her turn still stands — the words
		// are in `content`, which is the part the next turn actually reads.
		return nil
	}
	return b
}

// coachCardPayload renders a card into the jsonb column's bytes. A nil card
// gives nil bytes, which the column stores as SQL NULL — that is the whole
// reason the column is nullable: carrying nothing is the normal case, not one
// every caller has to build an empty shell for.
func coachCardPayload(c *coachCard) []byte {
	if c == nil {
		return nil
	}
	b, err := json.Marshal(coachMessagePayload{Card: c})
	if err != nil {
		// A struct of strings cannot fail to marshal; if it somehow did, the
		// turn is still hers — she loses the card, not the reply.
		return nil
	}
	return b
}
