package api

import "testing"

// 她真的写过的那一段（真学生走查里的分论点 2）。
const salvageSource = `其次，短视频让我们越来越不喜欢看长的东西了。以前我还能看完一本小说，现在看两页就想去刷手机。很多人都说自己现在没有耐心了。短视频一个只有十几秒，看完一个马上就有下一个，我们已经习惯了这种快节奏。`

// 🚨 这三条 text 是 `TestLiveDroppedIssue` 三趟里真模型原样写出来的，
// 一个字都没改。三趟的 quote 那一格都是空的，三趟都被判了 no_quote 丢掉。
func TestSalvageQuoteFromText_TheThreeLiveFailures(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			name: "第 1 趟",
			text: `「以前我还能看完一本小说，现在看两页就想去刷手机」这个例子摆完就直接跳到「很多人都说自己没有耐心了」，中间少了一句话把这两件事连起来`,
			want: "以前我还能看完一本小说，现在看两页就想去刷手机",
		},
		{
			name: "第 2 趟",
			text: `「以前能看完一本小说，现在看两页就想刷手机」是这段最实的材料，但它摆在那里就过去了`,
			// 🚨 模型漏抄了「我还」。逐字对不上，所以不能拿它当锚点 ——
			// quotematch 只补标点空格，不补内容。这一趟捞不到，照旧丢掉。
			want: "",
		},
		{
			name: "第 3 趟：第一处漏字，第二处逐字",
			text: `「以前能看完一本小说」是好事例，但「看完一个马上就有下一个」是在继续描述短视频`,
			want: "看完一个马上就有下一个",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := salvageQuoteFromText(c.text, salvageSource)
			if got != c.want {
				t.Fatalf("捞出来的是 %q，该是 %q", got, c.want)
			}
		})
	}
}

// 取最长的那个：锚点越长越具体，划在她屏幕上越说明问题。
func TestSalvageQuoteFromText_PrefersTheLongestVerbatimSpan(t *testing.T) {
	text := `「快节奏」只是个词，真正实的是「以前我还能看完一本小说，现在看两页就想去刷手机」`
	got := salvageQuoteFromText(text, salvageSource)
	want := "以前我还能看完一本小说，现在看两页就想去刷手机"
	if got != want {
		t.Fatalf("捞出来的是 %q，该取最长的那个 %q", got, want)
	}
}

// 🚨 载重的那条保证一个字都不能松：捞出来的必须**逐字**是她写的。
func TestSalvageQuoteFromText_NeverInvents(t *testing.T) {
	for _, text := range []string{
		`「短视频让人变得越来越笨」这句话你说得太满了`, // 她根本没写过这句
		`「」空的`,
		`没有任何引号的一段话`,
		`「太短」`, // 少于 4 个字，划出来没意义
	} {
		if got := salvageQuoteFromText(text, salvageSource); got != "" {
			t.Fatalf("从 %q 里捞出了 %q —— 这不是她写的字", text, got)
		}
	}
}

// 只差标点的，交给 quotematch 从原文里取回真的那一段。
func TestSalvageQuoteFromText_PunctuationOnlyDifferenceIsStillHers(t *testing.T) {
	text := `「以前我还能看完一本小说现在看两页就想去刷手机」摆完就过去了`
	got := salvageQuoteFromText(text, salvageSource)
	if got == "" {
		t.Fatal("只差一个逗号，该从原文里取回那一段")
	}
	if !containsStr(salvageSource, got) {
		t.Fatalf("取回来的 %q 不在她的原文里", got)
	}
}

// 英文引号那一套也认。
func TestSalvageQuoteFromText_English(t *testing.T) {
	src := `Every day the canteen throws away many rice. Last Friday I counted six bins are full.`
	text := `"Last Friday I counted six bins are full" is the only concrete thing here.`
	got := salvageQuoteFromText(text, src)
	if got != "Last Friday I counted six bins are full" {
		t.Fatalf("英文那一套没捞到：%q", got)
	}
}

// 整条链路：quote 空着的一条 issue，捞回来之后要**活着通过校验**。
// 这是线上真正坏掉的那一步 —— 上面几条只证明捞得出来，这一条证明它没被丢掉。
func TestValidateCommentPoints_KeepsAnIssueWhoseQuoteWasOnlyInTheText(t *testing.T) {
	in := []CommentPoint{{
		Kind:    "issue",
		Symptom: "evidence_not_explained",
		Text:    `「以前我还能看完一本小说，现在看两页就想去刷手机」这个例子摆完就直接跳走了，中间缺一句分析句。`,
		Action:  "在例子后面补一句分析句，用因果分析法把这两件事连起来。",
		Quote:   "", // 🚨 线上就是这样回的
	}}
	got, drops := validateCommentPointsVerbose(in, salvageSource, "zh", writingBlockCommentMaxIssues)
	if len(got) != 1 {
		t.Fatalf("这条意见被丢掉了（drops=%+v）—— 她又拿不到任何可以照着改的话", drops)
	}
	if got[0].Quote != "以前我还能看完一本小说，现在看两页就想去刷手机" {
		t.Fatalf("锚点不对：%q", got[0].Quote)
	}
	if !containsStr(salvageSource, got[0].Quote) {
		t.Fatal("锚点不是逐字她写的 —— 载重的那条保证破了")
	}
}

// 已经给了 quote 的，不许被 text 里的东西盖掉。
func TestValidateCommentPoints_DoesNotOverrideAQuoteThatWasGiven(t *testing.T) {
	in := []CommentPoint{{
		Kind:    "issue",
		Symptom: "evidence_not_explained",
		Text:    `「以前我还能看完一本小说，现在看两页就想去刷手机」这里`,
		Action:  "补一句分析句。",
		Quote:   "很多人都说自己现在没有耐心了",
	}}
	got, _ := validateCommentPointsVerbose(in, salvageSource, "zh", writingBlockCommentMaxIssues)
	if len(got) != 1 || got[0].Quote != "很多人都说自己现在没有耐心了" {
		t.Fatalf("模型自己给的 quote 被换掉了：%+v", got)
	}
}

func containsStr(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && idxOf(s, sub) >= 0
}

func idxOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
