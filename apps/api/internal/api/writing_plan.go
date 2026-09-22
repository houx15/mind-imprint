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
//     印记 说「你这三条是并排说的」——这仍是最扎实的一次；卡住时提前说一个方法
//     名不算破例。真正不允许的是把「并列论证/递进论证/对比论证」整张表甩给
//     她挑，那又变回填表了。
//
// ## 提示词里那套写作学（Level 1 / Level 2）
//
// 中学写作的结构其实是两层，学生真正卡住的是第二层：
//
//   - 篇章骨架：总—分 / 总—分—总 / 立场式 / 起承转合 / 从一件事讲起
//     🚨 2026-09-12：这一层原来**只在这条注释里**。下面那句「两层都写进系统
//     提示词」是假的 —— 只有方法那一层进去了，骨架一个字都没有，于是 印记
//     手上没有任何整篇结构的词，说不出「你这已经是总—分—总了」。
//     产品负责人正是从产品那一头看见了这个洞（「增加"总—分""总—分—总"等结构
//     模板」）。现在它真的在提示词里了，见「## 常见文章结构」。
//   - 单个分论点怎么展开：并排说几条（并列）、一层深一层（递进）、比一比
//     （对比论证）、举个例子（举例论证）、讲道理（道理论证）、先承认，再反驳
//     （让步）、说清前因后果（因果）——括号里是 语文 课上的正式名称，学生看到
//     的是括号外面那个说法（2026-08-28 裁定，见 vocab.Method）。
//
// 两层都写进系统提示词，但**只作为 印记 自己的知识**——用来决定问什么、什么
// 时候往深里追一层，以及事后怎么命名她已经做出来的东西。学生一次也不会看到
// 这两张表。

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
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

// writingPlanMaxDepth is the map's depth ceiling. Depth 0 is document-ordered
// siblings — opening, thesis, landing — not just the thesis alone; depth 1 is
// 分论点; depth 2 is 论据. Three levels is what a middle-school piece needs;
// deeper is an org chart.
const writingPlanMaxDepth = 2

// —— 这一块「是什么」的清单，按文体两份（R4，2026-09-21）——
//
// 🚨 在这之前只有议论文那一份，而且写死在 writingPlanSystem 里。
// 也就是说：闭表里就算加了记叙文那四种，模型**永远不会用到它们** ——
// prompt 明说「只能是下面这十个之一」。一篇记叙文于是被摆成
// 中心论点／分论点／论据，一副用不上的骨架。
//
// 清单和 writing_kind.go 的闭表必须对得上，由
// TestWritingPlanKindListsMatchTheClosedSet 钉住。

type writingPlanAdd struct {
	// Kind 是这一块**是什么**，取自 writing_kind.go 的闭表。
	//
	// 🚨 这一个字段取代了原来的 ParentID + Role 两个。模型不再决定摆在哪儿 ——
	// 深度和父节点由 kind 算出来（writingKindDepth / writingKindParentOf），
	// 标题也由 kind 算出来（writingKindLabel）。
	//
	// 原来那对字段出过三类错，全都是静默的：
	//  1. 模型抄错 UUID，节点被整个丢掉（2026-09-18，重试照样抄错）；
	//  2. 同一轮新建的分论点没有 id 可引用，它的例子落到最上层；
	//  3. 模型把结尾挂到中心论点底下，屏幕上印成「分论点 3」（2026-09-20）。
	// 三类错都来自同一件事：让模型决定位置。现在它不决定了。
	Kind string `json:"kind"`
	Text string `json:"text"`
	// Source 是这条材料从哪来（0158），只有她**找回来的**那种材料才有。
	//
	// 她在对话里说「我找到一份 2023 年的睡眠研究」，出处就跟着那句话一起进图。
	// 印记 下一轮据此查它 —— 见 system prompt 的「她拿回来一份材料的时候」。
	// 空 = 她自己的经历，或者她没说出处。
	Source string `json:"source"`
}

