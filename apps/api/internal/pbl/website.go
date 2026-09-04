package pbl

// website.go — 主页项目：定好的题目、预置的路线，以及印记在这个项目里的 prompt。
//
// ## 为什么这个项目和别的项目不一样，而又必须是同一个房间
//
// 2026-09-03 之前，主页项目打开的是 `SiteStudio`——一个自带三步和九个输入框的
// 独立工作面。`ProjectSurface.tsx:42` 在 `kind === "website"` 时直接返回它，于是
// 整个项目房间不渲染：没有印记、没有计划、没有工具、没有审核助手、没有分工建议、
// 没有复盘。而这是每个学生被强制做的**第一个**项目——产品给她的第一印象，恰好
// 是产品本身都不在里面的那一屏。
//
// 所以主页项目重新变成一个普通项目。它和别的项目的区别只有三份**数据**：
//
//  1. 题目和驱动问题是定好的（`WebsiteIdea`），不用问她；
//  2. 一份预置的五步路线（`WebsiteRoutine`），在建项目的同一个事务里落成第 1 版
//     计划，`decided_by = "ai"`、每步 `tentative`——她进来就看见一份真的任务清单，
//     并且仍然要自己审一遍才开始；
//  3. 一段按 `kind` 选中的 prompt 路线块（`websiteRoutine`），告诉印记这个项目
//     分五关、每关配哪件工具、以及**在这个项目里它多出哪三样本事**。
//
// 除此之外，一切都是已经存在的机器：coach 的一轮、计划审核、递工具、回灌、复盘。
//
// 见 docs/superpowers/specs/2026-09-03-website-as-pbl-project-design.md。

import "strings"

// WebsiteIdea 是这个项目的驱动问题。
//
// 存进 `pbl_project.idea`，也就是过程评估唯一读得到的「她当初怎么说的」。别的
// 项目那一格是她自己写的一句话；这一个项目是唯一一个「项目是什么」不需要问的，
// 所以那一格写的是**这个项目要回答的问题**，而不是一句对项目的描述。
//
// 🚨 之前这里写的是「做一个属于我自己的主页：把我读过的、写过的、做过的放在一个
// 地方，给别人看。」——那是一句施工说明，不是一个问题。PBL 项目是被问题定义的
// （产品负责人 2026-09-03：「it is a question-defined project」）。第一关回答它，
// 第二关把答案变成结构，第五关拿它检查做出来的页面。
const WebsiteIdea = "我想让谁，看见我的什么？"

// WebsiteName 是项目卡上的名字。
const WebsiteName = "我自己的主页"

// RoutineStep 是预置路线里的一步，字段和 pbl_plan_step 一一对应。
type RoutineStep struct {
	Title     string
	Blurb     string
	Goal      string
	YouBring  string
	IBring    string
	Decide    string
	ThenBring string
	// Tool 是这一步配的那件工具。只用来对齐 prompt 里的路线和真实工具箱，
	// 不写进计划行——递不递工具仍然是印记那一轮的判断。
	Tool string
}

// WebsiteRoutine 是那份「建议的路线」（产品负责人 2026-09-03：
// "with defined topic and suggested routine"）。
//
// 五步，顺序就是她走的顺序。每一步的 `Decide` 都非空且都是一件真的判断——服务端
// 的 `proposePblPlan` 会拒掉 `decide` 为空的步（`step_without_decision`），而那条
// 校验恰好就是这份路线不会退化成一个填表向导的原因：一步她什么都不判断，那一步
// 就不该占她的时间。
func WebsiteRoutine() []RoutineStep {
	return []RoutineStep{
		{
			Title:     "想清楚给谁看",
			Blurb:     "一个网站该是什么样，由读它的人决定，所以先把这个人想清楚。",
			Goal:      "定下一个真实的受众，和三到六个关键词。",
			IBring:    "两三个可能的受众，每个带一张生成的画像和一组关键词。",
			YouBring:  "你对这几个人的判断。",
			Decide:    "哪一个受众是真的，哪些关键词留下。",
			ThenBring: "留下的受众和关键词。",
			Tool:      "persona",
		},
		{
			Title:     "去看真的个人网站",
			Blurb:     "个人网站是有作者的。先看几个真站是怎么做的。",
			Goal:      "看懂三个真站的结构，再搭出你自己的结构。",
			IBring:    "六个真站做起点，以及你贴进来的每一站的结构分析。",
			YouBring:  "三个你自己找到、真的喜欢的网站。",
			Decide:    "哪些结构值得学，你的结构包含哪几块。",
			ThenBring: "一张你自己的结构导图。",
			Tool:      "sites",
		},
		{
			Title:     "给网站定调子",
			Blurb:     "配色、风格、头图，决定别人第一眼看到什么。",
			Goal:      "定下视觉基调，并补齐我手上还缺的材料。",
			IBring:    "我从你读过、写过、做过的东西里已经找到的材料，三组配色，几张头图草稿。",
			YouBring:  "我还没有的那些——你讲给我听。",
			Decide:    "配色、风格、要不要头图。",
			ThenBring: "定下的配色、风格和头图。",
			Tool:      "look",
		},
		{
			Title:     "我来生成，你来分工",
			Blurb:     "这一页由我生成，页面上说你的话的每一句归你。这份分工会写清楚。",
			Goal:      "拿到第一版页面。",
			IBring:    "结构、排版、配色、头图，和一份分工。",
			YouBring:  "你对这份分工的判断。",
			Decide:    "哪些活归我，哪些归你。",
			ThenBring: "第一版页面。",
			Tool:      "split",
		},
		{
			Title:     "逐处审改，然后上线",
			Blurb:     "AI 可能出错，需要对它产出的内容做一次深度审核。",
			Goal:      "审完、改完、拿到你的链接。",
			IBring:    "这一页，以及几处我自己也不确定的地方。",
			YouBring:  "你的意见。",
			Decide:    "这一页哪里还不对。",
			ThenBring: "上线的链接。",
			Tool:      "review",
		},
	}
}

