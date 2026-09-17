package api

// reading_genre_live_test.go —— 四种体裁各走一遍真模型。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveGenre -v -count=1
//
// # 为什么必须是实测
//
// 整条链子的第一环是模型判的那个词（genre），而它**不摆在屏幕上**。判错了，
// 后面每一步都还在正常运转：清单排得出来、板摆得出来、话说得通顺 —— 只是
// 她在一篇故事上被要求去找作者的主张。单元测试锁得住「换读法这件事怎么换」，
// 锁不住「它换对了没有」。
//
// 议论文那一行同时是**回归**：产品负责人的条件是议论文的体验一点都不许变，
// 所以它必须仍然落在议论文那两套读法上。

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

type genreSample struct {
	name  string
	lang  string
	title string
	want  string
	// wantRoutines 是这个体裁上可以接受的读法（议论文在英文里有两套）。
	wantRoutines []string
	body         string
}

var genreSamples = []genreSample{
	{
		name: "argument/en", lang: "en", want: genreArgument,
		wantRoutines: []string{"en-close-read", "en-argument"},
		title:        "Cities Should Stop Building Parking Lots",
		body: strings.Join([]string{
			"Every new apartment building in this city must come with parking spaces. That rule is quietly making housing more expensive, and it should go.",
			"A single underground parking space costs between $30,000 and $70,000 to build. That cost does not disappear. It is folded into the rent of every apartment above it, including the apartments of people who do not own a car.",
			"Supporters of the rule say that without it, drivers would clog residential streets looking for somewhere to leave their cars. That worry is real, but it is an argument for charging fairly for street parking, not for forcing every building to dig a garage.",
			"When Buffalo removed its parking requirements in 2017, developers still built parking where buyers wanted it. They simply built less of it, and they built more apartments instead.",
			"Admittedly, a family in a suburb with no bus service cannot give up a car tomorrow. Rules should differ between a downtown block and a distant neighborhood.",
			"But the current rule treats every block the same, and it charges the people least likely to drive for the privilege of parking that they never use. The city should let builders decide.",
		}, "\n\n"),
	},
	{
		name: "explain/en", lang: "en", want: genreExplain,
		wantRoutines: []string{"en-explain"},
		title:        "How a Vaccine Teaches Your Body to Fight",
		body: strings.Join([]string{
			"A vaccine works by showing your immune system a harmless piece of a germ, so that the body is ready if the real germ ever arrives.",
			"Your immune system keeps a kind of memory. When a virus enters the body for the first time, white blood cells need days to work out how to attack it. During those days, you get sick.",
			"A vaccine shortens that delay. It carries a fragment of the virus, usually a piece of the protein on its surface, or instructions for making that piece.",
			"Cells near the injection site read those instructions and build the surface protein themselves. The protein alone cannot make you ill; it is a shape, not a working virus.",
			"Immune cells find the shape, treat it as an intruder, and build antibodies that lock onto it. Some of those cells then become memory cells and stay in the body for years.",
			"If the real virus arrives later, the memory cells recognize the same shape within hours rather than days, and the infection is stopped before it spreads.",
			"This is why a sore arm or a mild fever after a shot is not a sign that something went wrong. It is the immune system doing the rehearsal.",
		}, "\n\n"),
	},
	{
		name: "narrative/en", lang: "en", want: genreNarrative,
		wantRoutines: []string{"en-narrative"},
		title:        "The Last Bus Home",
		body: strings.Join([]string{
			"Mei had missed the last bus twice that winter, and both times she had walked the four miles home without telling her mother.",
			"On the third night, the driver saw her running and waited. She climbed on, out of breath, and dug through her coat for coins that were not there.",
			"\"Sit down,\" the driver said, without looking at her. \"You can pay me on Friday.\"",
			"She sat near the back, by the heater, and watched her own reflection slide over the dark shop windows. Her hands would not stop shaking.",
			"She had been at the library until closing, rewriting an essay she had already rewritten twice, because the teacher had said it was careless.",
			"On Friday she brought the fare, folded flat, and held it out as she stepped on. The driver shook his head.",
			"\"Keep it,\" he said. \"My daughter used to stay late too.\"",
			"Mei did not answer. She put the coins back in her pocket, and for the first time that winter she did not mind the long ride.",
		}, "\n\n"),
	},
	{
		name: "argument/zh", lang: "zh", want: genreArgument,
		wantRoutines: []string{"zh-scan-focus-lens"},
		title:        "中学生该不该带手机进校园",
		body: strings.Join([]string{
			"每到开学，手机该不该带进校园就会被重新吵一遍。我认为学校不该一刀切地没收，而应该规定使用的时间和场合。",
			"支持全面禁止的人有一条很有力的理由：课堂上的一次消息提示，会让注意力恢复到原来的水平需要十几分钟。这条理由是站得住的。",
			"但是全面禁止解决的只是课堂，而不是自制力。一所寄宿制学校在收走手机的第二年发现，学生把游戏搬到了计算器和电子词典上。",
			"更重要的是，手机今天已经是查资料、联系家长、甚至交作业的工具。把它整个拿走，等于把这些事也一起拿走了。",
			"我承认，低年级学生确实还管不住自己，对他们收得严一点是合理的。",
			"所以真正该做的是：上课时统一存放，课间和午休归还，并且把「什么时候可以用」写清楚，让学生自己学会安排。",
		}, "\n\n"),
	},
}

