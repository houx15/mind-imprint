package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseOrchestratorOutput_FencedMultiTool(t *testing.T) {
	raw := "```json\n{\"narrate\":\"写作面板开好了。\",\"tools\":[" +
		"{\"name\":\"set_status\",\"args\":{\"stage\":\"body_writing\"}}," +
		"{\"name\":\"open_tool\",\"args\":{\"tool\":\"writing\",\"reason\":\"该写正文了\"}}," +
		"{\"name\":\"propose_note\",\"args\":{\"section\":\"objective\",\"value\":\"净影响\"}}" +
		"]}\n```"
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Narrate != "写作面板开好了。" {
		t.Fatalf("narrate=%q", dec.Narrate)
	}
	if len(dec.Tools) != 3 {
		t.Fatalf("want 3 tools, got %d", len(dec.Tools))
	}
}

func TestParseOrchestratorOutput_DropsUnknownAndInvalid(t *testing.T) {
	raw := `{"narrate":"ok","tools":[` +
		`{"name":"teleport","args":{}},` + // unknown → dropped
		`{"name":"set_status","args":{"stage":"not_a_stage"}},` + // invalid arg → dropped
		`{"name":"set_status","args":{"stage":"proposal_forming"}}` + // valid → kept
		`]}`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Tools) != 1 || dec.Tools[0].Name != "set_status" {
		t.Fatalf("expected only the valid set_status, got %+v", dec.Tools)
	}
}

// Whole-branch review Fix 2: an out-of-enum `kind` on a curate_reference item
// used to pass server-side (only `err == nil` on unmarshal was checked, and
// ReferenceRef.Kind is a plain Go string) and get persisted into studio_state
// — then the client's strict `z.enum(["material","note","annotation"])`
// (packages/contracts/src/orchestrator.ts) throws on OrchestratorReply.parse,
// silently swallowing the narration. A mix of one valid + one invalid item
// keeps only the valid one (item-level drop, not the whole tool).
func TestParseOrchestratorOutput_CurateReferenceDropsInvalidKindKeepsValid(t *testing.T) {
	raw := `{"narrate":"材料摆好了。","tools":[` +
		`{"name":"curate_reference","args":{"items":[` +
		`{"kind":"material","id":"m1","label":"NASA 数据"},` +
		`{"kind":"bogus","id":"b1","label":"越权项"}` +
		`]}}]}`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Tools) != 1 || dec.Tools[0].Name != "curate_reference" {
		t.Fatalf("expected the curate_reference tool to be kept, got %+v", dec.Tools)
	}
	args, aerr := CurateReferenceArgs(dec.Tools[0])
	if aerr != nil {
		t.Fatal(aerr)
	}
	if len(args.Items) != 1 || args.Items[0].Kind != "material" || args.Items[0].ID != "m1" {
		t.Fatalf("expected only the valid material item to survive, got %+v", args.Items)
	}
}

// When EVERY item's kind is out-of-enum, the whole tool call is dropped (a
// no-op turn) rather than persisting an empty/bad reference set.
func TestParseOrchestratorOutput_CurateReferenceAllInvalidDropsTool(t *testing.T) {
	raw := `{"narrate":"ok","tools":[` +
		`{"name":"curate_reference","args":{"items":[{"kind":"bogus","id":"b1","label":"x"}]}}` +
		`]}`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Tools) != 0 {
		t.Fatalf("expected the curate_reference tool to be dropped entirely, got %+v", dec.Tools)
	}
}

func TestParseOrchestratorOutput_ProseWrappedEnvelope(t *testing.T) {
	// A reasoning model sometimes brackets the envelope with prose. The parser
	// must salvage the first balanced {...} object rather than fail the turn.
	raw := "好的，我来给你配一下工作台：\n" +
		`{"narrate":"写作面板开好了。","tools":[{"name":"open_tool","args":{"tool":"writing","reason":"该写正文了"}}]}` +
		"\n希望这样清楚一些。"
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Narrate != "写作面板开好了。" {
		t.Fatalf("narrate=%q", dec.Narrate)
	}
	if len(dec.Tools) != 1 || dec.Tools[0].Name != "open_tool" {
		t.Fatalf("expected the salvaged open_tool, got %+v", dec.Tools)
	}
}

func TestParseOrchestratorOutput_NarrateWithBraceInString(t *testing.T) {
	// A brace inside a JSON string value must not confuse the balanced-object
	// scan when it has to extract from prose-wrapped output.
	raw := `前言。{"narrate":"用集合 {A} 打个比方","tools":[]}后记。`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if dec.Narrate != "用集合 {A} 打个比方" {
		t.Fatalf("narrate=%q", dec.Narrate)
	}
}

func TestProposeNoteArgs_NormalizesSection(t *testing.T) {
	// A propose_note whose section is a Chinese label / synonym must be recovered
	// to the canonical code, not dropped (bug A2: the narrate promised a note that
	// then vanished because validSection rejected "缘由").
	cases := map[string]string{
		"缘由": "reason", "motivation": "reason",
		"目标": "objective", "research question": "objective",
		"活动与时间": "activities", "plan": "activities",
		"资源": "resources", "反例": "counterpoints",
		"reason": "reason", // canonical passes through
	}
	for in, want := range cases {
		raw := `{"section":"` + in + `","value":"x"}`
		a, err := ProposeNoteArgs(OrchestratorToolCall{Name: "propose_note", Args: json.RawMessage(raw)})
		if err != nil {
			t.Fatalf("section %q: %v", in, err)
		}
		if a.Section != want {
			t.Fatalf("section %q → %q, want %q", in, a.Section, want)
		}
		if !validSection(a.Section) {
			t.Fatalf("normalized section %q for input %q is not valid", a.Section, in)
		}
	}
}

