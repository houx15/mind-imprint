package pbl

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"mindimprint/api/internal/gateway"
)

func TestEvidenceCategoriesAreBoundedWithoutChangingVerdict(t *testing.T) {
	for _, category := range []string{"student_constraints", "unsupported_claim", "invalid_method", "missing_action", "other", "", "private model-written content"} {
		t.Run(category, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"supported": false, "issues": []map[string]string{{"quote": "五分钟就能证明", "reason": "没有效果证据", "category": category}}})
			out, err := parseEvidenceCheck(string(raw), "五分钟就能证明")
			if err != nil || out.Supported || len(out.Issues) != 1 {
				t.Fatalf("changed verdict: %#v %v", out, err)
			}
			want := category
			if category == "" || category == "private model-written content" {
				want = "other"
			}
			if out.Issues[0].Category != want || out.Issues[0].Quote != "五分钟就能证明" || out.Issues[0].Reason != "没有效果证据" {
				t.Fatalf("category or source changed: %#v", out.Issues[0])
			}
		})
	}
}

func TestEvidenceCheckRejectsTruncatedEvenParseableVerdict(t *testing.T) {
	for _, stop := range []string{gateway.StopStop, gateway.StopLength} {
		provider := gateway.NewStubProvider([]gateway.StreamEvent{
			{Kind: gateway.EventTextDelta, TextDelta: `{"supported":true,"issues":[]}`},
			{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 12, OutputTokens: 8}},
			{Kind: gateway.EventDone, StopReason: stop},
		})
		check, usage, err := CheckEvidence(context.Background(), provider, gateway.Resolved{}, []string{"没有现场记录"}, "尚未现场观察")
		if usage.InputTokens != 12 || usage.OutputTokens != 8 {
			t.Fatal("lost billable usage")
		}
		if stop == gateway.StopLength {
			if !errors.Is(err, ErrInvalidEvidence) || check.Supported {
				t.Fatal("accepted truncated verdict", check, err)
			}
		} else if err != nil || !check.Supported {
			t.Fatal("rejected complete verdict", check, err)
		}
	}
}

func TestEvidenceCheckRequiresGroundedVerdict(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"supported":false,"issues":[]}`,
		`{"supported":true,"issues":[{"quote":"现场事实","reason":"虚构"}]}`,
		`{"supported":false,"issues":[{"quote":"没有说过","reason":"虚构"}]}`,
	} {
		if _, err := parseEvidenceCheck(raw, "这是现场事实"); err == nil {
			t.Fatalf("accepted invalid verdict: %s", raw)
		}
	}
	for _, raw := range []string{`{"supported":true,"issues":[]}`, `{"supported":false,"issues":[{"quote":"现场事实","reason":"原始记录是虚构"}]}`} {
		if _, err := parseEvidenceCheck(raw, "这是现场事实"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEvidenceQuoteUsesDecodedTextWithoutJoiningFields(t *testing.T) {
	candidate := `{"Reply":"请保留\"不确定\"分类。","Produce":{"Payload":{"body":"第一行\n第二行","label":"现场"}},"other":"事实"}`
	for _, c := range []struct {
		quote string
		valid bool
	}{
		{"保留\"不确定\"分类", true},
		{"第一行\n第二行", true},
		{"现场事实", false},
		{"Payload", false},
		{"第一行\\n第二行", false},
		{"没有说过", false},
	} {
		raw, _ := json.Marshal(EvidenceCheck{Supported: false, Issues: []EvidenceIssue{{Quote: c.quote, Reason: "测试引用归属"}}})
		_, err := parseEvidenceCheck(string(raw), candidate)
		if (err == nil) != c.valid {
			t.Errorf("quote %q: valid=%v err=%v", c.quote, c.valid, err)
		}
	}
}

func TestEvidenceRequestKeepsStructuredCandidate(t *testing.T) {
	req := evidenceRequest([]string{"原始记录"}, `{"Reply":"请保留\"不确定\"分类。"}`)
	var input struct {
		Candidate map[string]string `json:"candidate"`
	}
	if err := json.Unmarshal([]byte(req.Messages[1].Content), &input); err != nil {
		t.Fatal(err)
	}
	if input.Candidate["Reply"] != "请保留\"不确定\"分类。" {
		t.Fatalf("candidate lost decoded text: %+v", input)
	}
}
