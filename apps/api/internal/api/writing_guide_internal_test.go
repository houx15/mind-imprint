package api

// writing_guide_internal_test.go — a white-box test for parseWritingGuide,
// which is the 铁律① boundary of the writing room.
//
// The endpoint's whole safety claim is that it can only ever hand the student
// QUESTIONS — because a question cannot be pasted into an essay, while a
// demonstration sentence can. That claim rests entirely on this one function,
// so it is tested directly rather than only through the HTTP handler.

import "testing"

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
