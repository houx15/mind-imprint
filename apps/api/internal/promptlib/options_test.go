package promptlib

import (
	"strings"
	"testing"
)

// 说谎的引子：源数据已经把两个题目拆成了两条，引子留着没删。
// 同事 2026-09-22：「下图说从两个题目中任选一题，但下面只显示了一个题」。
func TestSplitPromptText_StaleStemGoesAway(t *testing.T) {
	// GKYW-004 的真实题面（已经过 CleanPromptText）。
	in := CleanPromptText(`从下面两个题目中任选一题，按要求作答。不少于700字。不透露所在区、学校及个人信息。
（1）学海无涯，读书有法。元代学者程端礼编撰的《读书分年日程》，分阶段详细规定了核心经典的阅读顺序与精读方法。
请以“做规划与下功夫”为题目，写一篇议论文。`)

	parts := SplitPromptText(in)
	if len(parts) != 1 {
		t.Fatalf("只有一个题目，不该拆：得到 %d 份", len(parts))
	}
	got := parts[0].Text
	if parts[0].Suffix != "" {
		t.Errorf("不拆的时候 id 一个字都不该动，却给了后缀 %q", parts[0].Suffix)
	}
	// 「不少于700字」也从引子里去掉：卡片上本来就单独有一栏字数
	// （writingPromptDTO.WordLimit，截图里那行「命题作文（议论文）·不少于700字·
	// 50 分」），留在这里，题面的第一行就成了「不少于700字。」。
	for _, bad := range []string{"任选一题", "按要求作答", "不透露", "（1）", "不少于700字"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q 还在：\n%s", bad, got)
		}
	}
	// 🚨 真话要留着：整段材料和那个题目。
	for _, keep := range []string{"程端礼", "做规划与下功夫"} {
		if !strings.Contains(got, keep) {
			t.Errorf("%q 被砍掉了：\n%s", keep, got)
		}
	}
}

// 引子那一行里除了「任选一题」还有真话，只能丢掉那一小句。
func TestSplitPromptText_KeepsTheRestOfTheStemLine(t *testing.T) {
	in := `从下面两个题目中任选一题，根据所给的中文和英文提示，完成一篇不少于50词的英语文段写作。
题目①
假设你是李华，你校英语社团将接待国外学生代表团来访。`
	parts := SplitPromptText(in)
	if len(parts) != 1 {
		t.Fatalf("只有一个题目：得到 %d 份", len(parts))
	}
	got := parts[0].Text
	if strings.Contains(got, "任选一题") {
		t.Errorf("引子没去掉：\n%s", got)
	}
	if !strings.Contains(got, "根据所给的中文和英文提示") || !strings.Contains(got, "不少于50词") {
		t.Errorf("🚨 整句丢掉就不知道要写英文短文了：\n%s", got)
	}
}

// 真的有两个题目：拆成两份，共用的材料跟着走。
func TestSplitPromptText_SharedMaterialTravelsWithEveryOption(t *testing.T) {
	// ZKYW-098：材料在引子和选项之间。
	in := `阅读下面的材料，任选一题作文。
人在年少时，总以为要得到珍贵的东西必须付出昂贵的代价。随着年龄渐长、阅历渐深，逐渐明白：世上最珍贵的东西，往往都是免费的。
（1）请结合你的体验和思考，以“最珍贵的是______”为题，写一篇记叙文。
（2）上述材料引发了你怎样的感悟和思考？请自选角度，自拟题目，写一篇议论文。`
	parts := SplitPromptText(in)
	if len(parts) != 2 {
		t.Fatalf("两个题目该拆成两份：得到 %d 份", len(parts))
	}
	if parts[0].Suffix != "-1" || parts[1].Suffix != "-2" {
		t.Errorf("后缀不对：%q / %q", parts[0].Suffix, parts[1].Suffix)
	}
	for i, p := range parts {
		// 🚨 材料在选项前面。不跟着走，第二张卡上就只剩一句
		// 「请自选角度，自拟题目，写一篇议论文」—— 那道题的材料整段不见了。
		if !strings.Contains(p.Text, "世上最珍贵的东西") {
			t.Errorf("第 %d 份丢了共用的材料：\n%s", i+1, p.Text)
		}
		if !strings.Contains(p.Text, "阅读下面的材料") {
			t.Errorf("第 %d 份丢了引子剩下的那句真话：\n%s", i+1, p.Text)
		}
	}
	if strings.Contains(parts[0].Text, "自拟题目") {
		t.Errorf("第 1 份里混进了第 2 个题目：\n%s", parts[0].Text)
	}
	if strings.Contains(parts[1].Text, "写一篇记叙文") {
		t.Errorf("第 2 份里混进了第 1 个题目：\n%s", parts[1].Text)
	}
}

