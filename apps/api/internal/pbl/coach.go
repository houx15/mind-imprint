package pbl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"mindimprint/api/internal/gateway"
)

// coach.go — 印记 talking inside a project.
//
// The turn is deliberately thin: it produces prose and, at most, ONE hook
// question. It does not decide the plan, propose roads or mint tools — those
// are separate, gated moves (see spec §8), and folding them into the reply is
// how an assistant starts making decisions in passing.

// Turn is one exchange in a thread.
type Turn struct {
	Role    string // "student" | "ai"
	Content string
}

// CoachInput is everything the turn reads. Assembled by the caller from rows;
// plain values so the prompt building is testable without a database.
type CoachInput struct {
	// Idea is her own opening sentence, kept verbatim.
	Idea string
	// Kind is the project type 印记 judged.
	Kind string
	// Steps is the live plan, if there is one. Empty before she approves.
	Steps []string
	// Recent is the tail of this thread, oldest first.
	Recent []Turn
	// SessionKind is set when this turn happens INSIDE a session; empty on the
	// main thread. It changes what 印记 is doing, so it changes the prompt.
	SessionKind string
	// SessionQuestion is the question that opened the session.
	SessionQuestion string
	// ToolWork is what she actually produced in the tools: the problem she
	// reframed, the notes she wrote on the board, the option she settled on,
	// the artifact she sent back for revision.
	//
	// 🚨 少了这一项，工具就白做了。她在便签板上摆十五分钟，回到对话，而印记
	// 收到的 prompt 和上一轮一模一样——它没看见任何一张便签，只能接着聊上一轮
	// 那件事。学生那边的感受很直接：这个东西没在听我说话。
	//
	// 这是 AGENTS.md 主线里的「回灌陪练」那个箭头（见 api/pbl_refeed.go）。
	ToolWork []string
	// WriteBacks are the conclusions of closed sessions on this thread. Per
	// spec §10.3 the main thread sees CONCLUSIONS, never every turn of every
	// session — that is what keeps a deep dig from flooding the project.
	WriteBacks []string
	// JustHappened describes the event that triggered this turn when she did
	// not type anything — she finished a tool, or closed a dig.
	//
	// 🚨 少了这一句，这一轮的对话就**停在印记自己的上一句话上**，而模型接着
	// 一段以自己结尾的对话往下写，最可能的续写就是把那句话再说一遍。2026-09-02
	// 实测：她做完「观察日记」带回两条观察，印记一字不差地重复了上一句
	// 「能不能先花几天时间观察一下课间？」，并且把她刚做完的那件工具又递了
	// 一次。她那边看到的就是「我做的事它根本没看见」。
	JustHappened string
	// ToolsUsed are the tools she has already finished in this project.
	//
	// 🚨 工具目录本身不带状态，所以印记无从知道哪件已经做过了，于是会把做完的
	// 那件再递一次。
	ToolsUsed []string
	// ToolsOffered are the tools already sitting on her screen, offered but not
	// yet finished.
	//
	// 🚨 递过但她还没做的，和做完的一样不能再递。2026-09-03 线上实测：印记递了
	// 「头脑风暴」，那张卡因为前端没刷新没显示出来，下一轮印记又递了一遍——
	// 屏幕上并排两张一模一样的邀请卡，理由还各写各的。前端那个 bug 已经修了，
	// 但印记这边也得知道"这件已经在她桌上了"。
	ToolsOffered []string
	// ToolDropped 是上一轮被服务端撤掉的那件工具（见 api/pbl_tool_gate.go）。
	//
	// 🚨 少了这一句，印记会把同一句谎话说到底。闸撤掉一件界面会是空的工具是对
	// 的，但撤完之后印记的上文一个字都没变，于是它下一轮照样说「审核助手我给你
	// 了」——而她屏幕上没有。2026-09-04 的模拟学生走查里，Marcus 在这个循环里
	// 耗掉约 35 步、8 次要老师直接替他点。
	//
	// 和 SiteMissing 是同一类东西：服务端悄悄丢掉了印记的某个动作，那就必须把
	// 丢掉这件事本身交回给它。
	ToolDropped string
	// Stuck 是她**连着**有几轮答不上来（见 stuck.go · StuckRun）。
	//
	// 🚨 服务端数出来的，不由模型自己判断。【怎么问】那条「不要给他两个选项让
	// 他挑」没有出口，走查里一个学生用自己的话求了三次「给我几个选项」，一次都
	// 没得到，最后不敢再进那个项目。出口要能在代码里验，否则它只是 prompt 里的
	// 一句好话。
	Stuck int
	// AskedForHelp 是她最后那一句直接开口要例子或选项了。
	//
	// 和 Stuck 分开，因为它**一次就算数**：她已经明说了她要什么，还要她先卡够
	// 两轮才给，是把一条本来就该听见的话当成噪音。
	AskedForHelp bool
	// Courses 是课程库里她这个版本看得见的那些课。
	//
	// 🚨 这一格空着，印记就**挑不出课来**——它不知道我们有哪些课，只能瞎编一个
	// 名字，而服务端会把编出来的 slug 丢掉，她那边看到的是印记说了要给她一课、
	// 然后什么也没出现。所以 course 这件工具的目录必须是真的库存，不是模型的
	// 记忆。派生自 /courses 的同一批行（见 api/pbl_course.go）。
	Courses []CourseOption
	// CoursesTaken 是她在这个项目里已经上完的课，用它们的标题。
	//
	// 和 ToolsUsed 一个道理：目录本身不带状态，不说它就会被重复递。
	CoursesTaken []string
}

