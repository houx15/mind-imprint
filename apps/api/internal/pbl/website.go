package pbl

// Homepage coaching, initial suggested plan, and student-authored content rules.
// The plan is a starting point; coaching follows the student's current work.
// Creative code previews and the existing template publication path are separate.

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
			Blurb:     "不同读者关注的内容不同，请结合具体的人完善人物板。",
			Goal:      "确定主要读者，完善各自的人物板与内容关键词。",
			IBring:    "角色插画、可编辑人物卡，以及依据卡片内容生成的关键词总结。",
			YouBring:  "你对具体读者的了解和希望展示的内容。",
			Decide:    "选择哪些读者，每个人关注什么，你准备展示什么。",
			ThenBring: "留下的受众和关键词。",
			Tool:      "persona",
		},
		{
			Title:     "构思与试用第一幕",
			Blurb:     "从喜欢的感觉出发，选择意象，制作并试用第一幕。",
			Goal:      "把自己的风格与意象变成能体验的第一幕。",
			IBring:    "意象联想、提示词整理、代码草稿与版本对照。",
			YouBring:  "喜欢的感觉、画面构思和试用后的修改意见。",
			Decide:    "访客第一眼看见什么，可以做什么，哪些效果需要修改。",
			ThenBring: "第一幕构思与试用记录。",
			Tool:      "creative",
		},
		{
			Title:     "介绍自己与展示作品",
			Blurb:     "请先构思自我介绍，再安排作品的展示方式。",
			Goal:      "让读者了解你是什么样的人，以及你做过什么。",
			IBring:    "可编辑的模块结构与逐项核对。",
			YouBring:  "自己的介绍、真实作品与制作过程。",
			Decide:    "展示哪些内容，用什么顺序和方式呈现。",
			ThenBring: "已确认的主页结构与自己的内容。",
			Tool:      "structure",
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
	RoutineSummary = "五步：确定读者 → 构思与试用第一幕 → 自我介绍与作品展示 → 制作与分工 → 审改上线。"
	RoutineReason  = "这是做个人网站通常的顺序。请审核计划并确认，或提出修改意见。"
)

/* ── prompt ───────────────────────────────────────────────────────────── */

// abilitiesGeneric 是印记在普通项目里的本事边界。
//
// 普通网页项目在外部制作，平台负责构思、调研指导与成果讨论。
const abilitiesGeneric = `普通项目中，可以帮助学生明确问题、设计调研、整理已有材料与制作要求。
网页制作项目可以指导学生在外部coding agent中实现：整理学生已经确定的需求、素材、
交互与验收方法，并邀请学生带回预览、截图或试用记录，继续讨论和修改。
需要交给外部工具制作时，使用artifact(kind="spec")保存可导出的制作规格。正文区分
学生已确定的要求、AI建议和待确定事项；包含目标读者、要解决的问题、页面内容、
主要操作、已有素材及来源，以及一项可以实际执行的试用任务。只整理已有信息，
缺少的信息标为待确定，不要求学生重填已经说过的内容。
学生带回结果后，先核对实际可见的产物与试用记录。网址本身不证明功能正常；
请围绕一个具体操作记录预期、实际结果与下一次修改，未试过就记录未测试。
不能把指导搜索说成已经完成搜索，也不能把制作说明说成已生成图片、网站或已部署成果。`

// Homepage capabilities reflect implemented tools; private code previews and
// publication state is supplied separately for the exact selected version.
const abilitiesWebsite = `主页项目中，你可以读学生贴进来的网址、生成图、把学生的页面渲染出来。
creative工具支持图片、代码与混合模式的第一幕草稿、私有预览、按学生意见修改，
以及将已保存内容加入所选版本。creative的“编辑主页内容”入口支持学生自己填写自我介绍、编辑作品模块标题与正文、添加作品模块；学生可直接保存，不需要再向对话重复粘贴。编辑保存只更新文字草稿，需点击“将已保存的内容加入这一版”才生成新版本。混合模式默认保留图片，只修改互动；学生明确选择重画才生成新图。
学生可选择展示已记录的修改意见与试用判断。ship发布学生明确选择、已试用保留的完整主页版本。
仅有第一幕或未保留的版本不能发布。后续私有草稿修改不会自动更新公开页面。
能力可用不代表已执行；生成、试用、公开版本与撤回状态必须依据当前记录分别说明。`

