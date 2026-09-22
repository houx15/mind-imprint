package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 陪练上文里那两条「接地」规矩，什么时候在、什么时候不在。
//
// 🚨 这一条测的是「提示词里写的东西真的进了提示词」—— 2026-09-11 已经栽过一次：
// 提示词写着「method 从【可用的方法】里挑」，而那张表**从来没进过 prompt**，
// 单元测试全绿、真模型一直在造 id。规矩写了没送到，和没写是一回事。
// 见 [[prompt-output-must-be-verifiable-2026-09-03]]。

func snippet(pos int32, text string) sqlc.WritingSnippet {
	return sqlc.WritingSnippet{Position: pos, Text: text}
}

func TestWritingCoachProjection_GroundingRulesArriveOnceSheHasWritten(t *testing.T) {
	wr := sqlc.Writing{Title: "食堂浪费", Lang: "zh"}
	got := buildWritingCoachProjection(wr, nil, []sqlc.WritingSnippet{
		snippet(0, "上周五我数了一下，六个桶是满的。"),
	}, "")

	for _, want := range []string{"逐字引用对应原文", "暂时无法取得", "给出一项针对原文的修改建议", "以这里为准"} {
		if !strings.Contains(got, want) {
			t.Errorf("她已经写了东西，上文里却没有 %q：\n%s", want, got)
		}
	}
	// 她的原话得在上文里 —— 不然「引一句」这条规矩没有可引的东西。
	if !strings.Contains(got, "六个桶是满的") {
		t.Error("她写的字没进上文")
	}
}

// 一张白纸上没有句子可引，也没有「她已经写过了」可言 —— 这两条不该出现，
// 白占提示词的地方（一轮只能做一件事，见 pbl-refeed-one-produce-slot）。
func TestWritingCoachProjection_NoGroundingRulesBeforeSheWrites(t *testing.T) {
	wr := sqlc.Writing{Title: "食堂浪费", Lang: "zh"}
	for _, snips := range [][]sqlc.WritingSnippet{
		nil,
		{snippet(0, "   ")}, // 只有空白，等于没写
	} {
		got := buildWritingCoachProjection(wr, nil, snips, "")
		if strings.Contains(got, "逐字引用对应原文") {
			t.Errorf("她还没写，不该加这两条：\n%s", got)
		}
	}
}

// 🚨 **成稿开了之后，她的正文就是成稿那一页，不再是段落那几块。**
//
// 2026-09-12 第二十七轮走查里她自己把这条诊断出来了，而且诊断得准：
//
//	「印记反复让我改一句我正文里根本没有的话（它在读下面那段只读的旧文本），
//	  导致对话死循环」
//
// 她在成稿里改的是 writing_draft.body；段落那几块停在她进成稿之前的样子
//（屏幕上那一栏本来就标着「只读、不会跟着上面变」）。而这个上文一直只喂
// snippets —— 陪练读的确实是旧的那份。
//
// 两份她的文字同时摆进上文，正是让它挑错一份的原因。所以有成稿就只给成稿。
func TestWritingCoachProjection_DraftWinsOverTheFrozenSnippets(t *testing.T) {
	wr := sqlc.Writing{Title: "食堂浪费", Lang: "zh", Stage: "draft"}
	stale := []sqlc.WritingSnippet{snippet(0, "我看见很多人倒饭，这个现象很严重。")}
	current := "上周五我在收餐台数了二十分钟，四十多个人把饭倒进桶里。"

	got := buildWritingCoachProjection(wr, nil, stale, current)

	if !strings.Contains(got, "四十多个人") {
		t.Fatalf("成稿那一版没进上文：\n%s", got)
	}
	if strings.Contains(got, "这个现象很严重") {
		t.Errorf("段落那份旧的还在上文里 —— 陪练会照着它提意见：\n%s", got)
	}
	if !strings.Contains(got, "以这里为准") {
		t.Error("没说清哪一份算数")
	}
	// 🚨 那三条规矩在成稿这一支上同样要有。成稿这一支是后加的、而且提前
	// return，差一点就把它们漏在另一支里 —— 上面那条测试当时用的是空成稿，
	// 绿着也发现不了。
	for _, want := range []string{"逐字引用对应原文", "暂时无法取得", "给出一项针对原文的修改建议"} {
		if !strings.Contains(got, want) {
			t.Errorf("成稿这一支上少了 %q", want)
		}
	}
}

// 还没进成稿（或成稿页还没落过字）的时候，照旧以段落为准。
func TestWritingCoachProjection_FallsBackToSnippetsWithoutADraft(t *testing.T) {
	wr := sqlc.Writing{Title: "食堂浪费", Lang: "zh", Stage: "snippets"}
	snips := []sqlc.WritingSnippet{snippet(0, "上周五我数了一下，六个桶是满的。")}
	for _, body := range []string{"", "   "} {
		got := buildWritingCoachProjection(wr, nil, snips, body)
		if !strings.Contains(got, "六个桶是满的") {
			t.Errorf("没有成稿时该以段落为准（body=%q）：\n%s", body, got)
		}
	}
}
