package api

import "testing"

// 🚨 这两段是 2026-09-21 生产日志里模型**原样回的**，一个字没改。
// 两趟真学生走查都红在它们身上，她看到的是「后台错误：AI 响应错误」。
const proseFromProd1 = `想不出来没关系，这不用你凭空想——咱们去找。现在这里没有搜索框，你在浏览器里查一下，查到后把那句话或数字和出处告诉我。要找的是：**关于短视频或屏幕使用影响青少年注意力的研究或调查**，比如哪一年、哪家机构、测的是什么、结论那句话。你查到什么直接贴过来就行，我来帮你看看它能不能用。

如果你这会儿不方便查，也有一条不查就有的路：历史或语文课里读过的人，有没有谁是靠长时间专注才做成大事的？想到谁，说名字就行。`

const proseFromProd2 = `没卡住，咱们不写新东西，就改你已经想出来的那两句。「变笨」不一定只指脑子空——注意力碎了、读不进长的东西，其实都是「笨」的某种样子。试着这样问自己：刷短视频刷得注意力被切碎之后，**这件事让「我们」做不成什么了**？比如考试读题、看懂一篇长文章——那种做不成的样子，就是你要写的「变笨」。你想从哪一条开始改？`

func TestParseWritingPlanReply_KeepsAProseReplyInsteadOfDyingOnIt(t *testing.T) {
	for i, raw := range []string{proseFromProd1, proseFromProd2} {
		got, ok := parseWritingPlanReply(raw)
		if !ok {
			t.Fatalf("第 %d 段：整份是一句好好的人话，却被判成解析失败 —— "+
				"她那边会看到一个转不动的终端", i+1)
		}
		if got.Reply != raw {
			t.Fatalf("第 %d 段：交出去的不是模型原样说的那句话", i+1)
		}
		// 🚨 一个节点都不许加：模型这一轮本来就没打算往图上放东西。
		if len(got.Add) != 0 {
			t.Fatalf("第 %d 段：凭空往她的图上加了 %d 个节点", i+1, len(got.Add))
		}
		if got.Ready {
			t.Fatalf("第 %d 段：ready 该是 false", i+1)
		}
	}
}

// 🚨 半句话仍然报错。这道关是 2026-09-12 补的，一个字都不放松：
// 宁可让她重发一次，也不要把没说完的话摆给她看。
func TestWritingPlanReplyFromProse_StillRefusesAHalfSentence(t *testing.T) {
	half := `想不出来没关系，这不用你凭空想——咱们去找。现在这里没有搜索框，你在浏览器里查一下，查到后把那句话`
	if _, ok := writingPlanReplyFromProse(half); ok {
		t.Fatal("一句没说完的话被当成印记说的话交出去了")
	}
}

// 断掉的 JSON 不从这条路走 —— 它归 salvageWritingPlanReply 管，
// 那条只取 reply 字段。从这里走会把满屏的 {"add":[… 印到她脸上。
func TestWritingPlanReplyFromProse_LeavesBrokenJSONToTheOtherSalvage(t *testing.T) {
	for _, s := range []string{
		`{"reply":"想不出来没关系，咱们去找。","add":[{"text":"短视频`,
		`{"reply":"这一轮先不加东西。"}`,
		`[{"text":"某条"}]`,
		`看起来像话但其实带着字段名 "add": [ 的一段。`,
	} {
		if _, ok := writingPlanReplyFromProse(s); ok {
			t.Fatalf("这份是 JSON，不该走人话那条路：%s", s)
		}
	}
}

// 太短的不算一轮陪练发言。
func TestWritingPlanReplyFromProse_RefusesAStrayToken(t *testing.T) {
	for _, s := range []string{"", "   ", "好的。", "嗯？"} {
		if _, ok := writingPlanReplyFromProse(s); ok {
			t.Fatalf("把 %q 当成了一轮陪练发言", s)
		}
	}
}

// 正常的 JSON 一点都不受影响 —— 这条路只在解析失败之后才走得到。
func TestParseWritingPlanReply_HealthyJSONUnaffected(t *testing.T) {
	raw := `{"reply":"好，主张定下来了。你打算用哪件事来说明？","add":[{"text":"短视频让我们变笨了","kind":"thesis"}],"ready":false}`
	got, ok := parseWritingPlanReply(raw)
	if !ok {
		t.Fatal("一份好好的 JSON 解析失败了")
	}
	if got.Reply != "好，主张定下来了。你打算用哪件事来说明？" {
		t.Fatalf("reply 不对：%q", got.Reply)
	}
	if len(got.Add) != 1 || got.Add[0].Kind != "thesis" {
		t.Fatalf("节点不对：%+v", got.Add)
	}
}
