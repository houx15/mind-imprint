package liteworkspace

import "testing"

func TestClaimsOpenedPage(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		// Production, 2026-09-17.
		{"已经打开了本周报告页面。", true},
		{"已为您跳转到本周报告。", true},
		{"已打开林知遥的学习页", true},
		{"帮您打开了家长报告。", true},
		{"已切换到布置作业页面", true},
		{"要打开本周报告吗？", false},
		{"请点击下方按钮前往本周报告。", false},
		{"可以帮您打开本周报告", false},
		{"打开林知遥的学习页", false},
		{"林知遥已进入写作阶段。", false},
	} {
		if got := ClaimsOpenedPage(tc.text); got != tc.want {
			t.Errorf("ClaimsOpenedPage(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestClaimsPublished(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		// Production, 2026-09-17, before the teacher pressed 发布作业.
		{"已为林知遥、陈思远、赵一诺布置好议论文《城市是否应该限制私家车出行》：", true},
		{"已布置好。项目《手机学习时间调查》发给孙浩然、周子涵、李若溪。", true},
		{"作业已发布。", true},
		{"已经发给全班了。", true},
		{"作业布置好了。", true},
		{"作业卡已填好，请检查后点「发布作业」。", false},
		{"作业还没有发布，请检查后发布。", false},
		{"这份作业发布后，学生会在收件箱看到。", false},
		{"本周已布置的作业有两份。", false},
		{"已填好：标题《AI 与学习》，截止时间 9月18日 21:00。", false},
	} {
		if got := ClaimsPublished(tc.text); got != tc.want {
			t.Errorf("ClaimsPublished(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestNamesSectionAndChange(t *testing.T) {
	labels := []string{"总体概述", "阅读", "写作", "下一步建议"}
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"请把总体概述写得更具体一些，提到她完成了阅读作业并修改了作文。", true},
		{"把下一步建议改短一些，只留两条。", true},
		{"阅读那段删掉最后一句", true},
		{"帮我改一下", false},      // no section
		{"总体概述写得怎么样", true},   // 写得 counts; asking back is still wrong here
		{"这份报告可以发了吗？", false}, // no section, no change
		{"下一步建议", false},      // no direction
	} {
		if got := NamesSectionAndChange(tc.text, labels); got != tc.want {
			t.Errorf("NamesSectionAndChange(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestPointsAtButton(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"请点击下方按钮前往周子涵的学习页。", true},
		{"点下面的按钮就能看到。", true},
		{"这是本周报告的入口。", false},
		{"需要打开本周报告吗？", false},
	} {
		if got := PointsAtButton(tc.text); got != tc.want {
			t.Errorf("PointsAtButton(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestOnlyAQuestion(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"接下来想看什么？", true}, // live, 2026-09-17, twice
		{"接下来您想怎么做？", true},
		{"名单上的学生本周还没有开始学习。您想看看该生的学习页，还是查看全班本周报告？", false},
		{"本周整体情况已列在左侧！需要我打开本周报告吗？", false},
		{"只有这两名学生还没开始，接下来想做什么？", false},
		{"好的", false},
		{"", false},
	} {
		if got := OnlyAQuestion(tc.text); got != tc.want {
			t.Errorf("OnlyAQuestion(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestOffersMessage(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		// Live, 2026-09-17.
		{"提醒该生开始学习", true},
		{"需要提醒她们吗", true},
		{"通知这些学生", true},
		{"给家长发个消息", true},
		{"我无法直接提醒学生，可以为这些学生布置一份作业。", false},
		{"不能通知学生", false},
		{"查看该生学习页", false},
		{"看看全班本周概况", false},
	} {
		if got := OffersMessage(tc.text); got != tc.want {
			t.Errorf("OffersMessage(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestOffersSectionChange(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		// Production, 2026-09-17.
		{"新增一个『阅读』段落", true},
		{"添加一个新的部分", true},
		{"删除「兴趣」板块", true},
		{"增加段落", true},
		{"在「写作」里增加一段话", false},
		// The model's honest refusals, live 2026-09-17.
		{"抱歉，报告的段落是固定的，我只能改写现有的这三段，不能新增段落。", false},
		{"只能改写这几段：「总体概述」、「阅读」。不能新增或删除段落。", false},
		{"报告无法添加新的部分。", false},
		{"不能改标题，但可以新增一个段落", true},
		{"补充一句具体做法", false},
		{"改写「下一步建议」", false},
	} {
		if got := OffersSectionChange(tc.text); got != tc.want {
			t.Errorf("OffersSectionChange(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

func TestNamesMissingSection(t *testing.T) {
	missing := []string{"写作", "兴趣"}
	for _, tc := range []struct {
		text, want string
	}{
		{"可以改写「兴趣」", "兴趣"},
		{"把兴趣部分写具体", "兴趣"},
		{"写作段落再短一点", "写作"},
		{"她对写作很有兴趣。", ""},
		{"报告里没有「写作」这个段落，不能新增。可以改写「阅读」。", ""},
		{"没有问题。可以改写「兴趣」。", "兴趣"},
		{"改写「阅读」", ""},
	} {
		if got := NamesMissingSection(tc.text, missing); got != tc.want {
			t.Errorf("NamesMissingSection(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

func TestRedactNames(t *testing.T) {
	got := RedactNames("王丽华和王丽都还没交，王丽华先交。", []string{"王丽", "王丽华", ""})
	if want := "[学生]和[学生]都还没交，[学生]先交。"; got != want {
		t.Fatalf("RedactNames = %q, want %q", got, want)
	}
}

func TestPronounProblem(t *testing.T) {
	none := PronounsAllowed{}
	female := PronounsAllowed{Female: true}
	male := PronounsAllowed{Male: true}
	both := PronounsAllowed{Female: true, Male: true}
	for _, tc := range []struct {
		text    string
		p       PronounsAllowed
		wantBad bool
	}{
		// Production and live, 2026-09-17.
		{"给她们布置作业", none, true},
		{"需要提醒她们吗", none, true},
		{"她这周没有登录。", none, true},
		{"他这周没有登录。", female, true},
		{"她这周没有登录。", female, false},
		{"他这周没有登录。", male, false},
		{"给她们布置作业", female, false},
		{"给她们布置作业", both, true},
		{"他们都还没开始。", both, false},
		{"其他同学都已开始，请关注他人。", none, false},
		{"这些学生都还没开始，请点击下方按钮。", none, false},
	} {
		if got := PronounProblem(tc.text, tc.p) != ""; got != tc.wantBad {
			t.Errorf("PronounProblem(%q, %+v) bad = %v, want %v", tc.text, tc.p, got, tc.wantBad)
		}
	}

	var p PronounsAllowed
	p.AllowPronounsOf([]Student{{Name: "周子涵", Gender: GenderFemale}, {Name: "孙浩然", Gender: GenderMale}}, []string{"周子涵"})
	if !p.Female || p.Male {
		t.Fatalf("AllowPronounsOf = %+v, want female only", p)
	}
	var typed PronounsAllowed
	typed.AllowTyped("她这周怎么样？其他人呢")
	if !typed.Female || typed.Male {
		t.Fatalf("AllowTyped = %+v, want female only", typed)
	}
}

func TestGender(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"female", "female", true}, {" male ", "male", true}, {"", "", true}, {"other", "", false}, {"女", "", false},
	} {
		got, ok := ParseGender(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("ParseGender(%q) = %q, %v", tc.in, got, ok)
		}
	}
	bad := "x"
	for _, tc := range []struct {
		in   *string
		want string
	}{{nil, PronounUnset}, {&bad, PronounUnset}} {
		if got := Pronoun(GenderOf(tc.in)); got != tc.want {
			t.Errorf("Pronoun(GenderOf(%v)) = %q", tc.in, got)
		}
	}
	if Pronoun(GenderFemale) != "她" || Pronoun(GenderMale) != "他" {
		t.Fatal("pronoun table")
	}
}