func TestParseOrchestratorOutput_NormalizesNoteSectionKeepsTool(t *testing.T) {
	// End-to-end: a propose_note with a Chinese section survives parsing (was
	// dropped before the normalizer).
	raw := `{"narrate":"记一条缘由候选","tools":[{"name":"propose_note","args":{"section":"缘由","value":"个人经历"}}]}`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Tools) != 1 {
		t.Fatalf("expected the propose_note to survive, got %+v", dec.Tools)
	}
	a, _ := ProposeNoteArgs(dec.Tools[0])
	if a.Section != "reason" {
		t.Fatalf("section = %q, want reason", a.Section)
	}
}

func TestParseOrchestratorOutput_MalformedIsError(t *testing.T) {
	if _, err := ParseOrchestratorOutput("not json at all"); err == nil {
		t.Fatal("expected error on unparseable output")
	}
}

func TestOpenToolArgs(t *testing.T) {
	dec, _ := ParseOrchestratorOutput(`{"narrate":"","tools":[{"name":"open_tool","args":{"tool":"reading","reason":"去读那篇"}}]}`)
	args, err := OpenToolArgs(dec.Tools[0])
	if err != nil || args.Tool != ToolReading || args.Reason != "去读那篇" {
		t.Fatalf("OpenToolArgs wrong: %+v err=%v", args, err)
	}
}

func TestParseOrchestratorOutput_NewTools(t *testing.T) {
	raw := `{"narrate":"我来生成计划","tools":[{"name":"generate_plan","args":{}},{"name":"propose_question","args":{"text":"人均碳排放呢？"}}]}`
	dec, err := ParseOrchestratorOutput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(dec.Tools) != 2 {
		t.Fatalf("want 2 tools, got %d", len(dec.Tools))
	}
	q, err := ProposeQuestionArgs(dec.Tools[1])
	if err != nil || q.Text == "" {
		t.Fatalf("propose_question args: %v %q", err, q.Text)
	}
}

// P4 · the narration-of-configuration rule (spec §8: 印记 states what it set up
// + asks one next-step question) must stay in the prompt, and the open_tool
// bullet must list the forming(提案) room (P2b split). Guards a silent drop.
func TestOrchestratorPrompt_NarrationAndForming(t *testing.T) {
	if !strings.Contains(orchestratorSystemPrompt, "叙述规则") {
		t.Error("prompt lost the narration-of-configuration rule (P4)")
	}
	if !strings.Contains(orchestratorSystemPrompt, "forming(提案要点)") {
		t.Error("open_tool bullet must list forming(提案要点)")
	}
}

func TestClaimsNoteRecording(t *testing.T) {
	yes := []string{
		"你的研究问题很清晰，我先帮你把这条目标记进提案面板。",
		"这条我也给你记进提案要点里了。",
		"我把你这条三周计划记进要点里，当作活动的初稿。",
		"好，帮你记一条缘由。",
	}
	for _, s := range yes {
		if !ClaimsNoteRecording(s) {
			t.Errorf("expected claim detected in %q", s)
		}
	}
	no := []string{
		"这个问题你想怎么问？",
		"先跟我说说你打算怎么开头？",
		"我把提案面板给你打开了，我们一起理清楚四件事。",
	}
	for _, s := range no {
		if ClaimsNoteRecording(s) {
			t.Errorf("did NOT expect claim detected in %q", s)
		}
	}
}

func TestFilterToolsForStatus(t *testing.T) {
	reg := StatusRegistry()
	dec := OrchestratorDecision{
		Narrate: "x",
		Tools: []OrchestratorToolCall{
			{Name: "propose_note", Args: json.RawMessage(`{"section":"objective","value":"q"}`)},
			{Name: "generate_plan", Args: json.RawMessage(`{}`)},
			{Name: "open_tool", Args: json.RawMessage(`{"tool":"plan"}`)},
		},
	}
	// framework permits propose_note only (of these three).
	got := FilterToolsForStatus(dec, reg[FlowFramework])
	if len(got.Tools) != 1 || got.Tools[0].Name != "propose_note" {
		t.Fatalf("framework filter: want [propose_note], got %+v", got.Tools)
	}
	// essay does NOT permit propose_note → dropped to zero.
	essayDec := OrchestratorDecision{Tools: []OrchestratorToolCall{
		{Name: "propose_note", Args: json.RawMessage(`{"section":"objective","value":"q"}`)},
	}}
	if got := FilterToolsForStatus(essayDec, reg[FlowEssay]); len(got.Tools) != 0 {
		t.Fatalf("essay should drop propose_note, got %+v", got.Tools)
	}
	// essay permits finish_part.
	fin := OrchestratorDecision{Tools: []OrchestratorToolCall{{Name: "finish_part", Args: json.RawMessage(`{}`)}}}
	if got := FilterToolsForStatus(fin, reg[FlowEssay]); len(got.Tools) != 1 {
		t.Fatalf("essay should keep finish_part, got %+v", got.Tools)
	}
}

func TestBuildStatusRequest_UsesStatusPrompt(t *testing.T) {
	reg := StatusRegistry()
	req := BuildStatusRequest(reg[FlowEssay], "主题：X", DefaultStudioState(), []ChatTurn{{Role: "user", Content: "hi"}})
	if len(req.Messages) < 2 {
		t.Fatal("expected system + turns")
	}
	if req.Messages[0].Content != reg[FlowEssay].SystemPrompt {
		t.Error("system message should be the essay status prompt")
	}
}