type writingPlanReply struct {
	Reply string           `json:"reply"`
	Add   []writingPlanAdd `json:"add"`
	// Ready is 印记 saying THE PLAN IS ENOUGH — she can start writing now.
	//
	// ## Why this field exists (2026-09-04)
	//
	// The product owner, on 「永远不会带领学生真正开启写作吗？」:
	//
	//	> until student click the logic is good, ai never auto triggers and
	//	> guides students to start writing.
	//
	// Which was exactly right. 「去写」 has always been available from the
	// first render and is never gated — but nothing ever PROPOSED it. The
	// system prompt already told 印记 「有时候答案是什么都不缺，让她去写」 and
	// 「她想去写了，就让她去写」, and it had no way to say so: the reply
	// carried a sentence and a list of nodes, nothing else. So the judgement
	// was made and then thrown away every single turn, and a student who had
	// finished planning just kept being asked one more question.
	//
	// This is the channel for that judgement. It does not move her — 结构 is
	// not a gate in either direction, and shoving her into 段落 would be the
	// mirror of the bug. It puts a real invitation on screen at the moment
	// 印记 thinks the plan will hold.
	Ready bool `json:"ready"`
}

// stripWritingPlanFence 把模型爱加的围栏和前后闲话去掉，留下那对大括号之间
// 的东西。拆成一个函数是因为**救援那一路必须吃到和正解同一份字符串**——
// 2026-09-11 的教训：段落引导那边的救援喂的是没去围栏的原文，于是带围栏的
// 回复一次都没救到过，第一个 token 就不是 '{'。
func stripWritingPlanFence(text string) string {
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
	return strings.TrimSpace(c)
}

// salvageWritingPlanReply 从一份读不出来的回复里，把**她那句话**捞出来。
//
// # 为什么要捞
//
// 这一轮的钱已经花掉了，而整份 JSON 作废的代价不是「少一个节点」，是她眼前
// 弹一个「model_unavailable」，这一轮说的话石沉大海。2026-09-11 线上走查里
// 英文那个学生撞上一次，她的原话是「刚才报了个后台错误，不知道会不会影响发送」。
//
// 而**坏掉的地方几乎从来不是 reply**。实测到的两次都断在结构那一半：
// 一次是 `"questions":[...}]}`（该收 `]` 的地方收了 `}`），一次是流式少送
// 最后一个分片。那句陪练的话本身是完整的、可用的、已经付过钱的。
// 见 [[model-json-half-arrived-2026-09-08]]、[[streaming-drops-last-chunk-2026-09-10]]。
//
// # 捞什么、不捞什么
//
//   - `reply` 捞。它是一句人话，自己就成立。
//   - `add` 只收**在断点之前已经完整解出来**的那几个。半个节点宁可不要。
//   - `ready` 捞不到就当 false —— 判「够了没有」本来就有 planLooksReady
//     在兜底（结构判据），少模型这一票不会让她卡住。
//
// 🚨 这不是「编一句话糊弄她」。捞出来的每个字都是模型真的写的，
// 一个字都不是我们补的；补出来的那种才是 [[ai-errors-must-surface-never-fake]]
// 禁的事。捞不到 reply 就照旧报错。
func salvageWritingPlanReply(s string) (writingPlanReply, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	tok, err := dec.Token()
	if err != nil {
		return writingPlanReply{}, false
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return writingPlanReply{}, false
	}
	var got writingPlanReply
	for {
		key, kerr := dec.Token()
		if kerr != nil {
			break // 断在这里了，就用已经读到的那些
		}
		if d, isDelim := key.(json.Delim); isDelim && d == '}' {
			break
		}
		name, isStr := key.(string)
		if !isStr {
			break
		}
		if !salvagePlanField(dec, name, &got) {
			// 这个字段自己断了。后面的读不到了，但前面读到的
			//（可能已经包含 reply）仍然算数。
			break
		}
	}
	return got, looksLikeAWholeSentence(got.Reply)
}