// CourseOption 是课程目录里的一门课，印记挑课时看到的那一行。
//
// 只带四样：怎么点名（Slug）、叫什么、讲什么、要多久。够它判断"这门课对得上
// 她现在卡住的这件事吗"，又不至于把整个课程库塞进 prompt。
type CourseOption struct {
	Slug      string
	Title     string
	Blurb     string
	TimeLabel string
}

// CoachOutput is one turn's result.
type CoachOutput struct {
	// Reply is what she reads.
	Reply string
	// Hook is an optional question that opens a think-deeply session when she
	// taps it. At most one — 铁律③ is one question at a time, and a message
	// carrying three hooks is three questions wearing a coat.
	Hook string
	// HookKind is the session kind the hook would open.
	HookKind string
	// Tool is a tool 印记 offers her this turn, or "". At most one, and it is
	// an OFFER: she opens it or she does not (铁律②).
	Tool string
	// ToolReason is why this tool, right now, in 印记's own words. A tool with
	// no reason is an ambush, so the server refuses to record one without it.
	ToolReason string
	// Mission is the checklist she takes out with her — only for 观察日记.
	//
	// 🚨 这件工具一直没有 before-state：她带着一段话出门，回来面对几个空白框。
	// 设计文档要的是「a small real-world mission」加「a simple observation
	// method」，方法就落在这几条上：把「去看看」拆成几件她在现场做得到的事。
	Mission []MissionItem
	// Produce is a thing 印记 makes this turn, or nil.
	//
	// 🚨 2026-09-02 之前这一格不存在，于是印记**没有任何办法**做出计划、决定、
	// 成果、分工、结构。审核助手 / 理性决策 / 分工建议 / 结构审查 / 计划这五块
	// 界面因此永远是空的，按钮永远是灰的，而其中三块还写着「到对话里请印记给
	// 一个」——让学生去求一件印记结构上做不到的事。端点、客户端函数、表全都
	// 写好了，链子断在这一格上。
	//
	// 和 tool 一样只有一格、一次一件：多做几件就是一次把五张卡拍在她面前。
	Produce *Produced
}

// MissionItem 是清单上的一条：去看什么，以及要带回哪一类东西。
type MissionItem struct {
	Prompt string
	// WantKind 对齐便签的类别（observation / quote / assumption / question），
	// 空 = 印记没指定。她点掉这一条回来，便签的类别就是从这儿来的。
	WantKind string
}

// Produced is one thing 印记 makes: 用哪种、内容是什么。
//
// Payload 在这里不解释，由 api 层按 Kind 分派给对应的创建逻辑——那边本来就有
// 各自的校验（比如"每一步都要说清楚这一步你判断什么"），不该在这儿抄第二份。
type Produced struct {
	Kind    string
	Payload json.RawMessage
}

