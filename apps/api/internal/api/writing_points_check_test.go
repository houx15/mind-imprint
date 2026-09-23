package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func kindRow(kind, text string) sqlc.WritingOutline {
	return sqlc.WritingOutline{Kind: kind, Text: text, Depth: writingKindDepth(kind)}
}

// 讲义（四）的「扣得住」：分论点里要嵌进中心论点的关键词。
//
// 🚨 这条判据的方向是故意的：**拿不准就当扣得住**。判错的代价是告诉她一条
// 她写对了的分论点跑题了 —— 她会去改一句本来没问题的话，比不说更糟。
func TestWritingPointsOffThesis(t *testing.T) {
	zh := sqlc.Writing{Lang: "zh"}

	for _, tc := range []struct {
		name string
		rows []sqlc.WritingOutline
		want []string
	}{
		{
			name: "扣得住的不报",
			rows: []sqlc.WritingOutline{
				kindRow(writingKindThesis, "读书要读慢"),
				kindRow(writingKindPoint, "慢读才能发现问题"),
				kindRow(writingKindPoint, "读得慢才看得出作者的立场"),
			},
			want: nil,
		},
		{
			name: "整条跑题的报出来",
			rows: []sqlc.WritingOutline{
				kindRow(writingKindThesis, "读书要读慢"),
				kindRow(writingKindPoint, "慢读才能发现问题"),
				kindRow(writingKindPoint, "人应该多运动"),
			},
			want: []string{"人应该多运动"},
		},
		{
			// 🚨 时机：她只有一条分论点的时候不提这件事。讲义里「扣得住」是
			// 分论点都摆出来之后回头检查的一条。2026-09-21 的 LIVE_LLM 实测
			// 撞上过这一下，那一轮陪练不去帮她想，改成请她重新措辞。
			name: "只有一条分论点 —— 她还在想，不提措辞",
			rows: []sqlc.WritingOutline{
				kindRow(writingKindThesis, "上学时间该往后推一小时"),
				kindRow(writingKindPoint, "青少年生物钟本来就晚"),
			},
			want: nil,
		},
		{
			name: "两条之后才查 —— 这时候跑题的那条报出来",
			rows: []sqlc.WritingOutline{
				kindRow(writingKindThesis, "上学时间该往后推一小时"),
				kindRow(writingKindPoint, "青少年生物钟本来就晚"),
				kindRow(writingKindPoint, "上学时间往后推，第一节课的效率会高"),
			},
			want: []string{"青少年生物钟本来就晚"},
		},
		{
			name: "还没有中心论点 —— 扣不扣得住无从谈起",
			rows: []sqlc.WritingOutline{
				kindRow(writingKindPoint, "人应该多运动"),
			},
			want: nil,
		},
		{
			name: "空白的卡不算跑题",
			rows: []sqlc.WritingOutline{
				kindRow(writingKindThesis, "读书要读慢"),
				kindRow(writingKindPoint, "慢读才能发现问题"),
				kindRow(writingKindPoint, "   "),
			},
			want: nil,
		},
		{
			// 🚨 虚词不算共用。不挡这一下的话，「的」「是」会让每一条都判成
			// 扣得住，这条判据永远不响。
			name: "只共用虚词不算扣住",
			rows: []sqlc.WritingOutline{
				kindRow(writingKindThesis, "读书要读慢"),
				kindRow(writingKindPoint, "慢读才能发现问题"),
				kindRow(writingKindPoint, "运动是有好处的"),
			},
			want: []string{"运动是有好处的"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := writingPointsOffThesis(zh, tc.rows)
			if len(got) != len(tc.want) {
				t.Fatalf("得到 %v，want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("第 %d 条：得到 %q，want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// 🚨 英文作文不走这条判据。冠词、同义替换、代词回指都会让共用词消失，
// 判出来的「跑题」全是假的。产品负责人也说了英文要另找托福雅思的路子。
func TestWritingPointsOffThesis_SkipsEnglish(t *testing.T) {
	rows := []sqlc.WritingOutline{
		kindRow(writingKindThesis, "Schools should ban phones"),
		kindRow(writingKindPoint, "Students sleep better without them"),
	}
	if got := writingPointsOffThesis(sqlc.Writing{Lang: "en"}, rows); got != nil {
		t.Errorf("英文那一篇不该走这条判据，得到 %v", got)
	}
}

// 🚨 数出来是空的就一个字都不加。
//
// 2026-09-05 的教训：这类提示做成常驻，模型会一直去处理那条提示、
// 把该做的事挤掉（那次是六轮里一直在补一张卡）。
func TestPointsCheckBlockIsSilentWhenNothingIsWrong(t *testing.T) {
	zh := sqlc.Writing{Lang: "zh"}
	ok := []sqlc.WritingOutline{
		kindRow(writingKindThesis, "读书要读慢"),
		kindRow(writingKindPoint, "慢读才能发现问题"),
	}
	if got := writingPointsCheckBlock(zh, ok); got != "" {
		t.Errorf("全都扣得住的时候不该加任何字，得到：%q", got)
	}

	bad := append(append([]sqlc.WritingOutline{}, ok...),
		kindRow(writingKindPoint, "慢一点读才记得住"),
		kindRow(writingKindPoint, "人应该多运动"))
	block := writingPointsCheckBlock(zh, bad)
	if !strings.Contains(block, "人应该多运动") {
		t.Errorf("跑题那句她写的原话没出现在提示里：%q", block)
	}
	// 说她写的那句话，不说「第二条分论点」—— 后者她还要自己去数。
	if strings.Contains(block, "第二条") {
		t.Errorf("用序号指她写的东西，她得自己去数：%q", block)
	}
}

// 四个角度只在她还差分论点的时候摆。够了之后再教她怎么想，
// 是在她已经想好之后教她怎么想 —— 那一轮该做的是别的事。
//
// 🚨 2026-09-23 改过：这条测试原来断言「够了之后这一栏是空的」，理由就是上面
// 那句「那一轮该做的是别的事」—— 可它从来没说过别的事是什么，于是那一栏在
// 分论点够了之后**整轮沉默**，而结尾从头到尾没有一处请模型谈它。
// 产品负责人第 7 条报的就是这个后果（「some of them should be, e.g. ending」）。
// 现在它接力到结尾那一栏；原来那条守的东西（够了就不再摆四个角度）没有松。
func TestPointAnglesBlockOnlyWhenSheStillNeedsOne(t *testing.T) {
	zh := sqlc.Writing{Lang: "zh"}
	rows := []sqlc.WritingOutline{
		kindRow(writingKindThesis, "读书要读慢"),
		kindRow(writingKindPoint, "慢读才能发现问题"),
		kindRow(writingKindPoint, "读得慢才看得出作者的立场"),
	}
	// 两条分论点、要的也是两条 —— 够了，四个角度一个都不许再摆。
	got := writingPointAnglesBlock(zh, rows, 2)
	for _, angle := range []string{"是什么", "为什么", "怎么办", "会怎样"} {
		if strings.Contains(got, angle) {
			t.Errorf("分论点已经够了，不该再摆「可以从哪几个角度想」：%q", got)
		}
	}
	// 接力到下一件该谈的事：结尾。
	if !strings.Contains(got, "结尾") {
		t.Errorf("够了之后这一栏该点名下一件事，而不是沉默：%q", got)
	}
	if got := writingPointAnglesBlock(zh, rows, 3); got == "" {
		t.Error("还差一条分论点，四个角度该摆出来")
	} else {
		for _, want := range []string{"是什么", "为什么", "怎么办", "会怎样"} {
			if !strings.Contains(got, want) {
				t.Errorf("四个角度里缺 %q", want)
			}
		}
		// 🚨 一次一个问题（AGENTS.md 铁律③）。
		if !strings.Contains(got, "选择一个角度") {
			t.Error("没写「一次只问一个问题」—— 四个角度摆出来很容易变成一次问四件事")
		}
	}
	if got := writingPointAnglesBlock(sqlc.Writing{Lang: "en"}, rows, 3); got != "" {
		t.Errorf("英文那一篇不摆这个，得到：%q", got)
	}
}
