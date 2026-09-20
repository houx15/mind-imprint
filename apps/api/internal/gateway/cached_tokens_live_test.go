package gateway

import (
	"context"
	"os"
	"strings"
	"testing"
)

// The cached-token number is the difference between "the reading room is
// expensive" and "we costed it wrong": DashScope bills a matched prefix at
// CachedInputRate, and a reading turn re-sends the whole article, so most of
// its input arrives cached. A stub cannot check this — it only proves we can
// read back a field we wrote ourselves. So it runs against the real endpoint.
//
//	LIVE_LLM=1 go test ./internal/gateway -run TestLiveCachedTokens
func TestLiveCachedTokens(t *testing.T) {
	if os.Getenv("LIVE_LLM") == "" {
		t.Skip("set LIVE_LLM=1 to run")
	}
	if os.Getenv("DASHSCOPE_API_KEY") == "" {
		t.Skip("no DASHSCOPE_API_KEY")
	}
	cat, err := DefaultCatalog()
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := cat.Resolve(ClassDialogue, "", func(env string) string { return os.Getenv(env) })
	if err != nil {
		t.Fatal(err)
	}

	// The prefix has to clear the provider's minimum cacheable length (1024
	// tokens) or there is nothing to hit, and a short prompt would make this
	// test pass by reporting a true zero.
	head := strings.Repeat("这是一篇用来占位的文章段落，它只需要足够长。", 400)
	ask := func(tail string) ChatUsage {
		t.Helper()
		res, cerr := Collect(context.Background(), NewCatalogProvider(nil), resolved, ChatRequest{
			Messages: []ChatMessage{
				{Role: RoleSystem, Content: head},
				{Role: RoleUser, Content: tail},
			},
			MaxTokens: 1,
		})
		if cerr != nil {
			t.Fatalf("call failed: %v", cerr)
		}
		return res.Usage
	}

	first := ask("第一问。")
	if first.InputTokens < 1024 {
		t.Fatalf("prompt too short to be cacheable: %d tokens", first.InputTokens)
	}
	second := ask("第二问，换一句结尾。")

	if second.CachedInputTokens == 0 {
		t.Fatalf("no cached tokens reported on the repeat call (prompt=%d). "+
			"Either the channel stopped returning prompt_tokens_details.cached_tokens, "+
			"or we stopped parsing it — and every cost estimate silently became ~5x "+
			"too high on the input side.", second.InputTokens)
	}
	if second.CachedInputTokens > second.InputTokens {
		t.Fatalf("cached %d > prompt %d: cached tokens are a SUBSET of the prompt",
			second.CachedInputTokens, second.InputTokens)
	}
	t.Logf("repeat call: prompt=%d cached=%d (%.0f%%)", second.InputTokens,
		second.CachedInputTokens,
		100*float64(second.CachedInputTokens)/float64(second.InputTokens))
}
