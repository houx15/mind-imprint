package evalbench

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestProductionEvaluatorReplaysFourCallBaselineWithoutDatabase(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	result, err := productionEvalReportV1{}.Run(context.Background(), RunDeps{
		Resolved: gateway.Resolved{Provider: "deepseek", Model: "test"},
		ProviderForPurpose: func(purpose string) gateway.Provider {
			mu.Lock()
			seen[purpose]++
			mu.Unlock()
			return gateway.NewStubProvider(streamText(productionReply(purpose)))
		},
	}, singlePromptTestInput(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Complete || len(result.Report.Depth) != 6 || len(result.Report.Autonomy) != 6 {
		t.Fatalf("unexpected production baseline result: %#v", result)
	}
	for _, purpose := range []string{"eval_report_promptlens", "eval_report_risks", "eval_report_rubric", "eval_report_abstract"} {
		if seen[purpose] != 1 {
			t.Fatalf("%s calls = %d, want 1", purpose, seen[purpose])
		}
	}
}

func TestProductionEvaluatorKeepsBestEffortDegradation(t *testing.T) {
	result, err := productionEvalReportV1{}.Run(context.Background(), RunDeps{
		ProviderForPurpose: func(purpose string) gateway.Provider {
			if purpose == "eval_report_risks" {
				return gateway.NewStubProvider([]gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "not JSON"}, {Kind: gateway.EventDone}})
			}
			return gateway.NewStubProvider(streamText(productionReply(purpose)))
		},
	}, singlePromptTestInput(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Complete || len(result.Report.Risks) != 0 || len(result.Report.Depth) != 6 {
		t.Fatalf("best-effort degradation changed: %#v", result)
	}
}

func productionReply(purpose string) string {
	switch purpose {
	case "eval_report_promptlens":
		return `{"summary":"s","prompts":[]}`
	case "eval_report_risks":
		return `{"risks":[]}`
	case "eval_report_abstract":
		return `{"overview":"o","materialSentence":"m","writingSentence":"w","aiSentence":"a","suggestionParagraph":"g","suggestionSentences":[],"recommendedCourses":[]}`
	case "eval_report_rubric":
		depth, autonomy := make([]string, 0, 6), make([]string, 0, 6)
		for i := 1; i <= 6; i++ {
			depth = append(depth, fmt.Sprintf(`{"id":"D%d","level":2,"summary":"s","evidence":[],"suggestion":"x"}`, i))
			autonomy = append(autonomy, fmt.Sprintf(`{"id":"A%d","band":2,"summary":"s","evidence":[],"suggestion":"x"}`, i))
		}
		return `{"depth":[` + strings.Join(depth, ",") + `],"autonomy":[` + strings.Join(autonomy, ",") + `]}`
	default:
		return `{}`
	}
}
