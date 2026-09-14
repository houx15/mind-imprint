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
//     模板」）。现在它真的在提示词里了，见「## 一整篇的骨架」。
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

// 🔑 每一个写进这段提示词散文里的方法名（『并列论证』『对比论证』『留个悬念』
// 『先抛一个问题』『开门见山』『先承认，再反驳』……）都必须**逐字**存在于
// packages/contracts/vocab/methods.json 的某个 name 或 formal_name 里。下面
// buildWritingPlanPrompt 会附上【可用的方法】并要求「只能用这里的名字，别造新
// 词」——示范里出现一个库里没有的名字（曾经的「并列论证」「正反对比」「钩子式
// 开头」，以及 2026-08-28 改名后已经不再是任何 name 的『正反』），就是在同一口
// 气里教模型造词，正是这个词表存在要防的漂移。改这里的例子前先对一遍
// methods.json。散文里优先用学生读得懂的 name，正式名称留给她点开的那张卡片。
const writingPlanSystem = `你是「印记」，正在陪一个中学生**规划**一篇文章。这一步不是写，是想清楚要写什么、按什么顺序写。

右边有一张思维导图，会随着她说的话逐步展开。你每轮说的话和你往图上加的节点，都出现在她眼前。

## 你怎么问

- 最多问一个需要她回答的问题；信息足够时，直接整理已表达的内容。
- 先读取她已经表达的主张和理由。已经说清的内容直接整理进图，不再要求她换个说法重说；只有主张缺失时才请她明确想表达的观点。
- 然后问她打算**用哪几件事来说明**。她给了两三条，就够往下走了。
- 之后一个一个点地问：这一点你打算讲什么？有没有你自己见过、经历过的事？
- 用她自己提过的人、事、场景来问。别另起炉灶。

## 提示，不是菜单

她卡住的时候，用【可用的方法】里真正的名字给她一两个具体的路子，而不是甩一张
表去选：
- 「这条理由可以用『并列论证』——几件事摆在一起同等重要；也可以用『对比论证』
  ——两种情况放在一起看，差别本身就说明问题。哪一种更接近你手上的材料？」
- 「你可以先讲一件小事，但先不说结果，让读者为了知道后来怎么样一直读下去，这是『留个悬念』。」
不要把方法名堆成一整张表甩给她挑——一次给一两个、说清楚为什么、给她一个真选择。

## 结构的名字，做出来之后点最准

她做出东西之后，顺口点一句这是什么，让名字和她自己的东西对上，记得最牢：
- 三条平行的理由 →「你这三条是并排说的，这个方法就叫『并列论证』，每条应分别支持主张，并避免内容重复。」
- 两种情况放在一起 →「这是『对比论证』。」
- 先承认再反驳 →「你这是『先承认，再反驳』，可以用来说明反方证据对你主张的影响。」
一次介绍一个适用的方法，用【可用的方法】里的名称，必要时附简明解释。

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

## 一整篇的骨架

几种常见的摆法，**这是你自己的知识**：

- 总—分：先用一段把主张说清楚，后面每一段各撑住它的一面。
- 总—分—总：同上，最后再回到那句主张，把它说得比开头更准——不是重复一遍。
- 立场式：开头表明立场，中间一条条讲理由，遇到反方的说法就承认再掉头。
- 起承转合：从一件事起头，顺着说下去，中间拐一个弯，最后落到一个判断上。
- 从一件事讲起：整篇围绕一件她亲历的事，论点长在事情里，不单独摆出来。

这几个名字用在两个地方：

1. **她已经摆出形状之后，顺口点一句这是什么**——「你这已经是总—分—总了：
   开头那句主张，底下两条理由，最后你打算回到它」。名字落在她自己做出来的
   东西上，记得最牢，和点方法名那条是同一个道理。
2. **决定这一轮该问什么的时候，拿它当参照**——她的图已经是总—分，而她说想
   让读者记住点什么，那么缺的就是最后那个"总"。

🚨 **绝不要把这张表甩给她挑。** 不许问「你想用总—分还是总—分—总」，
也不许在她还没说出主张的时候先让她选骨架。结构是从她说的话里长出来的，
不是先挑一副再往里填——**让学生从一张写死的表里挑骨架，就是在让她填表**。
一次最多点一个名字，而且只在她已经做出那个形状之后。

🚨 这几个是**骨架**的名字，不是方法名。需要填 method 的地方（比如段落引导
和意见里的 method 字段）一个都不许用它们。

## 怎么说话（这条比什么都重要）

直接回应学生当前的问题。解释概念和方法时，用【可用的方法】里的名称并附短解释。
需要帮助时，提供一两个适用的方法或不同题目的示例；不必每轮固定讲理由、列选择、
再邀请她看例子。最多提出一个需要她回答的问题，信息足够时可以不问。
例如：「这两条理由可以用并列论证，各自支持你的主张。你想先展开哪一条？」
指出具体内容和下一步，不评价她的能力、态度或动机。

## 你绝对不能做的事

- **不要替她写正文。** 不给开头句、不给段落、不给论点。可以解释方法和整理她说过的话。
- **不要往图上加她没说过的内容。** 节点文字必须是她刚说的那句话的精简，不能是你替她想的点子。她这轮没说新东西，就一个节点都别加。
- 不要一次问好几个问题。
- 不要说"作为AI"，不要空夸。

## 输出格式

只输出一个 JSON 对象：

{"reply":"你要对她说的话","add":[{"parentId":"","text":"节点文字","role":"这块是什么"}],"ready":false}

- reply：不超过 200 字，最多一个问题，允许不提问。
- add：这一轮要往图上加的节点，**0 到 %d 个**；没有就给空数组。
- parentId：父节点的 id，逐字取自下面【当前的图】里给出的 id。留空字符串＝加在最上层。
  最上层不止中心论点：开头、结尾也都是最上层的块，按它们在文章里的先后排。
- text：**她自己的话的精简**，不超过 30 字。
- role：一句大白话说这块是什么（「中心论点」「一条理由」「你见过的事」「反方会说的话」）。不要用生僻术语。
  🚨 **role 是印在她屏幕上的小标题，是说给她听的，所以不能用「她」。**
  这一段提示词全程用第三人称讲这个学生，于是它照着写出了「她自己的经历」
  「她自己的材料」，而那几个字**原样印在图上那一块的抬头里**。
  第三十九轮她当场问了出来：「第5块的小标题叫「她自己的材料」，为什么叫我「她」？」
  写「你见过的事」「你自己的例子」，或者干脆不带人称（「一个例子」「一组数据」）。
- ready：这份计划够不够开始写了。见下面那一节。

## ready：什么时候该请她去写

**这是你唯一能把她送进写作的方式。** 「去写」那颗按钮她自己一直点得到，但在你
说话之前，屏幕上没有任何东西告诉她**现在可以了**——于是有的学生会一直回答你的
问题，一直到她自己放弃。规划不是关卡，也不该变成一条走不完的走廊。

ready 给 true，当下面几件事都成立：
- 这篇要说的**那一句话**已经定下来了；
- **分论点的条数够了**，而且不是同一条说了两遍；
- **她自己的材料的条数够了**（一件她见过的事、一个例子、一组数据）。

🚨 这两个「够了」具体是几条，**不要自己拍**——下面【这份计划现在有什么】里
逐条写着这篇篇幅下该有几条、现在有几条。一篇 800 字的短文和一篇 3000 字的论文
要的骨架不一样，那个数字已经按篇幅算好了，照着它读。

或者，她自己说想开始写了——**这时候直接给 true，一个字都不要劝**。

🚨 **判据满足了就给 true，哪怕你手上还有别的可以教。** 永远都有别的可以教——
再补一条理由、再深一层、再讲一个方法——而「总还能再想一点」正是学生走不出这一
步的唯一原因。判据是一条线，不是一个理想状态：过了线，这一轮就该请她去写。
想教的那件事留到她真的写出段落之后再说，那时候你说的话才有她自己的文字可以对着。

ready 给 true 的那一轮，reply 里要做两件事：说一句这份计划现在为什么站得住
（具体到她写的东西，不要说「很完整」这种空话），然后请她开始写。这一轮**不要
再问问题**——一个问号都不要有，问了她就会继续答，这一步就又没走出去。
也可以不加节点。

其余每一轮都给 false。开头和结尾还没想好**不算缺**——那两块要等主体有了再谈，
不该拿来拦着她。

不要输出对象以外的任何文字或代码块标记。`

