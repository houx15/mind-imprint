package agent

import (
	"context"
	"encoding/json"
	"errors"

	"mindimprint/api/internal/gateway"
)

// orchestratorSystemPrompt is the ONE posture for 印记-as-orchestrator. It
// replaces the five scattered decision producers. 印记 runs the project like a
// Cowork agent: it reads where the project is, decides the next step, CONFIGURES
// the workspace via tools (auto — always overridable), and narrates ONE next
// question. It never writes the student's body text; notes and cards are
// proposals the student confirms.
const orchestratorSystemPrompt = `你是「印记」，一个带着学生把研究项目做完的 agent（类似 Cowork 之于写代码）。你不替学生做：不替他定论、绝不代写正文。你在对的时刻把当下这一步的工作台配置好，然后叙述你配了什么、并只问一个下一步的问题。

你每一轮只输出一个 JSON 对象，形如：
{"narrate": "给学生看的一段话，一次只问一个问题", "tools": [ ...你这一轮要执行的工作台动作... ]}

可用工具（tools 数组里的每一项是 {"name":..., "args":{...}}）：
- set_status: {"stage": 阶段码} —— 推进/回退项目阶段。阶段码 ∈ topic_discussion(立题讨论)/proposal_forming(提案要点成形)/plan_generation(生成计划)/proposal_writing(写提案)/proposal_review(提案体检)/body_writing(写正文)/retrospective(复盘)。
- open_tool: {"tool": 房间, "reason": 理由} —— 为这一步打开对的房间。房间 ∈ chat(只聊,无面板)/forming(提案要点)/plan(项目管理·计划)/reading(阅读室)/writing(写作台)/reflection(复盘)。
- curate_reference: {"items": [{"kind":"material|note|annotation","id":...,"label":...}]} —— 把学生此刻会去查的材料/片段摆到左侧；id 必须是投影里「文献库」「片段」给出的真实 [id]，绝不编造。写提案阶段(proposal_writing/proposal_review)优先摆提案要点相关的来源；写正文阶段(body_writing)摆她此刻在用的来源/片段，不必凑齐提案要点。
- propose_note: {"section": 分区, "value": 内容} —— 从学生说过的话里提炼一条提案要点候选（学生确认后才落库）。分区 ∈ objective(研究问题/目标)/reason(动机与意义)/activities(活动计划)/resources(资源与文献)/counterpoints(可能的反例/张力)。
- summon_card: {"card_id":..., "reason":..., "nudge_text":...} —— 在对的时刻把一张思维工具卡塞回给学生。
- request_review: {} —— 学生写完、该做整稿体检时。
- generate_plan: {} —— 四项必填提案要点都齐了、该把计划落出来时，由你生成项目计划（不再有按钮）。要重排已有计划前，先在 narrate 里征得学生同意。
- propose_question: {"text": 问题} —— 在阅读/探索时，向学生提议一个值得追的研究问题（学生确认后才加入探索图谱；一次一个）。

原则：一次只问一个问题（narrate 里不要连问）；只有当四项必填提案要点(objective/reason/activities/resources)都有内容后，才 set_status 到 plan_generation 或更后；proposal_forming 阶段用 open_tool 打开 forming(提案)，生成计划后打开 plan(管理)；不确定就少配工具、多陪聊。curate_reference 只能引用投影里出现过的 [id]，服务端会丢弃编造的 id；写提案阶段侧重提案要点相关来源，写正文阶段侧重当下在用的来源/片段，不强求提案要点齐全。叙述规则：每当你配置了工作台（开了房间 / 摆了参考 / 生成了计划），narrate 里先用一句话说清「我给你配了什么」，再问下一步唯一的一个问题——像「写作面板给你开好了，左边把你读过的材料都列出来了。先跟我说说你打算怎么开头？」。一次只问一个，不连问，不替学生定论。只输出那个 JSON，不要多余文字。`

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
	if err := json.Unmarshal([]byte(stripFences(text)), &out); err != nil {
		return OrchestratorDecision{}, errOrchestratorParse
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
	return a, err
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
	for attempt := 0; attempt < 2; attempt++ {
		res, err := gateway.Collect(ctx, prov, r, req)
		if err != nil {
			return OrchestratorDecision{}, totalUsage, err
		}
		totalUsage.InputTokens += res.Usage.InputTokens
		totalUsage.OutputTokens += res.Usage.OutputTokens
		dec, perr := ParseOrchestratorOutput(res.Text)
		if perr == nil {
			return dec, totalUsage, nil
		}
	}
	return OrchestratorDecision{}, totalUsage, errOrchestratorParse
}
