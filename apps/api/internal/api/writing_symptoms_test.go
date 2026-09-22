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

// 🚨 英文今天**不分文体**：两种体裁拿到的是同一张表。
// 这是一个已知缺口（spec 四期补英文记叙那张表），不是这一次要改的行为。
// 钉住它，免得搬家时悄悄变了样。
func TestWritingSymptomTableENIgnoresGenreForNow(t *testing.T) {
	arg := writingSymptomTable("en", genreArgument)
	nar := writingSymptomTable("en", genreNarrative)
	if len(arg) != len(nar) || len(arg) != len(writingSymptomsEN) {
		t.Errorf("英文两种体裁今天该是同一张表：议论 %d、记叙 %d、表 %d",
			len(arg), len(nar), len(writingSymptomsEN))
	}
}