// ProduceKinds 是印记能做的东西，也是 prompt 里那份目录的来源。
//
// Only 和 Tool.Only 同一个意思：限定给某一类项目，空 = 都能用。
var ProduceKinds = []struct{ Kind, About, Only string }{
	{Kind: "plan", About: "一份计划：每一步写清楚这一步做什么、你带什么、我带什么、**她判断什么**、做完交回什么"},
	{Kind: "decision", About: "一个要她拿主意的选择：一句话说清在选什么，再给两到四个选项，每个选项写明它意味着什么"},
	{Kind: "artifact", About: "一份你写出来交给她审的东西：草稿、方案、或一个网址"},
	{Kind: "substeps", About: "某一步的分工：拆成几件小事，每件写清楚谁做、为什么是他做"},
	{Kind: "structure", About: "一份结构：一棵两到三层的提纲，让她看得见整件东西的全貌"},
	{Kind: "course", About: "从下面【课程库】里挑一门课给她上：写清楚是哪一门（slug 原样抄），以及这门课对她手上这件事有什么用"},
	{
		Kind:  "site_content",
		About: "把她说过的话摆到她主页的各个位置上。**只能摆她的原话**，逐字对不上的那句服务端会丢掉",
		Only:  "website",
	},
}

func IsProduceKind(k string) bool {
	for _, x := range ProduceKinds {
		if x.Kind == k {
			return true
		}
	}
	return false
}

// toolCatalogue renders the toolbox for the prompt.
//
// 🚨 派生自 registry，不手写第二份。手写的目录一定会和工具箱漂移，而漂移的
// 那一天，印记会开始召一件界面上不存在的工具。
func toolCatalogue(kind string) string {
	var b strings.Builder
	for _, name := range ToolNames() {
		t, ok := LookupTool(name)
		if !ok {
			continue
		}
		// 限定给某一类项目的工具，只进那一类项目的目录。见 tools.go · Tool.Only。
		if t.Only != "" && t.Only != kind {
			continue
		}
		where := "当场和他一起做完"
		if t.Kind == KindWorld {
			where = "他要离开屏幕去做，几天后才回来"
		}
		// 🚨 这一句是整个目录里最要紧的：这几件工具的界面是空的，摆的就是
		// 你这一轮做出来的那份东西。不写出来，模型只会看见一个工具名。
		if t.Needs != "" {
			where += "；**必须同一轮配一个 " + t.Needs + "**，否则他打开是一块白板"
		}
		fmt.Fprintf(&b, "  %s（%s）—— %s\n", t.Name, t.Label, where)
	}
	return b.String()
}

// recentWindow bounds how much thread goes into the prompt.
//
// Load-bearing, not a nicety: lite has no compaction layer, so this is the only
// thing between a long project and an unbounded prompt. Same reasoning and same
// size as the reading room's recentTurnsWindow.
const recentWindow = 12

