package agent

import (
	"encoding/json"
	"errors"
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
- open_tool: {"tool": 房间, "reason": 理由} —— 为这一步打开对的房间。房间 ∈ chat(只聊,无面板)/plan(立项与提案要点)/reading(阅读室)/writing(写作台)/reflection(复盘)。
- curate_reference: {"items": [{"kind":"material|note|annotation","id":...,"label":...}]} —— 把学生此刻会去查的材料摆到左侧。
- propose_note: {"section": 分区, "value": 内容} —— 从学生说过的话里提炼一条提案要点候选（学生确认后才落库）。分区 ∈ objective(研究问题/目标)/reason(动机与意义)/activities(活动计划)/resources(资源与文献)/counterpoints(可能的反例/张力)。
- summon_card: {"card_id":..., "reason":..., "nudge_text":...} —— 在对的时刻把一张思维工具卡塞回给学生。
- request_review: {} —— 学生写完、该做整稿体检时。

原则：一次只问一个问题（narrate 里不要连问）；只有当四项必填提案要点(objective/reason/activities/resources)都有内容后，才 set_status 到 plan_generation 或更后；不确定就少配工具、多陪聊。只输出那个 JSON，不要多余文字。`

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
		if !knownOrchestratorTools[tc.Name] || !validToolArgs(tc) {
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
	case "curate_reference":
		_, err := CurateReferenceArgs(tc)
		return err == nil
	case "propose_note":
		a, err := ProposeNoteArgs(tc)
		return err == nil && validSection(a.Section)
	case "summon_card":
		a, err := SummonCardToolArgs(tc)
		return err == nil && a.CardID != ""
	case "request_review":
		return true
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
