package docextract

import (
	"strings"
	"testing"
)

// 页眉、页脚、页码只在这一层去得掉 —— 页的边界只在这一层存在。
//
// 两处都吃这个亏：阅读室按空行切块，一行「第 3 页」会变成一个独立的段落块，
// 印记 会把它当一句话来引；写作那边的「重复说法」计数只看得到最终文本，
// 每页都出现的页眉会被如实数成「她重复了 12 次」。

func TestStripFurnitureRemovesRunningHeaderAndPageNumbers(t *testing.T) {
	pages := []string{
		"中国教育财政研究简报\n第一段正文，讲的是这项研究要回答的问题。\n1",
		"中国教育财政研究简报\n第二段正文，给出样本的来源和范围。\n2",
		"中国教育财政研究简报\n第三段正文，说明数据是怎么采集的。\n3",
		"中国教育财政研究简报\n第四段正文，给出主要结论。\n4",
	}
	got := strings.Join(stripFurniture(pages), "\n\n")

	if strings.Contains(got, "中国教育财政研究简报") {
		t.Errorf("每页都出现的页眉没去掉：\n%s", got)
	}
	for _, n := range []string{"\n1", "\n2", "\n3", "\n4"} {
		if strings.Contains(got, n) {
			t.Errorf("页码行没去掉（%q）：\n%s", n, got)
		}
	}
	// 正文一个字都不能少。
	for _, want := range []string{"要回答的问题", "样本的来源", "怎么采集", "主要结论"} {
		if !strings.Contains(got, want) {
			t.Errorf("正文被误删了：%q\n%s", want, got)
		}
	}
}

// 🚨 判据是**位置 + 重复**，不是长相。靠长相猜（「短的、带数字的」）会误伤
// 正文里真正的短句和数据行。
func TestStripFurnitureKeepsRepeatedProse(t *testing.T) {
	// 同一句话出现在两页的**中间**，不是页眉 —— 那可能正是她写了两遍，
	// 而「她写了两遍」是写作那边要看见的事实，不能在这里替她抹掉。
	pages := []string{
		"开头一段。\n教育公平是社会公平的基石。\n结尾一段。",
		"另一段开头。\n教育公平是社会公平的基石。\n另一段结尾。",
		"第三页开头。\n第三页中间。\n第三页结尾。",
	}
	got := strings.Join(stripFurniture(pages), "\n\n")
	if strings.Count(got, "教育公平是社会公平的基石") != 2 {
		t.Errorf("正文里重复的句子被当成页眉抹掉了：\n%s", got)
	}
}

func TestStripFurnitureLeavesShortDocumentsAlone(t *testing.T) {
	// 两页的文档里一行出现两次，很可能只是巧合 —— 「重复」这个判据立不住。
	pages := []string{
		"某某研究\n第一段正文。",
		"某某研究\n第二段正文。",
	}
	got := strings.Join(stripFurniture(pages), "\n\n")
	if !strings.Contains(got, "某某研究") {
		t.Errorf("只有两页就不该按重复去页眉：\n%s", got)
	}
}

func TestPageNumberOnlyLines(t *testing.T) {
	for _, ln := range []string{"12", "- 12 -", "第 12 页", "Page 12", "12 / 30", "3"} {
		if !pageNumberOnly.MatchString(ln) {
			t.Errorf("这是一行页码，没认出来：%q", ln)
		}
	}
	// 🚨 正文里带数字的句子一句都不能误伤。
	for _, ln := range []string{
		"2013 年全国共有 2 853 个县级行政区。",
		"78.1%",
		"第 12 段讲的是这件事。",
		"12 个样本",
	} {
		if pageNumberOnly.MatchString(ln) {
			t.Errorf("正文行被当成页码：%q", ln)
		}
	}
}
