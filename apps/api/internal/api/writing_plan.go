package api

// writing_plan.go — 规划对话：结构那一步的全部。
//
// 2026-08-27 的产品裁定（第二轮，推翻了同一天早些时候的结构库选择器）：
//
//   > 结构像 planning，不是一副固定的骨架。先引导学生想，再把结构**长**出来。
//
// 前一版让学生从一张写死的表里挑一副骨架，块名是「你承认它哪一部分是对的」
// 这种教科书黑话。对一个初中生来说那既读不懂、也不是在思考——那是在填表。
//
// 现在这一步是一段对话：印记 一次问一个问题，她回答，她说过的东西**一个一个
// 长到右边那张思维导图上**。图最后就是提纲的初始形状。
//
// ## 三条硬规则
//
//  1. **只加，不改不删。** 这一路唯一能碰提纲的写操作是
//     InsertWritingOutlineNode。印记 能把她刚说的话加成节点，但改不了、删不掉
//     任何一个既有节点——那是她自己的编辑权。这不是 prompt 里的请求，是这个
//     文件里根本没有那两个调用。
//  2. **节点文字必须来自她刚说的那句话。** 可以精简成短语，不能替她想出她没
//     说过的分论点。所以这一轮的提示词只喂**她最新的一条消息**加当前的图：
//     她这轮什么都没说，就没有东西可以长出来。
//  3. **方法名可以提前说，但绝不能做成菜单/选项卡。** 她给了三条平行的理由，
//     印记 说「你这三条是并列的」——这仍是最扎实的一次；卡住时提前说一个方法
//     名不算破例。真正不允许的是把「并列/递进/正反」整张表甩给她挑，那又变
//     回填表了。
//
// ## 提示词里那套写作学（Level 1 / Level 2）
//
// 中学写作的结构其实是两层，学生真正卡住的是第二层：
//
//   - 篇章骨架：立场式 / 起承转合 / 钩子式 / 记叙
//   - 单个分论点怎么展开：并列、递进、正反、举例（PEE）、让步、因果
//
// 两层都写进系统提示词，但**只作为 印记 自己的知识**——用来决定问什么、什么
// 时候往深里追一层，以及事后怎么命名她已经做出来的东西。学生一次也不会看到
// 这两张表。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/httpx"
	"mindimprint/api/internal/store/sqlc"
	"mindimprint/api/internal/vocab"
)

// writingPlanTurnsWindow bounds the transcript fed to the planning turn. Same
// reasoning as writingTurnsWindow: lite has no compaction layer, so this is
// the only thing bounding prompt growth on a long planning conversation.
const writingPlanTurnsWindow = 16

// writingPlanMaxNewNodes caps one turn's additions. A turn that tried to add
// eight nodes has stopped having a conversation and started transcribing —
// and 铁律③ (one question at a time) implies one small step at a time.
const writingPlanMaxNewNodes = 4

// writingPlanMaxDepth is the map's depth ceiling. Depth 0 is document-ordered
// siblings — opening, thesis, landing — not just the thesis alone; depth 1 is
// 分论点; depth 2 is 论据. Three levels is what a middle-school piece needs;
// deeper is an org chart.
const writingPlanMaxDepth = 2

