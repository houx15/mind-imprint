package promptlib

import (
	"strings"
	"testing"
)

// 🚨 左边这些**全是库里真有的题面**，一个字没改。
func TestCleanPromptText_RealExamScaffolding(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"中考语文：三层脚手架压在题目上",
			"第三部分 写作（共50分）\n五、（1小题，50分）\n20. 请从“说谢谢”“往前走”“看见美”中选择一个，将题目《那个教会我____的人》补充完整，写一篇文章。（50分）",
			"请从“说谢谢”“往前走”“看见美”中选择一个，将题目《那个教会我____的人》补充完整，写一篇文章。",
		},
		{
			"中考英语：第二节 书面表达(满分10分)",
			"第二节 书面表达(满分10分)\n你校校刊英语专栏正在开展以\"Science and Me\"为题的征文比赛。",
			"你校校刊英语专栏正在开展以\"Science and Me\"为题的征文比赛。",
		},
		{
			"中考英语：两层，外加「共两节」",
			"四、读写结合（共两节，满分25分）\n第二节 书面表达（共1题，满分15分）\n美好人生需要方向指引。",
			"美好人生需要方向指引。",
		},
		{
			"高考语文：题号 + 分值",
			"23．阅读下面的材料，根据要求写作。（60分）",
			"阅读下面的材料，根据要求写作。",
		},
		{
			"栏目名单独成行",
			"微写作（10分）\n请以“我的一天”为题写一段话。",
			"请以“我的一天”为题写一段话。",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CleanPromptText(c.in); got != c.want {
				t.Fatalf("清出来的是：\n%q\n该是：\n%q", got, c.want)
			}
		})
	}
}

// 🚨 反面才是真正的风险：**把题目本身砍掉**。
// 这一类错了不会报错，只会让她拿到一道读不懂的题。
func TestCleanPromptText_NeverEatsTheTaskItself(t *testing.T) {
	keep := []string{
		// 括号里不是分数，一个字都不许动。
		"请以“桥”为题写一篇文章（不少于两种修辞手法）。",
		"阅读下面的材料（选自《平凡的世界》），根据要求写作。",
		// 「第三部」在这里是内容，不是卷面结构。
		"第三部电影上映之后，观众的评价出现了分歧。请就此写一篇评论。",
		// 中间出现的「第二节」是材料的一部分。
		"阅读下面的材料：\n第二节课的铃声响了，他还坐在操场上。\n根据材料写一篇记叙文。",
		// 英文那一套原样保留 —— Directions 是任务本身，不是脚手架。
		"Directions: Read the following passage. Summarize the main idea in no more than 60 words.",
		// 数字开头但不是题号。
		"2024 年，某地推行了一项新政策。请就此写一篇议论文。",
	}
	for _, s := range keep {
		if got := CleanPromptText(s); got != s {
			t.Fatalf("动了不该动的：\n原：%q\n后：%q", s, got)
		}
	}
}

func TestCleanPromptText_TidiesWhitespace(t *testing.T) {
	got := CleanPromptText("第二节 书面表达\n\n\n\n请写一篇短文。   \n\n\n再写一段。\n\n")
	want := "请写一篇短文。\n\n再写一段。"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// 整份库过一遍：清完不许有空题面，也不许把题面清短一大截。
//
// 🚨 后半条是这条测试真正的价值：规则写宽一格，最容易的失败不是留下垃圾，
// 而是**把一道题吃掉一半**，而那种失败在抽查里看不出来。
func TestCleanPromptText_WholeLibraryStaysIntact(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		got := CleanPromptText(p.Text)
		if got == "" {
			t.Fatalf("%s 清完是空的：\n%q", p.ID, p.Text)
		}
		// 🚨 这里原来用的是「清掉的字不超过三成」。**那个判据是错的**：
		// ZKYW-092 三行脚手架压着一行短题目，清掉 41% 完全正确
		// （那正是产品负责人举的例子）。按比例判，只会逼我把规则改松。
		//
		// 真正要守的是两件具体的事：
		//   ① 只砍掉开头很少的几行 —— 砍多了就是在吃题面。
		//   ② 题目的**结尾**还在 —— 任务的落点几乎总在最后一句。
		if before, after := strings.Count(p.Text, "\n"), strings.Count(got, "\n"); before-after > 4 {
			t.Fatalf("%s 砍掉了 %d 行，脚手架没有这么厚：\n清后：%q\n原文：%q",
				p.ID, before-after, got, p.Text)
		}
		tail := lastRunes(stripScores(strings.TrimSpace(p.Text)), 12)
		if tail != "" && !strings.Contains(got, tail) {
			t.Fatalf("%s 的结尾没了（%q）——任务的落点被吃掉了：\n清后：%q", p.ID, tail, got)
		}
	}
}

// 幂等：已经清过的再清一遍不该再变。编译流水线会反复跑。
func TestCleanPromptText_Idempotent(t *testing.T) {
	all, _ := All()
	for _, p := range all {
		once := CleanPromptText(p.Text)
		if twice := CleanPromptText(once); twice != once {
			t.Fatalf("%s 清第二遍又变了：\n一遍：%q\n两遍：%q", p.ID, once, twice)
		}
	}
}

func lastRunes(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[len(r)-n:])
}