const coachSystem = `你是「印记」，在陪一个中学生做他自己的项目。

你可以解释问题、整理已知信息，并通过下列工具和 produce 生成计划等产物。
你不能替他判断，不能替他决定，%s项目是他的。

【回应与提问】
先回应学生当前的问题。概念、词义和操作问题可以直接解释；必要时给一个与当前
作业不同题目的例子。不要把知识讲解变成反问，也不要用一个问号连接两项独立任务。
最多提出一个需要学生回答的问题；无需补充信息时可以不提问。

学生已经说明的内容不要反复询问。核对关键条件时可以简短确认，但不逐句复述。
指出问题时说明具体依据和下一步，不评价学生的能力、态度或动机。
如果你误解了他或重复提问，简短承认并修正。不要猜测他为什么生气，也不套用安慰。
用具体对象和动作说明事情；专业词第一次出现时，可以附一句简明解释。
用自然完整的句子说明产物，例如「计划已生成，请检查步骤和时间安排」。不要只用两三个字命令学生。
例子是为说明方法构造的候选，不声称来自你亲眼见过的人或经历。

需要了解他的经历时，问一件具体的事，不要预设他只可能遇到你猜的两种情况。
信息足够且有该执行的工作时，先执行，不再追加不影响下一步的问题。
真有需要单独分析的矛盾、过大的问题或预设了答案的问题时，才给钩子。

【他答不上来的时候】
上文写着【他连着答不上来了】，或者学生明确要提示、例子、选项时，先提供帮助。
不要换个措辞重复同一个大问题。根据困难选一种帮助：
① 把问题换成一件具体、已经发生过的事。
② 他要例子或选项时，给三个不同方向的候选，并说明也可以都不选。
   例如：「个人主页可以面向申请学校的老师、社团同学，或关注你作品的读者。
   哪类更接近你的用途，也可以提出其他读者？」
③ 需要区分困难时，只确认一个关键点，例如是否需要先解释当前步骤。

候选不自动成为学生的答案。需要进入个人页面的内容，仍请他用自己的话表述；
服务端只认原话，不把你的示例当成他已经确认的经历或判断。

【工具】
你手边有这些，一次最多递一件：

` + "%s" + `

递之前先想清楚这一刻他卡在哪，然后用一句话说明为什么现在需要它。
他可以不用，不用劝。

🚨 目录里标着「必须同一轮配一个 X」的那几件，**工具和那份 X 要在同一轮一起
给**。那几个界面本身没有内容——理性决策摆的是你做的那个选择，结构审查摆的是
你给的那棵提纲，分工建议摆的是你拆的那几件小事，审核助手摆的是你交上去的那份
东西。工具是他**审**你的地方。

只递工具、不做那份东西，他点进去看到的是一块白板和一句「到对话里请印记先给
一个」——他刚照着你说的点进来，你却让他回来求你再做一遍。要么这一轮 tool 和
produce 一起给，要么这一轮两个都别给。

%s

已经做完的那几件在上文里列着，不要再递。

🚨 **上文列的就是他手上全部的东西。上文里没有的，就是还不存在。**
不要说「分工已经在那张卡里了」「你把它标在了某一格」这一类话——他会去找，
找不到；要么以为自己做过、其实没做，要么发现你在编。两种都比直说「还没有」
糟得多。

该有而没有的，就地递那件工具做出来：他说「这活我一个人干不完，帮我分一分」，
那就是 split + substeps 一起给，不是告诉他分工已经在别处了。

【你自己动手做的东西】
有些东西该由你做出来，交给他看、由他判断——这不是替他做作业，是把一个具体的
东西摆到他面前，好让他有得可判。他要写的正文永远是他自己写。

你能做的：

` + "%s" + `

什么时候做：

- 他把要做的事说清楚了，还没有计划 → 出一份计划。**每一步都要写清楚这一步
  他判断什么**；一步他什么都不用判断，那一步就不该占他的时间。
- 谈到一个岔路口，往哪边走会影响后面 → 给他一个选择，把选项和各自意味着什么
  摆开，由他定。不要替他选。
- 他需要一份东西才能往下走（一版方案、一份草稿） → 你写出来交给他审，并且
  老实说清楚你猜了什么、这一版你自己觉得哪里还不对。

  🚨 交的同时要说清楚**这份东西该怎么看**，不然他只会从头读到尾、点一下通过，
  那不是审：
  · marks —— 从正文里原样抄几句出来，每一句配一个问题。抄的必须是正文里真有
    的字，一字不差，否则划不到。划一句出来却不问什么，只是在把字标黄。
  · dimensions —— 两三个审这份东西非看不可的方面，每个说清为什么要紧。
  他答了其中任何一条，就算他留下了意见，这份东西的结论就变成「执行修改」。
- 某一步要好几个人一起做 → 给这一步的分工，每件小事写清楚谁做、为什么是他做。
- 要做的东西大到看不清全貌 → 给一份结构，两三层就够。
- 他卡在一件**没学过所以做不了**的事上（要写代码、要做一份产品方案、要读一组
  数据、要拍一段片子） → course + course，一起给：从下文【课程库】里挑一门
  真有的课，说清楚这门课能让他接下来的哪一步走得通。

  🚨 只能挑**下文列出来的**课，slug 一字不差地抄。库里没有对得上的，就不要
  递这件工具——编一个课名出来，服务端会把它丢掉，他那边看到的是你说要给他
  一课、然后什么也没出现。
  🚨 上课不是他项目的一步，是他为了走下一步去补的一件本事。所以递之前先想
  清楚：他现在到底缺什么、这门课学完他能立刻做成什么。答不上来就先别递。

produce 每轮最多做一件。它和递工具**不冲突**：一件要配产出的工具，本来就是
和它的产出一起给的。

一轮里**最多问他一个问题，允许不提问**。除此之外，该做的东西就做出来，
不要为了"这一轮已经做过一件事了"把他晾在一个空界面前。

【输出】
只返回一个 JSON 对象，不要别的字：
{"reply": "你说的话", "hook": "钩子问题，没有就空字符串", "hook_kind": "free",
 "tool": "工具名，不递就空字符串", "tool_reason": "为什么是现在",
 "mission": [], "produce": null}

hook_kind 只能是 free / reframe / brainstorm / observation。

🚨 **递 observe 的那一轮，mission 必须一起给。**
她带着一句「去看看」出门，回来只会写一句「大家好像都挺忙的」——那不是观察，
是印象。mission 是她在现场照着做的清单，一条一条点掉：

mission: [{"prompt": "中午 12:30 在走廊数一数站着的人", "want_kind": "observation"},
          {"prompt": "找一个站着的人问他为什么不回教室，把他的话记下来", "want_kind": "quote"}]

写清单的规矩：
· 3 到 5 条，每条是他**在现场十分钟内做得到**的一件事，不是一个研究方向。
· 带上时间、地点、次数——「数一数」「问一个人」「连着看三天中午」。
· want_kind 说这一条要带回哪一类：observation（他亲眼看到的）/ quote（别人的
  原话）/ assumption（他的推论）/ question（他答不上来的）。至少要有一条
  observation 和一条 quote——只带回推论的观察等于没出门。
· 不递 observe 的轮次，mission 一律留空数组。

reply 必须有完整、非空的回应；即使同时生成产物或递工具，也要说明已做什么。
reply、hook、tool_reason 和产物内的文字会直接显示给学生。不要把字段名、生成指令或内部备注写进这些文字。
回复和 hook 合起来最多提出一个需要学生回答的问题，允许没有问号。
只返回合法 JSON，字符串内的换行和双引号必须转义。
判断的是实际任务数量，不是标点数量：「停了多少车，分别是什么类型？」仍是两项任务。
需要说明多个并列要点时可用短列表；只问影响下一步的那一个问题。

🚨 tool_reason **是印记说给她本人看的一句话**，会原样印在工具卡上。所以用
「你」称呼她，不要用「他」「她」「这个学生」——上文这份说明里用的是第三人称，
那是写给你看的，不是她该读到的。写成「你刚说没仔细看过，先去看三天中午」，
不要写成「他需要从观察事实开始」。

produce 不做就是 null。要做就写成 {"kind": "…", "payload": {…}}，payload 的
形状按 kind：

plan:      {"summary": "一句话概括这版计划", "reason": "为什么是这样安排",
            "steps": [{"title": "", "blurb": "", "goal": "", "youBring": "",
                       "iBring": "", "decide": "他在这一步判断什么", "thenBring": ""}]}
decision:  {"subject": "在选什么", "options": [{"label": "", "description": "它意味着什么"}]}
artifact:  {"kind": "draft|spec|site", "title": "", "body": "正文，site 时留空",
            "url": "网址，只有 site 用", "guessed": ["我猜了什么"], "admits": ["这一版哪里还不对"],
            "marks": [{"part": "这是哪一部分", "partNote": "这一部分要留意什么",
                       "quote": "从正文里原样抄一句", "question": "针对这一句要她回答什么"}],
            "dimensions": [{"prompt": "审这份东西必须看的一个方面", "why": "为什么这个方面要紧"}]}
substeps:  {"stepTitle": "这是计划里哪一步", "items": [{"title": "", "owner": "yinji|student|both", "why": "为什么是他做"}]}
structure: {"nodes": [{"title": "", "body": "", "children": [{"title": "", "body": ""}]}]}
course:    {"slug": "课程库里那一门的 slug，一字不差", "why": "这门课对他手上这件事有什么用，用「你」跟他说"}`

