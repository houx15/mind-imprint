package evalreport

func strptr(s string) *string { return &s }

// Placeholder returns a full, abundant, deterministic fake EvaluationReport
// (the Phoebe / 中国是否让地球变得更可持续 content, lifted from the approved
// mockup docs/reference/2026-08-13-eval-report-mockup.html).
//
// This stands in for the real generation algorithm, which a later change
// builds behind the same signature. It takes generatedAt as an argument and
// contains no time.Now()/randomness, so it is deterministic and testable.
func Placeholder(projectID, reportID, studentID, studentName, title, ptype, generatedAt string) Report {
	r := Report{
		Version: 1, ReportID: reportID, ProjectID: projectID, GeneratedAt: generatedAt,
	}
	r.Student.ID = studentID
	r.Student.Name = studentName

	r.Basics = Basics{
		Title: title, Type: ptype,
		StartDate: "2026-08-01T09:00:00Z", EndDate: strptr("2026-08-06T16:30:00Z"),
		Milestones: Milestones{
			Started:           strptr("2026-08-01T09:00:00Z"),
			FrameworkFinished: strptr("2026-08-02T11:00:00Z"),
			ProposalFinished:  strptr("2026-08-03T14:00:00Z"),
			WritingFinished:   strptr("2026-08-05T15:00:00Z"),
			ProjectFinished:   strptr("2026-08-06T16:30:00Z"),
		},
		Counters: Counters{AITurns: 612, MaterialsRead: 9, WordsWritten: 812, AICommentCount: 23, EditCount: 18},
	}

	r.Abstract = Abstract{
		Overview:            "Phoebe 呈现出一条相当完整的 AI 协作写作证据链。她从一篇微信公众号文章出发，没有直接接受标题叙事，而是逐步追到 NASA Ames 页面、Nature Sustainability 论文、ESSD/GCB 与 OWID 排放数据、The Economist 视频与 World Bank 背景材料，再处理推荐流里的反方视频与 co2science 页面，最后在写作与 Review 中把来源功能、数据口径、反方边界与 AI 使用边界逐项收束。最关键的证据不是「找到很多来源」，而是她反复说明每条来源能证明什么、不能证明什么；最关键的变化是结论从「中国是否让地球变绿」收束为「qualified yes：在 leaf-area greening 维度成立，但不能推出整体可持续」。",
		MaterialSentence:    "共 9 条来源，为每条记录了打开理由、来源层级与数据口径，并在 Review 中把留痕不足的 MEE 移出正文——来源纪律真实影响了成品。",
		WritingSentence:     "18 次打磨中不仅改措辞，更把 prove / largest polluter / China caused 等越界表达逐一收窄，结论稳定在 qualified yes。",
		AISentence:          "让 AI 递工具卡、做追问、检查越界、模拟答辩；写了 AI use / not-used-for 清单，保留最终取舍与判断署名。",
		SuggestionParagraph: "把 source log 做得更标准化——每条来源都固定填入 opened by、why opened、source type、claim supported、cannot support、citation use、status。这样你在答辩时会更轻松，也更容易证明最终文章的判断归属于你。",
		SuggestionSentences: []string{
			"如果继续扩展：可单独开一段讨论 per-capita / historical emissions，但不要在 800 词正文里随手加。",
			"若重新加入 MEE：必须补齐真实 URL、打开理由、摘录与来源功能，并说明它是 policy context，不是独立结果证明。",
			"若更严谨地使用视频：保留观看时间点、人物、场景、字幕或自己的观看笔记，避免出现 AI 生成的视频细节。",
			"若要把反方写得更强：先承认 CO2/WUE 窄机制，再说明为什么不能跳到 system-level climate safety。",
		},
		RecommendedCourses: []RecommendedCourse{
			{CourseID: "course:source-triage", Reason: "你已能分配来源功能，这门课把「哪些进正文、哪些进 source log、哪些进兔子洞」变成可复用的标准。"},
			{CourseID: "course:data-scope", Reason: "巩固 annual share / 历史累计 / 人均 的区分，避免不同口径的数字互相抵消。"},
		},
	}

	r.Events = []EventEntry{
		{TS: "2026-08-01T09:05:00Z", Kind: "chat", Summary: "从微信公众号标题进入，提取可追源关键词，自建 source / why opened / can show / cannot show 字段，并立下「未打开不进来源表」的红线。", AITurns: 24,
			Ref: &Ref{ID: "message:t003", Label: "研究问题成形", TS: "2026-08-01T09:20:00Z"}},
		{TS: "2026-08-02T10:14:00Z", Kind: "reading", Summary: "从公众号追到 NASA Ames 页面，核查作者/日期/China-India 口径，再追到 Nature 论文，确认 25% 与 6.6% greening 数字。", AITurns: 64,
			Ref: &Ref{ID: "message:t048", Label: "NASA → Nature 追源", TS: "2026-08-02T10:50:00Z"}},
		{TS: "2026-08-02T15:20:00Z", Kind: "reading", Summary: "自行检索 CO2 排放份额，建立 ESSD/GCB 与 OWID 数据链（约 31.8%）；用 The Economist 视频 + World Bank 补强影响侧背景。", AITurns: 87},
		{TS: "2026-08-03T11:00:00Z", Kind: "graph", Summary: "从推荐反方视频进入 co2science，查 About/Mission/Staff 追机构身份，横向读到 DeSmog/Exxon 线索，因 off-topic 与字数限制退出，作为 research log 保留。", AITurns: 46,
			Ref: &Ref{ID: "message:t263", Label: "退出兔子洞", TS: "2026-08-03T11:40:00Z"}},
		{TS: "2026-08-05T13:02:00Z", Kind: "writing", Summary: "设 800 词与段落功能、列不能写的红线词；分段写 greening / emissions / impacts / counterclaim，拒绝 AI 代写、只要结构提示与越界检查。", AITurns: 130},
		{TS: "2026-08-06T10:00:00Z", Kind: "review", Summary: "把 Review 定义为可答辩检查而非润色：逐项核对 claim / 段落功能 / 数据口径 / 引用，发现 MEE 缺 URL 并移出正文，形成 AI use statement。", AITurns: 64,
			Ref: &Ref{ID: "message:t461", Label: "MEE 证据审查", TS: "2026-08-06T10:35:00Z"}},
		{TS: "2026-08-06T16:00:00Z", Kind: "milestone", Summary: "逐条说明每条来源为什么用、能证明什么、不能证明什么；整理 essay / source log / evidence cards / rabbit-hole log / citation audit / reflection / AI use 提交包。", AITurns: 22},
	}

	r.Materials = []MaterialEntry{
		{MaterialID: "material:wechat", AddedAt: "2026-08-01T09:05:00Z", Source: "微信公众号《卫星发现地球变绿，源头在印度和中国》",
			FinalStatus:   "origin only",
			Comment:       "真实看到的第一条材料，标题《卫星发现地球变绿，源头在印度和中国》。她识别出「源头」一词会制造强因果感，把它降级为追源入口。",
			CannotSupport: "不能证明 NASA 原意、中国单独贡献或整体可持续。"},
		{MaterialID: "material:nasa-ames", AddedAt: "2026-08-02T10:20:00Z", Source: "NASA Ames（Abby Tabor，2019）",
			FinalStatus:   "bridge source",
			Comment:       "从公众号线索自己搜到，记录了机构/作者/发布日期/更新状态；标出标题里的 dominates 为强动词，正文慎用。放 intro 一句，不与 Nature 重复投票。",
			CannotSupport: "公众摘要；China and India 合并口径不能写成 China alone。"},
		{MaterialID: "material:nature-sustainability", AddedAt: "2026-08-02T11:05:00Z", Source: "Nature Sustainability（同行评审论文，2019）",
			FinalStatus:   "greening 核心证据",
			Comment:       "从 NASA 页面底部打开，确认 China 贡献全球净叶面积增加 25%、仅占全球植被面积 6.6%，用对照支撑「disproportionately」。",
			CannotSupport: "只支持 leaf-area greening，不证明生物多样性、碳中和或整体生态健康。"},
		{MaterialID: "material:essd-gcb-owid", AddedAt: "2026-08-02T15:20:00Z", Source: "ESSD/GCB 数据集 + OWID 可视化",
			FinalStatus:   "限制性证据",
			Comment:       "自己检索得到，读出 2024 年前后约 31.8% 的年度全球 CO2 份额，用来限制 greening 的外推——31.8% 不是反驳 25%，而是限制它能代表什么。两者视为同一数据链，不当两张独立票。",
			CannotSupport: "annual share，不是历史累计、不是人均、不是所有污染。"},
		{MaterialID: "material:economist-worldbank", AddedAt: "2026-08-02T16:40:00Z", Source: "The Economist 评论视频 + World Bank 报告",
			FinalStatus:   "第二媒介 · 背景补强",
			Comment:       "对媒体身份做 CRAAP 初查，把视频从「老师要求的媒介」变为经评估的叙事材料；用 World Bank 孟加拉报告补强背景，让 human vulnerability 进入评价框架。视频与机构材料一起用，不让视频单独承重。",
			CannotSupport: "孟加拉个案不是中国归因证据；视频不能单独承重。"},
		{MaterialID: "material:co2science", AddedAt: "2026-08-03T11:10:00Z", Source: "co2science（WUE 倡议型网页）",
			FinalStatus:   "counterclaim trigger",
			Comment:       "从推荐反方视频进入，先不反驳、只问它能证明什么；查 About/Mission 后把它与 NASA/Nature 区分。steelman 化处理：承认 CO2 提高植物水分利用效率的窄机制，正文中只作一句让步。",
			CannotSupport: "不能推出排放无害或气候变化不严重。"},
		{MaterialID: "material:exxon-desmog", AddedAt: "2026-08-03T11:40:00Z", Source: "Exxon / DeSmog 线索（横向阅读发现）",
			FinalStatus:   "rabbit-hole log",
			Comment:       "横向阅读时第一次看到公司/资金线索，承认有吸引力但会把题目转成 fossil-fuel corporate communication，因 off-topic 与字数限制主动退出，作为 research log 保留。",
			CannotSupport: "情绪不能替代证据；off-topic 不进 800 词正文。"},
		{MaterialID: "material:mee-candidate", AddedAt: "2026-08-04T09:00:00Z", Source: "MEE 候选（政策来源候选）",
			FinalStatus:   "candidate / removed",
			Comment:       "曾被提到并一度想用作 policy context，但 Review 阶段发现当前表里没有具体 URL 和打开理由，于是从正文与答辩正式来源中移除，并准备有/无 MEE 两个版本。",
			CannotSupport: "未保留链接与摘录，不能作为已采用来源。"},
	}

	r.Depth = []DepthDimResult{
		{ID: "D1", Level: 3, Summary: "问题从「这真的假的、中国是不是主角」逐步被材料迫着收窄为 to what extent 的三维框架。",
			Evidence: []EvidenceItem{
				{ID: "message:t003", TS: "2026-08-01T09:20:00Z", Stage: "真实起点", Quote: "公众号说的地球变绿是真的吗？中国到底是不是主角？", Observation: "用人话说出最初问题，不先套研究框架。", Boundary: "此时只是直觉怀疑，尚未成为可检验的研究问题。"},
				{ID: "message:t179", TS: "2026-08-01T16:10:00Z", Stage: "研究问题形成", Quote: "To what extent has China contributed to planetary sustainability, if measured by greening, emissions, and impacts?", Observation: "在多轮追源之后才形成正式 RQ。", Boundary: "不是 AI 一开始给出三维框架。"},
			},
			Suggestion: "下次可显式写出被排除的子问题及原因，让问题边界更透明。"},
		{ID: "D2", Level: 4, Summary: "不仅找到来源，还记录打开理由、来源层级、数据口径与不能证明的内容；MEE 被移除说明来源纪律真实影响正文。",
			Evidence: []EvidenceItem{
				{ID: "message:t010", TS: "2026-08-01T09:35:00Z", Stage: "真实起点", Quote: "source = 我实际打开过的网页或视频。", Observation: "自建来源准入标准。", Boundary: "标准是自发定的，尚未验证后续是否真的被坚持执行。"},
				{ID: "message:t048", TS: "2026-08-02T10:50:00Z", Stage: "NASA 页面", Quote: "我把 NASA 状态改成 audited / bridge source，而不是 core evidence。", Observation: "继续追到论文而非停在新闻稿。"},
				{ID: "message:t461", TS: "2026-08-06T10:35:00Z", Stage: "Review：证据审查", Quote: "MEE 这版表里没有具体 URL。", Observation: "候选来源被排除在已用来源之外。", Boundary: "候选来源不能当已采用来源。"},
			},
			Suggestion: "把「口径是否一致」也写进每条来源的备注。"},
		{ID: "D3", Level: 3, Summary: "三类指标放进一个限定判断：greening 提供正证据，emissions 限制外推，impacts 说明为何不能只看植物。",
			Evidence: []EvidenceItem{
				{ID: "message:t324", TS: "2026-08-05T14:05:00Z", Stage: "emissions 段", Quote: "我需要写为什么 31.8% 不是反驳 25%。", Observation: "处理证据张力，而非互相抵消。", Boundary: "两个数字口径不同。"},
				{ID: "message:t366", TS: "2026-08-05T16:20:00Z", Stage: "结论段", Quote: "qualified yes，不是因为证据弱，而是指标方向不同。", Observation: "主张、证据与限制在结论处闭合。"},
			},
			Suggestion: "让步段后补一句「因此结论如何被限定」。"},
		{ID: "D4", Level: 3, Summary: "能区分标题框定、机构新闻稿、同行论文、媒体视频、倡议网页与反方入口的不同功能；Exxon 支线被主动退出。",
			Evidence: []EvidenceItem{
				{ID: "message:t054", TS: "2026-08-02T10:55:00Z", Stage: "NASA 页面", Quote: "combined China and India, not China alone。", Observation: "防止归因过度。", Boundary: "只纠正了归因主体，尚未处理数据口径本身。"},
				{ID: "message:t263", TS: "2026-08-03T11:40:00Z", Stage: "退出兔子洞", Quote: "这条线已经离我要写的题目很远了。", Observation: "off-topic 主动退出。"},
			},
			Suggestion: "为每条来源标注其立场与潜在利益，让视角识别更系统。"},
		{ID: "D5", Level: 3, Summary: "修订不是语言层面：多次把 prove / largest polluter / China caused 等越界表达改成范围更准的说法。",
			Evidence: []EvidenceItem{
				{ID: "message:t078", TS: "2026-08-05T13:40:00Z", Stage: "Nature 论文链", Quote: "删「China restored nature across the world」→ 改为可证的 leaf-area 贡献。", Observation: "修订是降低主张强度。", Boundary: "改的是单句表达，尚未确认后续段落是否统一收紧。"},
				{ID: "message:t308", TS: "2026-08-05T14:50:00Z", Stage: "greening / emissions 段", Quote: "「prove」→「supports a strong claim」；「largest polluter」→「major current emitter」。", Observation: "越界表达被逐一收窄。"},
			},
			Suggestion: "把每次修订「改了什么、为什么」记成一句修订日志。"},
		{ID: "D6", Level: 4, Summary: "反思落在 AI 使用说明、提交前终检与方法句：意识到证据需要 bounded, placed, connected to a warrant。",
			Evidence: []EvidenceItem{
				{ID: "message:t518", TS: "2026-08-06T15:40:00Z", Stage: "提交前终检", Quote: "strong evidence is not only credible; it is bounded, placed, and connected to a warrant.", Observation: "抽象为可迁移方法。", Boundary: "方法句本身不是证据，要看终检清单是否被真正执行。"},
				{ID: "message:t519", TS: "2026-08-06T15:50:00Z", Stage: "提交前终检", Quote: "终检确认：正文无未打开来源、无 AI 生成视频细节、无过度结论、无来源堆叠。", Observation: "把方法转成可执行检查表。"},
				{ID: "message:t520", TS: "2026-08-06T10:20:00Z", Stage: "复盘 Review", Quote: "如果重写，我会更早决定要不要留 MEE，而不是拖到最后一天。", Observation: "对时间管理与来源纪律做出具体反思。"},
			},
			Suggestion: "把边界口径固定成一句可复用的话，迁移到下个项目。"},
	}

	r.Autonomy = []AutonomyDimResult{
		{ID: "A1", Band: 3, Summary: "关键方向多次由她提出：自己搜 NASA、自己补排放、要求不假造反方。",
			Evidence: []EvidenceItem{
				{ID: "message:t022", TS: "2026-08-01T10:05:00Z", Stage: "真实起点", Quote: "我不是让 AI 给我 NASA 链接，而是自己用公众号里的线索搜。", Observation: "检索路线归属清楚。", Boundary: "AI 可帮术语，不替代打开路径。"},
				{ID: "message:t205", TS: "2026-08-01T17:00:00Z", Stage: "研究问题形成", Quote: "反方我还没有想法，一会我搜好资料确定了再回来写。", Observation: "不假造反方。"},
			},
			Suggestion: "把「为什么走这条线」也记一句，让方向选择可追溯。"},
		{ID: "A2", Band: 4, Summary: "主动启动追源、横向阅读、媒体身份检查与 Review 漏洞发现——但工具卡也参与推动，需保留触发方。",
			Evidence: []EvidenceItem{
				{ID: "message:t090", TS: "2026-08-02T15:25:00Z", Stage: "排放数据链", Quote: "我搜到了 China share of global CO2 emissions 2024…", Observation: "补证由研究问题需要驱动。"},
				{ID: "message:t221", TS: "2026-08-03T11:15:00Z", Stage: "查这个网站是谁", Quote: "主动点开 co2science 的 About Us 做来源身份检查。", Observation: "主动核查来源身份。", Boundary: "触发方：部分由 SIFT 工具卡推动，非全部自发。"},
			},
			Suggestion: "保持——可把核查清单固化成模板。"},
		{ID: "A3", Band: 4, Summary: "反复设边界：未打开不进来源表、AI 不替写、视频不由 AI 虚构、MEE 缺 URL 移除、红线词不写。",
			Evidence: []EvidenceItem{
				{ID: "message:t285", TS: "2026-08-05T13:05:00Z", Stage: "开稿边界", Quote: "我先不开全文，请你只检查写作计划，不替我写。", Observation: "AI 只能做边界检查，不能替生成终稿。", Boundary: "AI 只能做边界检查，不能替生成终稿。"},
				{ID: "message:t293", TS: "2026-08-05T13:20:00Z", Stage: "开稿边界", Quote: "列不能写的词：prove sustainability、saved the Earth、China caused Bangladesh migration。", Observation: "语言红线直接约束写作。"},
			},
			Suggestion: "把边界口径固定成一句可复用的话，迁移到下个项目。"},
		{ID: "A4", Band: 3, Summary: "没有编造反方，而是从推荐流进入真实反方，拆成窄机制并要求答辩追问。",
			Evidence: []EvidenceItem{
				{ID: "message:t213", TS: "2026-08-03T11:20:00Z", Stage: "反方视频后", Quote: "我先不反驳它，只问它能证明什么。", Observation: "对抗是功能判断，不是立刻否定。"},
				{ID: "message:t498", TS: "2026-08-06T16:10:00Z", Stage: "专业答辩", Quote: "请你像老师一样连续追问我每个来源的使用理由、证据功能和边界。", Observation: "主动要求高压答辩模拟。"},
			},
			Suggestion: "让 AI 标注反例的信源强度，把检验做得更细。"},
		{ID: "A5", Band: 4, Summary: "答辩段最清楚：逐一说明每条来源为什么用、为什么不用、能证明什么、不能证明什么。",
			Evidence: []EvidenceItem{
				{ID: "message:t499", TS: "2026-08-06T16:12:00Z", Stage: "专业答辩", Quote: "公众号我放 source log，不放正文——它能说明我怎么进入问题，但不是原始研究。", Observation: "清楚说明来源功能分配。"},
				{ID: "message:t422", TS: "2026-08-05T18:00:00Z", Stage: "最终正文草稿", Quote: "最终自查：正文没有公众号、DeSmog/Exxon、MEE，因为都不该进正文核心。", Observation: "自主排除不合格来源。", Boundary: "不是 AI 替她删。"},
			},
			Suggestion: "在复盘里点名哪些判断完全由自己做出。"},
		{ID: "A6", Band: 3, Summary: "同时拒绝 full yes 和 no：qualified yes 不是折中术，而是不同指标张力下的限制判断。",
			Evidence: []EvidenceItem{
				{ID: "message:t436", TS: "2026-08-06T10:15:00Z", Stage: "Review：claim 审查", Quote: "不能改成 full yes，因为 31.8% annual CO2 share 和 climate impacts 会顶回来。", Observation: "用证据张力约束主张强度。"},
				{ID: "message:t437", TS: "2026-08-06T10:16:00Z", Stage: "Review：claim 审查", Quote: "也不能改成 no，因为 Nature 的 25% leaf-area 增加不能被排放数据直接抹掉。", Observation: "拒绝用一条证据抹掉另一条。"},
			},
			Suggestion: "记录一次「被证据改变主意」的具体时刻，作为迁移样本。"},
	}

	r.PromptLens = PromptLens{
		Summary: "提示词本身不分档。真正能成立为过程证据的，是提示词之后是否产生了来源表、草稿修改、Review 记录或答辩回应。Phoebe 的提问以「要求过程工具 / 要求不替写 / 要求只查越界 / 要求模拟答辩」为主；少数「帮我总结视频」这类自然请求后续也被她自己补上了观看记录，因此不应自动判为低质量。",
		Prompts: []PromptItem{
			{Stage: "真实起点", Quote: "我刚刷到一篇微信公众号文章…这个真的假的？是不是说中国把地球变绿了？", Ref: Ref{ID: "message:t002"}, Observation: "暴露真实材料起点与初始问题，不是让 AI 直接写文章——后续产生了 source log 与追源路径。", RelatedDomains: []string{"D1", "A1"}, Attention: false},
			{Stage: "写作过程", Quote: "我先不开全文，请你只检查写作计划，不替我写。", Ref: Ref{ID: "message:t285"}, Observation: "把 AI 限定为计划检查者；后续写作分段功能与红线词由她自己设定。", RelatedDomains: []string{"A3", "D5"}, Attention: false},
			{Stage: "Review", Quote: "我把整文 v2 贴出来，先请你只检查 claim 有没有越界，不润色。", Ref: Ref{ID: "message:t389"}, Observation: "把 AI 放在审阅者位置，触发了 claim / 数据口径逐项审查。", RelatedDomains: []string{"D3", "A5"}, Attention: false},
			{Stage: "视频处理", Quote: "你能不能帮我总结视频。", Ref: Ref{ID: "message:t148"}, Observation: "自然的求助点；AI 无法真正观看视频，她随后自己看完并记录了人物与时间点，把视频降为叙事/影响侧证据——风险被她自己化解。", RelatedDomains: []string{"A3"}, Attention: true},
		},
	}

	r.ToolUsage = []ToolUsageEntry{
		{ToolID: "card:sift", Name: "SIFT 溯源", Stage: "真实起点 · 反方", Purpose: "停止判断、提取关键词、追源、横向阅读",
			Summary: "帮助她从公众号标题进入 NASA、Nature 与 co2science 身份检查；触发后仍看她是否实际打开。"},
		{ToolID: "card:craap", Name: "CRAAP 体检", Stage: "补证", Purpose: "媒体/网页是否可用的局部判断",
			Summary: "The Economist 被定位为综合媒体叙事证据，co2science 先问「能证明什么」再定位为反方入口。"},
		{ToolID: "card:data-three-questions", Name: "数据三问", Stage: "补证 · Review", Purpose: "追问数字对象、口径、时间范围与可比性",
			Summary: "25% / 6.6% / 31.8% / IEA 0.5% / WUE 都被拆口径；防止数字互相抵消，只限定适用范围。"},
		{ToolID: "card:warrant", Name: "论证解剖 / warrant", Stage: "写作", Purpose: "拆 claim–evidence–reasoning 与缺失 warrant",
			Summary: "把证据放回段落功能，而不是堆材料；构造让步段接住反方最强点。"},
		{ToolID: "subagent:review", Name: "审阅 / AI 伦理审计子代理", Stage: "Review", Purpose: "把 AI 从代写者改为审阅者、追问者、边界检查器",
			Summary: "对全稿给出 23 条批注，学生采纳后形成 AI use statement 与 not-used-for 清单。"},
		{ToolID: "subagent:reading-room", Name: "阅读室子代理", Stage: "追源", Purpose: "学科透镜下逐句共读",
			Summary: "以环境科学透镜共读 NASA / Nature，帮助分辨数据与结论、标出强动词。"},
	}

	r.Risks = []RiskEntry{
		{Type: "missing-source", Behaviour: "MEE 因缺少具体 URL 和打开理由被移出正文。", Ref: &Ref{ID: "message:t461"},
			Suggestion: "报告中写成「候选 / 未采用」，不能写成已使用；下次补齐 URL、打开理由与摘录再纳入。"},
		{Type: "argument-logic", Behaviour: "公众号「源头」与 NASA「dominates」都可能诱导 China-alone 或强因果归因。", Ref: &Ref{ID: "message:t006"},
			Suggestion: "表扬她识别了风险，但不把标题当证据；正文动词回到论文口径。"},
		{Type: "data-scope", Behaviour: "早期把 annual share、net leaf-area increase、vegetated area 口径混用了一次。", Ref: &Ref{ID: "message:t098"},
			Suggestion: "不能说 31.8% 反驳 25%，只能说它限制整体可持续外推；陈述前先声明口径。"},
		{Type: "rabbit-hole-offtopic", Behaviour: "横向阅读时进入 Exxon / DeSmog 资金线索，一度有被题目吸走的风险。", Ref: &Ref{ID: "message:t263"},
			Suggestion: "记录为 research log 而非正文证据；情绪化材料不能替代证据，off-topic 主动退出并保留记录。"},
		{Type: "ai-ghostwrite", Behaviour: "存在「帮我总结视频 / 能不能作为论据」等自然请求，接近让 AI 代劳阅读与判断。", Ref: &Ref{ID: "message:t148"},
			Suggestion: "她持续要求 AI 检查而非代写，并明确 AI not used for；最终反思与正文自写证据依赖对话记录可查。"},
		{Type: "missing-source", Behaviour: "The Economist 视频一度未记录观看时间点与人物，存在让视频单独承重的风险。", Ref: &Ref{ID: "message:t147"},
			Suggestion: "视频作媒介与个案叙事，不当科学证明或中国归因；保留时间点、场景与自己的笔记。"},
	}

	return r
}
