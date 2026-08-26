package evalbench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

var goldCaseIDs = []string{
	"persona2-deepdiver", "persona3-anxious", "persona4-confirmbias",
	"persona5-ghostwrite", "persona6-ordinary", "persona7-divergent",
	"persona8-lifeexp", "persona9-hoarder", "persona10-search",
}

func goldFixturePath(caseID string) string {
	return filepath.Join("..", "..", "tools", "evalbench", "cases", caseID+".manual.report.json")
}

func TestAllJSONGoldFixturesValidateAndHavePersonaInputs(t *testing.T) {
	for _, caseID := range goldCaseIDs {
		t.Run(caseID, func(t *testing.T) {
			gold, raw, err := LoadGoldReport(goldFixturePath(caseID))
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) == 0 || len(gold.Report.Depth) != 6 || len(gold.Report.Autonomy) != 6 {
				t.Fatalf("unexpected gold report: %#v", gold.Report)
			}
			persona := filepath.Join("..", "..", "tools", "evalbench", "cases", caseID+".json")
			if _, err := os.Stat(persona); err != nil {
				t.Fatalf("paired persona input: %v", err)
			}
		})
	}
}

func TestParseGoldReportRejectsInvalidContracts(t *testing.T) {
	raw, err := os.ReadFile(goldFixturePath("persona2-deepdiver"))
	if err != nil {
		t.Fatal(err)
	}
	decode := func(t *testing.T) map[string]any {
		t.Helper()
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	encode := func(t *testing.T, v map[string]any) []byte {
		t.Helper()
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing report", func(v map[string]any) { delete(v, "report") }},
		{"unknown wrapper field", func(v map[string]any) { v["extra"] = true }},
		{"unknown report field", func(v map[string]any) { v["report"].(map[string]any)["extra"] = true }},
		{"missing depth", func(v map[string]any) { delete(v["report"].(map[string]any), "depth") }},
		{"duplicate depth id", func(v map[string]any) {
			depth := v["report"].(map[string]any)["depth"].([]any)
			depth[1].(map[string]any)["id"] = "D1"
		}},
		{"out of range level", func(v map[string]any) {
			v["report"].(map[string]any)["depth"].([]any)[0].(map[string]any)["level"] = 5
		}},
		{"invalid risk type", func(v map[string]any) {
			v["report"].(map[string]any)["risks"].([]any)[0].(map[string]any)["type"] = "not-a-risk"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := decode(t)
			tc.mutate(v)
			if _, err := ParseGoldReport(encode(t, v)); err == nil {
				t.Fatal("invalid JSON Gold unexpectedly accepted")
			}
		})
	}
	if _, err := ParseGoldReport(append(raw, []byte("\n{}")...)); err == nil {
		t.Fatal("multiple JSON values unexpectedly accepted")
	}
}

func TestComparisonProjectionContainsNoFactEnvelope(t *testing.T) {
	gold, _, err := LoadGoldReport(goldFixturePath("persona2-deepdiver"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(projectForComparison(gold.Report))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range [][]byte{[]byte("student"), []byte("basics"), []byte("events"), []byte("materials"), []byte("toolUsage")} {
		if bytes.Contains(b, forbidden) {
			t.Fatalf("comparison projection leaked FACT field %q: %s", forbidden, b)
		}
	}
	if !bytes.Contains(b, []byte("depth")) || !bytes.Contains(b, []byte("promptLens")) {
		t.Fatalf("comparison projection omitted model fields: %s", b)
	}
}

func TestCompareSendsOnlyModelSectionsForGoldAndCandidate(t *testing.T) {
	gold, _, err := LoadGoldReport(goldFixturePath("persona2-deepdiver"))
	if err != nil {
		t.Fatal(err)
	}
	candidate := gold.Report
	candidate.Student.Name = "LEAK_STUDENT"
	candidate.Basics.Title = "LEAK_BASICS"
	candidate.Events[0].Summary = "LEAK_EVENTS"
	candidate.Materials[0].Source = "LEAK_MATERIALS"
	candidate.ToolUsage[0].Name = "LEAK_TOOLS"

	items := ExpectedComparisonItems()
	for i := range items {
		items[i].Comparison = "aligned"
		items[i].Confidence = "high"
		items[i].GoldExcerpt = "gold"
		items[i].CandidateExcerpt = "candidate"
		items[i].Reason = "test"
	}
	response, err := json.Marshal(Comparison{Items: items})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	rt := &Runtime{
		profiles: map[string]ModelProfile{"flagship": {Provider: "deepseek", Model: "test"}},
		provider: gateway.NewStubProvider(streamText(string(response))),
	}
	recorder := &CallRecorder{}
	if _, err := Compare(t.Context(), rt, ModelUse{Model: "flagship", PromptVersion: ComparatorPromptVersion}, candidate, gold.Report, recorder); err != nil {
		t.Fatal(err)
	}
	calls := recorder.Calls()
	if len(calls) != 1 || len(calls[0].Request.Messages) != 2 {
		t.Fatalf("comparator calls = %#v", calls)
	}
	request := calls[0].Request.Messages[1].Content
	for _, forbidden := range []string{"LEAK_STUDENT", "LEAK_BASICS", "LEAK_EVENTS", "LEAK_MATERIALS", "LEAK_TOOLS"} {
		if strings.Contains(request, forbidden) {
			t.Fatalf("comparator request leaked FACT value %q", forbidden)
		}
	}
	if !strings.Contains(request, "人工 Gold EvaluationReport 的模型部分 JSON") || !strings.Contains(request, "\"depth\"") {
		t.Fatalf("comparator request omitted JSON gold: %s", request)
	}
}

func TestWriteGoldReportPreservesJSONSnapshot(t *testing.T) {
	_, raw, err := LoadGoldReport(goldFixturePath("persona2-deepdiver"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "gold-report.json")
	if err := writeGoldReport(path, raw); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatal("gold snapshot changed source bytes")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "gold-report.md")); !os.IsNotExist(err) {
		t.Fatalf("unexpected legacy gold snapshot: %v", err)
	}
}

func TestConfigRejectsV2AndMarkdownGold(t *testing.T) {
	c, err := LoadConfig(filepath.Join("..", "..", "tools", "evalbench", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if c.SchemaVersion != SchemaVersion || c.Comparator.PromptVersion != ComparatorPromptVersion {
		t.Fatalf("default config versions = %#v", c)
	}
	wantCases := []string{"persona2-deepdiver", "persona4-confirmbias", "persona5-ghostwrite"}
	if len(c.Cases) != len(wantCases) {
		t.Fatalf("default cases = %#v, want %v", c.Cases, wantCases)
	}
	for i, want := range wantCases {
		if c.Cases[i].ID != want || c.Cases[i].GoldReport != "cases/"+want+".manual.report.json" {
			t.Fatalf("default case %d = %#v, want JSON gold for %q", i, c.Cases[i], want)
		}
	}
	c.SchemaVersion = 2
	if err := c.Validate(); err == nil {
		t.Fatal("v2 config unexpectedly accepted")
	}
	c.SchemaVersion = SchemaVersion
	c.Cases[0].GoldReport = "cases/legacy-gold.md"
	if err := c.Validate(); err == nil {
		t.Fatal("Markdown gold path unexpectedly accepted")
	}
}
