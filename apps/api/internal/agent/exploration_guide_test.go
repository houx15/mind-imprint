package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

// #12/#13 · a focused dig: a single lead (with the student's own thinking) is
// graph content to point from even with no sources/leads, and both land in the
// prompt so the directions center on THAT thread.
func TestComposeExplorationGuide_FocusLeadAndThought(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"directions":[{"direction":"d","why":"w"}]}`},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	in := ExplorationGuideInput{FocusLead: "中国人均排放 vs 总量的口径", Thought: "我怀疑这是两套叙事"}
	if !HasGraphContent(in) {
		t.Fatal("a focus lead alone must count as graph content to point from")
	}
	dirs, _, err := ComposeExplorationGuide(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, in)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("want 1 direction, got %d", len(dirs))
	}
	user := prov.LastRequest.Messages[1].Content
	if !strings.Contains(user, "中国人均排放 vs 总量的口径") || !strings.Contains(user, "我怀疑这是两套叙事") {
		t.Fatalf("focus lead / thought missing from prompt: %s", user)
	}
}

func TestComposeExplorationGuide_ParsesDirections(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "```json\n{\"directions\":[{\"direction\":\"找中国碳排放绝对量的一手数据\",\"why\":\"你缺反例检验的来源\"}]}\n```"},
		{Kind: gateway.EventDone},
	}
	prov := gateway.NewStubProvider(script)
	in := ExplorationGuideInput{
		ProposalObjective: "中国是否让地球变得更可持续？",
		Sources: []ExplorationGraphSource{
			{Title: "NASA carbon report", Decision: "use", Credibility: "strong", PhaseTag: "反例检验", State: "已归纳"},
		},
		OpenLeads: []string{"中国人均排放 vs 总量的口径"},
	}
	directions, _, err := ComposeExplorationGuide(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, in)
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	if len(directions) != 1 {
		t.Fatalf("want 1 direction, got %d", len(directions))
	}
	if directions[0].Direction != "找中国碳排放绝对量的一手数据" || directions[0].Why != "你缺反例检验的来源" {
		t.Fatalf("unexpected direction: %+v", directions[0])
	}
}

func TestComposeExplorationGuide_EmptyGraphErrors(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"directions":[]}`},
		{Kind: gateway.EventDone},
	}
	prov := &countingProvider{inner: gateway.NewStubProvider(script)}
	_, _, err := ComposeExplorationGuide(context.Background(), prov, gateway.Resolved{Provider: "fake", Model: "m"}, ExplorationGuideInput{})
	if err == nil {
		t.Fatalf("empty graph (no sources, no leads) should error, not fabricate")
	}
	if prov.calls != 0 {
		t.Fatalf("want zero provider calls on empty graph, got %d", prov.calls)
	}
}

func TestHasGraphContent(t *testing.T) {
	if HasGraphContent(ExplorationGuideInput{}) {
		t.Fatalf("empty input should have no graph content")
	}
	if !HasGraphContent(ExplorationGuideInput{Sources: []ExplorationGraphSource{{Title: "x"}}}) {
		t.Fatalf("a source should count as graph content")
	}
	if !HasGraphContent(ExplorationGuideInput{OpenLeads: []string{"lead"}}) {
		t.Fatalf("an open lead should count as graph content")
	}
}
