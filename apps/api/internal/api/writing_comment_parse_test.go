package api

import "testing"

// 🚨 模型回了**不止一个** JSON 对象 —— 2026-09-21 的 LIVE_LLM 实测抓到的。
//
// 它先写了一份，接着用大白话跟自己商量（「补一句好的话也可以说……
// 最终输出加一条 good」），然后又写了一份改好的。夹到「第一个 { 到最后一个 }」
// 得到的是 `{对象一} 大白话 {对象二}`，不是合法 JSON，于是整轮作废 ——
// 她那边是一个转不动的终端（[[ai-errors-must-surface-never-fake]] 的另一面：
// 不是给她一句假话，是什么都不给）。
//
// 下面这段是模型**原样**回的，一个字没改。
const liveTwoObjectsReply = `{"verdict":"revise","summary":"这段有一个具体的事，但还不知道你为什么要讲它。","points":[{"kind":"issue","symptom":"story_without_meaning","text":"读者不知道这段想证明什么。","action":"在这件事前面加一句观点句。","quote":"上周三五点半我放学等车"}]}

补一句好的话也可以说：她有具体时间地点动作，这是材料句写对了。但一轮最多一条issue加一条good，issue更紧。上面JSON已含。加good会挤占，且时间地点细节确实值得肯定——可以加进去。最终输出加一条good。

{"verdict":"polish","summary":"这段的材料句写对了，缺的是段首那一句观点句。","points":[{"kind":"good","method":"point_pee","quote":"上周三五点半我放学等车","text":"时间、地点、做了什么都在，这是一个完整的材料句。"},{"kind":"issue","symptom":"story_without_meaning","quote":"上周三五点半我放学等车","text":"读者不知道这段想证明什么。","action":"在这件事前面加一句观点句扣住「联系家长」。"}]}`

func TestParseWritingComment_TwoObjects(t *testing.T) {
	got, ok := parseWritingComment(liveTwoObjectsReply)
	if !ok {
		t.Fatal("解析不了 —— 线上这一轮会静默作废，她看见的是一个转不动的终端")
	}
	// 🚨 取**后面**那一份：模型自己说的是「最终输出」，改好的在后面。
	if got.Verdict != writingVerdictPolish {
		t.Errorf("取的是前面那一份（verdict=%q），该取后面那一份 polish", got.Verdict)
	}
	if len(got.Points) != 2 {
		t.Errorf("后面那一份有 2 条（一条 good 一条 issue），得到 %d", len(got.Points))
	}
}

// 老路子一个字都没变：单个对象、带围栏的、前后有闲话的，照旧。
func TestParseWritingComment_StillHandlesTheOrdinaryShapes(t *testing.T) {
	for _, tc := range []struct{ name, in, wantVerdict string }{
		{
			name:        "干净的一个对象",
			in:          `{"verdict":"pass","summary":"站得住。","points":[]}`,
			wantVerdict: writingVerdictPass,
		},
		{
			name:        "带 markdown 围栏",
			in:          "```json\n{\"verdict\":\"polish\",\"summary\":\"还差一句。\",\"points\":[]}\n```",
			wantVerdict: writingVerdictPolish,
		},
		{
			name:        "前后各有一句闲话",
			in:          "好的，我看完了。\n{\"verdict\":\"revise\",\"summary\":\"这一段要改。\",\"points\":[]}\n就这些。",
			wantVerdict: writingVerdictRevise,
		},
		{
			// 🚨 她正文里带一个右花括号。按裸括号数会在半路「配平」，
			// 切出一段断掉的 JSON。
			name:        "引文里带花括号",
			in:          `{"verdict":"pass","summary":"她写了 a} b 这样一句。","points":[]}`,
			wantVerdict: writingVerdictPass,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseWritingComment(tc.in)
			if !ok {
				t.Fatalf("解析不了：%s", tc.in)
			}
			if got.Verdict != tc.wantVerdict {
				t.Errorf("verdict = %q，want %q", got.Verdict, tc.wantVerdict)
			}
		})
	}
}

// 真的坏掉的还是要失败 —— 绝不能因为「兜得更宽」就把一句编出来的话当成结果。
// [[ai-errors-must-surface-never-fake]]。
func TestParseWritingComment_StillFailsOnRealGarbage(t *testing.T) {
	for _, in := range []string{
		"",
		"我觉得这一段挺好的。",
		`{"verdict":"pass","summary":"`, // 断在一半（流式丢了最后一块）
		`{"verdict":"pass","points":[]}`, // 没有 summary
	} {
		if _, ok := parseWritingComment(in); ok {
			t.Errorf("这一份该解析失败，却过了：%q", in)
		}
	}
}

func TestWritingJSONObjectSpans(t *testing.T) {
	spans := writingJSONObjectSpans(`前面 {"a":1} 中间 {"b":{"c":2}} 后面`)
	if len(spans) != 2 {
		t.Fatalf("该切出 2 段，得到 %d：%v", len(spans), spans)
	}
	if spans[0] != `{"a":1}` {
		t.Errorf("第一段是 %q", spans[0])
	}
	// 嵌套的算一段，不是两段。
	if spans[1] != `{"b":{"c":2}}` {
		t.Errorf("第二段是 %q —— 嵌套的对象该整个算一段", spans[1])
	}
	// 字符串里的括号不参与配平。
	one := writingJSONObjectSpans(`{"t":"这里有个 } 括号"}`)
	if len(one) != 1 || one[0] != `{"t":"这里有个 } 括号"}` {
		t.Errorf("字符串里的括号被当成了结构括号：%v", one)
	}
	// 没配平的不交出来。
	if got := writingJSONObjectSpans(`{"a":1`); len(got) != 0 {
		t.Errorf("没配平的不该交出来：%v", got)
	}
}
