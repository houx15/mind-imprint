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