// 🚨 结尾那一段归谁 —— 空行就是那条界线。
func TestSplitPromptText_TailBelongsToTheLastOptionUnlessABlankLineSaysOtherwise(t *testing.T) {
	// ZKYW-039：「注意：……给父母的一封信」紧贴着（2），只属于（2）。
	glued := `以下两题，选做一题。
（1）阅读下面这首小诗，自选角度，自拟题目，写一篇文章。
（2）初中毕业后的暑假，你需要给父母写一封信。
注意：①请在作文第一行居中写“给父母的一封信”；②文末署名“小渝”。`
	parts := SplitPromptText(glued)
	if len(parts) != 2 {
		t.Fatalf("得到 %d 份", len(parts))
	}
	if strings.Contains(parts[0].Text, "给父母的一封信") {
		t.Errorf("紧贴着（2）的注意事项跑到（1）上去了：\n%s", parts[0].Text)
	}
	if !strings.Contains(parts[1].Text, "文末署名") {
		t.Errorf("（2）自己的注意事项丢了：\n%s", parts[1].Text)
	}

	// ZKYY-073：答题模板隔着一个空行，两个选择都要用它。
	spaced := `你校English Reading Club正在组织一次英文小说读书分享会。请从以下两个标题中任选一个，写一篇英文读后感。
选择一：以Sometimes people surprise us为题，内容包括：
选择二：以Trouble is a friend为题，内容包括：

Title: ____________
After reading Auggie's story, I'm really touched.`
	parts = SplitPromptText(spaced)
	if len(parts) != 2 {
		t.Fatalf("得到 %d 份", len(parts))
	}
	for i, p := range parts {
		if !strings.Contains(p.Text, "After reading Auggie") {
			t.Errorf("第 %d 份丢了共用的答题模板：\n%s", i+1, p.Text)
		}
		if !strings.Contains(p.Text, "English Reading Club") {
			t.Errorf("第 %d 份丢了共用的情景：\n%s", i+1, p.Text)
		}
	}
	if !strings.Contains(parts[0].Text, "Sometimes people surprise us") ||
		strings.Contains(parts[0].Text, "Trouble is a friend") {
		t.Errorf("两个标题没分开：\n%s", parts[0].Text)
	}
}

// 「26.题目：」那种标号 —— 数字去掉，「题目：」留着。
// ZKYW-008 的第二个选项是「27.题目：______长伴我左右」：把「题目：」也砍了，
// 那一行就只剩一个填空，看不出它是要她补全的标题。
func TestSplitPromptText_NumberedTitleKeepsTheWordTitle(t *testing.T) {
	in := `请从下面的题目中任选一题完成写作。
26.题目：山坡在山脚和山顶之间。请以“走在山坡上”为题写一篇作文。
27.题目：______长伴我左右`
	parts := SplitPromptText(in)
	if len(parts) != 2 {
		t.Fatalf("得到 %d 份", len(parts))
	}
	if strings.HasPrefix(parts[1].Text, "27.") {
		t.Errorf("题号没去掉：\n%s", parts[1].Text)
	}
	if !strings.Contains(parts[1].Text, "题目：______长伴我左右") {
		t.Errorf("「题目：」被砍掉了，那一行看不出是个要补全的标题：\n%s", parts[1].Text)
	}
}

// 没有引子的题面一个字都不该动。
func TestSplitPromptText_LeavesOrdinaryPromptsAlone(t *testing.T) {
	for _, in := range []string{
		"请以“做规划与下功夫”为题目，写一篇议论文。",
		"阅读下面的材料，根据要求写作。\n词语是表达思想情感的载体。\n请结合自身经历，写一篇文章。",
		// 「选择一个」不是「任选一题」：这是题目自己的话。
		"请从“说谢谢”“往前走”“看见美”中选择一个，写一篇作文。",
	} {
		parts := SplitPromptText(in)
		if len(parts) != 1 || parts[0].Text != in {
			t.Errorf("被改了：\n输入 %q\n得到 %#v", in, parts)
		}
	}
}

// 整库跑一遍：拆完之后一个题目都不许消失，每一份都得有实质内容。
func TestSplitPromptText_WholeLibrary(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		for _, part := range SplitPromptText(p.Text) {
			// 🚨 下限只有四个字，不是十个：命题作文的一个选项就是一个标题
			// （ZKYW-056 的「无声之处，亦有深味」、ZKYW-018 的「读懂眼眸中的
			// 深意」）。按「一道题该有多长」去判，会把这一整类判成拆坏了，
			// 然后逼我把拆法改松。这里要守的是**没有剩下一个光秃秃的标号**。
			if len([]rune(strings.TrimSpace(part.Text))) < 4 {
				t.Errorf("%s%s 拆出来几乎是空的：%q", p.ID, part.Suffix, part.Text)
			}
			if strings.Contains(part.Text, "任选一题") || strings.Contains(part.Text, "选做一题") {
				t.Errorf("%s%s 里还留着引子：\n%s", p.ID, part.Suffix, part.Text)
			}
		}
	}
}
