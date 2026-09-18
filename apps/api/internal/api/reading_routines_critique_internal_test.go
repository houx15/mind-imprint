package api

import (
	"strings"
	"testing"
)

// 🚨 2026-09-17：读法库里一套带 lens 的都不该剩下。
//
// 产品负责人逐字：「at this stage, I think we can skip the 透镜 part. it is
// really not applicable in many papers. and difficult for students to
// understand. the above mentioned critical thinking can be a better
// replacement of lens.」
//
// 整套透镜的机器没删（哪天再排上它，原样就能用），但**排读法不再排它**。
// 这一条和 reading_coach.go 里的 planHasLens 是同一件事的两端：这里管的是
// 清单里长不出那一步，那里管的是清单里没有那一步就一副都不给。
func TestNoRoutineStillAsksForALens(t *testing.T) {
	for _, r := range readingRoutines {
		for _, s := range r.Steps {
			if s.Kind == taskLens {
				t.Errorf("读法 %s 还在排透镜那一步", r.Key)
			}
		}
	}
}

// 拆论证和你怎么看是**两件事**，必须分开走 —— 产品负责人 2026-09-17：
// 「split them. first is analyze what author written. then is students' self
// critical thinking.」
//
// 所以：有标注板那一步的读法，后面必须跟着一步 critique；而且两者一篇各只有
// 一次（那就是 owner 说的「only one such practice in one paper」）。
func TestLabelIsFollowedByCritique(t *testing.T) {
	for _, r := range readingRoutines {
		labels, critiques, labelAt, critiqueAt := 0, 0, -1, -1
		for i, s := range r.Steps {
			switch s.Kind {
			case taskLabel:
				labels++
				labelAt = i
			case taskCritique:
				critiques++
				critiqueAt = i
			}
		}
		if labels > 1 || critiques > 1 {
			t.Errorf("读法 %s: 标注 %d 步、你怎么看 %d 步 —— 一篇只做一次", r.Key, labels, critiques)
		}
		if labels == 1 && critiques == 0 {
			t.Errorf("读法 %s 拆了作者的论证却没请她自己判断一次", r.Key)
		}
		if labels == 1 && critiques == 1 && critiqueAt < labelAt {
			t.Errorf("读法 %s 把「你怎么看」排在了拆论证前面 —— 她还没看清作者写了什么", r.Key)
		}
	}
}

// 格子闭表的两条不变量。
func TestArgueBinsAreClosedAndSmall(t *testing.T) {
	// 2026-09-18：论证三要素（同事拿《敬业与乐业》测出来两格装不下提问、让步这类句子）。
	if strings.Join(coachArgueBinsBasic, "/") != "论点/论据/论证" {
		t.Errorf("基础那一套应该是 论点 / 论据 / 论证，拿到 %v", coachArgueBinsBasic)
	}
	if len(coachArgueBinsCounter) != 4 {
		t.Errorf("驳论那一套应该是四格，拿到 %v", coachArgueBinsCounter)
	}
	// 模型只能说用哪一套，说别的一律按基础那套办。
	for _, bad := range []string{"", "五个", "custom", "因果链"} {
		got := coachBinSetFor(bad)
		if len(got) != len(coachArgueBinsBasic) {
			t.Errorf("coachBinSetFor(%q) 没有退回基础那一套：%v", bad, got)
		}
	}
	if len(coachBinSetFor("counter")) != 4 || len(coachBinSetFor("COUNTER")) != 4 {
		t.Error("counter 那一套没认出来")
	}
}

// 🚨 换下来的那五个仍然要认得出：她三天前摆过的那块板还在转写里，
// lastBoardPlacement 靠 isRoleLabel 把它读回来。
func TestOldBinNamesAreStillRecognised(t *testing.T) {
	for _, l := range []string{"主张", "证据", "限制", "背景", "对比", "关键主张", "作者观点", "驳斥观点"} {
		if !isRoleLabel(l) {
			t.Errorf("%q 认不出来了 —— 老转写里那块板会变成一堆读不出来的行", l)
		}
	}
	if isRoleLabel("进不去") {
		t.Error("编出来的格子名不该认")
	}
}

// labelPromptInventsBins 2026-09-17 重写过：原来取斜杠两边各两个字，
// 而新名字有两字也有四字。
func TestInventedBinsDetectorHandlesLongNames(t *testing.T) {
	fine := []string{
		"分析下列句子，判断它们各自属于哪一类论证成分。",
		"把它们分成关键主张/证据两类。",
		"哪几句是作者观点/驳斥观点？",
	}
	for _, p := range fine {
		if labelPromptInventsBins(p) {
			t.Errorf("一句正常的题目被判成编格子名：%q", p)
		}
	}
	invented := []string{
		"放进进不去/动不了/快撑不住了。",
		"按「原因 / 结果」两格分。",
	}
	for _, p := range invented {
		if !labelPromptInventsBins(p) {
			t.Errorf("它自己编了一套格子名，没抓到：%q", p)
		}
	}
}
