package api

import (
	"sort"
	"strings"

	"github.com/google/uuid"

	"mindimprint/api/internal/store/sqlc"
)

// writingMaterialDepth is the outline depth at and below which a node is her
// material (an example, a number, a source), not a paragraph of its own.
// Mirrors apps/lite-web/src/writings/slots.ts MATERIAL_DEPTH.
const writingMaterialDepth = 2

// 开头 / 结尾 两张卡在结构图上没有对应的节点时，片段存在这两个保留位置上。
// 结构图的位置是 0..N-1，自由段落从 1000 起（SnippetsStage 的
// FREE_POSITION_BASE），这两个数两边都够不着。和 slots.ts 的
// OPENING_POSITION / CLOSING_POSITION 必须一致。
const (
	writingOpeningPosition int32 = 998
	writingClosingPosition int32 = 999
)

// 卡片的种类。和 slots.ts 的 SlotKind 一致。
const (
	writingCardOpening = "opening"
	writingCardPoint   = "point"
	writingCardClosing = "closing"
	writingCardFree    = "free"
)

// writingRoleIsExample 判断一个节点的 role 说的是不是「一份材料」
// （例子、经历、数据、研究、报道……），而不是一条分论点。
//
// 🚨 2026-09-18 产品负责人：「我有两个例子，结果就变成了两段。」
// 规划的时候例子常常被直接挂在中心论点下面（深度 1），和分论点平级，
// 于是段落那一步把**每个例子**都当成了一段，整篇只剩「例子一 / 例子二」两块。
// 例子是写进某一段里的东西，不是一段。判据只看 role 和出处：
// 带出处的一定是她找回来的材料。和 slots.ts 的 roleIsExample 同一张词表。
func writingRoleIsExample(role, source string) bool {
	if strings.TrimSpace(source) != "" {
		return true
	}
	r := strings.ToLower(role)
	for _, kw := range writingExampleRoleWords {
		if strings.Contains(r, kw) {
			return true
		}
	}
	return false
}

var writingExampleRoleWords = []string{
	"例", "经历", "的事", "事件", "故事", "材料", "数据", "研究", "报道", "访谈", "调查",
	"案例", "引用", "名言", "人物", "史实", "素材", "证据", "新闻", "实验", "统计", "场景", "现象",
	"example", "experience", "evidence", "data", "study", "research", "report",
	"story", "quote", "case", "survey", "statistic", "source",
}

var (
	writingOpeningRoleWords = []string{"开头", "引言", "开篇", "钩子", "导入", "opening", "hook", "introduction", "intro"}
	writingClosingRoleWords = []string{"结尾", "结论", "总结", "收尾", "落点", "结语", "closing", "conclusion", "ending"}
)

func roleHasAny(role string, words []string) bool {
	r := strings.ToLower(role)
	for _, w := range words {
		if strings.Contains(r, w) {
			return true
		}
	}
	return false
}

// writingNodeIsMaterial：这个节点是材料（写进某一段），不是一段。
// 深度 ≥ 2 的一定是；更上层的看 role —— 挂在中心论点下面、甚至落在最上层的
// 例子也是材料。
func writingNodeIsMaterial(o sqlc.WritingOutline) bool {
	if o.Depth >= writingMaterialDepth {
		return true
	}
	return writingRoleIsExample(o.Role, o.Source)
}

// writingCard 是段落那一步屏幕上的一张卡。
type writingCard struct {
	Kind string
	// Node 是这张卡对应的结构图节点；虚拟的开头/结尾卡和自由段落没有。
	Node *sqlc.WritingOutline
	// Position 是这张卡的片段存在哪个位置上（虚拟卡用保留位置）。
	Position int32
	Snippet  *sqlc.WritingSnippet
}

// Label 是给陪练看的那一小段名字。
func (c writingCard) Label() string {
	switch c.Kind {
	case writingCardOpening:
		return "开头"
	case writingCardClosing:
		return "结尾"
	}
	if c.Node != nil {
		return strings.TrimSpace(c.Node.Text)
	}
	return ""
}

