package api

// reading_block_grammar_live_test.go —— 语法卡的实测。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveGrammarCard -v -count=1
//
// 语法卡的判据是「每一块逐字来自那一句」，核不上的丢掉，不足两块整张作废。
// 单元测试只证明解析器读得懂我写的 JSON；这里量的是真模型有多少块真的抄对了 ——
// 抄不对的时候她拿到的是「AI 响应错误」，不是一张少了几块的卡。

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
)

func TestLiveGrammarCard(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassDialogue)
	blocks := SplitBlocks(strings.Join([]string{
		"The analysis also revealed two smaller, fuzzy body feathers that were more primitive in nature. These feathers, which likely helped insulate the birds from the cold, might be the key to why hesperornithiforms did not survive the mass extinction event.",
		"Exactly how the Chicxulub impact some 66 million years ago triggered so many deaths worldwide remains the subject of scientific debate.",
		"During this prolonged period of cold and darkness, plants were unable to photosynthesize, and many animals starved to death. Scientists who have studied the fossils believe that the birds had already lost the ability to fly.",
	}, "\n\n"))
	sentences := []string{
		"These feathers, which likely helped insulate the birds from the cold, might be the key to why hesperornithiforms did not survive the mass extinction event.",
		"Exactly how the Chicxulub impact some 66 million years ago triggered so many deaths worldwide remains the subject of scientific debate.",
		// 并列句：两个分句各有主谓，连词不是成分（线上实测标错过）。
		"During this prolonged period of cold and darkness, plants were unable to photosynthesize, and many animals starved to death.",
		// 从句里的过去完成时要进时态（线上实测漏过）。
		"Scientists who have studied the fossils believe that the birds had already lost the ability to fly.",
	}
	blockOf := []int{0, 1, 2, 2}
	var tool readingBlockTool
	for _, tt := range readingBlockTools {
		if tt.ID == "grammar" {
			tool = tt
		}
	}
	failed, parts, dropped, noWords, noMain, noPerfect, badPredicate := 0, 0, 0, 0, 0, 0, 0
	for i := 0; i < 8; i++ {
		si := i % 4
		idx := blockOf[si]
		sentence := sentences[si]
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: readingBlockSystemFor(tool)},
				{Role: gateway.RoleUser, Content: buildReadingBlockPrompt("恐龙粪便里的羽毛", blocks, idx, sentence)},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("sample %d: %v", i, err)
		}
			var raw readingGrammar
		_ = json.Unmarshal([]byte(sliceBlockJSON(res.Text)), &raw)
		g, ok := parseGrammarCard(sliceBlockJSON(res.Text), sentence)
		if !ok {
			failed++
			t.Logf("sample %d: 整张作废\n%s", i, res.Text)
			continue
		}
		parts += len(g.Parts)
		kept := len(g.Clauses) + len(g.Parts) + len(g.Words) + len(g.Tenses)
		sent := len(raw.Clauses) + len(raw.Parts) + len(raw.Words) + len(raw.Tenses)
		dropped += sent - kept
		if len(g.Words) == 0 {
			noWords++
		}
		// 🚨 主句由界面反推：从句要是把整句盖满了，「主从句」那一层就没有主句。
		rest := sentence
		for _, c := range g.Clauses {
			rest = strings.Replace(rest, c.Text, "", 1)
		}
		if len(strings.Fields(strings.Trim(rest, " ,.;"))) < 2 {
			noMain++
			t.Logf("sample %d: 从句之外剩不下主句：%q", i, rest)
		}
		if si == 3 {
			found := false
			for _, tn := range g.Tenses {
				found = found || strings.Contains(tn.Text, "had")
			}
			if !found {
				noPerfect++
			}
		}
		for _, p := range g.Parts {
			if p.Label == "谓语" && strings.Contains(p.Text, "animals") {
				badPredicate++
			}
		}
		t.Logf("sample %d ok — 从句=%d 成分=%d 词法=%d 时态=%d (丢 %d)", i, len(g.Clauses), len(g.Parts), len(g.Words), len(g.Tenses), sent-kept)
		for name, layer := range map[string][]readingGrammarSpan{"从句": g.Clauses, "成分": g.Parts, "词法": g.Words, "时态": g.Tenses} {
			for _, p := range layer {
				t.Logf("    %s [%s] %s — %s %s", name, p.Label, p.Text, p.Note, p.Example)
			}
		}
		t.Logf("    句意：%s", g.Meaning)
	}
	t.Logf("RESULT: 整张作废 %d/8 · 成分 %d 块 · 丢掉 %d 段 · 没有词法 %d · 没有主句 %d · 漏过去完成时 %d · 谓语带主语 %d", failed, parts, dropped, noWords, noMain, noPerfect, badPredicate)
	if failed > 1 {
		t.Errorf("%d/8 张语法卡整张作废 —— 她点语法会拿到「AI 响应错误」", failed)
	}
	if noMain > 0 || noWords > 1 {
		t.Errorf("没有主句 %d 张、没有词法 %d 张", noMain, noWords)
	}
}
