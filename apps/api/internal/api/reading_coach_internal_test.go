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
