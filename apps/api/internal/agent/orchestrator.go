package agent

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"mindimprint/api/internal/gateway"
)

// narrateIDLeak matches a workspace id (UUID) that the model sometimes copies
// from the projection ("…你的暂定论点片段[<uuid>]…") straight into user-facing
// narrate. Ids belong in tool-call args, never in prose shown to the student.
// Any surrounding brackets and one leading space are eaten so "片段[<uuid>]，"
// collapses cleanly to "片段，".
var narrateIDLeak = regexp.MustCompile(`[ \t]*\[?[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\]?[ \t]*`)

// SanitizeNarrate strips leaked workspace ids from a narrate string before it
// reaches the student (defence-in-depth alongside the prompt rule that forbids
// ids in narrate). Idempotent; leaves id-free narrate untouched.
func SanitizeNarrate(s string) string {
	return narrateIDLeak.ReplaceAllString(s, "")
}

// orchestratorSystemPrompt is the ONE posture for 印记-as-orchestrator. It
// replaces the five scattered decision producers. 印记 runs the project like a
// Cowork agent: it reads where the project is, decides the next step, CONFIGURES
// the workspace via tools (auto — always overridable), and narrates ONE next
// question. It never writes the student's body text; notes and cards are
// proposals the student confirms.
const orchestratorSystemPrompt = `你是「印记」，一个带着学生把研究项目做完的 agent（类似 Cowork 之于写代码）。你不替学生做：不替他定论、绝不代写正文。你在对的时刻把当下这一步的工作台配置好，然后叙述你配了什么、并只问一个下一步的问题。

narrate 全程用中文写（哪怕学生用英文跟你说、哪怕他的成品要用英文写——「印记」始终用中文陪他想）。提炼进 propose_note 的 value 用学生原话的语言。

你每一轮只输出一个 JSON 对象，形如：
{"narrate": "给学生看的一段话，一次只问一个问题", "tools": [ ...你这一轮要执行的工作台动作... ]}

可用工具（tools 数组里的每一项是 {"name":..., "args":{...}}）：
- set_status: {"stage": 阶段码} —— 推进/回退项目阶段。阶段码 ∈ topic_discussion(立题讨论)/proposal_forming(提案要点成形)/plan_generation(生成计划)/proposal_writing(写提案)/proposal_review(提案体检)/body_writing(写正文)/retrospective(复盘)。
- open_tool: {"tool": 房间, "reason": 理由} —— 为这一步打开对的房间。房间 ∈ chat(只聊,无面板)/forming(提案要点)/plan(项目管理·计划)/reading(阅读室)/writing(写作台)/reflection(复盘)。
- curate_reference: {"items": [{"kind":"material|note|annotation","id":...,"label":...}]} —— 把学生此刻会去查的材料/片段/批注摆到左侧；id 必须是投影里「文献库」「片段」「批注」给出的真实 [id]，绝不编造。写提案阶段(proposal_writing/proposal_review)优先摆提案要点相关的来源；写正文阶段(body_writing)摆她此刻在用的来源/片段，体检后可摆相关批注(kind="annotation")，不必凑齐提案要点。
- propose_note: {"section": 分区, "value": 内容} —— 从学生说过的话里提炼一条提案要点候选（学生确认后才落库）。分区必须用英文码之一，按内容严格归类：objective=想回答的研究问题本身/核心变量怎么测量；reason=为什么研究这个/动机/个人经历；activities=打算怎么做/步骤/方法/时间安排（学生说「先读文献再做问卷最后写作、大概三周」这类就是 activities，不是 objective）；resources=能用或需要的数据/文献/工具/渠道；counterpoints=可能的反例/混淆因素/张力。凡是你在 narrate 里说「我把这点记成了一条候选」之类的话，本轮就必须真的放出对应的 propose_note，别只说不做。
- summon_card: {"card_id":..., "reason":..., "nudge_text":...} —— 在对的时刻把一张思维工具卡塞回给学生。
- request_review: {} —— 学生写完、该做整稿体检时。
- generate_plan: {} —— 四项必填提案要点都齐了、该把计划落出来时，由你生成项目计划（不再有按钮）。计划一旦生成（投影里会显示「计划：已生成 N 项」），就绝不再提议或重复生成，改为带学生走计划里的下一步。要重排已有计划前，先在 narrate 里征得学生同意。
- propose_question: {"text": 问题} —— 在阅读/探索时，向学生提议一个值得追的研究问题（学生确认后才加入探索图谱；一次一个）。

原则：一次只问一个问题（narrate 里不要连问）；只有当四项必填提案要点(objective/reason/activities/resources)都有内容后，才 set_status 到 plan_generation 或更后；proposal_forming 阶段用 open_tool 打开 forming(提案)，生成计划后打开 plan(管理)；不确定就少配工具、多陪聊。curate_reference 只能引用投影里出现过的 [id]，服务端会丢弃编造的 id；写提案阶段侧重提案要点相关来源，写正文阶段侧重当下在用的来源/片段，不强求提案要点齐全。【铁则】投影里的 [id]/UUID 只能进工具参数，narrate（给学生看的话）里绝不出现任何 id 或方括号编号——指代片段/材料就用自然语言（如「你左边那条暂定论点」），不要写 [xxxxxxxx-…]。叙述规则：每当你配置了工作台（开了房间 / 摆了参考 / 生成了计划），narrate 里先用一句话说清「我给你配了什么」，再问下一步唯一的一个问题——像「写作面板给你开好了，左边把你读过的材料都列出来了。先跟我说说你打算怎么开头？」。一次只问一个，不连问，不替学生定论。只输出那个 JSON，不要多余文字。`

