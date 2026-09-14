package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func stubProviderText(text string) gateway.Provider {
	return gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: text},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 12}},
		{Kind: gateway.EventDone, StopReason: gateway.StopStop},
	})
}

func TestReviewFramework_ParsesVerdict(t *testing.T) {
	prov := stubProviderText(`{"ready":true,"why":"四点都扎实。","suggestions":["资源那条可以更具体","补一个反例"]}`)
	v, usage, err := ReviewFramework(context.Background(), prov, gateway.Resolved{Provider: "stub"}, FrameworkReviewInput{
		Objective: "探究印刷术对宗教改革传播的影响", Reason: "读史时的惊讶", Activities: "读三篇+比较时间线", Resources: "图书馆+JSTOR",
	})
	if err != nil {
		t.Fatalf("ReviewFramework err = %v", err)
	}
	if !v.Ready || v.Why == "" || len(v.Suggestions) != 2 {
		t.Fatalf("verdict = %+v, want ready + why + 2 suggestions", v)
	}
	if usage.OutputTokens == 0 {
		t.Errorf("usage should be reported for metering")
	}
}

func TestReviewFramework_ParsesFencedReply(t *testing.T) {
	prov := stubProviderText("```json\n{\"ready\":false,\"why\":\"目标太泛。\",\"suggestions\":[\"把「多大程度」落到可比较的指标\"]}\n```")
	v, _, err := ReviewFramework(context.Background(), prov, gateway.Resolved{Provider: "stub"}, FrameworkReviewInput{Objective: "印刷术"})
	if err != nil {
		t.Fatalf("fenced reply err = %v", err)
	}
	if v.Ready || len(v.Suggestions) != 1 {
		t.Fatalf("verdict = %+v, want not-ready + 1 suggestion", v)
	}
}

func TestReviewFramework_ClampsSuggestionsAndDropsBlank(t *testing.T) {
	prov := stubProviderText(`{"ready":true,"why":"ok","suggestions":["a"," ","b","c","d","e"]}`)
	v, _, err := ReviewFramework(context.Background(), prov, gateway.Resolved{Provider: "stub"}, FrameworkReviewInput{Objective: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Suggestions) != 4 {
		t.Fatalf("suggestions = %v, want 4 (blank dropped, clamped)", v.Suggestions)
	}
}

func TestReviewFramework_GarbageErrors(t *testing.T) {
	prov := stubProviderText("对不起，我不太确定。")
	if _, _, err := ReviewFramework(context.Background(), prov, gateway.Resolved{Provider: "stub"}, FrameworkReviewInput{Objective: "x"}); err == nil {
		t.Fatal("expected an error on an unparseable reply (caller degrades to no-verdict)")
	}
}

func TestFrameworkReviewBenchCasesCarryStableGoldVerdicts(t *testing.T) {
	wants := map[string]bool{
		"review/framework-review":          false,
		"review/framework-vague-objective": false,
		"review/framework-minimum-ready":   true,
		"review/framework-strong-ready":    true,
	}
	seen := map[string]bool{}
	for _, c := range BenchCases() {
		want, ok := wants[c.ID]
		if !ok {
			continue
		}
		seen[c.ID] = true
		if c.GoldCheck == nil {
			t.Fatalf("%s has no GoldCheck", c.ID)
		}
		text := `{"ready":false,"why":"x","suggestions":[]}`
		if want {
			text = `{"ready":true,"why":"x","suggestions":[]}`
		}
		if err := c.GoldCheck(text); err != nil {
			t.Fatalf("%s rejects expected ready: %v", c.ID, err)
		}
		wrong := `{"ready":true,"why":"x","suggestions":[]}`
		if want {
			wrong = `{"ready":false,"why":"x","suggestions":[]}`
		}
		if err := c.GoldCheck(wrong); err == nil {
			t.Fatalf("%s did not reject wrong ready", c.ID)
		}
	}
	if len(seen) != len(wants) {
		t.Fatalf("found gold cases %v, want %v", seen, wants)
	}
}