const writingPlanSystem = `你是「印记」，正在陪一个中学生**规划**一篇文章。这一步不是写，是想清楚要写什么、按什么顺序写。

右边有一张思维导图，会随着她说的话一点点长出来。你每轮说的话和你往图上加的节点，都出现在她眼前。

## 你怎么问

- **一次只问一个问题。** 问完就停，等她回答。
- 从最上面开始：先帮她把**这篇到底要说的那一句话**定下来（不是题目，是主张/她想让读者明白的那件事）。定下来了，才往下问。
- 然后问她打算**用哪几件事来说明**。她给了两三条，就够往下走了。
- 之后一个一个点地问：这一点你打算讲什么？有没有你自己见过、经历过的事？
- 用她自己提过的人、事、场景来问。别另起炉灶。

## 提示，不是菜单

她卡住的时候，用【可用的方法】里真正的名字给她一两个具体的路子，而不是甩一张
表去选：
- 「这条理由可以用『并列论证』——几件事摆在一起同等重要；也可以用『正反对比』
  ——一好一坏两个例子放在一起看。哪一种更接近你手上的材料？」
- 「你可以先讲一件小事，再从那件事说开去，这是『钩子式开头』。」
不要把方法名堆成一整张表甩给她挑——一次给一两个、说清楚为什么、给她一个真选择。

## 结构的名字，做出来之后点最准

她做出东西之后，顺口点一句这是什么，让名字和她自己的东西对上，记得最牢：
- 三条平行的理由 →「你这三条是并列的，稳，但要小心三条一样重就没有高潮。」
- 一好一坏两个例子 →「这是正反对比。」
- 先承认再反驳 →「你这是让步，写出来最有说服力。」
一次最多点一句，别上课。

这不是说不能先说方法名——她卡住的时候提前说一个，是在给她一条路走；只是
「等她做出来再点」永远是最扎实的一次，因为名字这时候是在描述真实发生的事，
不是在预告一张要填的表。

## 你心里要装着「一整篇」

一篇写完的文章要为读者做三件事：**开头让他愿意读下去，中间真的在论证，
结尾让他带走点东西。** 这是你的判断力，不是一张要逐项打勾的清单。

每一轮，你看一眼整张图，只挑**这篇现在最需要的那一件事**说。可能是
「读者一上来不知道为什么要关心这件事」，可能是「理由二和理由三其实是同一条」，
也可能是「理由一底下什么都没有」。**有时候答案是什么都不缺，让她去写。**

开头和结尾要等主体有了再谈——不知道要把人领进哪里，就没法决定怎么开门。

她想去写了，就让她去写；或者她说「先这样」，就往下走。规划不是关卡。

## 怎么说话（这条比什么都重要）

你是老师，不是问答机器。每次开口都要做到四件事：
① 说清这件事为什么重要；② 说出真正的方法名（下面【可用的方法】里的，别自己造词）；
③ 给她一个真的选择，她也可以不选；④ 主动提出可以举例子一起看。

不要这样说：「有人会从一个具体场景切进去，有人直接抛个问题。你这篇你想怎么进？」
要这样说：「对于一篇文章来说，有意思的开头非常重要。留悬念、设问、开门见山等，
都是常见的方式。你想尝试哪一种？或者需要我给几个具体的案例我们一起来学习一下
这几种方法吗？」

一次仍然只问**一个**问题——「一次只问一个」说的是问题的数量，从来不是让你少说话。

## 你绝对不能做的事

- **不要替她写正文。** 不给开头句、不给段落、不给论点。你只问，只整理她说过的话。
- **不要往图上加她没说过的内容。** 节点文字必须是她刚说的那句话的精简，不能是你替她想的点子。她这轮没说新东西，就一个节点都别加。
- 不要一次问好几个问题。
- 不要说"作为AI"，不要空夸。

## 输出格式

只输出一个 JSON 对象：

{"reply":"你要对她说的话","add":[{"parentId":"","text":"节点文字","role":"这块是什么"}]}

- reply：不超过 200 字，一次一个问题。
- add：这一轮要往图上加的节点，**0 到 %d 个**；没有就给空数组。
- parentId：父节点的 id，逐字取自下面【当前的图】里给出的 id。留空字符串＝加在最上层。
  最上层不止中心论点：开头、结尾也都是最上层的块，按它们在文章里的先后排。
- text：**她自己的话的精简**，不超过 30 字。
- role：一句大白话说这块是什么（「中心论点」「一条理由」「她自己的经历」「反方会说的话」）。不要用生僻术语。

不要输出对象以外的任何文字或代码块标记。`

