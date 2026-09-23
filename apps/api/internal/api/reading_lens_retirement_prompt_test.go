package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// New reading requests must not advertise retired activities. Retaining the
// wire field is intentional: stored replies and clients still use its schema.
func TestReadingRequestDoesNotOfferRetiredLenses(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		system := buildReadingCoachSystem(lang)
		user := buildReadingCoachPrompt("文章", []Block{{ID: "b1", Text: "正文。"}}, readingOutline{}, []sqlc.ReadingTask{{ID: fixtureTaskID(1), Kind: "critique", Label: "评价文章", Status: "pending"}}, nil, nil, "", nil, "")
		request := system + user
		for _, id := range agent.ReadingDeckIDs {
			if strings.Contains(request, "lens="+id) {
				t.Fatalf("retired lens %s advertised", id)
			}
		}
		for _, obsolete := range []string{"%LENS%", "透镜是让学生亲手分析", "再给 lens", "下列透镜 id"} {
			if strings.Contains(request, obsolete) {
				t.Fatalf("obsolete instruction %q", obsolete)
			}
		}
		if !strings.Contains(system, `"lens":""`) || !strings.Contains(system, "本轮填写空字符串") {
			t.Fatal("lens compatibility field must remain empty")
		}
		if !strings.Contains(system, "tool=questions") || !strings.Contains(system, "short_text") {
			t.Fatal("current reading tools were removed")
		}
	}
}

func TestReadingLegacyLensCompletionRetainsStudentWorkWithoutNewActivity(t *testing.T) {
	quote := "原文中的句子。"
	user := buildReadingCoachPrompt("文章", []Block{{ID: "b1", Text: quote}}, readingOutline{}, nil, nil, nil, "", &readingLensDone{CardName: "历史工具", Quote: quote, Finding: "此前的模型分析"}, "")
	for _, want := range []string{quote, "此前的模型分析", "旧版活动的完成记录", `advance="done"`, "不创建新透镜"} {
		if !strings.Contains(user, want) {
			t.Fatalf("legacy contract missing %q", want)
		}
	}
	if strings.Contains(user, "lens=") {
		t.Fatal("legacy completion advertises a lens catalog")
	}
}
