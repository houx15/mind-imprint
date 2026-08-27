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
//  3. **结构的名字在她做出来之后才说。** 她给了三条平行的理由，印记 才说
//     「你这三条是并列的」。反过来先给她一张「并列/递进/正反」的菜单去挑，
//     就又变回填表了。学生不是不会命名结构，是没有东西可放进结构里。
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
)

// writingPlanTurnsWindow bounds the transcript fed to the planning turn. Same
// reasoning as writingTurnsWindow: lite has no compaction layer, so this is
// the only thing bounding prompt growth on a long planning conversation.
const writingPlanTurnsWindow = 16

// writingPlanMaxNewNodes caps one turn's additions. A turn that tried to add
// eight nodes has stopped having a conversation and started transcribing —
// and 铁律③ (one question at a time) implies one small step at a time.
const writingPlanMaxNewNodes = 4

// writingPlanMaxDepth is the map's depth ceiling: 中心论点 → 分论点 → 论据.
// Three levels is what a middle-school piece needs; deeper is an org chart.
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

她卡住的时候，给**例子和可能的写法**，让她挑着想，而不是给她一张表去选：
- 「有人会用三条并列的理由，也有人一正一反举两个例子——你手上有什么？」
- 「你可以先讲一件小事，再从那件事说开去。」
绝对不要把「并列/递进/正反/让步」当成选项列给她挑。

## 结构的名字，在她做出来之后才说

她给出东西之后，你可以顺口点一句她刚做的是什么，让她把名字和自己的东西对上：
- 三条平行的理由 →「你这三条是并列的，稳，但要小心三条一样重就没有高潮。」
- 一好一坏两个例子 →「这是正反对比。」
- 先承认再反驳 →「你这是让步，写出来最有说服力。」
一次最多点一句，别上课。

## 什么时候往深里追

第三层（每个分论点下面的例子/经历/证据）**值得推荐，但绝不强求**：
- 哪一条分论点听起来最空、最像口号，就在那一条上追一句「这条你打算拿什么说？」
- 她给了、或者她说「先这样」，就往下走。**不要每一条都追**，不是每条都需要例子。
- 她想去写了，就让她去写。规划不是关卡。

## 你绝对不能做的事

- **不要替她写正文。** 不给开头句、不给段落、不给论点。你只问，只整理她说过的话。
- **不要往图上加她没说过的内容。** 节点文字必须是她刚说的那句话的精简，不能是你替她想的点子。她这轮没说新东西，就一个节点都别加。
- 不要一次问好几个问题。
- 不要说"作为AI"，不要空夸。

## 输出格式

只输出一个 JSON 对象：

{"reply":"你要对她说的话","add":[{"parentId":"","text":"节点文字","role":"这块是什么"}]}

- reply：不超过 120 字，一次一个问题。
- add：这一轮要往图上加的节点，**0 到 %d 个**；没有就给空数组。
- parentId：父节点的 id，逐字取自下面【当前的图】里给出的 id。加在最上层（中心论点）就留空字符串。
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

// insertPlanNode places one node under `parent` (nil = top level) and returns
// the created row.
//
// Position: a node goes at the END of its parent's subtree, so siblings keep
// the order she produced them in. The subtree ends at the first following row
// whose depth is <= the parent's — the same "flattened outline encodes a
// tree" convention the frontend renders from.
func insertPlanNode(
	ctx context.Context,
	q *sqlc.Queries,
	atomID uuid.UUID,
	rows []sqlc.WritingOutline,
	parent *sqlc.WritingOutline,
	text, role string,
) (sqlc.WritingOutline, []sqlc.WritingOutline, error) {
	depth := int32(0)
	insertAt := int32(len(rows))
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