// momentsGeneric —— 通用项目里「什么时候递哪一件」。
//
// 抽成一格是因为主页项目要把它整块换掉：那个项目的题目和路线都定好了，这份
// 临场判断已经在路线里做完了（见 website.go · websiteRoutine）。
const momentsGeneric = `【什么时候递哪一件】
这不是一条要走完的流程，是七个不同的时刻。他到了那个时刻，那件工具才有用；
没到就递，是打断。

- 他还说不清自己想弄明白什么，或者只有一个模糊的兴趣 → observe。
- 他一口气说了好几件不太一样的事，或者刚带回来一堆观察 → board。摊开才看得出
  哪几条其实是一回事。
- 板上看得出线索了，问题还是一大团 → reframe。收成「谁需要什么，因为什么」，
  再变成一句「我们可以怎样」。
- 问题定下来了，往哪走还有好几条路 → ideas。
- 你写了一份东西要交给他（一版方案、一份草稿、一个网址） → review + artifact，
  一起给。
- 走到一个岔路口，往哪边走会影响后面 → decide + decision，一起给。
- 要做的东西大到看不清全貌（一份文档、一个网站、一场活动） → structure +
  structure，一起给。先看结构，是为了让他知道结构是可以改的——不然他会照着
  第一版一路做下去，从没想过它可以是另一种做法。
- 进了实施，某一步要好几个人一起做 → split + substeps，一起给。
- 他要动手了，可是这件事他没学过（写代码、做产品方案、读一组数据、剪一段片子）
  → course + course，一起给。挑下文【课程库】里真有的那一门。
- 东西做完了 → lookback。
- 东西放出去了，真的有人在用了 → keep。`

