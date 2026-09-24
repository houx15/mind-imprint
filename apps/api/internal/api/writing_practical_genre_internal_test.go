package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 应用文那一档装的不只是信。
//
// 🚨 2026-09-24 查出来的：「写一则通知」「写一篇演讲稿」两张词表一条都不命中，
// 于是落到 writingGenreOf 最后那句「拿不准就是议论文」—— 一则通知被要求写
// 中心论点、分论点和论据。这正是产品负责人当初报的那个毛病
// （「a letter under the structure of 议论文」），只不过换了一种应用文。
//
// 倡议书本来就在词表里，所以这一档从一开始就不只是「信」，只是名单漏了几种。
func TestPracticalFormsDoNotFallThroughToArgument(t *testing.T) {
	titles := []string{
		"写一则通知，告诉同学们运动会改期",
		"以学生会的名义写一份公告",
		"写一篇演讲稿，题目是我的梦想",
		"国旗下讲话发言稿",
		"给校长写一封邮件",
		"写一份调查报告",
		"给校刊投稿",
		"Write a notice for the school noticeboard",
		"Write a speech for the English speech contest",
		"Write a news report about the sports meeting",
	}
	for _, title := range titles {
		got := writingGenreOf(sqlc.Writing{Title: title}, nil)
		if got == genreArgument {
			t.Errorf("「%s」被判成了议论文 —— 它会拿到中心论点/分论点/论据", title)
			continue
		}
		if got != genreLetter {
			t.Errorf("「%s」判成了 %q，要的是应用文那一档", title, got)
		}
	}
}

// 🚨 反方向，这一条比上面那条更容易坏：**议论文不许被收进应用文**。
//
// 加词表有一种对称的翻车方式 —— 词收得太宽，一道议论文题里偶然出现
// "notice" / "report" 就被判成应用文，而她看到的是「印记按应用文在教」。
// 所以英文那几条收的都是带冠词或动词的整串，不收光秃秃的单词。
func TestArgumentTitlesAreNotSweptIntoPractical(t *testing.T) {
	titles := []string{
		"学校应不应该允许学生带手机",
		"Should students be allowed to use phones at school?",
		"Many students notice that their sleep is getting shorter. Do you agree?",
		"Scientists report that the climate is changing. Discuss.",
		"谈谈你对人工智能的看法",
	}
	for _, title := range titles {
		if got := writingGenreOf(sqlc.Writing{Title: title}, nil); got == genreLetter {
			t.Errorf("「%s」被判成了应用文", title)
		}
	}
}

// 她在界面上看到的那个词要和这一档真正装的东西对得上。
//
// 只写「书信」的时候，一个写通知的学生看到的是「印记按书信在教这一篇」——
// 那是句假话，而且她会以为自己选错了文体。
func TestPracticalGenreLabelIsNotJustLetters(t *testing.T) {
	label := writingGenreLabel(genreLetter)
	if label == "书信" {
		t.Error("这一档还叫「书信」，而它装着通知、演讲稿、倡议书")
	}
	if !strings.Contains(label, "应用文") {
		t.Errorf("这一档的名字是 %q，读不出它装的是应用文", label)
	}
	// 选项卡上的那一句要说清什么时候选它，并且举到信以外的形态。
	var blurb string
	for _, c := range writingGenreChoices() {
		if c.ID == genreLetter {
			blurb = c.Blurb
		}
	}
	if blurb == "" {
		t.Fatal("选项里没有应用文这一档")
	}
	for _, want := range []string{"通知", "演讲稿"} {
		if !strings.Contains(blurb, want) {
			t.Errorf("那一句里没举到 %q，学生写通知时不会选这一档", want)
		}
	}
}
