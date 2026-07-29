package agent

import (
	"strings"
	"testing"
)

func TestReadingBrief_ReachesPrompt(t *testing.T) {
	in := ReadingRouteInput{
		StudentText: "这段说碳排放很高",
		Article:     "……全球碳排放……",
		Brief: ReadingBrief{
			Reason:       "验证碳排放是否构成反例",
			PhaseTag:     "反例检验",
			ProposalSnap: "论点：中国推动可持续",
		},
	}
	prompt := buildReadingRouteUserPrompt(in) // the pure prompt assembler
	if !strings.Contains(prompt, "验证碳排放是否构成反例") || !strings.Contains(prompt, "反例检验") {
		t.Fatalf("brief must appear in the router prompt:\n%s", prompt)
	}
}
