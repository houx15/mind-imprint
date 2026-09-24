package api

// writing_letter_en_live_test.go —— 英文书信那一档发给真模型跑一遍。
//
// 跑法：
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveEnglishLetter -v -count=1
//
// 🚨 为什么这条要用真模型：2026-09-24 英文书信换上了自己的篇章结构，
// 那一节比原来长了一千多个字（80 词上限、要点来自题干、称呼与结束语的对仗、
// 十种信的安排）。往一份提示词里加一千个字，真正的风险不是它教得对不对
// ——那几条离线测试钉得住——而是**模型从输出契约上漂走**：回的 kind 不在闭表里，
// 或者干脆回议论文那套节点。这两样离线一个都验不了
// （[[prompt-output-must-be-verifiable-2026-09-03]]：stub 只证明解析器读得懂
// 我自己写的 JSON）。
//
// 判据钉的是**真失败本身**，不是它的影子
// （[[detector-must-target-the-real-failure]]）：一封信被套上议论文的骨架，
// 在数据里的样子就是 thesis / point 这两个 kind 出现。这正是这一档存在的
// 理由 —— 产品负责人当初报的毛病原话是「a letter under the structure of
// 议论文」。措辞好不好不在这里判，那是判官的活。

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 2022 新课标 I 卷那一型：请外教来做一次访谈。
//
// 这一轮故意把她的计划停在「活动介绍」上 —— 时间地点都有了，就是没有真正
// 发出邀请。语料里记着这正是这一档最常见的致命失分：通篇在介绍活动，
// 最后没有一句「我想邀请您来」，而那一句正好是一档的分。
func TestLiveEnglishLetterStaysALetter(t *testing.T) {
	wr := sqlc.Writing{
		Title: "An invitation to our school's English Corner",
		Lang:  "en",
	}
	words := int32(80)
	wr.TargetWords = &words

	rows := []sqlc.WritingOutline{
		{Text: "The English Corner is held every Friday afternoon in the library",
			Kind: writingKindMatter, Role: "要点", Depth: 0, Position: 0},
	}
	said := "我想写给我们外教 Mr. Smith，告诉他英语角这周五下午在图书馆，" +
		"主题是校园生活，大概一个小时。"

	out := livePlanTurnFor(t, genreLetter, wr, rows, said)

	parsed, ok := parseWritingPlanReply(out)
	if !ok {
		t.Fatalf("真模型的回复读不出来：\n%s", out)
	}

	// 🚨 看它**原样回的**那一份：解析器会把非法的 kind 丢掉，
	// 所以「解析成功」证明不了模型守了规矩。
	var raw struct {
		Add []map[string]any `json:"add"`
	}
	if err := json.Unmarshal([]byte(stripWritingPlanFence(out)), &raw); err != nil {
		t.Fatalf("原始 JSON 读不出来：%v\n%s", err, out)
	}

	for i, node := range raw.Add {
		kind := strings.ToLower(strings.TrimSpace(asString(node["kind"])))
		if !writingKindValid(kind) {
			t.Errorf("第 %d 个节点的 kind 不在闭表里：%q（整份：%s）", i+1, kind, out)
			continue
		}
		// 这一条就是这一档存在的理由：一封信不许长出中心论点和分论点。
		if kind == writingKindThesis || kind == writingKindPoint {
			t.Errorf("第 %d 个节点是 %q —— 一封英文信被套上了议论文的骨架（整份：%s）",
				i+1, kind, out)
		}
	}

	t.Logf("reply=%q add=%d", parsed.Reply, len(parsed.Add))
	t.Logf("模型原样回的：\n%s", out)
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
