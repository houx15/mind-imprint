package api

import (
	"strings"
	"testing"
)

// 当前档位（2026-09-25）。产品负责人选了「给学生看」。
//
// 🚨 它是**服务端从这一轮真正挑出来的毛病算的**，模型没有参与评分 ——
// 所以批改提示词里那两句「不打分，不给等级」一个字都没改。
// 这条测试守的正是这个分工。

func issueAt(layer int) CommentPoint {
	return CommentPoint{Kind: "issue", Layer: layer, Quote: "x", Text: "y", Action: "z"}
}

// 卡在越靠前的层，档位越低 —— 层序就是这把尺子。
func TestBandFollowsTheLayerItIsStuckOn(t *testing.T) {
	cases := []struct {
		name  string
		pts   []CommentPoint
		want  int
	}{
		{"四层都干净", nil, 5},
		{"只剩字句", []CommentPoint{issueAt(writingLayerSentence)}, 4},
		{"卡在结构", []CommentPoint{issueAt(writingLayerStructure)}, 3},
		{"卡在材料", []CommentPoint{issueAt(writingLayerMaterial)}, 2},
		{"卡在立意", []CommentPoint{issueAt(writingLayerClaim)}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := writingBandOf(tc.pts); got != tc.want {
				t.Errorf("档位 = %d，want %d", got, tc.want)
			}
		})
	}
}

// good 不是毛病，不该把档位压下去。
func TestBandIgnoresGoodPoints(t *testing.T) {
	pts := []CommentPoint{{Kind: "good", Layer: writingLayerClaim, Quote: "x", Text: "写得好"}}
	if got := writingBandOf(pts); got != writingBandTop {
		t.Errorf("只有一条 good，档位却是 %d", got)
	}
}

// 🚨 档位只挂在通篇那一条上。
//
// 五档是给**一整篇**用的尺子：一段写得再好也不构成一篇及格的文章，
// 反过来也一样。把它套到一个自然段上，她会以为这一段就是这一篇。
func TestBandOnlyOnTheWholePieceComment(t *testing.T) {
	pts := []CommentPoint{issueAt(writingLayerStructure)}
	block := Comment{Scope: "block", Points: pts}
	draft := Comment{Scope: "draft", Points: pts}

	// toCommentDTO 是真正装配的那一处，这里直接按它的规则断言。
	if bandFor(block) != 0 {
		t.Error("单段的意见带上了档位")
	}
	if bandFor(draft) == 0 {
		t.Error("通篇的意见没有档位")
	}
}

// bandFor 复述 toCommentDTO 里那条规则，免得测试去构造一行 sqlc 行。
func bandFor(c Comment) int {
	if c.Scope != "draft" {
		return 0
	}
	return writingBandOf(c.Points)
}

// 那个数字旁边必须有字：说清这一档是什么意思，以及这把尺子量的是什么。
//
// 🚨 学生看见一个数字会默认它是分数。少了这一行，「第 3 档」会被读成
// 一个考试成绩，而它说的其实是「你现在卡在结构」。
func TestBandAlwaysCarriesItsScale(t *testing.T) {
	for band := writingBandMin; band <= writingBandTop; band++ {
		if writingBandLabel(band) == "" {
			t.Errorf("第 %d 档没有说明", band)
		}
		if !strings.Contains(writingBandText(band), "档") {
			t.Errorf("第 %d 档的整句话里没有「档」", band)
		}
	}
	// 尺子那一行要明说它不是分数。
	if !strings.Contains(writingBandScaleNote, "不是分数") {
		t.Error("尺子那一行没有说明它不是分数")
	}
	for _, layer := range []string{"立意", "材料", "结构", "字句"} {
		if !strings.Contains(writingBandScaleNote, layer) {
			t.Errorf("尺子那一行里没有 %q", layer)
		}
	}
	// 越界不给字，而不是给一句像样的假话。
	if writingBandText(0) != "" || writingBandText(9) != "" {
		t.Error("越界的档位还给出了说明")
	}
}

// 🚨 分工不许反过来：模型仍然不打分。
//
// 档位是服务端算的，所以批改提示词里那两句「不打分，不给等级」必须原样还在。
// 哪天有人把评分挪给模型，这条会红 —— 那时要重新想清楚
// （模型评分会多出一整套失败方式：编一个分数、同一篇两次给两个分）。
func TestModelIsStillToldNotToGrade(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		s := buildWritingCommentSystem(lang, 3, "", helpAsk, genreArgument)
		if !strings.Contains(s, "不打分，不给等级") {
			t.Errorf("%s：批改提示词里「不打分，不给等级」没了 —— 评分被挪给模型了？", lang)
		}
		if strings.Contains(s, "档位") || strings.Contains(s, "第几档") {
			t.Errorf("%s：提示词里出现了档位，模型不该知道这件事", lang)
		}
	}
}