// OrchestratorToolCall is one raw tool call the model emitted; Args stays raw
// until a typed accessor validates it.
type OrchestratorToolCall struct {
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

// OrchestratorDecision is the parsed, validated turn.
type OrchestratorDecision struct {
	Narrate string
	Tools   []OrchestratorToolCall
}

type rawOrchestratorOutput struct {
	Narrate string                 `json:"narrate"`
	Tools   []OrchestratorToolCall `json:"tools"`
}

var errOrchestratorParse = errors.New("orchestrator: output not parseable")

// knownOrchestratorTools is the closed set; anything else is dropped.
var knownOrchestratorTools = map[string]bool{
	"set_status": true, "open_tool": true, "curate_reference": true,
	"propose_note": true, "summon_card": true, "request_review": true,
	"generate_plan": true, "propose_question": true,
}

// ParseOrchestratorOutput parses the model output, dropping unknown tools and
// tools whose args fail validation. Returns an error ONLY when the output will
// not unmarshal at all (caller retries once, then falls back).
func ParseOrchestratorOutput(text string) (OrchestratorDecision, error) {
	var out rawOrchestratorOutput
	cleaned := stripFences(text)
	if err := json.Unmarshal([]byte(cleaned), &out); err != nil {
		// The model wrapped the envelope in prose (a lead-in like "好的，我来配一下：",
		// a trailing note, or a reasoning preamble). Rather than discard an
		// otherwise-good turn — and dump the generic fallback line that ignores the
		// student and poisons the next turn's history — salvage the first balanced
		// {...} object and parse THAT. Only when no balanced object survives do we
		// report a parse failure.
		obj := extractJSONObject(cleaned)
		if obj == "" {
			return OrchestratorDecision{}, errOrchestratorParse
		}
		if err2 := json.Unmarshal([]byte(obj), &out); err2 != nil {
			return OrchestratorDecision{}, errOrchestratorParse
		}
	}
	kept := make([]OrchestratorToolCall, 0, len(out.Tools))
	for _, tc := range out.Tools {
		if !knownOrchestratorTools[tc.Name] {
			continue
		}
		// curate_reference gets item-level filtering (drop only the bad items,
		// not the whole tool) rather than the all-or-nothing validToolArgs check
		// the other tools use — see filterCurateReferenceCall.
		if tc.Name == "curate_reference" {
			if filtered, ok := filterCurateReferenceCall(tc); ok {
				kept = append(kept, filtered)
			}
			continue
		}
		if !validToolArgs(tc) {
			continue
		}
		kept = append(kept, tc)
	}
	return OrchestratorDecision{Narrate: out.Narrate, Tools: kept}, nil
}

// extractJSONObject returns the first top-level balanced {...} substring in s,
// or "" if none is found. It tracks string literals and escapes so a brace
// INSIDE a JSON string value (e.g. a narrate mentioning "{" ) never miscounts
// the depth. Used by ParseOrchestratorOutput to recover the envelope when the
// model surrounded it with prose.
func extractJSONObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func validToolArgs(tc OrchestratorToolCall) bool {
	switch tc.Name {
	case "set_status":
		a, err := SetStatusArgs(tc)
		return err == nil && a.Stage.IsValid()
	case "open_tool":
		a, err := OpenToolArgs(tc)
		return err == nil && a.Tool.IsValid()
	case "propose_note":
		a, err := ProposeNoteArgs(tc)
		return err == nil && validSection(a.Section)
	case "summon_card":
		a, err := SummonCardToolArgs(tc)
		return err == nil && a.CardID != ""
	case "request_review":
		return true
	case "generate_plan":
		return true
	case "propose_question":
		a, err := ProposeQuestionArgs(tc)
		return err == nil && a.Text != ""
	}
	return false
}

func validSection(s string) bool {
	switch s {
	case "objective", "reason", "activities", "resources", "counterpoints":
		return true
	}
	return false
}

// validReferenceKind reports whether kind is one of the closed set the client
// contract's strict `z.enum` accepts (`packages/contracts/src/orchestrator.ts`).
// An out-of-enum kind that slips past the server would make the client's
// OrchestratorReply/StudioState parse throw downstream, silently swallowing
// the narration and leaving the student stuck on a sticky chat-first resume —
// so it must never reach `studio_state`.
func validReferenceKind(kind string) bool {
	switch kind {
	case "material", "note", "annotation":
		return true
	}
	return false
}

// filterCurateReferenceCall unmarshals a curate_reference call's args and
// drops any item whose kind is outside the closed set (validReferenceKind),
// keeping the rest. Returns ok=false when the args don't unmarshal at all, OR
// when filtering leaves zero valid items — in both cases the whole tool call
// is dropped (a no-op turn) rather than persisting a bad/empty reference set.
func filterCurateReferenceCall(tc OrchestratorToolCall) (OrchestratorToolCall, bool) {
	a, err := CurateReferenceArgs(tc)
	if err != nil {
		return OrchestratorToolCall{}, false
	}
	valid := make([]ReferenceRef, 0, len(a.Items))
	for _, item := range a.Items {
		if validReferenceKind(item.Kind) {
			valid = append(valid, item)
		}
	}
	if len(valid) == 0 {
		return OrchestratorToolCall{}, false
	}
	args, merr := json.Marshal(CurateReferenceArgsT{Items: valid})
	if merr != nil {
		return OrchestratorToolCall{}, false
	}
	return OrchestratorToolCall{Name: tc.Name, Args: args}, true
}

// --- typed arg accessors ---

type SetStatusArgsT struct {
	Stage StudioStage `json:"stage"`
}
type OpenToolArgsT struct {
	Tool   OpenTool `json:"tool"`
	Reason string   `json:"reason"`
}
type CurateReferenceArgsT struct {
	Items []ReferenceRef `json:"items"`
}
type ProposeNoteArgsT struct {
	Section string `json:"section"`
	Value   string `json:"value"`
}
type SummonCardToolArgsT struct {
	CardID    string `json:"card_id"`
	Reason    string `json:"reason"`
	NudgeText string `json:"nudge_text"`
}
type ProposeQuestionArgsT struct {
	Text string `json:"text"`
}

func SetStatusArgs(tc OrchestratorToolCall) (SetStatusArgsT, error) {
	var a SetStatusArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func OpenToolArgs(tc OrchestratorToolCall) (OpenToolArgsT, error) {
	var a OpenToolArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func CurateReferenceArgs(tc OrchestratorToolCall) (CurateReferenceArgsT, error) {
	var a CurateReferenceArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func ProposeNoteArgs(tc OrchestratorToolCall) (ProposeNoteArgsT, error) {
	var a ProposeNoteArgsT
	err := json.Unmarshal(tc.Args, &a)
	a.Section = normalizeSection(a.Section)
	return a, err
}

// normalizeSection maps the section labels the model sometimes emits (Chinese
// names, English synonyms) onto the five canonical codes. Without it a
// propose_note whose section is "缘由"/"motivation" fails validSection and is
// silently dropped — AFTER the narrate already told the student "我把这点记成了
// 一条候选", leaving a promised note that never appears and a dim that never fills.
func normalizeSection(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "objective", "目标", "研究问题", "research question", "question", "aim", "goal":
		return "objective"
	case "reason", "缘由", "动机", "motivation", "why":
		return "reason"
	case "activities", "活动", "活动与时间", "plan", "计划", "方法", "method", "timeline", "schedule":
		return "activities"
	case "resources", "资源", "文献", "materials", "sources", "data":
		return "resources"
	case "counterpoints", "反例", "张力", "反例/张力", "counterpoint", "counterargument", "tension":
		return "counterpoints"
	}
	return s // unknown → unchanged; validSection rejects it as before
}
func SummonCardToolArgs(tc OrchestratorToolCall) (SummonCardToolArgsT, error) {
	var a SummonCardToolArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}
func ProposeQuestionArgs(tc OrchestratorToolCall) (ProposeQuestionArgsT, error) {
	var a ProposeQuestionArgsT
	err := json.Unmarshal(tc.Args, &a)
	return a, err
}

// buildOrchestratorRequest assembles the ONE LLM call's request: the
// orchestrator posture as the system message, the continuous history mapped
// straight through, and a final user turn carrying the spine projection plus
// the current AI-managed state (stage/openTool) — the same shape as
// BuildProjectCoachContext, but tool-aware.
func buildOrchestratorRequest(spineProjection string, state StudioState, history []ChatTurn) gateway.ChatRequest {
	messages := make([]gateway.ChatMessage, 0, len(history)+2)
	messages = append(messages, gateway.ChatMessage{Role: gateway.RoleSystem, Content: orchestratorSystemPrompt})
	for _, t := range history {
		role := gateway.RoleUser
		if t.Role == "assistant" {
			role = gateway.RoleAssistant
		}
		messages = append(messages, gateway.ChatMessage{Role: role, Content: t.Content})
	}
	messages = append(messages, gateway.ChatMessage{
		Role:    gateway.RoleUser,
		Content: spineProjection + "\n\n当前阶段：" + string(state.Stage) + "，当前打开：" + string(state.OpenTool),
	})
	return gateway.ChatRequest{Messages: messages}
}

// BuildStatusRequest assembles the per-status LLM request: the status's SHORT
// system prompt (not the mega-prompt), the continuous history, and a final user
// turn carrying the spine projection + current stage/open tool. Mirrors
// buildOrchestratorRequest but swaps the posture for def.SystemPrompt.
func BuildStatusRequest(def StatusDef, spineProjection string, state StudioState, history []ChatTurn) gateway.ChatRequest {
	messages := make([]gateway.ChatMessage, 0, len(history)+2)
	messages = append(messages, gateway.ChatMessage{Role: gateway.RoleSystem, Content: def.SystemPrompt})
	for _, t := range history {
		role := gateway.RoleUser
		if t.Role == "assistant" {
			role = gateway.RoleAssistant
		}
		messages = append(messages, gateway.ChatMessage{Role: role, Content: t.Content})
	}
	messages = append(messages, gateway.ChatMessage{
		Role:    gateway.RoleUser,
		Content: spineProjection + "\n\n当前阶段：" + string(state.Stage) + "，当前打开：" + string(state.OpenTool),
	})
	return gateway.ChatRequest{Messages: messages}
}

// FilterToolsForStatus drops any tool the model emitted that this status does
// not permit — the enforcement half of progressive disclosure. Even if the fast
// model hallucinates a tool outside its subset (e.g. generate_plan while
// writing), it never takes effect.
func FilterToolsForStatus(dec OrchestratorDecision, def StatusDef) OrchestratorDecision {
	allowed := make(map[string]bool, len(def.Tools))
	for _, t := range def.Tools {
		allowed[t] = true
	}
	kept := make([]OrchestratorToolCall, 0, len(dec.Tools))
	for _, tc := range dec.Tools {
		if allowed[tc.Name] {
			kept = append(kept, tc)
		}
	}
	return OrchestratorDecision{Narrate: dec.Narrate, Tools: kept}
}

// ProposeOrchestratorTurn makes up to two LLM calls for a student turn (a
// retry on a parse failure) and returns the parsed decision. The returned
// usage is the SUM across every attempt actually made — attempt 0's tokens
// are real spend even when its output failed to parse and attempt 1 had to
// run, so they must still be metered, not silently dropped in favor of only
// the last attempt's usage. The caller falls back to a plain narration when
// both attempts fail to parse.
func ProposeOrchestratorTurn(
	ctx context.Context,
	prov gateway.Provider,
	r gateway.Resolved,
	spineProjection string,
	state StudioState,
	history []ChatTurn,
) (OrchestratorDecision, gateway.ChatUsage, error) {
	req := buildOrchestratorRequest(spineProjection, state, history)
	var totalUsage gateway.ChatUsage
	var lastText string
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, prov, r, req)
		if err != nil {
			return OrchestratorDecision{}, totalUsage, err
		}
		totalUsage.InputTokens += res.Usage.InputTokens
		totalUsage.OutputTokens += res.Usage.OutputTokens
		lastText = res.Text
		dec, perr := ParseOrchestratorOutput(res.Text)
		if perr == nil {
			return dec, totalUsage, nil
		}
	}
	// Neither attempt yielded a parseable envelope. If the model instead answered
	// in PLAIN PROSE (a real coaching sentence, no JSON at all — which reasoning
	// models drift into once the history window is rich, ~12 turns in), show that
	// prose as the narration rather than throwing the student's turn away for the
	// generic canned fallback (which reads as a non-sequitur and makes the NEXT
	// turn confabulate an apology). A reply that carries braces is broken JSON,
	// not prose — leave that to the caller's fallback so we never surface a raw
	// or half-formed envelope.
	if prose := strings.TrimSpace(stripFences(lastText)); prose != "" && !strings.ContainsAny(prose, "{}") {
		return OrchestratorDecision{Narrate: prose}, totalUsage, nil
	}
	return OrchestratorDecision{}, totalUsage, errOrchestratorParse
}

// noteRecoveryClaimMarkers are the Chinese phrases 印记's narrate uses when it
// TELLS the student it recorded a proposal note. narrate is pinned to Chinese,
// so matching these is reliable. When one appears but the turn emitted NO
// propose_note (a reasoning-model contract violation: it writes "I recorded it"
// yet omits the tool), the server recovers the note via ExtractProposalNote so
// the panel never contradicts what 印记 just said.
var noteRecoveryClaimMarkers = []string{
	"记进", "记下", "记成", "记到", "记入", "记了一条", "帮你记", "记一条", "写进提案", "记进提案", "记进要点", "加进提案",
}

// ClaimsNoteRecording reports whether narrate tells the student a proposal note
// was recorded — the trigger for the server-side note backstop.
func ClaimsNoteRecording(narrate string) bool {
	for _, m := range noteRecoveryClaimMarkers {
		if strings.Contains(narrate, m) {
			return true
		}
	}
	return false
}

const noteExtractPrompt = `印记（一个陪学生做研究的 agent）刚才对学生说，要把学生说的话记成一条「提案要点」候选，但没有真正给出结构化的记录。请你根据【学生的话】和【印记的话】，判断这条要点属于哪一维，并用学生原话的语言提炼一句简洁的候选内容。
维度码（section）只能取其一：objective（研究问题本身/核心变量怎么测量）、reason（为什么研究这个/动机/个人经历）、activities（打算怎么做/步骤/方法/时间安排）、resources（能用或需要的数据/文献/工具/渠道）、counterpoints（可能的反例/混淆因素/张力/会挑战我立场的情形）。

严格判断规则：
1) 若学生明确在说一个「反例 / 会削弱或推翻我立场的情形 / 张力 / 反方观点」，即使他顺带提到了某些文献、法条或来源，也归 counterpoints——不要因为出现了文献/来源字样就归 resources。counterpoints 的核心是「可能反驳我论点的东西」，resources 的核心是「我手上有或还需要的材料」。
2) 若学生这句话只是一句流程 / 操作指令，没有研究实质内容（例如「帮我生成计划」「可以进入下一步了吗」「我准备好了」「继续」「换个话题」「这样就行」之类），不要提炼任何要点——把 section 设为空字符串。

只输出一个 JSON：{"section":"<维度码或空字符串>","value":"<候选内容>"}。判断不出维度、或这是一句流程指令，就把 section 设为空字符串。value 用学生原话的语言，别替他扩写或下结论。不要输出别的文字。`

// ExtractProposalNote is the server-side backstop for the note contract: given
// the student's message and 印记's narration (which claimed a recording), it
// makes ONE focused LLM call to recover the {section, value} the missing
// propose_note should have carried. Returns ok=false (with usage still metered
// by the caller) when the model can't classify a section or yields no value —
// the panel then simply stays as-is rather than filling a wrong dim. Section is
// normalized + validated exactly as a real propose_note would be.
func ExtractProposalNote(ctx context.Context, prov gateway.Provider, r gateway.Resolved, studentMsg, narrate string) (ProposeNoteArgsT, gateway.ChatUsage, bool) {
	req := gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: noteExtractPrompt},
		{Role: gateway.RoleUser, Content: "【学生的话】\n" + studentMsg + "\n\n【印记的话】\n" + narrate},
	}}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return ProposeNoteArgsT{}, res.Usage, false
	}
	var parsed ProposeNoteArgsT
	cleaned := stripFences(res.Text)
	if uerr := json.Unmarshal([]byte(cleaned), &parsed); uerr != nil {
		obj := extractJSONObject(cleaned)
		if obj == "" || json.Unmarshal([]byte(obj), &parsed) != nil {
			return ProposeNoteArgsT{}, res.Usage, false
		}
	}
	parsed.Section = normalizeSection(parsed.Section)
	if !validSection(parsed.Section) || strings.TrimSpace(parsed.Value) == "" {
		return ProposeNoteArgsT{}, res.Usage, false
	}
	return parsed, res.Usage, true
}

