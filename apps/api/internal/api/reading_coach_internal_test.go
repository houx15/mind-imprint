package api

// reading_coach_internal_test.go — white-box tests for the coach's reply
// parsing and prompt assembly (unexported symbols), following the
// internal-test convention used by reading_brief_internal_test.go /
// writing_guide_internal_test.go / writing_plan_internal_test.go. Lives
// separately from reading_coach_test.go (package api_test, black-box) which
// cannot see unexported functions like parseReadingCoachReply.

import "testing"

func TestParseReadingCoachReplyLens(t *testing.T) {
	valid := map[string]bool{"b1": true, "b2": true}
	allow := func(id string) bool { return id == "craap" }

	cases := []struct {
		name string
		json string
		want string
	}{
		{"aimed and allowed", `{"reply":"看这段","advance":"","focusBlock":"b2","lens":"craap"}`, "craap"},
		// An un-aimed lens is indistinguishable from her opening 透镜库
		// herself — the whole point of the coach summoning one is that it
		// lands on a paragraph it just talked about.
		{"un-aimed is dropped", `{"reply":"来看看来源","advance":"","focusBlock":"","lens":"craap"}`, ""},
		{"not allowed right now", `{"reply":"x","advance":"","focusBlock":"b1","lens":"sift"}`, ""},
		{"unknown id", `{"reply":"x","advance":"","focusBlock":"b1","lens":"nope"}`, ""},
		{"absent", `{"reply":"x","advance":"","focusBlock":"b1"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseReadingCoachReply(tc.json, valid, "zh", allow)
			if !ok {
				t.Fatalf("parse failed for %s", tc.json)
			}
			if got.Lens != tc.want {
				t.Errorf("lens = %q, want %q", got.Lens, tc.want)
			}
			if got.Reply == "" {
				t.Error("a dropped lens must not take the reply down with it")
			}
		})
	}
}

// TestParseReadingCoachReplyLens_NilLensOK — the guard's `lensOK == nil`
// short-circuit has no caller yet exercising it: every real call site passes
// a real predicate. A nil lensOK must still drop the lens rather than panic
// on the nil call.
func TestParseReadingCoachReplyLens_NilLensOK(t *testing.T) {
	valid := map[string]bool{"b1": true, "b2": true}

	got, ok := parseReadingCoachReply(
		`{"reply":"看这段","advance":"","focusBlock":"b2","lens":"craap"}`, valid, "zh", nil)
	if !ok {
		t.Fatalf("parse failed")
	}
	if got.Lens != "" {
		t.Errorf("lens = %q, want dropped when lensOK is nil", got.Lens)
	}
	if got.Reply == "" {
		t.Error("a dropped lens must not take the reply down with it")
	}
}
