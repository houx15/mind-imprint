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