// ProposeStatusTurn is the status-router turn: same 2-attempt loop + prose
// salvage as ProposeOrchestratorTurn, but built from a status's SHORT prompt
// (BuildStatusRequest) and with the result filtered to that status's tool
// subset (FilterToolsForStatus). This is what runs on the fast model — the
// per-turn decision is small enough that a non-reasoning model handles it.
func ProposeStatusTurn(
	ctx context.Context,
	prov gateway.Provider,
	r gateway.Resolved,
	def StatusDef,
	spineProjection string,
	state StudioState,
	history []ChatTurn,
) (OrchestratorDecision, gateway.ChatUsage, error) {
	req := BuildStatusRequest(def, spineProjection, state, history)
	var totalUsage gateway.ChatUsage
	var lastText string
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, prov, r, req)
		if err != nil {
			return OrchestratorDecision{}, totalUsage, err
		}
		totalUsage.InputTokens += res.Usage.InputTokens
		totalUsage.OutputTokens += res.Usage.OutputTokens
		lastText = res.Text
		if dec, perr := ParseOrchestratorOutput(res.Text); perr == nil {
			return FilterToolsForStatus(dec, def), totalUsage, nil
		}
	}
	// Prose-only reply (no JSON) → show it as the narration (same salvage as the
	// orchestrator path). A braced-but-broken reply is left to the caller's fallback.
	if prose := strings.TrimSpace(stripFences(lastText)); prose != "" && !strings.ContainsAny(prose, "{}") {
		return OrchestratorDecision{Narrate: prose}, totalUsage, nil
	}
	return OrchestratorDecision{}, totalUsage, errOrchestratorParse
}