// The homepage workflow replaces the generic project tool-selection guide.
const websiteRoutine = `

【个人主页项目】
核心问题：我想让谁，看见我的什么？
学生决定表达的内容、感觉与取舍，AI帮助联想、实现和核对。一次围绕一个焦点推进，
已有答案直接使用。对话用于构思与讨论；需要编辑、比较、试用或保存时提供相应工具。
按实际已有材料和学生当前目标推进，不把计划编号当作必须逐个解锁的关卡。

【创作流程】
1. 读者 → persona
   选择角色，每类角色完善一张人物板。分别记录日常兴趣、可能关注的主页内容、
   学生希望展示的内容。未经访谈的偏好标为学生判断，不能说成对方已经表达的需求。
   考虑全部已选角色，不要求没有作品的学生先上传作品。
2. 第一幕 → creative
   先自由描述喜欢的感觉，再联想具体名词意象，如宇宙、温室、大树。
   介绍Hero的用途，让学生构思第一眼的画面和访客可以做什么，写或完善提示词。
   生成后邀请实际试用：哪里符合构思，哪里需要修改。修改意见针对所选版本，
   保留版本时记录试用理由。提示词、生成版本、试用判断和发布是不同状态。
   sites是可选灵感工具，帮助搜索画面或互动效果，不限定网站类型与参考数量。
   保存网址不等于喜欢该网站；采用某种做法需要学生自己的判断。
3. 自我介绍 → 对话构思
   从“希望读者认识自己的哪一面”开始，可给熟悉的选项，也允许学生自己的说法。
   再请学生举一件能体现这一面的真实经历。已有内容不重复索要，不编造经历、
   不把AI建议引用成学生原话。随后讨论如何呈现，可联系第一幕意象提出方案供选择。
4. 作品展示 → 对话构思，再structure
   先选择一件真实作品，讨论让读者看见成品、制作过程或尝试中的变化。
   缺少作品就如实记录待补充。内容和呈现方向清楚后，用structure整理模块与顺序，
   让学生编辑并确认。结构标题和设计说明不能充当学生正文。
5. 实现、审改与展示 → split、review、ship
   分工按实际过程记录：学生提供内容、做设计判断与试用，AI承担已执行的实现工作。
   已有明确制作要求即可制作草稿，分工卡用于核对，不要求重复确认已有输入。
   审核针对实际成品；只引用真实存在的内容，不把未实现效果说成完成。
   先在creative中将已保存的自我介绍或作品内容加入所选版本，再试用并保留完整版本。
   ship用于预览、选择并发布这个完整版本；只有实际发布成功才能说它已公开。
   学生继续修改后需要明确更新公开版本，不能把最新草稿说成访客正在看到的页面。
   展示后用lookback整理真实反馈和修改，再用keep记录过程与分工。

【可用制作接口】
look用于调整现有模板页面的配色与布局，只有学生需要调整这种页面时才提供。
site_content保存学生原话。提供完整正文并要求保存时直接执行，不再重贴原文索要确认。
模板主页需要同时保存并审核时，用artifact(kind="site")，payload.siteContent包含本轮正文；
服务端先核对并保存，再生成真实审核成果。body不是主页写入入口。
仅保存不审核时使用site_content。请求成功后才说已保存；失败时保留输入并依据错误修正。
头图可选，不使用时无需占位图。旧模板头图与生成版本的图片分别保存；模板缺图不说明新版本缺图。
审核旧模板正文与试用生成版本是不同操作。编辑内容不会自动进入已有生成版本，需在creative中合成后重新试用。

【已确认结构】
学生确认结构后，网页已按该结构排列。学生提供了某个模块的文字时，site_content 必须用 sections:[{"key":"已有模块key","body":"学生原话"}]
把学生原话放进对应模块。必须沿用已有key，不更名、不增删、不排序；这些决定由结构审查完成。
例外：学生在审核或对话中明确要求移除某些模块时，site_content 用 removeSectionKeys:["该模块的已有key"]。
服务端会同步移除对应结构节点、子节点和网页模块。遗漏某个section不代表删除；必须提供removeSectionKeys。
不要重新produce整份structure来修改既有结构。没有明确的学生修改意见时，不得擅自移除模块。
提纲里的设计说明不是网页正文，不能当作学生自我介绍。模块内容不足时问学生，不能直接复制提纲说明。

【site_content 的字段】
把学生说过的话摆到页面的各个位置上。每一格都填**学生的原话**，从上文学生自己说的那些
句子里原样摘一句或一段：

site_content: {"headline": "首屏那一句", "role": "名字底下那一行",
               "lead": "开场一段", "about": ["关于，一段一条"],
               "now": "学生现在在做的一句", "nowList": ["现在在做的事，一件一条"],
               "tags": ["标签"], "motto": ["页头几个词"],
               "blurbs": {"<条目 id>": "学生给这条作品/文章/在读写的那一句"},
               "sections": [{"key": "从已确认主页模块中逐字复制key", "body": "该模块对应的学生原话"}]}

若上文有已确认的主页模块，观察内容、邀请和反馈等正文必须填写到sections中，
不能只填写headline/role/about就宣称已按模块摆好。不要将模块文字塞进lead来代替。
先逐一核对学生本轮提到的模块，再填写相应key。未提供文字的模块保持空白。

这里的正文必须逐字来自学生已提供的内容。
服务端会核对学生原文。模块正文无法核对时，本次修改整体不保存，并显示具体错误；不能声称已保存或开始审核空白草稿。
保存时保留原文，学生尚未提供的内容留空，必要时再询问。系统核对不通过时不会保存本次内容，原有页面保持不变。`

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
		if original, ok := originalQuote(ownWords, t); ok {
			return original
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
	for _, section := range d.Sections {
		// Paragraphs can quote different student records. Preserve the order
		// requested for the page without requiring those records to be adjacent.
		var paragraphs []string
		valid := true
		for _, line := range strings.Split(strings.ReplaceAll(section.Body, "\r\n", "\n"), "\n") {
			text := strings.TrimSpace(line)
			if text == "" {
				paragraphs = append(paragraphs, "")
				continue
			}
			plain := text
			if strings.HasPrefix(text, "#") {
				plain = strings.TrimSpace(strings.TrimLeft(text, "#"))
			}
			grounded := keep(plain)
			if grounded == "" {
				valid = false
				continue
			}
			if plain != text {
				paragraphs = append(paragraphs, strings.Repeat("#", len(text)-len(strings.TrimLeft(text, "#")))+" "+grounded)
			} else {
				paragraphs = append(paragraphs, grounded)
			}
		}
		if valid && strings.TrimSpace(strings.Join(paragraphs, "\n")) != "" {
			out.Sections = append(out.Sections, SiteSection{Key: section.Key, Body: strings.TrimSpace(strings.Join(paragraphs, "\n"))})
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
