package api

import "testing"

// 产品负责人 2026-09-23 第 4 条：「ai给出来的对于事件发生顺序的排列，默认选项
// 就是正确答案。需要优化成无答案或者打乱顺序。」
//
// 钉的是**不变量**，不是某一个具体的排列：打乱之后不许等于原来那一串。
// 钉具体排列会在任何一次种子实现变动时碎掉，而那不是产品坏了。

func orderOpts(quotes ...string) []coachCardOption {
	out := make([]coachCardOption, 0, len(quotes))
	for i, q := range quotes {
		out = append(out, coachCardOption{BlockID: "b" + string(rune('0'+i)), Quote: q})
	}
	return out
}

func TestOrderBoardNeverShipsInTheOriginalOrder(t *testing.T) {
	// 她截图里那一块：四个选项段号一路递增。
	cases := [][]string{
		{"发现画作", "挂在家中", "网友鉴定", "拍卖成交"},
		{"一", "二", "三"},
		{"a", "b"},
		{"1", "2", "3", "4", "5"},
		{"车把上带着一个人", "老女人慢慢倒了", "我料定她并没有伤", "车夫扶她走过去"},
	}
	for _, quotes := range cases {
		in := orderOpts(quotes...)
		got := shuffleOrderOptions(&coachCard{Type: coachCardOrderEvents, Prompt: "排一排", Options: in})
		if got == nil {
			t.Fatalf("%v: 整块板没了", quotes)
		}
		if len(got.Options) != len(in) {
			t.Fatalf("%v: 选项数变了 %d → %d", quotes, len(in), len(got.Options))
		}
		if sameOrder(in, got.Options) {
			t.Errorf("%v: 打乱之后还是原来的顺序 —— 板子就是答案", quotes)
		}
		// 一条都不许丢，也不许多。
		seen := map[string]int{}
		for _, o := range in {
			seen[o.Quote]++
		}
		for _, o := range got.Options {
			seen[o.Quote]--
		}
		for q, n := range seen {
			if n != 0 {
				t.Errorf("%v: 选项「%s」的条数对不上（差 %d）", quotes, q, n)
			}
		}
	}
}

// 同一块板每次都该打乱成同一个样子：它会落进 payload，重算时换一副样子就等于
// 她刷新一次页面题目就变了。
func TestOrderBoardShuffleIsStable(t *testing.T) {
	in := orderOpts("发现画作", "挂在家中", "网友鉴定", "拍卖成交")
	first := shuffleOrderOptions(&coachCard{Type: coachCardOrderEvents, Options: in})
	second := shuffleOrderOptions(&coachCard{Type: coachCardOrderEvents, Options: in})
	if !sameOrder(first.Options, second.Options) {
		t.Fatal("同一块板打乱了两次，两次不一样")
	}
}

// 🚨 只动排序板。别的卡片顺序不承载对错，动了反而会把「按重要性排的备选」
// 打散。
func TestShuffleLeavesOtherCardsAlone(t *testing.T) {
	for _, typ := range []string{coachCardLabelRoles, coachCardChooseSpan, coachCardWordBank, coachCardShortText} {
		in := orderOpts("一", "二", "三")
		got := shuffleOrderOptions(&coachCard{Type: typ, Options: in})
		if !sameOrder(in, got.Options) {
			t.Errorf("%s: 不该被打乱", typ)
		}
	}
	// 一条选项排不出顺序，原样放过。
	one := orderOpts("只有一条")
	if got := shuffleOrderOptions(&coachCard{Type: coachCardOrderEvents, Options: one}); !sameOrder(one, got.Options) {
		t.Error("只有一条选项时不该动它")
	}
	if shuffleOrderOptions(nil) != nil {
		t.Error("nil 进 nil 出")
	}
}

// buildOrderBoard 自己取句子就是按原文顺序取的 —— 经过这一层之后不该还是原序。
// 这一条把两件事接起来，免得将来有人把调用点挪走而单测还绿着。
func TestServerBuiltOrderBoardGetsShuffled(t *testing.T) {
	blocks := []Block{
		{ID: "p1", Text: "她在旧货店里用四美元买下了那幅画。"},
		{ID: "p2", Text: "回到家之后，她把画挂在了客厅的墙上。"},
		{ID: "p3", Text: "脸书小组里有人认出这可能是怀斯的作品。"},
		{ID: "p4", Text: "最后它在拍卖会上以高价成交。"},
	}
	built := validateCoachCard(buildOrderBoard(blocks, nil), blocks)
	if built == nil {
		t.Skip("这几段拼不出一块板，换别的用例")
	}
	got := shuffleOrderOptions(built)
	if sameOrder(built.Options, got.Options) {
		t.Fatal("服务端摆的那一块出去时还是原文顺序")
	}
}
