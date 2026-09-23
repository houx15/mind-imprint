package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 产品负责人 2026-09-23 第 7 条：
//
//	「our AI always puts all students' talked points as one 分论点, but actually
//	  some of them should be, e.g. ending, they would be different part of the
//	  article, or they may have some relationships, but currently, we seems to
//	  be mechanically drag everything students said to one 分论点, and the
//	  discussion is also not focused.」
//
// 🚨 她说的两件事在代码里是同一件。
//
// 【当前还需要补充分论点】那一栏在分论点不够的时候**每一轮**都在推「再来一条
// 分论点」；够了之后它**什么都不说**。而开始写作的条件里又写着「开头与结尾
// 尚未确定，不单独作为不能开始写作的理由」——
// 于是从头到尾没有一处请模型谈结尾，学生说的每一句都只能落成分论点。
//
// 这几条钉的是那一栏的**接力**：分论点够了就点名下一件该谈的事。

func planRows(kinds ...string) []sqlc.WritingOutline {
	rows := make([]sqlc.WritingOutline, 0, len(kinds))
	for i, k := range kinds {
		rows = append(rows, sqlc.WritingOutline{
			Kind: k, Text: "第" + string(rune('0'+i+1)) + "条", Position: int32(i),
		})
	}
	return rows
}

func zhWriting() sqlc.Writing { return sqlc.Writing{Lang: "zh"} }

func TestPointAnglesHandsOverToTheEndingOncePointsSuffice(t *testing.T) {
	// 分论点不够：照旧推分论点，不提结尾。
	short := writingPointAnglesBlock(zhWriting(), planRows("thesis", "point"), 3)
	if !strings.Contains(short, "还需要补充分论点") {
		t.Error("分论点不够时该继续推分论点")
	}
	if strings.Contains(short, "结尾") {
		t.Error("分论点还不够就提结尾 —— 这一栏一次只点一件事")
	}

	// 分论点够了、还没有结尾：换成谈结尾。
	enough := writingPointAnglesBlock(zhWriting(), planRows("thesis", "point", "point", "point"), 3)
	if strings.Contains(enough, "还需要补充分论点") {
		t.Error("分论点够了还在推分论点")
	}
	if !strings.Contains(enough, "结尾") {
		t.Error("🚨 分论点够了之后这一栏什么都不说 —— 结尾从头到尾没人请模型谈它")
	}
	// 🚨 最要紧的那一句：结尾不是又一条理由。第 7 条报的就是它。
	if !strings.Contains(enough, "不是又一条理由") {
		t.Error("没说清楚结尾不是一条分论点")
	}
	if !strings.Contains(enough, "closing") {
		t.Error("没告诉模型该用哪个 kind")
	}

	// 已经有结尾了：这一栏闭嘴。它的作用是点名下一件事，不是每轮提醒。
	done := writingPointAnglesBlock(zhWriting(), planRows("thesis", "point", "point", "point", "closing"), 3)
	if strings.TrimSpace(done) != "" {
		t.Errorf("图上已经有结尾了，这一栏还在说话：%s", done)
	}
}

// 空文本的结尾节点不算数 —— 和分论点那一头的判据一致（都要 TrimSpace 非空）。
func TestEmptyClosingNodeDoesNotCountAsAnEnding(t *testing.T) {
	rows := planRows("thesis", "point", "point", "point")
	rows = append(rows, sqlc.WritingOutline{Kind: writingKindClosing, Text: "   ", Position: 9})
	got := writingPointAnglesBlock(zhWriting(), rows, 3)
	if !strings.Contains(got, "结尾") {
		t.Error("一个空的结尾节点被当成已经有结尾了")
	}
}

// 英文那一篇这一整栏本来就不出（角度那四个词是语文课的框架）。
// 🚨 记下来不是说它对：英文写作者因此一条角度提示都拿不到。
// 这次不扩大范围，但别让它静默地变成「英文也谈结尾了」。
func TestEnglishPieceStillGetsNoAnglesBlock(t *testing.T) {
	en := sqlc.Writing{Lang: "en"}
	if got := writingPointAnglesBlock(en, planRows("thesis", "point", "point", "point"), 3); got != "" {
		t.Errorf("英文那一篇不该出这一栏：%s", got)
	}
}
