package agent

// studioflow.go — the status-router registry (spec:
// docs/superpowers/specs/2026-08-09-status-router-studio-redesign.md). It
// COLLAPSES the seven persisted StudioStage values into five canonical
// FlowStatuses, each owning a short prompt + a small tool set + a surface. No
// data migration: StudioState.Stage keeps storing the existing values;
// StatusForStage projects them onto the router's coarse-grained view, and
// StageFloor maps a status back to the canonical stage the deterministic router
// persists.

// WritingDoc names which writing document a status operates on (Phase B keys
// buffer/snapshots on this). Empty for non-writing statuses.
type WritingDoc string

const (
	DocNone     WritingDoc = ""
	DocProposal WritingDoc = "proposal"
	DocEssay    WritingDoc = "essay"
)

// StatusDef is one status's whole posture: the short prompt the fast model
// sees, the closed tool subset it may use, the cards it may summon, the room
// that opens, and the writing document it targets. This is the progressive-
// disclosure unit — small enough that a non-reasoning model drives it.
type StatusDef struct {
	Goal         string
	SystemPrompt string
	Tools        []string
	Cards        []string
	Surface      OpenTool
	Doc          WritingDoc
}

// knownStatusTools is the closed set a status may list. set_status / open_tool /
// generate_plan are DELIBERATELY absent — status transitions and plan
// generation are deterministic server-side (advanceStudioFlow), never the
// model's job. open_reading + finish_part are new status tools.
var knownStatusTools = map[string]bool{
	"propose_note": true, "summon_card": true, "open_reading": true,
	"finish_part": true, "request_review": true, "propose_question": true,
	"curate_reference": true, "update_plan": true, "note_resource_need": true,
}

// IsKnownStatusTool reports whether name is a tool a status may list.
func IsKnownStatusTool(name string) bool { return knownStatusTools[name] }

// studioIdentity is the shared 印记 posture prefixed to every status prompt.
const studioIdentity = `你是「印记」，陪学生把研究项目做完的 agent（像 Cowork 之于写代码）。你不替他定论、绝不代写正文。narrate 全程用中文（哪怕学生用英文说、成品用英文写），一次只问一个问题、不连问。你只输出一个 JSON：{"narrate":"给学生看的一段话","tools":[{"name":...,"args":{...}}]}，不要多余文字。别提议你没有的工具；阶段推进、生成计划由系统自动完成，不用你操心。`

// Tool-contract lines reused across status prompts (kept identical to the old
// mega-prompt's wording where they overlap, so behavior transfers).
const (
	toolProposeNote   = `- propose_note: {"section":分区,"value":内容} —— 从学生说过的话里提炼一条提案要点候选（学生确认后才落库）。分区用英文码之一，按内容严格归类：objective=研究问题本身/核心变量怎么测量；reason=为什么研究这个/动机/个人经历；activities=打算怎么做/步骤/方法/时间安排（「先读文献再做问卷最后写作、大概三周」是 activities，不是 objective）；resources=能用或需要的数据/文献/工具/渠道；counterpoints=可能的反例/混淆因素/张力。凡是你在 narrate 里说「我把这点记下了/记进提案了」，本轮就必须真的放出对应的 propose_note，别只说不做。`
	toolSummonCard    = `- summon_card: {"card_id":...,"reason":...,"nudge_text":...} —— 在对的时刻把一张思维工具卡塞回给学生自己填（你不替他填）。`
	toolOpenReading   = `- open_reading: {"reason":...} —— 学生要读某个来源/需要查资料时，打开阅读室。`
	toolFinishPart    = `- finish_part: {} —— 学生说这一部分写完了、想收尾时。`
	toolRequestReview = `- request_review: {} —— 学生写完、该做整稿体检时。`
	toolProposeQ      = `- propose_question: {"text":问题} —— 向学生提议一个值得追的研究问题（学生确认后才采纳；一次一个）。`
	toolCurateRef     = `- curate_reference: {"items":[{"kind":"material|note|annotation","id":...,"label":...}]} —— 把学生此刻要用的来源/片段/批注摆到左侧；id 必须用投影里出现过的真实 [id]，绝不编造。`
	toolUpdatePlan    = `- update_plan: {"op":"complete|start|reopen|add|edit|remove","id":计划项id,"match":标题关键词,"title":新标题,"stage":"阶段一 · …","tag":"read|write|review","days":天数} —— 帮学生管理项目计划：他说某件事做完了，就 op=complete 把对应任务标成完成（用投影「计划项」里的 [id]，认不准就用 match 传标题关键词）；op=start 标为进行中；要加/改/删任务用 add/edit/remove。这是确定性的系统动作、不是替他写正文，他在对话里让你改计划你就直接改；较大的重排先在 narrate 里跟他确认。`
	toolNoteNeed      = `- note_resource_need: {"text":关键词或要查的东西,"why":一句话原因} —— 写作/讨论中你想到一个值得去查、去探索的关键词或资料，就把它记进「还需要探索的」清单，学生之后能一键去检索。一次记一个，别一次塞一堆。`
)