// TestLiveGenreRoutesEachArticle —— 每篇文章走一次排读法，看体裁判得对不对、
// 最后排上的是不是服务这个体裁的那一套。
func TestLiveGenreRoutesEachArticle(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassCompose)
	wrong := 0
	for _, s := range genreSamples {
		blocks := SplitBlocks(s.body)
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: readingPlanSystem},
				{Role: gateway.RoleUser, Content: buildReadingPlanPrompt(s.lang, s.title, blocks)},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("%s: live call failed (%s): %v", s.name, r.ModelID, err)
		}
		plan, routine, reject := parseReadingPlan(res.Text, s.lang)
		if reject != planOK {
			t.Errorf("%s: REJECTED (%s) — %s", s.name, reject, tailRunes(res.Text, 200))
			continue
		}
		out, why := validateOutlineWhy(plan.outline(), blocks)
		// 🚨 读的是**模型给的那个词**，不是校验过的导读里那个 —— 导读整份作废
		// 的时候（她的地图没了）体裁仍然留着，读法也仍然照它排。2026-09-18 实测
		// 抓到的正是这一幕：一篇故事的 oneLine 写成了英文，导读被丢掉，而体裁
		// 当时也跟着没了。
		genre := validateGenre(plan.Genre)
		if out.Genre != "" && out.Genre != genre {
			t.Errorf("%s: 校验过的导读里体裁是 %q，模型给的是 %q", s.name, out.Genre, genre)
		}
		fitted := pickRoutineForGenre(routine, genre)
		kinds := make([]string, 0, len(fitted.Steps))
		for _, st := range fitted.Steps {
			kinds = append(kinds, string(st.Kind))
		}
		t.Logf("%s — 体裁=%q（想要 %s）模型挑的=%s 最终=%s\n  导读=%s\n  步骤=%s",
			s.name, genre, s.want, routine.Key, fitted.Key, why, strings.Join(kinds, " "))
		if genre != s.want {
			wrong++
			t.Errorf("%s: 体裁判成了 %q", s.name, genre)
			continue
		}
		ok := false
		for _, k := range s.wantRoutines {
			if fitted.Key == k {
				ok = true
			}
		}
		if !ok {
			t.Errorf("%s: 排上的是 %s，想要 %v", s.name, fitted.Key, s.wantRoutines)
		}
	}
	t.Logf("RESULT: %d/%d 判错了体裁", wrong, len(genreSamples))
}

// TestLiveGenreBoardsAndCards —— 报道那一篇上走两轮带读：标注步该给一块
// 「事实 / 引述 / 解释」的板，排序步该给一块排序板。
//
// 板的格子是**服务端填的**，所以这一条真正在测的是：模型在这一节体裁说明底下
// 还认不认得出该发哪一种卡片，以及它说的话和板上的格子对不对得上。
func TestLiveGenreBoardsAndCards(t *testing.T) {
	prov, r := liveClass(t, gateway.ClassDialogue)
	blocks := liveReportBlocks()
	outline := readingOutline{
		OneLine: "加沙的援助为什么进不去", Gist: "各方在争援助能不能进加沙。",
		Genre: genreReport, Shape: "事件 → 各方说法 → 未解决的部分",
	}
	cases := []struct {
		name     string
		kind     string
		label    string
		wantType string
	}{
		{"标注步", string(taskLabel), "分清事实与说法", coachCardLabelRoles},
		{"排序步", string(taskSequence), "排出事件时间线", coachCardOrderEvents},
	}
	for _, c := range cases {
		tasks := []sqlc.ReadingTask{{Kind: c.kind, Label: c.label, Status: "pending"}}
		prompt := buildReadingCoachPrompt("Aid groups scramble to help", blocks, outline,
			tasks, nil, nil, "我读完了，接下来做什么？", nil, "")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		res, err := gateway.Collect(ctx, prov, r, gateway.ChatRequest{
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: buildReadingCoachSystem("en")},
				{Role: gateway.RoleUser, Content: prompt},
			},
		})
		cancel()
		if err != nil {
			t.Fatalf("%s: live call failed: %v", c.name, err)
		}
		got, ok := parseReadingCoachReply(res.Text, blocks, "en", func(string) bool { return false })
		if !ok {
			t.Errorf("%s: 回复解析不了：%s", c.name, tailRunes(res.Text, 200))
			continue
		}
		card := fitBoardToGenre(got.Card, genreReport)
		t.Logf("%s — 卡片=%v（想要 %s）丢卡理由=%q\n  话：%s", c.name,
			cardTypeOf(card), c.wantType, got.cardWhy, headRunes(got.Reply, 220))
		if card == nil || card.Type != c.wantType {
			// 排序板模型没给时服务端会兜一块（见 postReadingCoachTurn），所以
			// 这里不是致命的 —— 但它说明这一节说明还没让它自己想到。
			t.Errorf("%s: 拿到的是 %v", c.name, cardTypeOf(card))
			continue
		}
		if card.Type == coachCardLabelRoles {
			if strings.Join(card.Labels, "/") != "事实/引述/解释" {
				t.Errorf("%s: 格子 = %v", c.name, card.Labels)
			}
			// 话里不该再提「主张」—— 这篇没有作者的主张。
			if strings.Contains(got.Reply, "主张") {
				t.Errorf("%s: 报道上还在说「主张」：%s", c.name, headRunes(got.Reply, 160))
			}
		}
	}
}

func cardTypeOf(c *coachCard) string {
	if c == nil {
		return "（没有卡片）"
	}
	return c.Type
}