// sprintCoachSystem 只把四格填进系统提示：本事边界、工具目录、什么时候递哪一件、
// 你自己动手做的东西。填什么由 website.go · coachPrompt 决定。
func sprintCoachSystem(abilities, tools, moments, produces string) string {
	return fmt.Sprintf(coachSystem, abilities, tools, moments, produces)
}

var errNoReply = errors.New("pbl: coach produced no reply")

// produceCatalogue 把印记能做的东西渲染进 prompt。
//
// 🚨 和 toolCatalogue 一样派生自那份表，不手写第二份——手写的目录一定会漂移，
// 而漂移的那天，印记会开始做一件服务端认不出的东西。
func produceCatalogue(kind string) string {
	var b strings.Builder
	for _, k := range ProduceKinds {
		if k.Only != "" && k.Only != kind {
			continue
		}
		fmt.Fprintf(&b, "  %s —— %s\n", k.Kind, k.About)
	}
	return b.String()
}

// buildCoachContext renders the project state the turn reasons over.
func buildCoachContext(in CoachInput) string {
	var b strings.Builder
	fmt.Fprintf(&b, "他一开始是这么说的：%s\n", strings.TrimSpace(in.Idea))
	if in.Kind != "" {
		fmt.Fprintf(&b, "项目类型：%s\n", in.Kind)
	}
	if len(in.Steps) > 0 {
		b.WriteString("现在的计划：\n")
		for i, s := range in.Steps {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, s)
		}
	} else {
		b.WriteString("还没有计划——你们还在把这件事聊清楚。\n")
	}
	// 🚨 她在工具里真做出来的东西。这一段是整个 prompt 里最该被用上的部分：
	// 她写下的句子在这儿，印记就能指着其中一句往下说，而不是把她刚做完的事
	// 再问一遍。
	if len(in.ToolWork) > 0 {
		b.WriteString("\n【他在工具里已经做出来的东西】\n")
		for _, w := range in.ToolWork {
			fmt.Fprintf(&b, "  · %s\n", strings.TrimSpace(w))
		}
		b.WriteString("这些是他自己写下的原话。接着往下说的时候，" +
			"指着其中具体的一句说，不要笼统地夸他「做得不错」，" +
			"更不要把他刚做完的事再问一遍。\n")
	}
	// Conclusions of closed digs, never their working.
	if len(in.WriteBacks) > 0 {
		b.WriteString("他之前深挖过，挖出来的结论：\n")
		for _, w := range in.WriteBacks {
			fmt.Fprintf(&b, "  · %s\n", strings.TrimSpace(w))
		}
	}
	if in.SessionKind != "" {
		fmt.Fprintf(&b, "\n【你们现在在一条支线里】要单独想清楚的是：%s\n",
			strings.TrimSpace(in.SessionQuestion))
		b.WriteString("在支线里不要拉回整个项目，就把这一个问题想透。" +
			"这里不要再给钩子，也不要递工具——支线就是为了想一件事。\n")
		if len(in.Recent) == 0 || in.Recent[len(in.Recent)-1].Role != "student" {
			b.WriteString("这条支线刚开，由你先说第一句：接住他上面说的那件具体的事，" +
				"说清楚这一层要一起看的是什么，然后问他一个问题。" +
				"不要把上面那个问题原样重复一遍。\n")
		}
	}
	tail := in.Recent
	if len(tail) > recentWindow {
		tail = tail[len(tail)-recentWindow:]
	}
	if len(tail) > 0 {
		b.WriteString("\n刚才说到：\n")
		for _, t := range tail {
			who := "印记"
			if t.Role == "student" {
				who = "学生"
			}
			fmt.Fprintf(&b, "%s：%s\n", who, strings.TrimSpace(t.Content))
		}
	}
	// 已经做过的工具。目录本身不带状态，不说它就会被重复递出来。
	if len(in.ToolsUsed) > 0 {
		fmt.Fprintf(&b, "\n【已经做完的工具】%s\n"+
			"这几件不要再递了。\n", strings.Join(in.ToolsUsed, "、"))
	}
	// 递过、她还没做完的。这几件已经在她屏幕上摆着了。
	if len(in.ToolsOffered) > 0 {
		fmt.Fprintf(&b, "\n【已经递过、她还没做的工具】%s\n"+
			"这几张卡已经在她屏幕上摆着了，不要再递一遍——"+
			"她看到的会是两张一模一样的卡。她想做自然会点。\n",
			strings.Join(in.ToolsOffered, "、"))
	}
	// 🚨 她连着答不上来。服务端数的（stuck.go），不让模型自己判断——「她是不是
	// 卡住了」交给模型判，等于这条出口存不存在全看它那一轮的心情。
	if in.Stuck >= stuckThreshold || in.AskedForHelp {
		b.WriteString("\n【他连着答不上来了】")
		if in.AskedForHelp {
			b.WriteString("他刚刚**自己开口要例子或选项**了。这是他明说的请求，不要绕开。\n")
		} else {
			fmt.Fprintf(&b, "这是第 %d 轮他说不上来。\n", in.Stuck)
		}
		b.WriteString("这一轮走【他答不上来的时候】那一条：不要再抛敞开的问题，" +
			"换成一个更小更具体的问题、三个明确说明为候选的例子、" +
			"或者把「不知道」拆开问他是哪一种。\n")
	}
	// 🚨 上一轮被撤掉的那件工具。放在两份工具清单后面，因为它说的正是「你以为
	// 递出去了的那件，其实不在那两份清单里」。
	if d := strings.TrimSpace(in.ToolDropped); d != "" {
		fmt.Fprintf(&b, "\n【上一轮有一件工具没递出去】\n%s\n", d)
	}
	// 🚨 课程库。印记挑课**只能**从这里挑——它对课程库没有任何记忆，凭空写一个
	// slug 出来服务端会丢掉，她看到的是"说好的那一课没出现"。
	//
	// 库里一门课都没有（这个版本还没上课，或者课程服务挂了）就整段不写：一份
	// 空目录只会让模型以为"这里有课，只是这次没列出来"，然后照样编一个。
	if len(in.Courses) > 0 {
		b.WriteString("\n【课程库】他这个版本能上的课，只能从这几门里挑：\n")
		for _, c := range in.Courses {
			line := "  " + c.Slug + " —— 《" + strings.TrimSpace(c.Title) + "》"
			if t := strings.TrimSpace(c.TimeLabel); t != "" {
				line += "（" + t + "）"
			}
			if bl := strings.TrimSpace(c.Blurb); bl != "" {
				line += "：" + bl
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("slug 一字不差地抄。这里没有的课就是没有，不要编。\n")
	}
	if len(in.CoursesTaken) > 0 {
		fmt.Fprintf(&b, "\n【他在这个项目里已经上过的课】%s\n"+
			"这几门不要再递了。\n", strings.Join(in.CoursesTaken, "、"))
	}
	// 🚨 这一段必须在最后，而且必须存在：她没打字的那一轮，上面的对话是以
	// 印记自己的话结尾的，模型顺着写下去最可能的就是把那句重说一遍。
	if e := strings.TrimSpace(in.JustHappened); e != "" {
		fmt.Fprintf(&b, "\n【她刚做完这件事】%s\n", e)
		b.WriteString("她这一轮没有打字——她是刚做完上面这件事回来的。\n" +
			"接着这件事往下说：指着她带回来的其中一句具体的话，" +
			"然后往前走一步。不要重复你上一句，也不要再把这件事请她做一遍。\n")
	}
	return b.String()
}

// parseCoachOutput reads the model's JSON.
//
// A hook naming a kind we do not have is dropped rather than rejected: the
// reply is still worth showing, and a silently missing hook costs her far less
// than a dead turn.
func parseCoachOutput(raw string) (CoachOutput, error) {
	s := strings.TrimSpace(raw)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return CoachOutput{}, errNoReply
	}
	var out struct {
		Reply      string `json:"reply"`
		Hook       string `json:"hook"`
		HookKind   string `json:"hook_kind"`
		Tool       string `json:"tool"`
		ToolReason string `json:"tool_reason"`
		Mission    []struct {
			Prompt   string `json:"prompt"`
			WantKind string `json:"want_kind"`
		} `json:"mission"`
		Produce *struct {
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		} `json:"produce"`
	}
	if err := json.Unmarshal([]byte(s[start:end+1]), &out); err != nil {
		return CoachOutput{}, fmt.Errorf("pbl: %w", err)
	}
	reply := strings.TrimSpace(out.Reply)
	if reply == "" {
		return CoachOutput{}, errNoReply
	}
	hook := strings.TrimSpace(out.Hook)
	kind := strings.TrimSpace(strings.ToLower(out.HookKind))
	if hook == "" || !IsSessionKind(kind) || kind == "plan_check" {
		// plan_check is never opened by a hook — it is opened by a staged
		// structural change, which is a different trigger entirely.
		hook, kind = "", ""
	}
	// 🚨 没有理由就当没递。一件说不出为什么的工具，对她是一次打断；而且服务端
	// 本来就会拒绝落库，与其让它半路失败，不如在这里就当它没发生。
	tool := strings.TrimSpace(out.Tool)
	reason := strings.TrimSpace(out.ToolReason)
	if tool == "" || reason == "" {
		tool, reason = "", ""
	}
	// 清单只属于观察日记。别的工具带回来的当没看见——一件当场做完的工具挂一张
	// 出门清单，只会让她以为自己还得出门一趟。
	var mission []MissionItem
	if tool == "observe" {
		for _, m := range out.Mission {
			prompt := strings.TrimSpace(m.Prompt)
			if prompt == "" {
				continue
			}
			mission = append(mission, MissionItem{
				Prompt: prompt, WantKind: strings.TrimSpace(m.WantKind),
			})
		}
	}
	// 🚨 认不出的 kind、空 payload，一律当作没做——半个产出比没有产出更糟：
	// 界面会为它腾出位置，然后摆一块空白。
	var produced *Produced
	if p := out.Produce; p != nil {
		k := strings.TrimSpace(strings.ToLower(p.Kind))
		if IsProduceKind(k) && len(p.Payload) > 0 {
			produced = &Produced{Kind: k, Payload: p.Payload}
		}
	}
	return CoachOutput{
		Reply: reply, Hook: hook, HookKind: kind,
		Tool: tool, ToolReason: reason, Mission: mission, Produce: produced,
	}, nil
}

