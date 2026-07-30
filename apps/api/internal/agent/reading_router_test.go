package agent

import (
	"context"
	"testing"

	"mindimprint/api/internal/gateway"
)

func readingStubResolver() gateway.KeyResolver {
	return func(ctx context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{Provider: "deepseek", Model: "x", Tier: "flagship"}, nil
	}
}

func TestRouteReading_ParsesSummon(t *testing.T) {
	script := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"decision":"summon","card_id":"argument-map",` +
			`"reason":"这句像是一个没给证据的结论","reply":"这句话读起来确实像是跳过了证据直接下结论。",` +
			`"example_block_id":"b1",` +
			`"example_quote":"因此这项政策必然失败","example_why":"它用'必然'下了强结论",` +
			`"followup_plan":["fact-opinion-value"]}`},
		{Kind: gateway.EventDone},
	}
	p := gateway.NewStubProvider(script)
	in := ReadingRouteInput{
		StudentText: "这段读着怪怪的",
		Catalog:     []ReadingCard{{CardID: "argument-map", Name: "论证地图", Trigger: "结论缺证据时"}},
	}
	d, resolved, _, err := RouteReading(context.Background(), p, readingStubResolver(), in)
	if err != nil {
		t.Fatalf("RouteReading error: %v", err)
	}
	if d.Decision != "summon" || d.CardID != "argument-map" || d.ExampleQuote != "因此这项政策必然失败" {
		t.Fatalf("unexpected decision: %+v", d)
	}
	if d.Reply != "这句话读起来确实像是跳过了证据直接下结论。" {
		t.Fatalf("reply not parsed: %+v", d)
	}
	if len(d.FollowupPlan) != 1 || d.FollowupPlan[0] != "fact-opinion-value" {
		t.Fatalf("followup plan not parsed: %+v", d.FollowupPlan)
	}
	if resolved.Provider != "deepseek" {
		t.Fatalf("resolved not returned for a real call: %+v", resolved)
	}
}

func TestRouteReading_FallsBackToRespondOnGarbage(t *testing.T) {
	p := gateway.NewStubProvider([]gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: "not json at all"},
		{Kind: gateway.EventDone},
	})
	d, _, _, err := RouteReading(context.Background(), p, readingStubResolver(),
		ReadingRouteInput{StudentText: "hi"})
	if err != nil {
		t.Fatalf("RouteReading should not error on garbage, got %v", err)
	}
	if d.Decision != "respond" {
		t.Fatalf("garbage should degrade to respond, got %+v", d)
	}
}

func TestRouteReading_RetriesOnEmptyReply(t *testing.T) {
	// 1st attempt: valid JSON but EMPTY reply (the generic-fallback trigger).
	// 2nd attempt: a real reply. RouteReading must retry and return the real one,
	// with usage accumulated across BOTH calls (cost stays accurate).
	empty := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"decision":"respond","reply":""}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 30, OutputTokens: 2}},
		{Kind: gateway.EventDone},
	}
	good := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"decision":"respond","reply":"这句里最关键的词是哪一个？"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 40, OutputTokens: 6}},
		{Kind: gateway.EventDone},
	}
	p := gateway.NewSequenceStubProvider(empty, good)
	d, _, usage, err := RouteReading(context.Background(), p, readingStubResolver(), ReadingRouteInput{StudentText: "hi"})
	if err != nil {
		t.Fatalf("RouteReading error: %v", err)
	}
	if p.Calls != 2 {
		t.Fatalf("empty reply must trigger exactly one retry (2 calls), got %d", p.Calls)
	}
	if d.Reply != "这句里最关键的词是哪一个？" {
		t.Fatalf("retry reply not returned: %+v", d)
	}
	if usage.InputTokens != 70 || usage.OutputTokens != 8 {
		t.Fatalf("usage must accumulate across attempts, got %+v", usage)
	}
}

func TestRouteReading_NoRetryWhenFirstReplyIsGood(t *testing.T) {
	good := []gateway.StreamEvent{
		{Kind: gateway.EventTextDelta, TextDelta: `{"decision":"respond","reply":"你先说说这句让你困惑在哪？"}`},
		{Kind: gateway.EventUsage, Usage: &gateway.ChatUsage{InputTokens: 40, OutputTokens: 6}},
		{Kind: gateway.EventDone},
	}
	// If a retry fired it would hit `bad` and blank the reply — so a passing
	// assertion proves no needless second call.
	bad := []gateway.StreamEvent{{Kind: gateway.EventTextDelta, TextDelta: "garbage"}, {Kind: gateway.EventDone}}
	p := gateway.NewSequenceStubProvider(good, bad)
	d, _, _, err := RouteReading(context.Background(), p, readingStubResolver(), ReadingRouteInput{StudentText: "hi"})
	if err != nil {
		t.Fatalf("RouteReading error: %v", err)
	}
	if p.Calls != 1 {
		t.Fatalf("a good first reply must not retry, got %d calls", p.Calls)
	}
	if d.Reply != "你先说说这句让你困惑在哪？" {
		t.Fatalf("first good reply not returned: %+v", d)
	}
}

func TestRouteReading_ResolverErrorDegradesToRespond(t *testing.T) {
	p := gateway.NewStubProvider(nil)
	badResolver := gateway.KeyResolver(func(ctx context.Context) (gateway.Resolved, error) {
		return gateway.Resolved{}, context.DeadlineExceeded
	})
	d, resolved, _, err := RouteReading(context.Background(), p, badResolver, ReadingRouteInput{})
	if err != nil {
		t.Fatalf("resolver error should degrade, not error: %v", err)
	}
	if d.Decision != "respond" || resolved.Provider != "" {
		t.Fatalf("want respond + no resolved, got %+v / %+v", d, resolved)
	}
}