// looksLikeAWholeSentence 判断救出来的这句话像不像**说完了**。
//
// 🚨 这道关是 2026-09-12 补的，补的是**救援自己造成的回归**，而那个回归比它
// 取代的错误更糟 —— 因为它不出声。
//
// 模型偶尔会在 JSON 字符串里写一个没转义的引号：
//
//	{"reply":"…一个是李浩然"同学那件事"，还有…","add":[…]}
//
// 整份 Unmarshal 失败，走到救援；救援把 reply 解到**第一个没转义的引号**为止，
// 得到一个语法合法、语义半截的字符串，然后当成成功交了出去。第十五轮线上走查
// 里她连着六步在说这件事（「印记的话没说完就断了，停在『后面』」），
// 而日志上一条 unparseable 都没有。
//
// 判据是句末那个标点。一轮说完了的陪练发言几乎总是以终止标点收尾；
// 被一个游离引号切断的字符串几乎从不。
//
// 🚨 方向是故意偏向报错的：宁可让她重发一次（她刚说的话还在输入框里），
// 也不要把半句话当成印记说的话摆给她看 —— 她没法判断那是印记没说完，
// 还是自己漏读了什么。这和 [[ai-errors-must-surface-never-fake]] 是同一条：
// 半句话也是一种「看起来像真的」的假回答。
func looksLikeAWholeSentence(reply string) bool {
	t := strings.TrimSpace(reply)
	if t == "" {
		return false
	}
	// 收尾的引号/括号不算话说完了，但它后面那个标点算 —— 先把它们剥掉，
	// 再看剩下的最后一个字符。「…那一句「浪费不是个别现象」」是说完了的。
	t = strings.TrimRight(t, "」』）)\"'”’】》")
	if t == "" {
		// 整句就是一对引号，没别的 —— 当它没说完。
		return false
	}
	last := []rune(t)[len([]rune(t))-1]
	switch last {
	case '。', '！', '？', '…', '.', '!', '?', '；', ';', '：', ':', '~', '～':
		return true
	}
	return false
}

// salvagePlanField 读一个字段，返回「还能不能接着往下读」。
func salvagePlanField(dec *json.Decoder, name string, got *writingPlanReply) bool {
	switch name {
	case "reply":
		var v string
		if err := dec.Decode(&v); err != nil {
			return false
		}
		got.Reply = v
		return true
	case "ready":
		var v bool
		if err := dec.Decode(&v); err != nil {
			return false
		}
		got.Ready = v
		return true
	case "add":
		open, err := dec.Token()
		if err != nil {
			return false
		}
		if d, isDelim := open.(json.Delim); !isDelim || d != '[' {
			return false
		}
		for dec.More() {
			var item writingPlanAdd
			if derr := dec.Decode(&item); derr != nil {
				// 这一个断在半路，它和它后面的都当没给；数组也就没法
				// 正常收尾，所以这一份到此为止。
				return false
			}
			got.Add = append(got.Add, item)
		}
		// 吃掉收尾的 ']'。吃不到说明它根本没收尾。
		if _, cerr := dec.Token(); cerr != nil {
			return false
		}
		return true
	default:
		var skip json.RawMessage
		return dec.Decode(&skip) == nil
	}
}