// RoutineSummary / RoutineReason 是第 1 版计划的那两句。
const (
	RoutineSummary = "五步：想清楚给谁看 → 去看真的个人网站 → 给网站定调子 → 生成与分工 → 审改上线。"
	RoutineReason  = "这是做个人网站通常的顺序。请审核计划并确认，或提出修改意见。"
)

/* ── prompt ───────────────────────────────────────────────────────────── */

// abilitiesGeneric 是印记在普通项目里的本事边界。
//
// 说自己能查资料、能做图，学生一试就知道是假的——所以普通项目里这句话是禁令。
const abilitiesGeneric = `也不要说你能查资料、做图、写网站——你现在还
做不了这些，说了他一试就知道。`

// abilitiesWebsite 是主页项目里的本事边界。
//
// 🚨 这个项目里那条禁令**必须**放开三样，否则印记会拒绝做它其实做得到的事：
// 她贴进来的网址是服务端真去读的（materialize.FetchReadable），头图和受众画像
// 是真生成的（draw 档），页面是真渲染出来的。禁令留着的部分一个字不改——放开
// 的只有这三样，多一样都不行。
const abilitiesWebsite = `这个项目里你能做的多三样：读他贴进来的网址、
生成图、把他的页面渲染出来。除了这三样，别说你还能做别的——说了他一试就知道。`

// websiteRoutine 是主页项目的路线块，接在系统提示后面。
//
// 它替掉通用提示里那份「七个不同的时刻」。通用那份的逻辑是「他到了那个时刻，
// 那件工具才有用」——对一个题目和路线都定好的项目，那份判断已经做完了，再让
// 印记临场判断一遍，只会让它跳步或者卡住。
const websiteRoutine = `

【这个项目】
他在做自己的个人网站。这是他的第一个项目，题目和路线都已经定好了，你不用再问
「你想做什么」。

这个项目要回答的问题是：**我想让谁，看见我的什么？** 第一关回答它，第二关把
答案变成结构，第五关拿它检查做出来的页面。他偏离这个问题的时候，把他带回来。

【五关，以及每一关配哪件工具】
计划里那五步就是这五关。按顺序走，一关没完不要跳到下一关。

🚨 **每一关都从"把那件工具递出去"开始。** 先递，再在对话里说这一关要做什么。
用一段话描述这一关该做什么、却不把工具递出去，等于让他对着一个空房间：那些事
只有在工具里做得了，对话里做不了。

尤其是第二关。「他自己去搜、把网址贴进来」说的是**在那件工具里**贴——「站点采集」
本身就给了他六个真站做起点，还有一个贴网址的地方。所以不要回一句「我没办法替你
搜，你自己去搜」就完了；把 sites 递给他，那句话在工具里已经写着了。

1. 想清楚给谁看 → persona + persona 一起给。
   你先做出两三个可能的受众（他是谁、为什么会知道他、想看到什么、这一页该给他
   什么感觉），每个配一组关键词。他挑、他否、他改。留下的关键词后面几关都要用。
2. 去看真的个人网站 → sites。
   他自己去搜、把网址贴进来，你读那几页，每一页回一张卡：这一站在做什么、结构
   是什么、最值得学的一处在哪。三张之后，你提一版「他们的共同点」和「哪里可以是他
   独有的」，让他划掉不同意的。
   然后 structure + structure 一起给：把他要的结构做成一棵两三层的提纲，他在
   导图上改，改完拿第一关的关键词对一遍。
3. 给网站定调子 → look。
   先把你手上**已经有的**材料摆给他看（他读过、写过、做过的那些），再问他还有
   什么要补——他讲给你听，你不要让他填表。然后按关键词给三组配色、一个风格、
   几张头图草稿，他挑。
4. 我来生成，你来分工 → split + substeps 一起给。
   分工要老实：结构、排版、配色、头图是你做的；这一页上说他的话的每一句是他
   自己的。**你不替他写正文。**他在第三关讲给你听的那些话，你可以原样放到页面
   上，但那是搬他的原话，不是你替他写。分工卡上把这件事写清楚。
   然后 artifact 一起给（kind 用 "site"），把这一版页面交给他审。
5. 逐处审改，然后上线 → review + artifact 一起给。
   marks 里抄的必须是这一页上真有的句子，一字不差。他改完，递 ship：那一屏上
   他看这一页、看还缺什么，然后自己按上线。链接归他。

页面放出去之后 → lookback，再 → keep。发布不是终点，他随时可以回来改。

🚨 上面这五关是**这个项目**的路线，通用的那份「什么时候递哪一件」在这里不适用。

【site_content 的字段】
把他说过的话摆到页面的各个位置上。每一格都填**他的原话**，从上文他自己说的那些
句子里原样摘一句或一段：

site_content: {"headline": "首屏那一句", "role": "名字底下那一行",
               "lead": "开场一段", "about": ["关于，一段一条"],
               "now": "他现在在做的一句", "nowList": ["现在在做的事，一件一条"],
               "tags": ["标签"], "motto": ["页头几个词"],
               "blurbs": {"<条目 id>": "他给这条作品/文章/在读写的那一句"}}

🚨 **这一格里的每一句，都必须能在上文他说过的话里逐字找到。**
服务端会逐字比对，对不上的那一句直接丢掉——不报错、不提示，那句话就是不上页面。
所以不要润色、不要改写、不要替他补一句更好听的。他还没说到的位置就留空，下一轮
问他，问出来再摆。

一句都对不上（比如他其实还什么都没说，而你整页都自己写了）时，这一次产出整个
失败。他那边看到的就是页面没有变化。`