// writingCards 把结构图排成段落那一步的一叠卡片 —— 一篇文章的写作结构：
//
//	开头（提出中心论点） → 每条分论点一张（它下面的例子是这一段的材料）→ 结尾
//
// 🚨 2026-09-18 产品负责人：「应该根据例子设计一个写作结构，而不是把例子直接
// 变成段落。」原来是**每个节点一块**：两个例子就是两块，没有开头也没有结尾。
// 从标准篇章骨架（总—分—总）派生卡片是确定性的系统步骤（AGENTS.md 铁律
// 适用边界）；卡片上的字仍然只有她的节点文字和骨架的名字，一个字都不是 AI 写的。
//
// 规则（和 slots.ts 的 buildSlots 逐条一致）：
//   - 深度 0：开头类 role → 开头卡；结尾类 role → 结尾卡；第一个其余节点是中心论点，
//     它就是开头卡要提出的那句话（没有单独的开头节点时，开头卡就绑在它上面）；
//     再有别的深度 0 节点，各自一张主体卡。
//   - 深度 1 的分论点 → 一张主体卡。
//   - 材料（深度 ≥ 2，或挂在上层的例子）并进前面最近的主体卡；前面还没有主体卡的
//     例子自己成一张主体卡（这一段要先说清它证明了什么）。
//   - 她已经在某个节点上写过字的，那个节点一定有自己的卡（老数据里材料节点上
//     写过的段落不能消失）。
//   - 没有结尾节点就补一张虚拟结尾卡；没有任何可绑定的开头节点就补虚拟开头卡。
//   - 没挂在当前结构图上的片段排在最后，按位置。
//
// 结构图是空的就没有卡：段落那一步照旧显示「先去理思路」。
func writingCards(outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet) []writingCard {
	sorted := append([]sqlc.WritingOutline(nil), outline...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })

	snippetOf := map[uuid.UUID]*sqlc.WritingSnippet{}
	for i := range snippets {
		s := &snippets[i]
		if s.OutlineID.Valid {
			id := uuid.UUID(s.OutlineID.Bytes)
			if _, seen := snippetOf[id]; !seen {
				snippetOf[id] = s
			}
		}
	}
	inOutline := map[uuid.UUID]bool{}
	for _, o := range sorted {
		inOutline[o.ID] = true
	}
	var openingFree, closingFree *sqlc.WritingSnippet
	var free []sqlc.WritingSnippet
	for i := range snippets {
		s := snippets[i]
		if s.OutlineID.Valid && inOutline[uuid.UUID(s.OutlineID.Bytes)] {
			continue
		}
		if !s.OutlineID.Valid && s.Position == writingOpeningPosition && openingFree == nil {
			openingFree = &snippets[i]
			continue
		}
		if !s.OutlineID.Valid && s.Position == writingClosingPosition && closingFree == nil {
			closingFree = &snippets[i]
			continue
		}
		free = append(free, s)
	}
	sort.SliceStable(free, func(i, j int) bool { return free[i].Position < free[j].Position })

	cardFor := func(kind string, o sqlc.WritingOutline) writingCard {
		node := o
		return writingCard{Kind: kind, Node: &node, Position: o.Position, Snippet: snippetOf[o.ID]}
	}
	written := func(o sqlc.WritingOutline) bool {
		s := snippetOf[o.ID]
		return s != nil && strings.TrimSpace(s.Text) != ""
	}

	var cards []writingCard
	if len(sorted) > 0 {
		// 找开头卡要绑的节点：显式的开头节点优先，否则中心论点。
		var openingNode, thesisNode *sqlc.WritingOutline
		for i := range sorted {
			o := sorted[i]
			if o.Depth != 0 {
				continue
			}
			if roleHasAny(o.Role, writingOpeningRoleWords) {
				if openingNode == nil {
					openingNode = &sorted[i]
				}
				continue
			}
			if roleHasAny(o.Role, writingClosingRoleWords) || writingNodeIsMaterial(o) {
				continue
			}
			if thesisNode == nil {
				thesisNode = &sorted[i]
			}
		}
		opening := writingCard{Kind: writingCardOpening, Position: writingOpeningPosition, Snippet: openingFree}
		switch {
		case openingNode != nil:
			opening = cardFor(writingCardOpening, *openingNode)
		case thesisNode != nil:
			opening = cardFor(writingCardOpening, *thesisNode)
		}
		cards = append(cards, opening)
		bound := func(o sqlc.WritingOutline) bool { return opening.Node != nil && opening.Node.ID == o.ID }

		var closings []writingCard
		lastBody := -1 // index into cards of the most recent body card
		for _, o := range sorted {
			if bound(o) {
				continue
			}
			switch {
			case o.Depth == 0 && roleHasAny(o.Role, writingOpeningRoleWords):
				// 第二个开头节点：没写过字就算开头的材料，写过就单独一张。
				if written(o) {
					cards = append(cards, cardFor(writingCardOpening, o))
				}
			case o.Depth == 0 && roleHasAny(o.Role, writingClosingRoleWords):
				closings = append(closings, cardFor(writingCardClosing, o))
			case thesisNode != nil && o.ID == thesisNode.ID:
				// 有显式开头节点时，中心论点是开头要提出的那句话；写过字就单独一张。
				if written(o) {
					cards = append(cards, cardFor(writingCardPoint, o))
				}
			case writingNodeIsMaterial(o):
				if written(o) || lastBody < 0 {
					cards = append(cards, cardFor(writingCardPoint, o))
					lastBody = len(cards) - 1
				}
			default:
				cards = append(cards, cardFor(writingCardPoint, o))
				lastBody = len(cards) - 1
			}
		}
		if len(closings) == 0 {
			closings = append(closings, writingCard{Kind: writingCardClosing, Position: writingClosingPosition, Snippet: closingFree})
		} else if closingFree != nil {
			// 结尾节点是后来才加的：保留位置上那一段仍要看得见。
			free = append([]sqlc.WritingSnippet{*closingFree}, free...)
		}
		cards = append(cards, closings...)
		if opening.Node != nil && openingFree != nil {
			free = append([]sqlc.WritingSnippet{*openingFree}, free...)
		}
	} else {
		// 没有结构图（带进来的一篇、或者还没规划）：只有她写过的片段。
		for _, s := range []*sqlc.WritingSnippet{openingFree, closingFree} {
			if s != nil {
				free = append(free, *s)
			}
		}
		sort.SliceStable(free, func(i, j int) bool { return free[i].Position < free[j].Position })
	}

	for i := range free {
		s := free[i]
		cards = append(cards, writingCard{Kind: writingCardFree, Position: s.Position, Snippet: &s})
	}
	return cards
}