func mkPrompt(goal string, tools ...string) string {
	s := studioIdentity + "\n\n本阶段目标：" + goal + "\n\n本阶段你可用的工具："
	for _, t := range tools {
		s += "\n" + t
	}
	return s
}

// StatusRegistry is the single source of truth: status → posture. Built fresh
// each call (cheap; keeps the strings immutable to callers).
func StatusRegistry() map[FlowStatus]StatusDef {
	return map[FlowStatus]StatusDef{
		FlowTopic: {
			Goal:         "学生还没定研究问题——陪他把一个模糊的兴趣收成一句清晰、可研究的问题。",
			SystemPrompt: mkPrompt("学生还没定研究问题——陪他把一个模糊的兴趣收成一句清晰、可研究的问题。别替他定题。", toolProposeQ),
			Tools:        []string{"propose_question"},
			Cards:        nil, // 提问卡 is now a chatbox button (student-opened modal), not an AI-summonable card
			Surface:      ToolChat,
			Doc:          DocNone,
		},
		FlowFramework: {
			Goal:         "把研究计划的五件事聊清楚：目标、缘由、活动与时间、资源、可能的反例/张力。四项核心齐了、也谈过反例后系统会自动生成计划。",
			SystemPrompt: mkPrompt("把研究计划的五件事聊清楚——目标、缘由、活动与时间、资源，以及可能的反例/张力（这条也要问到，别跳过——想想什么证据或情形可能挑战当前的想法）。针对学生刚说的那一维给一条具体反馈，再往还没谈到的一维带一步，一次只带一个。四项核心都有内容、且谈过反例之后，系统会自动生成计划；你不用提议生成，只需继续把内容聊扎实。计划一旦生成，你可以帮他管理（update_plan）、也可以把想查的关键词记进探索清单（note_resource_need）。", toolProposeNote, toolSummonCard, toolUpdatePlan, toolNoteNeed),
			Tools:        []string{"propose_note", "summon_card", "update_plan", "note_resource_need"},
			Cards:        nil, // 提问卡 is now a chatbox button (student-opened modal), not an AI-summonable card
			Surface:      ToolForming,
			Doc:          DocNone,
		},
		FlowProposal: {
			Goal:         "陪学生把研究提案写成一段紧凑的文字（研究问题 + 文献范围 + 执行计划）。他自己写，你只陪想、查论证、点反例。",
			SystemPrompt: mkPrompt("学生在写研究提案文档（研究问题+文献范围+执行计划，约一页）。他自己写正文，你绝不代写——只陪他想清楚、检查论证与结构、一次一问。他想读资料就 open_reading；写完想收尾就 finish_part；要整稿体检就 request_review。他说计划里某步做完了、或想调整计划，就用 update_plan；想到值得查的关键词就 note_resource_need。", toolOpenReading, toolCurateRef, toolRequestReview, toolFinishPart, toolSummonCard, toolUpdatePlan, toolNoteNeed),
			Tools:        []string{"open_reading", "curate_reference", "request_review", "finish_part", "summon_card", "update_plan", "note_resource_need"},
			Cards:        nil, // doc §4: 提案无卡组；仍可召唤横切按需卡
			Surface:      ToolWriting,
			Doc:          DocProposal,
		},
		FlowEssay: {
			Goal:         "陪学生写正文（大纲 → 片段 → 正文）。他自己写，你只陪想、查论证、撞反例、点结构。",
			SystemPrompt: mkPrompt("学生在写论文正文（大纲/片段/正文）。他自己写正文，你绝不代写——陪他把论证一根根立起来、撞反例、检查结构，一次一问。缺资料就 open_reading；写完想收尾就 finish_part；要整稿体检就 request_review。他说计划里某步做完了、或想调整计划，就用 update_plan；想到值得查的关键词就 note_resource_need。", toolOpenReading, toolCurateRef, toolRequestReview, toolFinishPart, toolSummonCard, toolUpdatePlan, toolNoteNeed),
			Tools:        []string{"open_reading", "curate_reference", "request_review", "finish_part", "summon_card", "update_plan", "note_resource_need"},
			Cards:        []string{"pee", "toulmin", "argument-map"},
			Surface:      ToolWriting,
			Doc:          DocEssay,
		},
		FlowReview: {
			Goal:         "陪学生写回顾——不是答辩，别追问、别考他，帮他把自己的思考和收获说清楚。绝不替他下结论。",
			SystemPrompt: mkPrompt("学生在写回顾/复盘（目标是否达成、方法与数据、过程中的问题、局限、收获，或与 AI 互动的使用声明）。这不是答辩——顺着他卡住的那部分一次问一个开放问题，帮他想起细节、找到自己的措辞。绝不替他下结论、绝不替他把话写出来。"),
			Tools:        []string{},
			Cards:        nil, // doc §7: 回顾无可召唤卡组（learning-report 是 function 产出器，非召唤）
			Surface:      ToolReflection,
			Doc:          DocNone,
		},
	}
}

