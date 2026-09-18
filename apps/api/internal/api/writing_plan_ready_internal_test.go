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
			name: "thesis, two reasons, an example under each",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("青少年生物钟本来就晚", 1),
				node("上学期第一节课睡倒一片", 2),
				node("睡不够影响上午听课", 1),
				node("美国儿科学会建议中学八点半后上课", 2),
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
			// 2026-09-18 产品负责人：800 字的议论文起码 2–3 个例子。
			name: "two reasons but only one example",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("青少年生物钟本来就晚", 1),
				node("上学期第一节课睡倒一片", 2),
				node("睡不够影响上午听课", 1),
			},
			want: false,
			why:  "one reason has nothing under it but reasoning",
		},
		{
			// 「个人经历是信效度最低的」—— 两个例子都是她自己的，还不够。
			name: "two examples, both her own experience",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				node("青少年生物钟本来就晚", 1),
				{Text: "我自己早上七点总是犯困", Depth: 2, Role: "你经历过的事"},
				node("睡不够影响上午听课", 1),
				{Text: "同桌第一节课睡着了", Depth: 2, Role: "你见过的事"},
			},
			want: false,
			why:  "an argument resting only on her own anecdotes needs one wider example",
		},
		{
			// 2026-09-18 实测：一条「道理」被当成了一个不是个人经历的例子。
			name: "one personal example and one line of reasoning",
			rows: []sqlc.WritingOutline{
				node("人可以脆弱", 0),
				node("脆弱没有打垮我", 1),
				{Text: "爸爸入狱、妹妹抑郁", Depth: 2, Role: "你经历过的事"},
				node("脆弱让人区别于机器", 1),
				{Text: "脆弱里有关于渴望的信息", Depth: 2, Role: "一条道理"},
			},
			want: false,
			why:  "reasoning is not an example, and the only example is her own",
		},
		{
			// 例子直接挂在中心论点下面：算例子，不算分论点。
			name: "examples filed under the thesis are not reasons",
			rows: []sqlc.WritingOutline{
				node("应该往后推一小时", 0),
				{Text: "西雅图推迟上课的研究", Depth: 1, Role: "一项研究"},
				{Text: "美国儿科学会的建议", Depth: 1, Role: "一个例子"},
			},
			want: false,
			why:  "two examples are not two 分论点",
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
				node("西雅图推迟上课后学生多睡了34分钟", 2),
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
			if got := planLooksReady(sqlc.Writing{}, tc.rows); got != tc.want {
				t.Errorf("planLooksReady = %v, want %v — %s", got, tc.want, tc.why)
			}
		})
	}
}
