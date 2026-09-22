package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestOfflineExamplesHaveUniqueIDsAndFaithfulTraces(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range examples() {
		if e.ID == "" || seen[e.ID] || len(e.Request.Messages) < 2 {
			t.Fatalf("invalid case %q", e.ID)
		}
		seen[e.ID] = true
		for name, d := range e.ContextFragments {
			found := false
			for _, msg := range e.Request.Messages {
				if strings.Contains(msg.Content, d.Text) {
					found = true
				}
			}
			if !found {
				t.Fatalf("context fragment not in request: %s/%s", e.ID, name)
			}
		}
		for i, d := range e.Documents {
			if i >= len(e.Request.Messages) || d.Text != e.Request.Messages[i].Content {
				t.Fatalf("trace diverges from request: %s", e.ID)
			}
		}
	}
	var out bytes.Buffer
	if err := run([]string{"-case", "teacher/assignment"}, &out); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	req := decoded["request"].(map[string]any)
	// Tools must be inspectable alongside messages, since their descriptions
	// also instruct the model. The exact number/text may change intentionally.
	tools, ok := req["tools"].([]any)
	if !ok || len(tools) == 0 {
		t.Fatal("tool schemas missing from preview")
	}
}

func TestCLIRejectsAmbiguousOrUnknownRequests(t *testing.T) {
	for _, args := range [][]string{{}, {"-list", "-all"}, {"-case", "missing"}, {"-template", "missing"}, {"-list", "unexpected"}} {
		var out bytes.Buffer
		if run(args, &out) == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
