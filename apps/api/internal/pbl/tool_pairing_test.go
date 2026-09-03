package pbl

import (
	"strings"
	"testing"
)

// 🚨 有四件工具的界面本身没有内容，摆的是印记做出来的那份东西。
//
// 少了这张对应关系，prompt 里的目录就只是十个名字，模型看不出「理性决策」和
// 「观察日记」有什么不同——一个是审它做的东西的地方，另一个她自己就能动手。
func TestToolNeeds_ReviewSurfacesDeclareWhatTheyRead(t *testing.T) {
	want := map[string]string{
		"decide":    "decision",
		"structure": "structure",
		"split":     "substeps",
		"review":    "artifact",
	}
	for tool, kind := range want {
		if got := ToolNeeds(tool); got != kind {
			t.Errorf("%s 该读 %s，实际是 %q", tool, kind, got)
		}
		if !IsProduceKind(kind) {
			t.Errorf("%s 声明要 %q，但印记做不出这种东西", tool, kind)
		}
	}
	// 自带内容的那些不能有依赖，否则它们会被闸挡住，永远递不出去。
	for _, tool := range []string{"observe", "board", "reframe", "ideas", "lookback", "keep"} {
		if got := ToolNeeds(tool); got != "" {
			t.Errorf("%s 自己就有内容，不该声明依赖，实际是 %q", tool, got)
		}
	}
	// 表外的名字退回朴素卡片，没有依赖。
	if got := ToolNeeds("something-we-have-not-built"); got != "" {
		t.Errorf("表外的工具不该有依赖，实际是 %q", got)
	}
}

// 目录必须把这件事说给模型听。派生自 registry，所以加一件要配产出的工具，
// prompt 自动跟上——手写第二份目录一定会漂移。
func TestToolCatalogue_MarksTheToolsThatNeedContent(t *testing.T) {
	cat := toolCatalogue("")
	for tool, kind := range map[string]string{
		"decide": "decision", "structure": "structure",
		"split": "substeps", "review": "artifact",
	} {
		line := ""
		for _, l := range strings.Split(cat, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), tool+"（") {
				line = l
			}
		}
		if line == "" {
			t.Fatalf("目录里没有 %s：\n%s", tool, cat)
		}
		if !strings.Contains(line, kind) || !strings.Contains(line, "同一轮") {
			t.Errorf("%s 那一行没说清楚要同一轮配一个 %s：%s", tool, kind, line)
		}
	}
	// 观察日记那一行不该冒出这句话——它自己就有内容。
	for _, l := range strings.Split(cat, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "observe（") && strings.Contains(l, "同一轮") {
			t.Errorf("observe 自带内容，不该要求配产出：%s", l)
		}
	}
}

// 🚨 一轮里的上限是「只问一个问题」，不是「只做一件事」。
//
// 原来 prompt 写的是「一轮最多做一件。大多数轮次一件也不做」，读起来就是「这
// 一轮已经递过工具了，那就别再做东西」——于是工具和它要审的那份东西**在结构
// 上永远到不了同一轮**，三个界面因此常年是空的。产品负责人 2026-09-03：
// 「每次尽量只问学生一个问题，instead of one turn do one thing」。
func TestCoachSystem_LimitIsOneQuestionNotOneAction(t *testing.T) {
	if strings.Contains(coachSystem, "一轮最多做一件。") {
		t.Error("旧的「一轮最多做一件」还在——它会把工具和它的产出永远分到两轮")
	}
	if !strings.Contains(coachSystem, "只问他一个问题") {
		t.Error("没写清楚一轮只问一个问题（铁律③）")
	}
	// 两个问题连着抛，她只答后一个。这条要硬。
	if !strings.Contains(coachSystem, "只能出现一个问号") {
		t.Error("没有那条硬规则：reply 里只能有一个问号")
	}
	// 铁律③ 的另一半：真有多个要点就分点列出。
	if !strings.Contains(coachSystem, "分点列出") {
		t.Error("没给出「多个要点分点列出」这条出路（铁律③）")
	}
	// 工具和产出必须能同一轮一起给。
	if !strings.Contains(coachSystem, "tool 和\nproduce 一起给") &&
		!strings.Contains(coachSystem, "同一轮一起") {
		t.Error("没写清楚工具和它的产出要同一轮一起给")
	}
}

