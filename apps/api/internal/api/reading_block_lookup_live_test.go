package api

// reading_block_lookup_live_test.go —— 「查词」走 digest 档（更便宜的那个模型）
// 之后，词卡还过不过得了逐字核对，中文讲得顺不顺。
//
//	LIVE_LLM=1 go test ./internal/api -run TestLiveLookupWord -v -count=1

import (
	"context"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

func TestLiveLookupWord(t *testing.T) {
	var tool readingBlockTool
	for _, tt := range readingBlockTools {
		if tt.ID == "lookup" {
			tool = tt
		}
	}
	if tool.Class != gateway.ClassDigest {
		t.Fatalf("查词应该走 digest，现在是 %q", tool.Class)
	}
	prov, r := liveClass(t, tool.Class)
	blocks := SplitBlocks("During this prolonged period of cold and darkness, plants were unable to photosynthesize, and as a result, many animals starved to death. The fluffy body feathers the researchers found in the coprolite might have been good enough to keep the birds warm.")
	words := []string{"prolonged", "photosynthesize", "starved", "coprolite", "fluffy", "good enough"}
	failed, wrong := 0, 0
	for i, w := range words {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		start := time.Now()
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: readingBlockSystemFor(tool)},
				{Role: gateway.RoleUser, Content: buildReadingBlockPromptFor(tool, "恐龙粪便里的羽毛", blocks, 0, w)},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
		got, ok := parseWordCards(sliceBlockJSON(res.Text), blocks[0].Text)
		if !ok {
			failed++
			t.Logf("sample %d (%s): 作废\n%s", i, w, res.Text)
			continue
		}
		// 🚨 生产上还会再问一次（explainReadingBlock），这里量的是第一次就对的比例。
		c := lookupCardFor(got, w)
		if c == nil {
			wrong++
			t.Logf("sample %d: 她点的是 %q，卡片讲的是 %q", i, w, got[0].Term)
			continue
		}
		t.Logf("sample %d %s | %v | %s（%s）%s — %s | %s", i, r.ModelID, time.Since(start).Round(time.Millisecond),
			c.Term, c.Pos, c.Meaning, c.Note, c.Example)
	}
	t.Logf("RESULT: 作废 %d/%d · 讲错了词 %d/%d", failed, len(words), wrong, len(words))
	if wrong*2 > len(words) {
		t.Errorf("%d/%d 次讲的不是她点的那个词", wrong, len(words))
	}
	if failed > 1 {
		t.Errorf("%d/%d 次查词作废 —— 她点一个词会拿到「AI 响应错误」", failed, len(words))
	}
}