// orchestratorOpeningPrompt is the ONE crafted posture for 印记's real-AI
// welcome — the student's very first turn in a project, before she has said
// anything. Unlike orchestratorSystemPrompt this call is tool-less: it only
// ever produces the framing message itself (no JSON envelope), one message,
// not a barrage (design 铁律 ③ 一次只问一个).
const orchestratorOpeningPrompt = `你是「印记」，学生刚进入这个写作项目，还没开始。用一段话欢迎他，语气温暖、克制、不啰嗦。你必须：
1) 欢迎他来到写作空间；
2) 复述你看到的题目（用投影里的项目题目，别编造；若没有题目就说「你还没定题目」）；
3) 用一句话点明：完整做完一个写作项目，会一路经过 立项 → 阅读 → 写作 → 回顾；
4) 说明我们先一起把研究计划的四件事讨论清楚，并列成一个短清单：
   - 目标（research question）
   - 缘由（motivation）
   - 活动与时间（plan）
   - 资源（resources）
5) 最后问一句：准备好开始了吗？
全程用中文和学生说话（他的写作语言可能是英文，但「印记」始终用中文陪他想）。只输出给学生看的这段话本身，不要 JSON、不要工具、不要列出多于四条、不要连问多个问题。`

// ProposeOpeningTurn makes ONE LLM call producing 印记's welcome message. It
// has no tools and no history — just the opening posture + the spine
// projection (which carries the project title). Returns the narrate text and
// usage.
func ProposeOpeningTurn(ctx context.Context, prov gateway.Provider, r gateway.Resolved, spineProjection string) (string, gateway.ChatUsage, error) {
	req := gateway.ChatRequest{Messages: []gateway.ChatMessage{
		{Role: gateway.RoleSystem, Content: orchestratorOpeningPrompt},
		{Role: gateway.RoleUser, Content: spineProjection + "\n\n（这是开场，学生还没说话。）"},
	}}
	res, err := gateway.Collect(ctx, prov, r, req)
	if err != nil {
		return "", res.Usage, err
	}
	return strings.TrimSpace(res.Text), res.Usage, nil
}
