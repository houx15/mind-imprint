package agent

import "testing"

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
