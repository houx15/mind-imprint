package api

import (
	"strings"
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// words 在 writing_lang_internal_test.go 里已经有了，同一个包，直接用。

// 第三十三轮那个学生站的位置：目标 800 字，手上 590 字左右，还差两百多。
//
// 她的原话：「它说这样能帮我凑够两百字，但就加一句话怎么可能多出两百字啊……」
// 所以这个缺口下，上文必须明确拦住「一句话尺寸」的动作。
func TestWritingLengthGap_BigGapForbidsASentenceSizedAction(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", TargetWords: words(800)}
	body := strings.Repeat("这是一段正文。", 85) // 510 字（句号不计），差 290

	got := writingLengthGapBlock(wr, body)

	if !strings.Contains(got, "离目标还差") {
		t.Fatalf("没说还差多少：\n%s", got)
	}
	if !strings.Contains(got, "一句话补不上") {
		t.Errorf("缺口这么大，必须拦住一句话尺寸的动作：\n%s", got)
	}
	if !strings.Contains(got, "不要让她凑字数") {
		t.Errorf("铁律①：说的是往哪儿深下去，不是凑字数：\n%s", got)
	}
}

// 篇幅到了要说出口 —— 她的另一条卡壳是「不知道是让我自己随便加，还是点完成这篇」。
func TestWritingLengthGap_SaysWhenSheIsDone(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", TargetWords: words(800)}
	for _, body := range []string{
		strings.Repeat("这是一段正文。", 120), // 720 字，差 80 —— 正好一成
		strings.Repeat("这是一段正文。", 140), // 840 字，超了
	} {
		got := writingLengthGapBlock(wr, body)
		if !strings.Contains(got, "篇幅已经到了") {
			t.Errorf("到了就要说出口（%d 字）：\n%s", countWordsForLang(body, "zh"), got)
		}
		if strings.Contains(got, "一句话补不上") {
			t.Errorf("篇幅够了还在说缺口：\n%s", got)
		}
	}
}

// 还没落笔的时候不说 —— 那时候「差 800 字」是废话，而且像催。
func TestWritingLengthGap_SilentBeforeSheStarts(t *testing.T) {
	wr := sqlc.Writing{Lang: "zh", TargetWords: words(800)}
	for _, body := range []string{"", "   \n  "} {
		if got := writingLengthGapBlock(wr, body); got != "" {
			t.Errorf("还没开始写就谈缺口：%q", got)
		}
	}
	// 没设目标也不说。
	if got := writingLengthGapBlock(sqlc.Writing{Lang: "zh"}, "写了一些"); got != "" {
		t.Errorf("没定目标篇幅还在谈缺口：%q", got)
	}
}

// 🚨 英文按词算。这一族在这个仓库里栽过一次（500 词的英文作文被当成 500 个
// 汉字，建议直接错了五倍），所以单独钉一条。
func TestWritingLengthGap_EnglishCountsWords(t *testing.T) {
	wr := sqlc.Writing{Lang: langEnglish, TargetWords: words(300)}
	body := strings.Repeat("the canteen bucket was full again ", 30) // 180 词

	got := writingLengthGapBlock(wr, body)
	if !strings.Contains(got, "180") || !strings.Contains(got, "120") {
		t.Fatalf("英文该数词（现在 180、差 120）：\n%s", got)
	}
	// 单位要跟着语言走。英文那一篇的单位是「词」不是「字」（lengthUnit 定的，
	// 提示词本身是中文，所以单位词也是中文）。
	// 比的是**数字后面那个词**，不是整段里有没有「字」—— 那段给模型的话里
	// 本来就有「不要让她凑字数」。
	if !strings.Contains(got, "180 词") || !strings.Contains(got, "120 词") {
		t.Errorf("英文那一篇的单位该是「词」：\n%s", got)
	}
	if strings.Contains(got, "180 字") {
		t.Errorf("英文那一篇用了「字」做单位 —— 这一族在这个仓库里栽过一次：\n%s", got)
	}
}