// 🚨 已经递过、她还没做的工具，和做完的一样不能再递。
//
// 2026-09-03 线上实测：她屏幕上并排两张「头脑风暴」，理由各写各的。印记不知道
// 第一张还挂着，因为 prompt 里的工具目录不带状态。
func TestBuildCoachContext_SaysWhichToolsAreAlreadyOnHerScreen(t *testing.T) {
	ctx := buildCoachContext(CoachInput{
		Idea:         "下课没人去操场",
		ToolsUsed:    []string{"观察日记"},
		ToolsOffered: []string{"头脑风暴"},
	})
	if !strings.Contains(ctx, "已经递过、她还没做的工具") ||
		!strings.Contains(ctx, "头脑风暴") {
		t.Fatalf("没告诉印记哪几张卡还挂在她屏幕上：\n%s", ctx)
	}
	if !strings.Contains(ctx, "不要再递一遍") {
		t.Fatalf("没说清楚挂着的那张不要再递：\n%s", ctx)
	}
	// 做完的那一段还在，两段互不干扰。
	if !strings.Contains(ctx, "已经做完的工具") || !strings.Contains(ctx, "观察日记") {
		t.Fatalf("做完的那一段丢了：\n%s", ctx)
	}
}

// 一张卡都没挂着的时候不该出现这一段——凭空多一句"这几张已经在她屏幕上"，
// 只会让印记不敢递工具。
func TestBuildCoachContext_NoOfferedSectionWhenHerScreenIsClear(t *testing.T) {
	ctx := buildCoachContext(CoachInput{Idea: "下课没人去操场"})
	if strings.Contains(ctx, "已经递过、她还没做的工具") {
		t.Fatalf("没有挂着的卡却出现了那一段：\n%s", ctx)
	}
}

// 🚨 递「观察日记」的那一轮要带一张出门清单。
//
// 她带着一句「去看看」出门，回来只会写「大家好像都挺忙的」——那是印象，不是
// 观察。清单是这件工具缺的 before-state（产品负责人 2026-09-02 说它 boring，
// 根子在这儿），设计文档要的「a simple observation method」也落在这几条上。
func TestParseCoachOutput_KeepsTheMissionForObserve(t *testing.T) {
	out, err := parseCoachOutput(`{"reply":"先去看三天。","tool":"observe",
	  "tool_reason":"你还没仔细看过，先去看三天中午",
	  "mission":[{"prompt":"中午 12:30 在走廊数一数站着的人","want_kind":"observation"},
	             {"prompt":"问一个站着的人为什么不回教室","want_kind":"quote"},
	             {"prompt":"","want_kind":"observation"}]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Mission) != 2 {
		t.Fatalf("清单没带过来（空的那条该丢掉）：%+v", out.Mission)
	}
	if out.Mission[1].WantKind != "quote" {
		t.Fatalf("要带回哪一类丢了：%+v", out.Mission[1])
	}
}

// 清单只属于观察日记。一件当场做完的工具挂一张出门清单，只会让她以为自己还得
// 出门一趟。
func TestParseCoachOutput_DropsAMissionOnAnyOtherTool(t *testing.T) {
	out, err := parseCoachOutput(`{"reply":"摊开看看。","tool":"board",
	  "tool_reason":"你刚带回来一堆观察",
	  "mission":[{"prompt":"去走廊数数人","want_kind":"observation"}]}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(out.Mission) != 0 {
		t.Fatalf("头脑风暴不该带出门清单：%+v", out.Mission)
	}
}

// prompt 里必须真的把清单要求出去，否则模型不会凭空开始给。
func TestCoachSystem_AsksForTheMission(t *testing.T) {
	if !strings.Contains(coachSystem, "mission") {
		t.Fatal("system prompt 里没有 mission 这一格")
	}
	if !strings.Contains(coachSystem, "want_kind") {
		t.Fatal("没说清楚每一条要带回哪一类——回来那一屏就填不出类别")
	}
}
