package api

// writing_guide_internal_test.go — a white-box test for parseWritingGuide,
// which is the 铁律① boundary of the writing room.
//
// The endpoint's whole safety claim is that it can only ever hand the student
// QUESTIONS — because a question cannot be pasted into an essay, while a
// demonstration sentence can. That claim rests entirely on this one function,
// so it is tested directly rather than only through the HTTP handler.

import (
	"strings"
	"testing"
)

func TestParseWritingGuide_DropsAnythingThatIsNotAQuestion(t *testing.T) {
	// A model that slips a declarative sentence into the list has just handed
	// her a sentence for her essay. That is the exact failure this filter
	// exists to make impossible, so it is asserted on the worst realistic
	// case: a reply where the model helpfully offers to write the line.
	const reply = `{"questions":[
		"你身边有没有哪个同学因为手机吃过亏？",
		"你可以这样开头：手机正在悄悄偷走我们的专注力。",
		"支持禁手机的老师最常说的一句话是什么？",
		"建议你先写一个让步段。"
	]}`

	got, ok := parseWritingGuide(reply)
	if !ok {
		t.Fatalf("parse failed on a well-formed reply")
	}
	if len(got.Questions) != 2 {
		t.Fatalf("kept %d entries (%q), want only the 2 questions", len(got.Questions), got.Questions)
	}
	for _, q := range got.Questions {
		if q == "你可以这样开头：手机正在悄悄偷走我们的专注力。" {
			t.Fatalf("a ready-to-paste sentence survived the filter — 铁律① is broken")
		}
		if q == "建议你先写一个让步段。" {
			t.Fatalf("a declarative suggestion survived the filter")
		}
	}
}

func TestParseWritingGuide_AcceptsBothQuestionMarks(t *testing.T) {
	// An English writing gets ASCII '?'; a Chinese one gets '？'. Dropping
	// either script would silently empty the box for half the students.
	got, ok := parseWritingGuide(`{"questions":["What did you actually see happen?","你当时是什么感觉？"]}`)
	if !ok {
		t.Fatalf("parse failed")
	}
	if len(got.Questions) != 2 {
		t.Fatalf("kept %d, want 2 — both question marks must be accepted; got %q", len(got.Questions), got.Questions)
	}
}

func TestParseWritingGuide_NoQuestionsIsAFailure(t *testing.T) {
	// A reply that survives JSON decoding but holds nothing usable must be
	// treated as a failed call (→ 502), never as an empty guide box. An empty
	// box looks like "印记 had nothing to ask", which is a lie about what
	// happened and gives her nothing to act on.
	for _, reply := range []string{
		`{"questions":[]}`,
		`{"questions":["先写你的立场。","再写理由。"]}`,
		`{"questions":["   "]}`,
	} {
		if _, ok := parseWritingGuide(reply); ok {
			t.Fatalf("reply %q was accepted; want it treated as a failure", reply)
		}
	}
}

func TestParseWritingGuide_CapsTheList(t *testing.T) {
	// More than four reads as a worksheet, which is the thing the guiding box
	// is trying not to be.
	got, ok := parseWritingGuide(`{"questions":["a？","b？","c？","d？","e？","f？"]}`)
	if !ok {
		t.Fatalf("parse failed")
	}
	if len(got.Questions) != writingGuideMaxQuestions {
		t.Fatalf("kept %d, want the cap of %d", len(got.Questions), writingGuideMaxQuestions)
	}
}

func TestParseWritingGuide_ToleratesCodeFences(t *testing.T) {
	// Models wrap JSON in fences unprompted; a fenced reply is a good reply.
	got, ok := parseWritingGuide("```json\n{\"questions\":[\"你为什么这么想？\"]}\n```")
	if !ok {
		t.Fatalf("a fenced reply must still parse")
	}
	if len(got.Questions) != 1 {
		t.Fatalf("kept %d, want 1", len(got.Questions))
	}
}

func TestParseWritingGuide_GarbageIsAFailure(t *testing.T) {
	for _, reply := range []string{"", "不是 JSON", "{", `{"questions":"not an array"}`} {
		if _, ok := parseWritingGuide(reply); ok {
			t.Fatalf("garbage %q was accepted", reply)
		}
	}
}

