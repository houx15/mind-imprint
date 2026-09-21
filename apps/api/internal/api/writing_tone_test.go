package api

import "testing"

// 🚨 这四句是 2026-09-21 真学生走查里**真模型原样写给她的**，一个字没改。
// 同事的原话是「我觉得它一直在挑衅我」，指的就是这一类。
func TestWritingHostileTone_TheFourSentencesFromTheWalk(t *testing.T) {
	cases := []struct {
		name string
		res  writingCommentResult
	}{
		{
			name: "把四段判成白写",
			res:  writingCommentResult{Summary: "结尾把整篇从「短视频让我们变笨」滑回了「有好有坏、合理利用」，前四段白立的主张在这里松了手。"},
		},
		{
			name: "抓包",
			res: writingCommentResult{Points: []CommentPoint{{
				Kind: "issue",
				Text: "读者读完最后一段会以为你其实没下过判断；可你第 1 张明明说了「短视频正在悄悄地让我们变笨」。",
			}}},
		},
		{
			name: "灾难化",
			res: writingCommentResult{Points: []CommentPoint{{
				Kind: "issue",
				Text: "读者会觉得你已经站到对面去了，全篇的结论跟着塌了。",
			}}},
		},
		{
			name: "她的字谁写都一样",
			res: writingCommentResult{Points: []CommentPoint{{
				Kind: "issue",
				Text: "这三处各说一边，换成谁来写都成立，你开头那句主张等于没说。",
			}}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := writingHostileTone(c.res); got == "" {
				t.Fatalf("这一句该被拦下来重问一次：%+v", c.res)
			}
		})
	}
}

// action 里那句祈使也要过这一关 —— 她三处都会读到。
func TestWritingHostileTone_ChecksTheActionToo(t *testing.T) {
	res := writingCommentResult{Points: []CommentPoint{{
		Kind: "issue", Text: "结尾和开头不是一个方向。",
		Action: "把这句删了重写，现在这样等于没说。",
	}}}
	if writingHostileTone(res) == "" {
		t.Fatal("action 里的挑衅没被看见")
	}
}

// 🚨 反面才是这张表真正的风险：指出问题本来就要转折，
// 把普通的转折收进来，每一轮都要多花一次调用。
func TestWritingHostileTone_LeavesOrdinaryCriticismAlone(t *testing.T) {
	fine := []writingCommentResult{
		{Summary: "这一段的例子具体、站得住，但摆完例子就直接收结论，中间少一句分析句。"},
		{Summary: "开头把结论说出来了，方向清楚；只是前两句绕了一圈才到题目。"},
		{Points: []CommentPoint{{
			Kind: "issue",
			Text: "这三处各指一个方向，读者读完会不确定你站哪一边。",
			// 提示词里给的那个改写样子，必须是干净的 —— 否则我们在教它一句会被自己拦下的话。
			Action: "先定一个方向，再把这三句改成同一个方向。",
		}}},
		{Points: []CommentPoint{{
			Kind:   "issue",
			Text:   "结尾这句回到了「有好有坏」，和前四段立的方向不是同一个。",
			Action: "把最后一句改成正面回答第 1 张那个问题。",
		}}},
		{Points: []CommentPoint{{
			Kind: "good",
			Text: "摸了五次手机、每次十几分钟，有次数有时长，读者一读就能看见那个场景。",
		}}},
	}
	for _, res := range fine {
		if got := writingHostileTone(res); got != "" {
			t.Fatalf("把一句正常的意见判成挑衅了（命中「%s」）：%+v", got, res)
		}
	}
}

// 三道闸的**顺序**：一次只说一件事，而且先说最伤人的那件。
func TestWritingCommentProblem_ReportsTheMostHarmfulFirst(t *testing.T) {
	// 既没有她能照着改的意见，总评又说「缺」，话还难听 —— 三样齐了。
	res := writingCommentResult{
		Verdict: writingVerdictPolish,
		Summary: "这一段缺一句分析句，写成这样等于没说。",
		Points:  []CommentPoint{{Kind: "good", Text: "例子挑得好。"}},
	}
	marker, nudge := writingCommentProblem(res, res.Points)
	if marker != writingVerdictNoPointMarker {
		t.Fatalf("该先说「没有一条她能照着改的」，却先说了 %q", marker)
	}
	if nudge != writingNoPointNudge {
		t.Fatal("提醒和命中的那道闸对不上")
	}
}

// pass 那一档本来就允许没有 issue —— 不能因此被拦。
func TestWritingCommentProblem_PassWithNoIssueIsFine(t *testing.T) {
	res := writingCommentResult{
		Verdict: writingVerdictPass,
		Summary: "这一段站得住，可以去写下一段。",
		Points:  []CommentPoint{{Kind: "good", Text: "例子具体。"}},
	}
	if marker, _ := writingCommentProblem(res, res.Points); marker != "" {
		t.Fatalf("pass 且没有 issue 是正常的，却被拦下了：%q", marker)
	}
}

// 🚨 闸门要查**她真的会看到的那一份**。
// 模型给了 issue、服务端把它丢掉了 —— 这正是 2026-09-21 修的那个 bug，
// 原来的闸门站在筛子上游，这一整类一次都看不见。
func TestWritingCommentProblem_LooksAtWhatSheActuallySees(t *testing.T) {
	raw := []CommentPoint{
		{Kind: "good", Text: "例子挑得好。"},
		{Kind: "issue", Text: "缺一句分析句。", Action: "补一句。", Quote: "某句"},
	}
	deliveredAfterTheFilterAteIt := []CommentPoint{{Kind: "good", Text: "例子挑得好。"}}

	res := writingCommentResult{Verdict: writingVerdictPolish, Summary: "这一段还差一步。", Points: raw}

	if marker, _ := writingCommentProblem(res, raw); marker != "" {
		t.Fatalf("模型原样那份是有 issue 的，不该拦：%q", marker)
	}
	marker, _ := writingCommentProblem(res, deliveredAfterTheFilterAteIt)
	if marker != writingVerdictNoPointMarker {
		t.Fatal("她看到的那份没有一条 issue，闸门必须拦住并重问一次")
	}
}
