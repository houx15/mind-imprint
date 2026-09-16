package pbl

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAudienceSummaryAndStudentKeywordsRoundTrip(t *testing.T) {
	doc := AudienceDocument{Step: "summary", Boards: []AudienceBoard{{ID: "a", Role: "同学", Person: "社团同学", AgeRange: "13至15岁", Interests: []string{"摄影"}, Offerings: []string{"作品照片"}}}, Summary: &AudienceSummary{Boards: []AudienceBoardSummary{{BoardID: "a", Keywords: []AudienceKeyword{{Text: "作品", SourceField: "offerings", SourceIndex: 0}}}}}, Keywords: map[string][]string{"a": {"校园摄影"}}}
	got, err := NormalizeAudienceDocument(doc, true)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(got)
	var restored AudienceDocument
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Summary.Boards[0].Keywords[0].Text != "作品" || restored.Keywords["a"][0] != "校园摄影" {
		t.Fatal("student edits or original source lost")
	}
	restored.Summary.Boards[0].Keywords[0].SourceIndex = 5
	if _, err = NormalizeAudienceDocument(restored, false); err == nil {
		t.Fatal("stale summary accepted")
	}
}

func TestAudienceDocumentDraftAndConfirmation(t *testing.T) {
	draft := AudienceDocument{Step: "person", ActiveBoardID: "teacher", Boards: []AudienceBoard{{ID: "teacher", Role: "老师"}, {ID: "peer", Role: "同学"}}}
	if _, err := NormalizeAudienceDocument(draft, false); err != nil {
		t.Fatal(err)
	}
	if _, err := NormalizeAudienceDocument(draft, true); err == nil {
		t.Fatal("incomplete boards accepted")
	}
	for i := range draft.Boards {
		draft.Boards[i].Person = "认识的一位读者"
		draft.Boards[i].AgeRange = "不确定"
		draft.Boards[i].Interests = []string{" 摄影 ", "摄影", " "}
		draft.Boards[i].Offerings = []string{"校园摄影作品"}
	}
	got, err := NormalizeAudienceDocument(draft, true)
	if err != nil || len(got.Boards) != 2 || len(got.Boards[0].Interests) != 1 {
		t.Fatalf("multi-reader normalization: %+v %v", got, err)
	}
}

func TestAudienceDocumentRejectsBrokenIdentityAndBounds(t *testing.T) {
	for _, doc := range []AudienceDocument{
		{Step: "unknown"},
		{Step: "roles", ActiveBoardID: "missing"},
		{Step: "roles", Boards: []AudienceBoard{{ID: "x", Role: "同学"}, {ID: "x", Role: "老师"}}},
		{Step: "roles", Boards: []AudienceBoard{{ID: "x", Role: "同学", Interests: []string{strings.Repeat("字", 301)}}}},
	} {
		if _, err := NormalizeAudienceDocument(doc, false); err == nil {
			t.Fatalf("invalid draft accepted: %+v", doc)
		}
	}
	if _, err := NormalizeAudienceDocument(AudienceDocument{Step: "roles"}, true); err == nil {
		t.Fatal("empty confirmation accepted")
	}
}

// Old saved boards remain usable; hobbies are optional background, separate from page needs.
func TestAudienceHobbiesRemainSeparateFromPageInterests(t *testing.T) {
	doc := AudienceDocument{Step: "person", Boards: []AudienceBoard{{ID: "peer", Role: "同学", Person: "社团同学", AgeRange: "13至15岁", Hobbies: []string{" 科幻 ", "科幻", "动漫"}, Interests: []string{"互动小游戏"}, Offerings: []string{"我的作品"}}}}
	got, err := NormalizeAudienceDocument(doc, true)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(got)
	var restored AudienceDocument
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if strings.Join(restored.Boards[0].Hobbies, ",") != "科幻,动漫" || restored.Boards[0].Interests[0] != "互动小游戏" {
		t.Fatal("hobbies lost or mixed into page needs")
	}
	restored.Boards[0].Hobbies = nil
	if _, err := NormalizeAudienceDocument(restored, true); err != nil {
		t.Fatal("optional hobbies blocked existing board", err)
	}
	restored.Boards[0].Hobbies = []string{strings.Repeat("字", 301)}
	if _, err := NormalizeAudienceDocument(restored, false); err == nil {
		t.Fatal("oversized hobby accepted")
	}
}

func TestAudienceArchiveSurvivesAndDoesNotRequireCompletion(t *testing.T) {
	doc := AudienceDocument{Step: "roles", ArchivedBoards: []AudienceBoard{{ID: "old", Role: "teacher", Hobbies: []string{" animation "}}}}
	normalized, err := NormalizeAudienceDocument(doc, false)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(normalized)
	var restored AudienceDocument
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.ArchivedBoards) != 1 || restored.ArchivedBoards[0].Hobbies[0] != "animation" {
		t.Fatalf("lost archived work: %s", data)
	}
	restored.Boards = []AudienceBoard{{ID: "old", Role: "teacher"}}
	if _, err := NormalizeAudienceDocument(restored, false); err == nil {
		t.Fatal("accepted active/archive collision")
	}
}

func TestAudienceKeywordDraftSurvivesWithoutBecomingAKeyword(t *testing.T) {
	editing := "原词"
	doc := AudienceDocument{Step: "person", Boards: []AudienceBoard{{ID: "teacher", Role: "老师", Person: "老师", AgeRange: "不确定", Interests: []string{"作品"}, Offerings: []string{"制作过程"}, KeywordDrafts: map[string]AudienceKeywordDraft{"hobbies": {Text: " 科幻 ", Editing: &editing}}}}}
	normalized, err := NormalizeAudienceDocument(doc, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		t.Fatal(err)
	}
	var restored AudienceDocument
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Boards[0].KeywordDrafts["hobbies"].Text != " 科幻 " || len(restored.Boards[0].Hobbies) != 0 {
		t.Fatal("draft was lost or silently committed")
	}
	if _, err := NormalizeAudienceDocument(restored, true); err == nil {
		t.Fatal("unfinished input must be handled before summary or confirmation")
	}
	restored.Boards[0].KeywordDrafts = nil
	if _, err := NormalizeAudienceDocument(restored, true); err != nil {
		t.Fatal(err)
	}
}