// CrossCuttingCardIDs are the on-demand thinking cards the coach may summon
// REGARDLESS of status — they answer a contextual need (AI might be
// hallucinating, the student is one-sided / stuck / showing confirmation bias),
// not a phase. They sit in no status deck; IsSummonable adds them to every
// status's summonable set. Re-catalog 2026-08-09 (bucket 3).
var CrossCuttingCardIDs = []string{
	"ai-boundary", "knower-perspective", "metacognition", "emotional-alignment",
	"rabbit-hole", "ethics-lenses", "ai-decision-tree", "perspective-matrix", "concession",
}

// QuestionCardAISummonable reports whether the coach (AI) may PROPOSE the 提问卡
// (all-statuses.md §2): the card is a START-OF-FRAMEWORK aid, available ONLY
// while the 目标 is empty (the student hasn't yet figured out what to research).
// Once a research question is formed (目标 non-empty) the card is no longer
// available — not manually (the button hides) and not in the AI chat (this
// returns false). The deterministic empty→available / filled→gone split.
func QuestionCardAISummonable(objectiveEmpty bool) bool { return objectiveEmpty }

// IsSummonable reports whether the coach may summon cardID in the given status:
// the status's own deck ∪ the cross-cutting pool. Reading-deck / reading-toolkit
// cards are summoned inside the reading room, not via the writing-flow statuses,
// so they are NOT summonable here.
func IsSummonable(cardID string, status FlowStatus) bool {
	for _, id := range StatusRegistry()[status].Cards {
		if id == cardID {
			return true
		}
	}
	for _, id := range CrossCuttingCardIDs {
		if id == cardID {
			return true
		}
	}
	return false
}

// FlowStatus is the coarse-grained project position the router dispatches on.
type FlowStatus string

const (
	FlowTopic     FlowStatus = "topic"     // no research question yet
	FlowFramework FlowStatus = "framework" // the four 立项 points (+ plan auto-gen)
	FlowProposal  FlowStatus = "proposal"  // write the proposal document
	FlowEssay     FlowStatus = "essay"     // write the essay (outline/snippets/body)
	FlowReview    FlowStatus = "review"    // 复盘 / reflection
)

// StatusForStage collapses a persisted StudioStage onto its FlowStatus. Unknown
// or empty stages default to FlowFramework (the safe working default for a
// started project — never a dead end).
func StatusForStage(s StudioStage) FlowStatus {
	switch s {
	case StageTopicDiscussion:
		return FlowTopic
	case StageProposalForming, StagePlanGeneration:
		return FlowFramework
	case StageProposalWriting, StageProposalReview:
		return FlowProposal
	case StageBodyWriting:
		return FlowEssay
	case StageRetrospective:
		return FlowReview
	}
	return FlowFramework
}

// StageFloor is the canonical persisted stage a FlowStatus maps back to — the
// value the deterministic router writes into StudioState.Stage when it lands a
// project in that status. (The collapse is many→one; the floor is the earliest
// stage of each status so the router never jumps a project past sub-steps the
// model may still drive within a status.)
func (f FlowStatus) StageFloor() StudioStage {
	switch f {
	case FlowTopic:
		return StageTopicDiscussion
	case FlowFramework:
		return StageProposalForming
	case FlowProposal:
		return StageProposalWriting
	case FlowEssay:
		return StageBodyWriting
	case FlowReview:
		return StageRetrospective
	}
	return StageProposalForming
}