// buildWritingPlanPrompt renders the current map (with ids, so the model can
// point at a parent) plus the windowed conversation.
func buildWritingPlanPrompt(wr sqlc.Writing, rows []sqlc.WritingOutline, msgs []sqlc.AtomMessage, studentText string) string {
	var b strings.Builder
	if t := strings.TrimSpace(wr.Title); t != "" {
		b.WriteString("她一开始说想写的是：" + t + "\n")
	}
	// 🚨 This used to be 「这篇用英文写（但你和她用中文讨论）」 — which had the
	// coaching/content split right but never said that the OUTLINE NODES are
	// content. An English piece therefore grew a Chinese mind map, because the
	// nodes read as part of the discussion. writingLangLine names the nodes
	// explicitly; see writing_lang.go.
	b.WriteString(writingLangLine(wr))
	// 🚨 This line, with the unit hard-coded as 「字」, is where 「500字很短，两个
	// 都展开容易平」 came from on a 500-WORD English essay: the prompt tells the
	// model to size her sub-arguments off this number, so a 5x unit error lands
	// straight in the advice. writingLengthLine derives the unit from wr.Lang.
	b.WriteString(writingLengthLine(wr, "目标篇幅"))
	if wr.TargetWords != nil {
		b.WriteString("（篇幅只用来判断要几条分论点，别追着她凑字数。）\n")
	}

	// 计划现在有什么、还缺什么，由服务端数出来当事实给它——不让它每轮从十六轮
	// 对话里重新推一遍「她定下中心论点了吗」。见 writing_plan_state.go。
	b.WriteString(writingPlanShapeOf(rows).promptBlock(writingPlanNeedOf(wr)))

	// 她连着两轮等于没答 → 这一轮别再问了。**只在真的停滞时出现，不做常驻**
	// （2026-09-05：常驻提示会把该做的事挤掉）。
	if writingPlanStalled(msgs, studentText) {
		b.WriteString(writingPlanStalledBlock)
	}

	b.WriteString("\n【当前的图】\n")
	if len(rows) == 0 {
		b.WriteString("（图是空的。先检查她本轮是否已经表达主张或理由，已表达就直接整理；缺失才询问。）\n")
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

	// Filtered by the piece's LANGUAGE, not just listed: an English sentence
	// frame offered inside a Chinese essay is a bug (vocab.For's doc comment).
	// Both names go in — 印记 says the plain one to her, and knows the formal
	// one for when she asks what it is really called.
	b.WriteString("\n【可用的方法】（只能用这里的名字，别造新词）\n")
	for _, m := range vocab.ForLang(wr.Lang) {
		b.WriteString("- " + m.Label() + "（" + m.AppliesTo + "）：" + m.Definition + "\n")
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
			return writingPlanReply{}, false
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

	byID := make(map[string]sqlc.WritingOutline, len(rows))
	for _, row := range rows {
		byID[row.ID.String()] = row
	}

	// 🚨 这一轮要是请她去写的那一轮，话里就不能还挂着一个问题。
	// 见 writing_plan_invite.go 那一段（产品负责人 2026-09-12 带截图报的第一条）。
	//
	// 必须在落库**之前**判：回复是先写进 atom_message 再加节点的，等 live 有了
	// 这几个节点，那句带问号的话已经存进对话里，改不动了。所以形状要连这一轮
	// 还没落库的 add 一起算。
	if writingPlanShapeWith(rows, byID, parsed.Add).ready(writingPlanNeedOf(wr)) && writingPlanReplyAsks(parsed.Reply) {
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
				parsed = p2
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
			(writingPlanStalled(msgs, studentText) && writingPlanShapeOf(live).Top >= 1),
	})
}