// The ？ filter is the security boundary and it must survive the guide growing
// prose fields: a declarative sentence in `questions` is a sentence for her essay.
func TestParseWritingGuide_QuestionFilterAppliesToQuestionsOnly(t *testing.T) {
	got, ok := parseWritingGuide(`{
      "job":"这一段要让读者相信「便宜」这个说法不成立。",
      "method_ids":["point_contrast","no_such_method"],
      "questions":["你见过哪条街上的树长不开？","你可以写：树会抢水。","这跟成本有什么关系？"]
    }`)
	if !ok {
		t.Fatal("parse failed")
	}
	if len(got.Questions) != 2 {
		t.Fatalf("questions = %v, want the declarative one dropped", got.Questions)
	}
	if got.Job == "" {
		t.Error("job was dropped; the prose field must survive the filter")
	}
	// An id the model invented is not a method. Dropping it is what keeps
	// terminology ours.
	if len(got.MethodIDs) != 1 || got.MethodIDs[0] != "point_contrast" {
		t.Fatalf("method ids = %v, want only the known one", got.MethodIDs)
	}
}

func TestParseWritingGuide_FailsWhenNoQuestionsSurvive(t *testing.T) {
	if _, ok := parseWritingGuide(`{"job":"x","method_ids":[],"questions":["你可以写：手机让人分心。"]}`); ok {
		t.Fatal("parse succeeded with zero surviving questions; a guide with no questions teaches her nothing")
	}
}

// 🚨 同事 2026-09-20：「每一次刷新就会变成新的东西」。
//
// 「卡住了？」原来直接覆盖，她读过的那一组当场没了 —— 她按那颗按钮是想再要
// 一个角度，不是想把刚才那几个问题扔掉。
func TestRegenerateGuideKeepsPrevious(t *testing.T) {
	old := writingGuideDTO{Job: "旧的任务", Questions: []string{"旧问题一？", "旧问题二？"}}
	fresh := writingGuideDTO{Job: "新的任务", Questions: []string{"新问题？"}}

	got := writingGuideWithPrevious(fresh, &old)
	if got.Previous == nil || got.Previous.Job != "旧的任务" {
		t.Fatalf("上一组没留住：%+v", got.Previous)
	}
	if len(got.Previous.Questions) != 2 {
		t.Errorf("上一组的问题少了：%+v", got.Previous.Questions)
	}
	// 🚨 只留一层。再往上叠会变成一份她读不完的历史。
	if got.Previous.Previous != nil {
		t.Error("previous 不该套娃")
	}

	// 第一次生成（之前什么都没有）不该凭空造一个空的上一组出来。
	if first := writingGuideWithPrevious(fresh, nil); first.Previous != nil {
		t.Error("第一次生成不该有 previous")
	}
	empty := writingGuideDTO{}
	if first := writingGuideWithPrevious(fresh, &empty); first.Previous != nil {
		t.Error("空的上一组等于没有，不该挂上去")
	}
}

// 她已经写了字的那一块最多两个问题 —— 提示词里也写了，但真正算数的是这里。
func TestWritingGuideQuestionCap(t *testing.T) {
	if got := writingGuideQuestionCap(""); got != writingGuideMaxQuestions {
		t.Errorf("空白的一块该给 %d 条，得到 %d", writingGuideMaxQuestions, got)
	}
	if got := writingGuideQuestionCap("   "); got != writingGuideMaxQuestions {
		t.Errorf("只有空白也算空白，得到 %d", got)
	}
	if got := writingGuideQuestionCap("她已经写了三百字。"); got != writingGuideMaxQuestionsWritten {
		t.Errorf("写过字的一块该给 %d 条，得到 %d", writingGuideMaxQuestionsWritten, got)
	}
}

// 重新生成时要把上一组喂回去，否则模型会原地换个说法重写一遍。
func TestWritingGuideAnotherAngle(t *testing.T) {
	if got := writingGuideAnotherAngle(nil); got != "" {
		t.Errorf("第一次生成不该加这一段，得到 %q", got)
	}
	prior := writingGuideDTO{Questions: []string{"闹钟响了你还想睡那次？"}}
	got := writingGuideAnotherAngle(&prior)
	if !strings.Contains(got, "闹钟响了你还想睡那次？") {
		t.Errorf("上一组的问题没喂回去：%s", got)
	}
	if !strings.Contains(got, "选择不同的构思方向") {
		t.Errorf("没说清这一轮要做什么：%s", got)
	}
}