// coachPrompt 渲染系统提示。
//
// 🚨 拼装在这里，而不是在调用点 Sprintf：主页项目要换掉两段、追加一段，散在
// 调用点上迟早会有一个地方少换一段，而少换的那一段不会报错，只会让印记在主页
// 项目里说自己不会做图。
func coachPrompt(kind string) string {
	abilities, moments := abilitiesGeneric, momentsGeneric
	tail := ""
	if kind == "website" {
		abilities, moments = abilitiesWebsite, ""
		tail = websiteRoutine
	}
	return sprintCoachSystem(abilities, toolCatalogue(kind), moments, produceCatalogue(kind)) + tail
}

/* ── 页面上的字必须是她说过的 ─────────────────────────────────────────── */

// GroundSiteDraft 丢掉页面草稿里**并非出自她自己所说**的每一句，返回留下的那
// 一份和被丢掉的那些。
//
// ## 为什么这道闸必须在代码里，而不只是写在 prompt 里
//
// 第四关是「我来生成」：印记把她在对话里说过的话摆到页面上。这件事和铁律①
// 「AI 不替学生撰写」之间只隔一层——**摆她的原话是搬运，自己写一句就是代笔**。
// prompt 里写「你不替他写正文」只能降低概率；真正兜住它的是这里。
//
// 同一个教训 2026-09-03 已经付过一次（见 interest.KeepGrounded）：真模型会把
// prompt 里的脚手架当成学生原话返回。所以规矩写成可验的不变量：一句话在她自己
// 说过的文字里逐字找不到，就不上页面。
//
// 逐字包含（折掉空白之后）是故意的严格：它允许印记从她一整段话里**摘**一句短
// 的放上去——那是选择和编排，是它该做的事；它不允许印记改写、润色、或者补一句
// 她没说过的话。
//
// 联系方式永远清空：她是未成年人，页面是公开的，留不留联系方式只能由她自己在
// 界面上决定，不能由一次模型输出决定。见 site.go 顶部那段。
func GroundSiteDraft(d SiteDraft, ownWords string) (SiteDraft, []string) {
	corpus := foldSpace(ownWords)
	var dropped []string

	keep := func(s string) string {
		t := strings.TrimSpace(s)
		if t == "" {
			return ""
		}
		if strings.Contains(corpus, foldSpace(t)) {
			return t
		}
		dropped = append(dropped, t)
		return ""
	}
	keepAll := func(in []string) []string {
		out := make([]string, 0, len(in))
		for _, s := range in {
			if k := keep(s); k != "" {
				out = append(out, k)
			}
		}
		return out
	}

	out := SiteDraft{
		Role:     keep(d.Role),
		Headline: keep(d.Headline),
		Lead:     keep(d.Lead),
		Now:      keep(d.Now),
		Motto:    keepAll(d.Motto),
		Tags:     keepAll(d.Tags),
		About:    keepAll(d.About),
		NowList:  keepAll(d.NowList),
		// 🚨 不是「没被引用所以丢掉」，是**这一格模型永远不许写**。
		Contact: "",
		Blurbs:  map[string]string{},
	}
	for id, v := range d.Blurbs {
		if k := keep(v); k != "" {
			out.Blurbs[id] = k
		}
	}
	return out, dropped
}

// foldSpace 把连续空白折成一个空格，好逐字比对。
//
// 和 interest.foldSpace 同一个做法。没有共用是因为那是另一个包的私有细节：为了
// 三行代码把它导出，会让一个内部实现变成两个包之间的契约。
func foldSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
