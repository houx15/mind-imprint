package agent

import (
	"context"
	"strings"
	"testing"

	"mindimprint/api/internal/gateway"
)

func spotTargets() []SpotCheckTarget {
	return []SpotCheckTarget{
		{ID: "m1", Name: "NASA 全球变绿观测", Detail: "档位：一手数据；作用与风险：仅是遥感叶面积。"},
		{ID: "m2", Name: "BP 世界能源统计", Detail: "档位：机构报告；作用与风险：（未写）"},
	}
}

func TestProposeSpotCheckResolvesNamesServerSide(t *testing.T) {
	prov := scriptedProvider(`[
	  {"target_id":"m1","target_name":"模型瞎编的名字","evidence":"写了作用与风险","missing":"没说清遥感口径的局限","fix":"补一句这条数据不能回答什么"},
	  {"target_id":"m2","evidence":"档位已定","missing":"还没写作用与风险","fix":"写这条在论证里承担什么"}
	]`)
	items, _, err := ProposeSpotCheck(context.Background(), prov, gateway.Resolved{Provider: "stub"}, SpotCheckSources, spotTargets())
	if err != nil {
		t.Fatalf("ProposeSpotCheck: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].TargetName != "NASA 全球变绿观测" {
		t.Errorf("TargetName = %q — must be resolved server-side, never taken from the model", items[0].TargetName)
	}
	if items[1].TargetName != "BP 世界能源统计" {
		t.Errorf("TargetName = %q, want the server-resolved name", items[1].TargetName)
	}
}

func TestProposeSpotCheckDropsUnknownTargets(t *testing.T) {
	prov := scriptedProvider(`[
	  {"target_id":"m1","evidence":"e","missing":"m","fix":"f"},
	  {"target_id":"ghost","evidence":"e","missing":"m","fix":"f"}
	]`)
	items, _, err := ProposeSpotCheck(context.Background(), prov, gateway.Resolved{Provider: "stub"}, SpotCheckSources, spotTargets())
	if err != nil {
		t.Fatalf("ProposeSpotCheck: %v", err)
	}
	if len(items) != 1 || items[0].TargetID != "m1" {
		t.Fatalf("items = %+v, want only the known target", items)
	}
}

func TestProposeSpotCheckRejectsWholeOrderOnBannedPhrasing(t *testing.T) {
	// One violating field must reject the WHOLE order — the same
	// all-or-nothing discipline ProposeReview uses (review.go:162-165).
	// The banned literal here is the same "rewritten-sentence-zh" phrase
	// review_test.go's TestProposeReview_RejectsBannedPhrase already exercises
	// (enforcement/banned_phrasing.go), inlined directly rather than via a helper.
	prov := scriptedProvider(`[
	  {"target_id":"m1","evidence":"ok","missing":"ok","fix":"ok"},
	  {"target_id":"m2","evidence":"ok","missing":"ok","fix":"你应该这样写：这条数据只能证明遥感叶面积上升。"}
	]`)
	items, usage, err := ProposeSpotCheck(context.Background(), prov, gateway.Resolved{Provider: "stub"}, SpotCheckSources, spotTargets())
	if err == nil {
		t.Fatal("expected the whole order to be rejected")
	}
	if items != nil {
		t.Error("no items may be returned when enforcement rejects the order")
	}
	// scriptedProvider (the agent package's shared gateway.Provider stub, see
	// coach_test.go) always reports a fixed usage regardless of the scripted
	// text, so this checks the INTENT the brief calls for — a rejected order
	// still surfaces the usage the caller needs to bill — using the fields
	// gateway.ChatUsage actually has (InputTokens/OutputTokens), not the
	// PromptTokens/CompletionTokens the brief's draft named.
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		t.Error("usage must be populated even on rejection — a rejected call still cost money")
	}
}

func TestSpotCheckPromptsDifferByStation(t *testing.T) {
	src := spotCheckSystemPrompt(SpotCheckSources)
	arg := spotCheckSystemPrompt(SpotCheckArgument)
	if src == arg {
		t.Fatal("the two stations must use different postures")
	}
	for _, p := range []string{src, arg} {
		if !strings.Contains(p, "绝不替学生改写句子") {
			t.Error("every posture must carry the RL-1 iron rule verbatim")
		}
	}
	if strings.Contains(src, "分点") || strings.Contains(arg, "分点") {
		t.Error("spot-check postures must not ask for points — that is 0457 readiness data")
	}
}

func TestSpotCheckFingerprintStability(t *testing.T) {
	a := []SpotCheckTarget{{ID: "m1", Name: "NASA", Detail: "作用与风险：仅是遥感叶面积。"}}
	if SpotCheckFingerprint(a) != SpotCheckFingerprint(a) {
		t.Fatal("fingerprint must be stable for identical input")
	}
	changedDetail := []SpotCheckTarget{{ID: "m1", Name: "NASA", Detail: "作用与风险：补充了口径说明。"}}
	if SpotCheckFingerprint(a) == SpotCheckFingerprint(changedDetail) {
		t.Error("rewriting what the check reads must change the fingerprint")
	}
	added := []SpotCheckTarget{
		{ID: "m1", Name: "NASA", Detail: "作用与风险：仅是遥感叶面积。"},
		{ID: "m2", Name: "BP", Detail: "作用与风险：总量仍高。"},
	}
	if SpotCheckFingerprint(a) == SpotCheckFingerprint(added) {
		t.Error("adding a target must change the fingerprint")
	}
	// The NAME is display-only; it must still participate, because renaming a
	// source changes what the student sees in the work order.
	renamed := []SpotCheckTarget{{ID: "m1", Name: "NASA (v2)", Detail: "作用与风险：仅是遥感叶面积。"}}
	if SpotCheckFingerprint(a) == SpotCheckFingerprint(renamed) {
		t.Error("renaming a target must change the fingerprint")
	}
}

func TestSpotCheckFingerprintIsNotConcatenationAmbiguous(t *testing.T) {
	// A naive strings.Join without a separator would hash these identically.
	x := []SpotCheckTarget{{ID: "ab", Name: "c", Detail: "d"}}
	y := []SpotCheckTarget{{ID: "a", Name: "bc", Detail: "d"}}
	if SpotCheckFingerprint(x) == SpotCheckFingerprint(y) {
		t.Error("field boundaries must be unambiguous in the hashed serialization")
	}
}
