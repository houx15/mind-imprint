package api

import (
	"testing"

	"mindimprint/api/internal/store/sqlc"
)

// 用例的话都是 2026-09-17 入口走查里印记的原话。

func alignPlan(done int) []sqlc.ReadingTask {
	rows := []sqlc.ReadingTask{
		{Kind: "predict", Label: "先预测"},
		{Kind: "read", Label: "通读第1–2段·摆出现象"},
		{Kind: "read", Label: "通读第3–4段·亮出观点"},
		{Kind: "read", Label: "通读第5–6段·两方例证"},
		{Kind: "read", Label: "通读第7段·让步收束"},
		{Kind: "focus_block", Label: "精读重点段落第4段"},
	}
	for i := range rows {
		rows[i].Status = "pending"
		if i < done {
			rows[i].Status = "done"
		}
	}
	return rows
}

func TestAlignAdvance(t *testing.T) {
	cases := []struct {
		name    string
		done    int
		advance string
		reply   string
		want    string
	}{
		{
			// 上传那篇：当前是 5–6，话里才刚领她去读 5–6 —— 不能打勾。
			name: "introducing the current part is not finishing it", done: 3, advance: "done",
			reply: "你找得很准——就是这一句。\n\n走，往下读第5到第6段。这两段作者搬出了两个例子。读完告诉我，这两段各举了谁的例子。",
			want:  "",
		},
		{
			// 粘贴那篇：当前是 3–4，话里已经领她去读 5–6 —— 3–4 做完了。
			name: "sending her to the next part finishes this one", done: 2, advance: "",
			reply: "对，反对者的担心确实不是没道理的。\n\n接下来读第5到6段，看作者举了哪两个例子。",
			want:  "done",
		},
		{
			name: "和 joins the range too", done: 1, advance: "",
			reply: "好，前两段读完了。接下来我们看第3和第4段：作者要亮出观点了。",
			want:  "done",
		},
		{
			name: "a single-paragraph part", done: 3, advance: "",
			reply: "例子找对了。接下来读第7段，看作者怎么收尾。",
			want:  "done",
		},
		{
			// 先预测 → 第一部分：同样是「当前做完、去读下一步」。
			name: "predict hands over to the first part", done: 0, advance: "",
			reply: "猜得有意思。现在读第1到第2段，看作者先摆出了什么现象。",
			want:  "done",
		},
		{
			name: "a quote from a paragraph is not a directive", done: 2, advance: "",
			reply: "第3段那句「读书的价值不在于读了多少本」就是他的立场。你觉得他凭什么这么说？",
			want:  "",
		},
		{
			name: "looking back at a finished part changes nothing", done: 3, advance: "",
			reply: "你刚才读的那一段是第3到第4段。现在这一步请你读完第5–6段再告诉我。",
			want:  "",
		},
		{
			name: "pointing two parts ahead is left alone", done: 1, advance: "",
			reply: "接下来读第5到6段。",
			want:  "",
		},
		{
			name: "skipped is never overridden", done: 3, advance: "skipped",
			reply: "好，这部分先跳过。往下读第5到第6段也行，随你。",
			want:  "skipped",
		},
		{
			name: "no directive keeps the model's call", done: 3, advance: "done",
			reply: "两个例子你都找到了：一个是苏轼，一个是心理学研究。",
			want:  "done",
		},
	}
	for _, c := range cases {
		if got := alignAdvanceWithReply(alignPlan(c.done), c.advance, c.reply); got != c.want {
			t.Errorf("%s: advance = %q, want %q", c.name, got, c.want)
		}
	}
}
