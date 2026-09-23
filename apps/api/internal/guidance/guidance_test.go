package guidance

import "testing"

func TestGradeBandFoldsGradeToBand(t *testing.T) {
	for grade, want := range map[string]string{
		"junior1": "junior", "junior2": "junior", "junior3": "junior",
		"senior1": "senior", "senior2": "senior", "senior3": "senior",
		"": "", "primary4": "",
	} {
		if got := GradeBand(grade); got != want {
			t.Errorf("GradeBand(%q) = %q, 想要 %q", grade, got, want)
		}
	}
}

// 🚨 这一条是整个注册表的核心：越具体的那一行越该赢。
// 年级逐字命中 > 只命中学段 > 不限学段。
func TestPickPrefersTheMostSpecificRow(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "不限学段"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Grades: []string{"junior"}}, Value: "整个初中"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Grades: []string{"junior2"}}, Value: "初二"},
	}
	k := Key{Surface: SurfaceWrite, Lang: "zh", Grade: "junior2"}
	if got, ok := Pick(k, rows); !ok || got != "初二" {
		t.Fatalf("初二那一行该赢，拿到 %q ok=%v", got, ok)
	}
	k.Grade = "junior3"
	if got, ok := Pick(k, rows); !ok || got != "整个初中" {
		t.Fatalf("初三没有自己那一行，该退到整个初中，拿到 %q ok=%v", got, ok)
	}
	k.Grade = "senior1"
	if got, ok := Pick(k, rows); !ok || got != "不限学段" {
		t.Fatalf("高一两行都不服务，该退到不限学段，拿到 %q ok=%v", got, ok)
	}
}

func TestPickGenreBeatsNoGenre(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "任何文体"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh", Genres: []string{"narrative"}}, Value: "记叙文"},
	}
	got, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh", Genre: "narrative"}, rows)
	if !ok || got != "记叙文" {
		t.Fatalf("记叙文那一行该赢，拿到 %q ok=%v", got, ok)
	}
	got, ok = Pick(Key{Surface: SurfaceWrite, Lang: "zh", Genre: "argument"}, rows)
	if !ok || got != "任何文体" {
		t.Fatalf("议论文没有专门的行，该退到任何文体，拿到 %q ok=%v", got, ok)
	}
}

// 面和语言对不上就是不匹配 —— 阅读的内容绝不能漏到写作那一侧去。
func TestPickNeverCrossesSurfaceOrLang(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceRead, Lang: "zh"}, Value: "阅读的"},
	}
	if _, ok := Pick(Key{Surface: SurfaceWrite, Lang: "zh"}, rows); ok {
		t.Error("写作面取到了阅读面的内容")
	}
	if _, ok := Pick(Key{Surface: SurfaceRead, Lang: "en"}, rows); ok {
		t.Error("英文取到了中文的内容")
	}
}

// 一样具体时先登记的赢，而且这件事要是稳定的：注册表的顺序是人排的，
// 排在前面就是更该用的那一条。
func TestPickIsStableOnTies(t *testing.T) {
	rows := []Row[string]{
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "先登记的"},
		{Scope: Scope{Surface: SurfaceWrite, Lang: "zh"}, Value: "后登记的"},
	}
	if got, _ := Pick(Key{Surface: SurfaceWrite, Lang: "zh"}, rows); got != "先登记的" {
		t.Errorf("平局该是先登记的赢，拿到 %q", got)
	}
}