const maxCoachAttempts = 3

// backoffBeforeRetry 在两次尝试之间等一小会儿。
//
// 🚨 上游 503（服务过载）是会自己好的那种错。两次尝试贴着发出去，撞的是同一
// 波过载——等于只试了一次。2026-09-02 那轮 walk 里连着五个 503，学生看到的是
// 「AI 暂时没接上」，而其实隔一秒再问就有了。
//
// 等待跟着 ctx 走：她把页面关了，就别再等下去。
func backoffBeforeRetry(ctx context.Context, attempt int) {
	d := time.Duration(1<<attempt) * time.Second
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// Coach runs one turn. Usage is returned even on failure so the caller meters:
// a call that produced nothing still cost money.
func Coach(ctx context.Context, prov gateway.Provider, resolved gateway.Resolved, in CoachInput) (CoachOutput, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: coachPrompt(in.Kind)},
			{Role: gateway.RoleUser, Content: buildCoachContext(in)},
		},
		// 🚨 推理模型（deepseek-reasoner）会先花掉一大截 completion token 想事情，
		// 之后才吐出可见内容。原来是 1200，被想事情吃光，返回空 content →
		// 解析失败 → 她看到一句"接口错误"。
		//
		// 16384 是产品负责人 2026-09-02 定的：对话是这个产品的主干，不该在这里
		// 省。见 [[llm-reasoning-model-budgets]]。
		MaxTokens: 16384,
	}
	var lastUsage gateway.ChatUsage
	var lastErr error
	for attempt := 0; attempt < maxCoachAttempts; attempt++ {
		if attempt > 0 {
			backoffBeforeRetry(ctx, attempt-1)
		}
		res, err := gateway.Collect(ctx, prov, resolved, req)
		lastUsage = res.Usage
		if err != nil {
			lastErr = err
			continue
		}
		out, perr := parseCoachOutput(res.Text)
		if perr != nil {
			lastErr = perr
			continue
		}
		return out, lastUsage, nil
	}
	return CoachOutput{}, lastUsage, lastErr
}
