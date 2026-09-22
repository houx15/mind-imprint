package api

import (
	"encoding/json"
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 摘抄进报告（2026-09-22）。产品负责人：「these sentences will have some kind of
// underline and be recorded in 阅读成果 and revealed in report」。

func ann(quote, note string) sqlc.AtomAnnotation {
	return sqlc.AtomAnnotation{Quote: quote, Note: note}
}

func TestReadingExcerptsAndNotesDoNotOverlap(t *testing.T) {
	rows := []sqlc.AtomAnnotation{
		ann("Some professors fear that ChatGPT could lead to cheating.", ""),
		ann("ChatGPT is a software system.", "这句是在下定义"),
		ann("  ", ""), // 空引文，两节都不该收
		ann("Not all people are like this.", ""),
	}

	excerpts := buildReadingExcerpts(rows)
	notes := buildReadingNotes(rows)

	// 摘抄 = 她一个字没写的那些。写了字的那条归「我的笔记」。
	if len(excerpts) != 2 {
		t.Fatalf("excerpts = %v, want the two she left without a note", excerpts)
	}
	if len(notes) != 1 || notes[0].Note != "这句是在下定义" {
		t.Fatalf("notes = %+v, want only the annotated one", notes)
	}
	// 🚨 同一行绝不能两节都出现 —— 报告上同一句引两遍读起来像我们数错了。
	for _, e := range excerpts {
		for _, n := range notes {
			if e == strings.TrimSpace(n.Quote) {
				t.Fatalf("%q landed in BOTH 我的摘抄 and 我的笔记", e)
			}
		}
	}
}

func TestReadingExcerptsDropDuplicateQuotes(t *testing.T) {
	// 她在同一句上划了两次（偏移不同、原句一样）。报告上只该有一行。
	got := buildReadingExcerpts([]sqlc.AtomAnnotation{
		ann("解决问题比记忆更费能量。", ""),
		ann("  解决问题比记忆更费能量。 ", ""),
	})
	if len(got) != 1 {
		t.Fatalf("excerpts = %v, want one row for the same sentence", got)
	}
}

func TestReadingExcerptsCappedLikeNotes(t *testing.T) {
	rows := make([]sqlc.AtomAnnotation, 0, maxReportNotes+5)
	for i := 0; i < maxReportNotes+5; i++ {
		rows = append(rows, ann(strings.Repeat("句", i+1), ""))
	}
	if got := len(buildReadingExcerpts(rows)); got != maxReportNotes {
		t.Fatalf("excerpts = %d, want the %d cap (报告是一张海报，不是转写)", got, maxReportNotes)
	}
}

// 🚨 按**前端真实读的那个名字**取，并且断言没有 null。
// [[go-nil-slice-becomes-null-2026-09-19]]：缺 json 标签 ⇒ 字段名全错而 Go 测试
// 全绿；nil 切片 marshal 成 `null` ⇒ 前端 `.map()` 崩，只在「上一趟有、这一趟
// 没有」时出现。
func TestReadingReportExcerptsMarshalUnderTheNameTheClientReads(t *testing.T) {
	dto := liteReportDTO{
		Version: 1, Kind: "reading",
		Excerpts: buildReadingExcerpts([]sqlc.AtomAnnotation{ann("她圈下的那一句。", "")}),
	}
	b, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := back["excerpts"].([]any)
	if !ok {
		t.Fatalf(`no "excerpts" key the client can read; got %s`, b)
	}
	if len(got) != 1 || got[0] != "她圈下的那一句。" {
		t.Fatalf("excerpts = %v", got)
	}
	// 🚨 判的是 **excerpts 这一格**，不是整份 JSON 里有没有 null。
	// 第一版写的是后者，于是它逮住了 `"stats":null` —— 那是这个空壳夹具自己的
	// 事，和摘抄无关。判据要盯住真失败，不是它的影子
	// （[[detector-must-target-the-real-failure]]）。
	if raw, hit := back["excerpts"]; hit && raw == nil {
		t.Fatalf("excerpts marshalled as null; the client would crash on .map(): %s", b)
	}

	// 一句都没摘的那一篇：omitempty ⇒ 整个键缺席（不是 `null`），
	// 客户端 normalizer 的 `?? []` 接得住，而旧报告的形状原样不动。
	empty, err := json.Marshal(liteReportDTO{Version: 1, Kind: "reading"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(empty), "excerpts") {
		t.Fatalf("an empty 我的摘抄 must be ABSENT, not an empty array: %s", empty)
	}
}
