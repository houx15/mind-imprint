package agent

// RubricDimension mirrors the TS RubricDimension. Anchors is keyed "L1".."L4".
type RubricDimension struct {
	ID        string
	Name      string
	Framework string
	Anchors   map[string]string
}

// SoloLabels mirrors TS SOLO_LABELS.
var SoloLabels = map[string]string{"L1": "萌芽", "L2": "发展中", "L3": "熟练", "L4": "卓越"}

// soloOrder is the level iteration order (TS soloLevels).
var soloOrder = []string{"L1", "L2", "L3", "L4"}

// FullRubric is the v2 cognitive-model rubric — ten evaluation dimensions, in order.
var FullRubric = []RubricDimension{
	{ID: "D1", Name: "提问清晰度", Framework: "ATL 思维 · 意图与编排（单轮）", Anchors: map[string]string{
		"L1": "开场问句只有一句话，无背景/目标/方向约束",
		"L2": "给了背景或目标之一，但约束/期望模糊，AI 需反问澄清",
		"L3": "单次提问即含背景+目标+约束，问题具体可执行",
		"L4": "单次提问还分层给出子问题与期望产出格式，便于 AI 精准应答",
	}},
	{ID: "D2", Name: "信源辨识", Framework: "信息素养 · CRAAP（单源可信度）", Anchors: map[string]string{
		"L1": "引入了来源却从不追问其出处/可信度",
		"L2": "偶尔问「这可靠吗？」但不深入",
		"L3": "主动要求出处，并能判断单一来源的等级/资质",
		"L4": "识别该来源的立场、资助或利益冲突",
	}},
	{ID: "D3", Name: "横向验证", Framework: "ATL 研究 · 横向阅读（多源对照）", Anchors: map[string]string{
		"L1": "只用单一来源，未另开查证",
		"L2": "口头说「该多看几个来源」但未真正找第二个",
		"L3": "主动多源对照，引入 2+ 独立来源",
		"L4": "溯到原始出处，比较各源权威性与一致性",
	}},
	{ID: "D4", Name: "多视角与让步", Framework: "论证评估 · 反方处理（独占反方）", Anchors: map[string]string{
		"L1": "给出立场但完全不提反方",
		"L2": "提到反方却轻描淡写或稻草人化",
		"L3": "主动取得反方观点并正面回应",
		"L4": "先把反方强化为最强论证(steelman)再让步反驳",
	}},
	{ID: "D5", Name: "论证拆解", Framework: "论证分析 · 拆解他人/AI 的论证", Anchors: map[string]string{
		"L1": "复述某来源/AI 的结论，却当作事实、不分论点与论据",
		"L2": "能准确复述该论证，但不标出论点/论据/假设",
		"L3": "明确标出论点-论据-假设结构，或指出来源对证据的扭曲/断章取义",
		"L4": "进一步指出未明说的隐藏前提，或点名某一具体谬误类型",
	}},
	{ID: "D6", Name: "反思与元认知", Framework: "ATL 反思 · TOK 认知者（反思式采纳）", Anchors: map[string]string{
		"L1": "直接采用 AI 的措辞或方向，无任何犹豫、限定或保留（默认式采纳）",
		"L2": "事后才回顾「也许该……」，但当时未自检",
		"L3": "当场陈述自己的不确定/信心，或点名盲点；采用 AI 方向前先说明理由（反思式采纳）",
		"L4": "觉察并把方法迁移到新子问题/新情境",
	}},
	{ID: "D7", Name: "论证质量", Framework: "ATL 沟通 · 自身论证产出", Anchors: map[string]string{
		"L1": "自己的论证只堆结论或观点，无论点-论据支撑",
		"L2": "有明确结论，但论据零散、claim 与 evidence 未连接",
		"L3": "自己的论证：论点-论据-解释结构完整，引用有出处",
		"L4": "论证链严密、经得起追问",
	}},
	{ID: "D8", Name: "信息再生产", Framework: "学术诚信 · 出处/署名（复制检测）", Anchors: map[string]string{
		"L1": "整段照搬 AI 输出，不标注、不改写",
		"L2": "有改写，但未标明哪些来自 AI、哪些是自己",
		"L3": "明确区分 AI 贡献与个人加工，主动声明 AI 使用",
		"L4": "诚信声明清晰可核，标注 AI 贡献边界",
	}},
	{ID: "D9", Name: "AI 边界与伦理", Framework: "TOK 知识与技术 · 事实核查（认知核查）", Anchors: map[string]string{
		"L1": "把 AI 当全知，对其事实主张不质疑是否出错/编造",
		"L2": "口头承认「AI 可能不准」，但不采取核查动作",
		"L3": "主动核查 AI 的可疑/可能幻觉处，发现问题即指出",
		"L4": "因核查结果实质修订或拒用 AI 的产出",
	}},
	{ID: "D10", Name: "协作编排", Framework: "意图与编排 · 跨轮驱动与贡献", Anchors: map[string]string{
		"L1": "把 AI 当答案机器：直接要成品，不带入自己的材料，不追问、不调整",
		"L2": "被 AI 追问后才补充自己的材料；不主动规划协作步骤",
		"L3": "未经提示带入自己的草稿/链接/提纲，并跨轮驱动改进、指派子任务",
		"L4": "跨步骤编排 AI 角色、管理上下文、复用/沉淀可重用结构，分清留与弃",
	}},
}