// writingBlockNumbers numbers the cards the 段落 screen shows, keyed by
// snippet id — 1-based, in writingCards order. 印记 说「第 N 块」必须和屏幕
// 对得上，所以两边用同一条规则（slots.ts buildSlots）。
func writingBlockNumbers(outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet) map[uuid.UUID]int {
	out := map[uuid.UUID]int{}
	for i, c := range writingCards(outline, snippets) {
		if c.Snippet != nil {
			out[c.Snippet.ID] = i + 1
		}
	}
	return out
}

// writingSnippetsInCardOrder 是拼成稿用的顺序：卡片的顺序，不是存储位置的顺序
// （虚拟开头卡存在 998 上，按位置排它会落到正文后面）。
func writingSnippetsInCardOrder(outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet) []sqlc.WritingSnippet {
	out := make([]sqlc.WritingSnippet, 0, len(snippets))
	seen := map[uuid.UUID]bool{}
	for _, c := range writingCards(outline, snippets) {
		if c.Snippet != nil && !seen[c.Snippet.ID] {
			seen[c.Snippet.ID] = true
			out = append(out, *c.Snippet)
		}
	}
	// 一个节点上的第二条片段（老数据）不在任何卡上 —— 仍然拼进去，不丢字。
	for _, s := range snippets {
		if !seen[s.ID] {
			out = append(out, s)
		}
	}
	return out
}
