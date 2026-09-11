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
	})

	for _, want := range []string{"逐字引出来", "取不到", "一句祈使收尾"} {
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
		got := buildWritingCoachProjection(wr, nil, snips)
		if strings.Contains(got, "逐字引出来") {
			t.Errorf("她还没写，不该加这两条：\n%s", got)
		}
	}
}
