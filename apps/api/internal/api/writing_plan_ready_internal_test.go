package api

// writing_plan_ready_internal_test.go — the structural floor under 「这份计划
// 够写了」.
//
// This is the half that must NOT depend on a model. The product owner's report
// was 「until student click the logic is good, ai never auto triggers and
// guides students to start writing」, and the live measurement
// (TestLiveWritingPlanSignalsReady) showed why leaving it to the prompt is not
// enough: on a plan that met every stated criterion, the model chose about
// half the time to teach one more method and end on a question instead. There
// is always one more thing worth teaching — which is exactly why the floor is
// computed. See planLooksReady's own comment.

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

func node(text string, depth int32) sqlc.WritingOutline {
	return sqlc.WritingOutline{Text: text, Depth: depth}
}

func TestPlanLooksReady(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []sqlc.WritingOutline
		want bool
		why  string
	}{
		{
			name: "thesis, two reasons, her own material",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("青少年生物钟本来就晚", 1),
				node("上学期第一节课睡倒一片", 2),
				node("睡不够影响上午听课", 1),
			},
			want: true,
			why:  "this is the shape a piece can actually be written from",
		},
		{
			name: "nothing at all",
			rows: nil,
			want: false,
			why:  "an empty map is the state she starts in",
		},
		{
			name: "a thesis and nothing under it",
			rows: []sqlc.WritingOutline{node("应该往后推一小时", 0)},
			want: false,
			why:  "a claim with no reasons is not a plan",
		},
		{
			name: "only one sub-point",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("青少年生物钟本来就晚", 1),
				node("上学期第一节课睡倒一片", 2),
			},
			want: false,
			why:  "one reason is an opinion, not an argument",
		},
		{
			name: "two sub-points but no material under either",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("青少年生物钟本来就晚", 1),
				node("睡不够影响上午听课", 1),
			},
			want: false,
			why:  "without her own material she has two headings and nothing to write",
		},
		{
			name: "blank nodes do not count",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("   ", 1),
				node("", 1),
				node("  \n ", 2),
			},
			want: false,
			why:  "an empty node is a placeholder she has not filled in",
		},
		{
			name: "no opening or closing yet is still ready",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("青少年生物钟本来就晚", 1),
				node("上学期第一节课睡倒一片", 2),
				node("推迟不等于减少课时", 1),
			},
			want: true,
			// 🚨 The one that would be easiest to get wrong by "being thorough":
			// requiring an opening and a closing holds her at exactly the step
			// this function exists to release. They are decided after the
			// middle exists — the plan prompt says so itself.
			why: "openings and closings are decided AFTER the middle exists",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := planLooksReady(tc.rows); got != tc.want {
				t.Errorf("planLooksReady = %v, want %v — %s", got, tc.want, tc.why)
			}
		})
	}
}
