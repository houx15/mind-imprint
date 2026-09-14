package api

import (
	"testing"

	"github.com/google/uuid"
)

// 线上真的收到过的那一种坏回复：**写到一半没了，而 stop_reason 说 "stop"**。
//
// 2026-09-11 第四轮走查，两次都是这个形状：
//
//	stop_reason=stop reply_len=2231 reply_tail="…你想让读<半个字>"
//	stop_reason=stop reply_len=4693 reply_tail="…周围是什么声音和<半个字>"
//
// 前面几块是完整的，断点在后面某一块的 questions 里。整批丢掉的话，
// 她那一屏每一块都是空的，外加一个「后台错误」的弹窗 —— 而模型明明已经
// 把前几块写完了，那几块的钱也付过了。
func TestParseWritingGuideBatch_KeepsTheBlocksThatArrivedWhole(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	known := map[uuid.UUID]bool{id1: true, id2: true, id3: true}

	// 两块完整，第三块断在 questions 中间（末尾连引号都没有）。
	truncated := `{"blocks":[` +
		`{"id":"` + id1.String() + `","job":"把浪费和不在意这两件事拧成一句话。",` +
		`"method_ids":["opening_direct"],"questions":["你最想让读者记住哪一件？","那一天你看见的是什么？"]},` +
		`{"id":"` + id2.String() + `","job":"用你自己数过的那一次把它坐实。",` +
		`"method_ids":["point_progressive"],"questions":["那天几点？","桶里装的是什么？"]},` +
		`{"id":"` + id3.String() + `","job":"收尾要回到开头那句话。",` +
		`"method_ids":["closing_return"],"questions":["先写收残食一幕再写李明的话，你想让读`

	out, ok := parseWritingGuideBatch(truncated, known)
	if !ok {
		t.Fatal("前两块是完整的，整批被丢掉了 —— 她那一屏会全空，外加一个后台错误弹窗")
	}
	if _, has := out[id1]; !has {
		t.Error("第一块完整到齐，却没留下")
	}
	if _, has := out[id2]; !has {
		t.Error("第二块完整到齐，却没留下")
	}
	// 🚨 断掉的那一块宁可不要：半句引导比没有引导更糟，她会照着半句去写。
	if _, has := out[id3]; has {
		t.Error("断在半路的那一块不该留下")
	}
}

// 第一块就断了 —— 一条都捞不到才是对的，该改的是提示词不是救援。
func TestParseWritingGuideBatch_FirstBlockBrokenSalvagesNothing(t *testing.T) {
	id1 := uuid.New()
	known := map[uuid.UUID]bool{id1: true}
	if _, ok := parseWritingGuideBatch(`{"blocks":[{"id":"`+id1.String()+`","job":"把浪费`, known); ok {
		t.Fatal("第一块就断了，不该报告成功")
	}
}

// 线上第二种坏法，**而且这一种才是真的**：回复写完了，但写坏了。
//
// 2026-09-11 第四轮走查抓到的原话，结尾是这样的：
//
//	…"这一段写到最后，你想让泔水桶落在「严重」那边还是「不当回事」那边收尾？"}]}
//
// 该收 `]` 的地方收了 `}` —— 和 2026-09-08 那次一模一样的形状
// （[[model-json-half-arrived-2026-09-08]]）。stop_reason 是 "stop"，
// reply_len 4693，一个字都没少。**「写完了」和「写对了」是两件事。**
//
// 🚨 一开始我以为是流式少送了最后一块（reply_tail 看着像断在半个字上），
// 那是我自己 `cut -c1-900` 切的。查一个坏掉的回复，得先确认自己没有
// 在看它的时候又切它一刀。
func TestParseWritingGuideBatch_LastBlockClosedWithTheWrongBracket(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	known := map[uuid.UUID]bool{id1: true, id2: true, id3: true}

	broken := `{"blocks":[` +
		`{"id":"` + id1.String() + `","job":"立住中心那句话。",` +
		`"method_ids":["opening_direct"],"questions":["你最想让读者记住哪一件？"]},` +
		`{"id":"` + id2.String() + `","job":"用你数过的那一次坐实它。",` +
		`"method_ids":["point_progressive"],"questions":["那天几点？"]},` +
		`{"id":"` + id3.String() + `","job":"收尾回到开头。",` +
		`"method_ids":["closing_return"],"questions":["落在哪边收尾？"}]}`

	out, ok := parseWritingGuideBatch(broken, known)
	if !ok {
		t.Fatal("前两块是好的，整批被丢掉了 —— 她那一屏会全空，外加一个后台错误弹窗")
	}
	if _, has := out[id1]; !has {
		t.Error("第一块是好的，却没留下")
	}
	if _, has := out[id2]; !has {
		t.Error("第二块是好的，却没留下")
	}
	if _, has := out[id3]; has {
		t.Error("括号收错了的那一块不该留下")
	}
}
