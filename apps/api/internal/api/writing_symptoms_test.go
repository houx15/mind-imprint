package api

import "testing"

// 中文记叙拿到的是「通用 + 记叙」两张表接起来，不是只有记叙那张。
func TestWritingSymptomTableNarrativeZHIsTheUnion(t *testing.T) {
	got := writingSymptomTable("zh", genreNarrative)
	if len(got) <= len(writingSymptomsZH) {
		t.Fatalf("记叙表 %d 条，该多于通用表的 %d 条", len(got), len(writingSymptomsZH))
	}
	has := func(id string) bool {
		for _, s := range got {
			if s.ID == id {
				return true
			}
		}
		return false
	}
	if !has(writingSymptomsZH[0].ID) {
		t.Error("记叙表里少了通用表的条目")
	}
	if !has(writingSymptomsNarrativeZH[0].ID) {
		t.Error("记叙表里少了记叙专有的条目")
	}
}

func TestWritingSymptomTableZHArgumentIsTheGeneralTable(t *testing.T) {
	if got := writingSymptomTable("zh", genreArgument); len(got) != len(writingSymptomsZH) {
		t.Errorf("中文议论该是通用表 %d 条，拿到 %d 条", len(writingSymptomsZH), len(got))
	}
}

// 英文记叙有自己的毛病，和英文议论文不是同一批（phase 4，2026-09-23）。
func TestWritingSymptomTableENSplitsByGenre(t *testing.T) {
	arg := writingSymptomTable("en", genreArgument)
	nar := writingSymptomTable("en", genreNarrative)
	if len(nar) == 0 {
		t.Fatal("英文记叙一条毛病都没有")
	}
	sameIDs := func(a, b []writingSymptom) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i].ID != b[i].ID {
				return false
			}
		}
		return true
	}
	if sameIDs(arg, nar) {
		t.Error("英文记叙和英文议论文拿到的是同一张表")
	}
	// 🚨 英文记叙那张是**加在**英文那张上面的，不是替换 ——
	// 照 writingSymptomsNarrativeZH 的做法（:191-193 的注释）。
	if len(nar) <= len(arg) {
		t.Error("记叙那张应该是在通用英文表之上再加几条")
	}
	if len(arg) != len(writingSymptomsEN) {
		t.Errorf("英文议论该是通用表 %d 条，拿到 %d 条", len(writingSymptomsEN), len(arg))
	}
}

// 🚨 语言认不出来（空串、老数据）时也要和 2026-09-22 之前一样。
// 搬家就是搬家 —— 哪怕这条路今天被 DB 的 CHECK 挡着走不到。
func TestWritingSymptomTableUnknownLangMatchesOldBehaviour(t *testing.T) {
	union := writingSymptomTable("zh", genreNarrative)
	for _, lang := range []string{"", "fr", "ZH"} {
		if got := writingSymptomTable(lang, genreNarrative); len(got) != len(union) {
			t.Errorf("lang=%q 记叙文拿到 %d 条，该和中文记叙一样是 %d 条",
				lang, len(got), len(union))
		}
		if got := writingSymptomTable(lang, genreArgument); len(got) != len(writingSymptomsZH) {
			t.Errorf("lang=%q 议论文拿到 %d 条，该是通用表的 %d 条",
				lang, len(got), len(writingSymptomsZH))
		}
	}
}