// parseWritingPlanReply decodes and clamps. Anything it cannot validate is
// DROPPED rather than guessed at: an unparseable parentId would otherwise
// silently reparent a node somewhere she never put it.
func parseWritingPlanReply(text string) (writingPlanReply, bool) {
	c := stripWritingPlanFence(text)
	var got writingPlanReply
	if err := json.Unmarshal([]byte(c), &got); err != nil {
		// 🚨 整份读不出来 ≠ 整份没到。见 salvageWritingPlanReply。
		salvaged, ok := salvageWritingPlanReply(c)
		if !ok {
			// 🚨 整份就是一句人话，根本没有信封 —— 实测她说「想不出来」的
			// 那两轮就是这样（见 writing_plan_prose.go）。把它当这一轮的
			// reply，一个节点都不加。判不准照旧报错。
			prose, pok := writingPlanReplyFromProse(c)
			if !pok {
				return writingPlanReply{}, false
			}
			slog.Warn("writing plan turn: reply arrived as plain prose, kept it and added nothing",
				"reply_bytes", len(c))
			return prose, true
		}
		// 🚨 **救援要出声。** 它原来一声不响地成功，于是 2026-09-12 那个
		// 「半句话」回归在线上跑了整整一轮都没被发现 —— 日志里一条
		// unparseable 都没有，因为救援报告的是成功。
		// 一行「这一轮是捡回来的」比事后翻数据库便宜得多。
		slog.Warn("writing plan turn: reply salvaged from broken JSON",
			"reply_bytes", len(c), "nodes_kept", len(salvaged.Add))
		got = salvaged
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
		a.Kind = strings.ToLower(strings.TrimSpace(a.Kind))
		if a.Text == "" {
			continue
		}
		// 🚨 编出来的 kind 整条丢掉，不猜一个最近的。
		// 猜错的代价是她的一句话落在一个她没放的地方，而她看不出发生过什么 ——
		// 和下面丢掉一个认不出的 parentId 是同一条理由。
		if !writingKindValid(a.Kind) {
			slog.Warn("writing plan turn: dropped node with unknown kind",
				"kind", a.Kind, "text", truncateRunes(a.Text, 40))
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

// planLooksReady answers, from the SHAPE of the map alone, whether this plan
// can carry a piece: a top-level block, at least two distinct sub-points, and
// at least one of them with her own material hanging under it.
//
// ## 🚨 Why this is computed and not left to the model
//
// The prompt states these three criteria and asks for `ready`. Measured
// against the live model (TestLiveWritingPlanSignalsReady, 2026-09-04) that
// worked reliably for one of the two cases and was a coin-flip for the other:
// on a plan that plainly met every criterion, the model kept choosing to teach
// one more method and end on a question — 「你打算这样承认了再反驳，还是直接
// 驳？」 — which is good teaching, and also exactly the behaviour the product
// owner reported as the bug:
//
//	> until student click the logic is good, ai never auto triggers and
//	> guides students to start writing.
//
// There is ALWAYS one more thing worth teaching. That is precisely why a model
// asked to judge 「够了吗」 keeps answering not yet, and why the floor has to be
// structural. Same lesson as enforceLensDoneTurn (reading_coach.go): a 「必须」
// that lives only in prose is a 「必须」 the model gets to overrule.
//
// The model's own `ready` is OR-ed on top rather than replaced, because it
// catches the case no shape can: she says 「我想开始写了」 over a three-node
// map, and the prompt's answer to that is to agree without arguing.
//
// Depth convention is writingPlanMaxDepth's: 0 = top-level blocks (opening,
// thesis, landing), 1 = 分论点, 2 = 论据. Openings and closings are deliberately
// NOT required — they are decided after the middle exists, so demanding them
// would hold her at exactly the step this function exists to release.
// 🚨 THE OTHER DIRECTION, 2026-09-11: this function is a FLOOR (the model is
// too perfectionist to release her), and it needed a CEILING to match (a
// student who says almost nothing never reaches the floor at all, so the
// questions never stop). That half is writingPlanStalled — see
// writing_plan_state.go, which also owns the shape counting below.
func planLooksReady(wr sqlc.Writing, rows []sqlc.WritingOutline) bool {
	return writingPlanShapeOf(rows).ready(writingPlanNeedOf(wr))
}

// topLevelInsertPosition 决定一个深度 0 的新块排在哪儿：开篇最前，结尾最后，
// 中心论点排在开篇之后。
//
// 取代 rootInsertPosition。那个函数对模型的自由散文 role 做子串匹配，认不出来
// 就排到末尾 —— 于是一个它没认出来的开篇会排在每一条分论点后面。现在块是什么
// 由 kind 说了算，这里就只剩三个确定的位置。
func topLevelInsertPosition(kind string, rows []sqlc.WritingOutline) int32 {
	switch kind {
	case writingKindOpening:
		return 0
	case writingKindClosing:
		return int32(len(rows))
	}
	// 中心论点（以及任何别的深度 0 的块）：排在开篇后面。
	var at int32
	for _, r := range rows {
		if writingKindOf(r) == writingKindOpening && r.Position+1 > at {
			at = r.Position + 1
		}
	}
	return at
}

// insertPlanNode 把一个节点放到它该在的地方，并返回新建的那一行。
//
// 🚨 **深度和父节点由 kind 算出来，不采信模型给的位置**（writing_kind.go）。
// 这是 2026-09-20 那一刀的核心：摆错在结构上不可表示，而不是摆错之后被纠正。
//
// 位置：有父的排在那个父的子树末尾，同辈保持她说出来的顺序。子树在第一行深度
// 不大于父的行处结束 —— 就是前端画图用的那套「扁平清单编码一棵树」的约定。
// 没有父的走 topLevelInsertPosition。
func insertPlanNode(
	ctx context.Context,
	q *sqlc.Queries,
	atomID uuid.UUID,
	rows []sqlc.WritingOutline,
	kind, text, source string,
) (sqlc.WritingOutline, []sqlc.WritingOutline, error) {
	depth := writingKindDepth(kind)
	parent := writingKindParentOf(kind, rows)

	// 一条论据（或待补、回应）来了，图上还没有它该挂的那一种：让它自己先当一条
	// 分论点占住这一段，段落那一步的 needsPoint 会请她先说清它证明了什么。
	//
	// 挂到中心论点底下冒充一条理由是更糟的选择 —— 那正是 2026-09-18 记下的
	// 毛病：例子落在最上层，段落那一步把它印成「分论点 3」。
	//
	// 🚨 **这段兜底是按议论文写的，记叙文不能走它。** 2026-09-21 线上实测：
	// 一篇「记一次难忘的经历」，立题那一轮模型正确地开了一个 `scene`，
	// 而空板上没有 `opening` 可挂 ⇒ 这里把它改成 `point`、再改成 `thesis`，
	// 于是她屏幕上出现一张写着「中心论点」的卡，内容是「大雨天，爸爸来补习班
	// 接我」。文体那条轴一路都对，最后一步把它抹掉了。
	if parent == nil && depth > 0 {
		if writingKindGenre(kind) == genreNarrative {
			// 细节还没有场景可挂 → 它自己先当一个场景（和论据→分论点同一个道理）。
			if kind == writingKindDetail {
				kind = writingKindScene
				depth = writingKindDepth(kind)
				parent = writingKindParentOf(kind, rows)
			}
			// 场景 / 转折 / 感悟挂不到开篇上是**正常的** —— 记叙文的开篇是可选的，
			// 她往往就是从一件事直接讲起。留它当一个顶层节点，不改它的种类。
		} else {
			kind = writingKindPoint
			depth = writingKindDepth(kind)
			parent = writingKindParentOf(kind, rows)
			if parent == nil {
				// 连中心论点都还没有：这一句就是这篇的第一块。
				kind = writingKindThesis
				depth = 0
			}
		}
	}
	if depth > writingPlanMaxDepth {
		depth = writingPlanMaxDepth
	}

	var insertAt int32
	if parent != nil {
		insertAt = parent.Position + 1
		for _, r := range rows {
			if r.Position > parent.Position && r.Depth > parent.Depth {
				insertAt = r.Position + 1
			} else if r.Position > parent.Position {
				break
			}
		}
	} else {
		insertAt = topLevelInsertPosition(kind, rows)
	}
	if err := q.ShiftWritingOutlinePositions(ctx, sqlc.ShiftWritingOutlinePositionsParams{
		AtomID: atomID, Position: insertAt,
	}); err != nil {
		return sqlc.WritingOutline{}, rows, err
	}
	created, err := q.InsertWritingOutlineNode(ctx, sqlc.InsertWritingOutlineNodeParams{
		AtomID: atomID, Text: text, Kind: kind, Depth: depth, Position: insertAt,
		// Role 不再由模型写，而是 kind 的标题。它仍然落库，因为报告、教师端和
		// 老前端都还读这一列 —— 但它现在是**派生值**，不是第二份真相。
		Role:   writingKindLabel(kind, source),
		Source: trimRunes(strings.TrimSpace(source), writingSourceMaxRunes),
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

	// §model-routing · compose. Planning is the hardest reasoning in this room:
	// it has to hear what she actually said, decide the ONE next question, and
	// judge where a point is too hollow to leave alone. It is not, however, the
	// never-downgrade reviewer — it derives a plan from what she has already
	// stated, which is the compose class's whole definition. compose keeps a
	// reasoning budget rather than none; routebench decides how large.
	resolved, okResolve := a.route(turnCtx, gateway.ClassCompose)
	if !okResolve {
		slog.Warn("writing plan turn: no provider resolved",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
		return
	}
	system := writingPlanSystemFor(writingGenreOf(wr, rows), wr.Lang)
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
		// 🚨 **把模型到底回了什么记下来。** 这一行原来只有 atom_id 和
		// request_id —— 也就是「它坏了」，没有一个字说它怎么坏的。
		// 2026-09-11 线上撞到一次，回头查日志，除了知道它发生过之外
		// 什么都得不到。
		//
		// 两头都要：只有尾巴，分不清「断在最后一块、救援本该救回前面那些」
		// 和「第一块就是坏的、救援什么都救不回才对」—— 这两种的修法相反。
		// 长度和 stop_reason 一起看，才分得出「没写完」和「写完了但写坏了」
		//（[[model-json-half-arrived-2026-09-08]]：finish_reason:"stop"
		// 不等于写完了）。
		slog.Warn("writing plan turn: reply unparseable",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
			"reply_bytes", len(res.Text), "stop_reason", res.StopReason,
			"reply_head", headRunes(res.Text, 220), "reply_tail", tailRunes(res.Text, 220))

		// 再要一次 —— 和段落引导那两条路同一个判断（writing_guide.go）。
		// 这一类坏法（字符串里一个没转义的引号、数组收错括号）和她写了什么
		// 无关，换一次采样几乎总能过；而这一轮的钱已经花掉了，直接报错等于
		// 让她白等一次，还得自己把刚才那句话再说一遍。
		//
		// 一次，不是三次：她正同步等着。第二次还坏就老实报错。
		res2, cerr2 := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, msgs, studentText)},
			},
		})
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "plan_turn", resolved, res2.Usage)
		if cerr2 != nil {
			slog.Warn("writing plan turn: retry provider call failed", "err", cerr2,
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		parsed, okParse = parseWritingPlanReply(res2.Text)
		if !okParse {
			slog.Warn("writing plan turn: retry also unparseable",
				"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()),
				"reply_bytes", len(res2.Text), "stop_reason", res2.StopReason,
				"reply_head", headRunes(res2.Text, 220), "reply_tail", tailRunes(res2.Text, 220))
			httpx.WriteError(w, r, httpx.ErrAIDialogueFailed("model_unavailable"))
			return
		}
		slog.Info("writing plan turn: retry parsed fine", "atom_id", at.ID)
	}

	// 🚨 印记 说放进图里了，图上就得有。见 writing_plan_place.go。
	// 两种丢法：reply 里「」引了她的话而图上没有；或者她说了一句新的，
	// 这一轮什么都没加（只在 reply 里转述）。
	// 一次重试；第二份没有更好（读不出来、或者放上去的没变多）就用第一份。
	//
	// 🚨 2026-09-20：原来这里还有第三种丢法 —— parentId 认不出来，节点落库时
	// 被丢掉。那一类随 kind 一起消失了：每一条通过解析的 add 都一定落得上去，
	// 所以这里不再需要 byID，也不再需要给模型看短号。
	unplaced := planReplyUnplaced(parsed.Reply, studentText, rows, parsed.Add)
	if len(unplaced) == 0 && planTurnDroppedHerPoint(studentText, rows, planAddsThatLand(rows, parsed.Add)) {
		unplaced = []string{truncateRunes(strings.TrimSpace(studentText), 60)}
	}
	if len(unplaced) > 0 {
		slog.Info("writing plan turn: reply names her words that are not on the map, retrying once",
			"atom_id", at.ID, "unplaced", len(unplaced), "request_id", httpx.RequestIDFromContext(r.Context()))
		prior, _ := json.Marshal(parsed)
		res2, cerr2 := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, msgs, studentText)},
				{Role: gateway.RoleAssistant, Content: string(prior)},
				{Role: gateway.RoleUser, Content: writingPlanPlaceNudge(unplaced)},
			},
		})
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "plan_turn", resolved, res2.Usage)
		if cerr2 == nil {
			if p2, ok2 := parseWritingPlanReply(res2.Text); ok2 &&
				planAddsThatLand(rows, p2.Add) > planAddsThatLand(rows, parsed.Add) &&
				len(planReplyUnplaced(p2.Reply, studentText, rows, p2.Add)) <= len(unplaced) {
				parsed = p2
			} else {
				slog.Warn("writing plan turn: place retry did not place them", "atom_id", at.ID)
			}
		}
	}

	// 🚨 模型说「可以写了」，而服务端数出来还差（她也没说要去写）：再要一次。
	// 2026-09-18 本地实测：两条分论点、一个例子（还是她自己的经历），印记就说
	// 「你的计划已经站得住了……现在就动笔写吧」—— 那条「800 字起码 2–3 个例子、
	// 至少一个社会或历史上的」的线被模型自己的 ready 越过去了。
	// 能越线的只有她自己说要写（studentWantsToWrite）和连着两轮没答（stalled）。
	stalled := writingPlanStalled(msgs, studentText)
	wantsToWrite := studentWantsToWrite(studentText)
	if shape := writingPlanShapeWith(rows, parsed.Add); parsed.Ready && !wantsToWrite && !stalled && !shape.ready(writingPlanNeedOf(wr)) {
		slog.Info("writing plan turn: model invited writing below the line, retrying once",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		prior, _ := json.Marshal(parsed)
		res2, cerr2 := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, msgs, studentText)},
				{Role: gateway.RoleAssistant, Content: string(prior)},
				{Role: gateway.RoleUser, Content: writingPlanReadyTooSoonNudge(shape.missing(writingPlanNeedOf(wr)))},
			},
		})
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "plan_turn", resolved, res2.Usage)
		if cerr2 == nil {
			if p2, ok2 := parseWritingPlanReply(res2.Text); ok2 && !p2.Ready &&
				planAddsThatLand(rows, p2.Add) >= planAddsThatLand(rows, parsed.Add) {
				parsed = p2
			}
		}
		// 重试没救回来也不把「可以写了」这个信号发出去：话留着，按钮不出来。
		parsed.Ready = false
	}

	// 🚨 她请我们替她搜索或替她写，而这一轮一个字都没说自己不做这件事：
	// 重试一次。见 writing_refusal.go（同事 2026-09-20 的意见 8）。
	//
	// 🚨 重试完**照旧把拿得到的那一份交出去**。为了一个检测项让她看见一个死掉
	// 的终端，是比生硬更大的毛病（[[ai-errors-must-surface-never-fake]] 的
	// 另一面：一句不够完美的真话，好过一个空白）。
	if writingAsksUsToDoIt(studentText) && !writingReplyOwnsTheRefusal(parsed.Reply) {
		slog.Info("writing plan turn: refusal not owned, retrying once",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		prior, _ := json.Marshal(parsed)
		res2, cerr2 := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, msgs, studentText)},
				{Role: gateway.RoleAssistant, Content: string(prior)},
				{Role: gateway.RoleUser, Content: writingRefusalNudge},
			},
		})
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "plan_turn", resolved, res2.Usage)
		if cerr2 == nil {
			if p2, ok2 := parseWritingPlanReply(res2.Text); ok2 && writingReplyOwnsTheRefusal(p2.Reply) {
				// 🚨 只换那句话，不换 add —— 和上面那条邀请重试同一个道理：
				// 整份换掉的话，第二份里的 add 可能少了节点。
				parsed.Reply = p2.Reply
			} else {
				slog.Warn("writing plan turn: retry still did not own the refusal", "atom_id", at.ID)
			}
		}
	}

	// 🚨 这一轮要是请她去写的那一轮，话里就不能还挂着一个问题。
	// 见 writing_plan_invite.go 那一段（产品负责人 2026-09-12 带截图报的第一条）。
	//
	// 必须在落库**之前**判：回复是先写进 atom_message 再加节点的，等 live 有了
	// 这几个节点，那句带问号的话已经存进对话里，改不动了。所以形状要连这一轮
	// 还没落库的 add 一起算。
	if writingPlanShapeWith(rows, parsed.Add).ready(writingPlanNeedOf(wr)) && writingPlanReplyAsks(parsed.Reply) {
		slog.Info("writing plan turn: invite turn still asked a question, retrying once",
			"atom_id", at.ID, "request_id", httpx.RequestIDFromContext(r.Context()))
		// assistant 那一轮用 parsed 重新序列化，不用 res.Text —— 上面解析失败
		// 重试过的话，res.Text 是那份坏掉的，parsed 才是真正在用的这一份。
		prior, _ := json.Marshal(parsed)
		res2, cerr2 := gateway.Collect(turnCtx, a.d.Provider, resolved, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: system},
				{Role: gateway.RoleUser, Content: buildWritingPlanPrompt(wr, rows, msgs, studentText)},
				{Role: gateway.RoleAssistant, Content: string(prior)},
				{Role: gateway.RoleUser, Content: writingPlanInviteNudge},
			},
		})
		a.recordLiteLLMCall(turnCtx, u.ID, at.ID, "plan_turn", resolved, res2.Usage)
		if cerr2 == nil {
			if p2, ok2 := parseWritingPlanReply(res2.Text); ok2 && !writingPlanReplyAsks(p2.Reply) {
				// 🚨 只换那句话，不换 add。这次重试只为去掉问号；整份换掉的话，
				// 第二份里的 add 可能少了节点（2026-09-18 实测：司马迁那个例子就是
				// 这样丢的），而「过线了」是按第一份的 add 算出来的 ——
				// 于是她被请去写了，图上却没有那个让它过线的例子。
				parsed.Reply = p2.Reply
				parsed.Ready = true
			} else {
				// 两次都带问号，或者第二次读不出来：用第一份。一句带问号的好
				// 教学，比扣下整轮让她什么都拿不到强 —— 同 firstGhostQuote 那条路。
				slog.Warn("writing plan turn: invite retry still asked or unparseable", "atom_id", at.ID)
			}
		}
	}

	tx, err := a.d.Pool.Begin(turnCtx)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	defer func() { _ = tx.Rollback(turnCtx) }()
	qtx := a.d.Queries.WithTx(tx)

	// 串行化这颗原子的 seq 分配。NextAtomMessageSeq 是先读后插，
	// 并发下两笔事务会读到同一个 MAX——唯一索引保住的是数据，代价是
	// 其中一轮直接失败，而那一轮的模型钱已经花掉了。见 queries/atom.sql。
	if _, err := qtx.LockAtom(turnCtx, at.ID); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
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
		// 🚨 图上已经有这句话了，就不要再加一个。
		//
		// 这一路是**只加不改**的，所以重复的节点谁也删不掉，它会一直摆在那儿。
		// 2026-09-11 线上走查里她就撞上了：「第五段的标签跟我第二段一模一样，
		// 不知道是不是系统搞错了」—— 而提纲的每一块都会变成段落那一步的一个
		// 写作格子，于是她要对着两个一模一样的标题各写一段。
		//
		// 模型这么干不是出错：她把同一件事又说了一遍，它就又记了一遍。
		// 判据放在**文字**上而不是让模型自己记得，理由和 parentId 那条一样 ——
		// 能在代码里验的，就别只写在提示词里。
		if outlineHasText(live, node.Text) {
			slog.Warn("writing plan turn: duplicate node text, dropped",
				"atom_id", at.ID, "text", truncateRunes(node.Text, 40))
			continue
		}
		// 位置、深度、标题全部由 kind 算出来 —— 见 insertPlanNode。
		// 同一轮里加进来的节点也会被后面那一条看见（live 随每次插入更新），
		// 所以「这一轮先加分论点、再加它的论据」是成立的：那条论据挂得上。
		created, next, ierr := insertPlanNode(turnCtx, qtx, at.ID, live, node.Kind, node.Text, node.Source)
		if ierr != nil {
			httpx.WriteError(w, r, ierr)
			return
		}
		live = next
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
		// See writingPlanReply.Ready and planLooksReady: the one thing the
		// planning room could never say before, which is 「这份计划够写了」.
		// The structural floor is COMPUTED; the model can only add to it.
		//
		// 🚨 第三项（2026-09-11）：她连着两轮等于没答，而图上至少有了一块，
		// 就也给 true。理由和前两项是同一个——「够了没有」不能只听模型的。
		// 一个话很少的学生永远到不了那三条判据，于是那三条判据在她身上从
		// 「一条线」变成了「一道关」，而这一步本来就不是关卡。
		//
		// 🚨 为什么要 `shape.Top >= 1` 这个下限：强制 ready 会把她送进段落，
		// 而提纲为空的段落页是一页空白——那不是放她走，是把她扔了。
		// 图上还什么都没有的时候，停止提问这件事只由 prompt 那一段来做
		// （writingPlanStalledBlock：这一轮不要再问，告诉她可以先去写）。
		"ready": parsed.Ready || planLooksReady(wr, live) ||
			(stalled && writingPlanShapeOf(live).Top >= 1),
	})
}
