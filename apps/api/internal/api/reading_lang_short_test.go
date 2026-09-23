package api

import "testing"

// 🚨 **线上走一遍真的诗才发现的。**
//
// 产品负责人 2026-09-23：「please always think from the students' perspective
// and really decide if the function is useful…… instead of just "all green",
// "no 50x".」—— 这一条正是那句话的产物：单测、契约、9 格矩阵全绿，
// 而线上拿《江雪》走一遍，回来的读法是 **en-narrative**。
//
// 原因：readingLangOf 原来只有一条判据 —— CJK 字数 ≥ 24 才算中文，否则英文。
// 而一首五言绝句正好 20 个字，**每一首都过不了那条线**。七绝 28 个字刚好过，
// 所以这件事一直没露头，直到诗词成为一种体裁、真的有人拿一首五绝进来。

func TestShortChinesePoemsAreChinese(t *testing.T) {
	poems := map[string]string{
		"江雪":   "千山鸟飞绝，万径人踪灭。\n\n孤舟蓑笠翁，独钓寒江雪。",
		"静夜思":  "床前明月光，疑是地上霜。\n\n举头望明月，低头思故乡。",
		"登鹳雀楼": "白日依山尽，黄河入海流。\n\n欲穷千里目，更上一层楼。",
		"春晓":   "春眠不觉晓，处处闻啼鸟。\n\n夜来风雨声，花落知多少。",
		// 五绝只有 20 个字 —— 这就是原来那条线拦下的长度。
		"相思": "红豆生南国，春来发几枝。\n\n愿君多采撷，此物最相思。",
	}
	for title, body := range poems {
		if got := readingLangOf(body); got != "zh" {
			t.Errorf("《%s》判成了 %q —— 一首中文诗会被排上英文那套读法，"+
				"段落工具条上摆的是翻译和查词", title, got)
		}
	}
}

// 🚨 反方向必须一起钉：把门放宽之后，英文文章不能跟着变成中文。
//
// 一篇夹了几个中文字的英文文章（引了一个人名、一句原话）照旧是英文。
func TestEnglishStaysEnglishEvenWithSomeChinese(t *testing.T) {
	cases := map[string]string{
		"纯英文短文":   "Schools should start an hour later. Teenagers need the sleep.",
		"英文里引了中文": `The sign read "小心地滑" — literally "careful, slippery floor".`,
		"英文里有人名":  "Li Hua (李华) wrote to the headmaster about the library.",
		"很短的英文":   "Hello.",
		"空的":      "",
	}
	for name, body := range cases {
		if got := readingLangOf(body); got != "en" {
			t.Errorf("%s 判成了 %q", name, got)
		}
	}
}

// 长文章那条快路一个字节都没变：够多的中文就是中文，不必数字母。
// 这一条守的是「改短文本的判据不许把长文章弄坏」。
func TestLongArticlesUnchanged(t *testing.T) {
	zh := ""
	for i := 0; i < 40; i++ {
		zh += "海水里的盐一部分来自陆地"
	}
	if got := readingLangOf(zh); got != "zh" {
		t.Errorf("长中文文章判成了 %q", got)
	}
	en := ""
	for i := 0; i < 60; i++ {
		en += "The ocean is salty because rivers carry minerals into it. "
	}
	if got := readingLangOf(en); got != "en" {
		t.Errorf("长英文文章判成了 %q", got)
	}
	// 🚨 一篇长英文里夹一整句中文：中文字数过了 24 那条快路。
	// 这是**改动之前就有的行为**，不是这次带来的 —— 记在这里，
	// 免得下一个人以为是新写的判据导致的。
	mixed := en + "这是一段很长的中文引文，用来说明上面那一段英文里讲的那件事，它有超过二十四个汉字。"
	if got := readingLangOf(mixed); got != "zh" {
		t.Logf("夹了长中文引文的英文文章判成 %q（改动前后一致）", got)
	}
}