// buildWritingPlanPrompt renders the current map (with ids, so the model can
// point at a parent) plus the windowed conversation.
func buildWritingPlanPrompt(wr sqlc.Writing, rows []sqlc.WritingOutline, msgs []sqlc.AtomMessage, studentText string) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("她一开始说想写的是：" + t + "\n")
	}
	if wr.Lang == "en" {
		b.WriteString("这篇用英文写（但你和她用中文讨论）。\n")
	} else {
		b.WriteString("这篇用中文写。\n")
	}
	if wr.TargetWords != nil {
		b.WriteString("目标篇幅：约 " + strconv.Itoa(int(*wr.TargetWords)) + " 字（只用来判断要几条分论点，别追着她凑字数）。\n")
	}

	b.WriteString("\n【当前的图】\n")
	if len(rows) == 0 {
		b.WriteString("（还是空的。先帮她把这篇要说的那一句话定下来。）\n")
	} else {
		for _, r := range rows {
			indent := strings.Repeat("  ", int(r.Depth))
			line := indent + "- id=" + r.ID.String() + " · " + r.Text
			if strings.TrimSpace(r.Role) != "" {
				line += "（" + r.Role + "）"
			}
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n【可用的方法】（只能用这里的名字，别造新词）\n")
	for _, m := range vocab.All() {
		if m.AppliesTo == "opening" || m.AppliesTo == "closing" || m.AppliesTo == "body" || m.AppliesTo == "any" {
			b.WriteString("- " + m.Name + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
		}
	}

	b.WriteString("\n【你们刚才聊的】\n")
	tail := msgs
	if len(tail) > writingPlanTurnsWindow {
		tail = tail[len(tail)-writingPlanTurnsWindow:]
	}
	any := false
	for _, m := range tail {
		var who string
		switch m.Role {
		case "student":
			who = "她"
		case "ai":
			who = "你"
		default:
			continue
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			b.WriteString(who + "：" + s + "\n")
			any = true
		}
	}
	if !any {
		b.WriteString("（还没聊过。）\n")
	}

	b.WriteString("\n【她刚刚说的】\n" + studentText + "\n")
	b.WriteString("\n只能从「她刚刚说的」这段话里提取节点。她这段话里没有新的点子，add 就给空数组。\n")
	return b.String()
}

type writingPlanAdd struct {
	ParentID string `json:"parentId"`
	Text     string `json:"text"`
	Role     string `json:"role"`
}

type writingPlanReply struct {
	Reply string           `json:"reply"`
	Add   []writingPlanAdd `json:"add"`
}

// parseWritingPlanReply decodes and clamps. Anything it cannot validate is
// DROPPED rather than guessed at: an unparseable parentId would otherwise
// silently reparent a node somewhere she never put it.
func parseWritingPlanReply(text string) (writingPlanReply, bool) {
	c := strings.TrimSpace(text)
	if strings.HasPrefix(c, "```json") {
		c = strings.TrimLeft(strings.TrimPrefix(c, "```json"), " \t\r\n")
	} else if strings.HasPrefix(c, "```") {
		c = strings.TrimLeft(c[3:], " \t\r\n")
	}
	if strings.HasSuffix(c, "```") {
		c = strings.TrimRight(c[:len(c)-3], " \t\r\n")
	}
	if i := strings.IndexByte(c, '{'); i > 0 {
		c = c[i:]
	}
	if j := strings.LastIndexByte(c, '}'); j >= 0 && j < len(c)-1 {
		c = c[:j+1]
	}
	var got writingPlanReply
	if err := json.Unmarshal([]byte(strings.TrimSpace(c)), &got); err != nil {
		return writingPlanReply{}, false
	}
	got.Reply = strings.TrimSpace(got.Reply)
	if got.Reply == "" {
		// A turn with no reply is a turn that said nothing — surfaced as a
		// failure rather than rendered as an empty coach bubble.
		return writingPlanReply{}, false
	}
	kept := make([]writingPlanAdd, 0, len(got.Add))
	for _, a := range got.Add {
		a.Text = strings.TrimSpace(a.Text)
		a.Role = strings.TrimSpace(a.Role)
		a.ParentID = strings.TrimSpace(a.ParentID)
		if a.Text == "" {
			continue
		}
		kept = append(kept, a)
		if len(kept) == writingPlanMaxNewNodes {
			break
		}
	}
	got.Add = kept
	return got, true
}

// rootInsertPosition decides where a NEW top-level (depth-0) node lands
// among the existing rows: an opening-ish role goes to position 0 (first in
// document order, ahead of the thesis); everything else — 中心论点, a
// closing, or a role we don't recognise — keeps the old behaviour of
// appending at the end, in the order she produced them.
//
// This exists because 印记 asks about the opening only AFTER the thesis and
// body already exist (see "开头和结尾要等主体有了再谈" in the system prompt),
// so a plain end-append would always land the opening LAST — after the
// thesis and every 分论点 — even though the spec requires the opening to
// render as the piece's first block, ahead of 中心论点.
//
// It is a heuristic over the model's free-form `role` text, matched by
// substring against a handful of Chinese synonyms for "opening". Its failure
// mode if a role doesn't match is narrow: the block sorts to the end instead
// of the front — a mis-ordered top-level node, never a wrong parent and
// never a lost one.
func rootInsertPosition(role string, rows []sqlc.WritingOutline) int32 {
	for _, kw := range []string{"开头", "引言", "开篇", "钩子", "导入"} {
		if strings.Contains(role, kw) {
			return 0
		}
	}
	return int32(len(rows))
}

// insertPlanNode places one node under `parent` (nil = top level) and returns
// the created row.
//
// Position: a node under a parent goes at the END of that parent's subtree,
// so siblings keep the order she produced them in. The subtree ends at the
// first following row whose depth is <= the parent's — the same "flattened
// outline encodes a tree" convention the frontend renders from. A top-level
// (parent == nil) node's position instead goes through rootInsertPosition,
// since the document order for root nodes is not simply "arrival order" —
// an opening has to sort ahead of the thesis that was already there.
func insertPlanNode(
	ctx context.Context,
	q *sqlc.Queries,
	atomID uuid.UUID,
	rows []sqlc.WritingOutline,
	parent *sqlc.WritingOutline,
	text, role string,
) (sqlc.WritingOutline, []sqlc.WritingOutline, error) {
	depth := int32(0)
	var insertAt int32
	if parent != nil {
		depth = parent.Depth + 1
		if depth > writingPlanMaxDepth {
			depth = writingPlanMaxDepth
		}
		insertAt = parent.Position + 1
		for _, r := range rows {
			if r.Position > parent.Position && r.Depth > parent.Depth {
				insertAt = r.Position + 1
			} else if r.Position > parent.Position {
				break
			}
		}
	} else {
		insertAt = rootInsertPosition(role, rows)
	}
	if err := q.ShiftWritingOutlinePositions(ctx, sqlc.ShiftWritingOutlinePositionsParams{
		AtomID: atomID, Position: insertAt,
	}); err != nil {
		return sqlc.WritingOutline{}, rows, err
	}
	created, err := q.InsertWritingOutlineNode(ctx, sqlc.InsertWritingOutlineNodeParams{
		AtomID: atomID, Text: text, Role: role, Depth: depth, Position: insertAt,
	})
	if err != nil {
		return sqlc.WritingOutline{}, rows, err
	}
	// Keep the in-memory list in step so a second node in the same turn sees
	// the shifted positions rather than colliding with the first.
	next := make([]sqlc.WritingOutline, 0, len(rows)+1)
	for _, r := range rows {
		if r.Position >= insertAt {
			r.Position++
		}
		next = append(next, r)
	}
	next = append(next, created)
	for i := 1; i < len(next); i++ {
		for j := i; j > 0 && next[j].Position < next[j-1].Position; j-- {
			next[j], next[j-1] = next[j-1], next[j]
		}
	}
	return created, next, nil
}

// postWritingPlanTurn is POST /api/v1/writings/{id}/plan/turn.
//
// A spend endpoint (one model call), metered as purpose="plan_turn". Writes
// the student turn, the AI turn, and any new nodes in ONE transaction: a
// reply that persisted while its nodes did not would leave the map
// contradicting the conversation that produced it.
func (a *API) postWritingPlanTurn(w http.ResponseWriter, r *http.Request) {
	at, ok := a.loadOwnedWritingAtom(w, r)
	if !ok {
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

	var req struct {
		Text string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	studentText := strings.TrimSpace(req.Text)
	if studentText == "" {
		httpx.WriteError(w, r, httpx.ErrBadRequest("missing_text", "先说点什么，我在听。", nil))
		return
	}

	turnCtx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 150*time.Second)
	defer cancel()

	wr, err := a.d.Queries.GetWriting(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rows, err := a.d.Queries.ListWritingOutline(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	msgs, err := a.d.Queries.ListAtomMessages(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// §model-routing: planning is the hardest reasoning in this room — it has
	// to hear what she actually said, decide the ONE next question, and judge
	// where a point is too hollow to leave alone. Flagship, like the guiding
	// box, not the chaperone the ordinary chat turn uses.
	resolved, okResolve := a.resolveEval(turnCtx)
	if !okResolve {
		slog.Warn("writing plan turn: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	system := strings.Replace(writingPlanSystem, "%d", strconv.Itoa(writingPlanMaxNewNodes), 1)
	res, cerr := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: system},
			{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, msgs, studentText)},
		},
	})
	a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "plan_turn", resolved, res.Usage)
	if cerr != nil {
		slog.Warn("writing plan turn: provider call failed", "err", cerr,
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	parsed, okParse := parseWritingPlanReply(res.Text)
	if !okParse {
		slog.Warn("writing plan turn: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}

	byID := make(map[string]sqlc.WritingOutline, len(rows))
	for _, row := range rows {
		byID[row.ID.String()] = row
	}

	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	seq, err := qtx.NextAtomMessageSeq(turnCtx, at.ID)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq, Role: "student", Content: studentText,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if _, err := qtx.AppendAtomMessage(turnCtx, sqlc.AppendAtomMessageParams{
		AtomID: at.ID, Seq: seq + 1, Role: "ai", Content: parsed.Reply,
	}); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	live := rows
	added := make([]string, 0, len(parsed.Add))
	for _, node := range parsed.Add {
		var parent *sqlc.WritingOutline
		if node.ParentID != "" {
			p, found := byID[node.ParentID]
			if !found {
				// An id the model invented. Dropping the node is right:
				// attaching it to a guessed parent would put her sentence
				// somewhere she never put it, which is worse than losing it —
				// she can always say it again.
				slog.Warn("writing plan turn: unknown parentId, node dropped",
					"atom_id", at.ID, "parent_id", node.ParentID)
				continue
			}
			parent = &p
		}
		created, next, ierr := insertPlanNode(turnCtx, qtx, at.ID, live, parent, node.Text, node.Role)
		if ierr != nil {
			httpx.WriteError(w, r, ierr)
			return
		}
		live = next
		byID[created.ID.String()] = created
		added = append(added, created.ID.String())
	}
	if err := tx.Commit(turnCtx); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	out := make([]writingOutlineItemDTO, 0, len(live))
	for _, row := range live {
		out = append(out, toWritingOutlineItemDTO(row))
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"reply":    parsed.Reply,
		"outline":  out,
		"addedIds": added,
	})
}
