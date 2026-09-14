package api

import "testing"

// 一份读不出来的回复里，她那句话通常是好的。
//
// 这几条用的是线上真的撞到过的坏法，不是我编的畸形 JSON：
// 2026-09-08 是 `"questions":[...}]}`（该收 `]` 的地方收了 `}`），
// 2026-09-10 是流式少送最后一个分片、整份断在半截。两次坏的都是结构那一半，
// `reply` 本身完整 —— 而整份作废的代价是她眼前弹一个 model_unavailable。
//
// 🚨 这些断言的对立面不是「宽容一点」，是 [[ai-errors-must-surface-never-fake]]：
// 捞出来的每个字都必须是模型真的写的。所以最后两条测的是**捞不到就照旧报错**。

func TestParseWritingPlanReply_SalvagesTheSentenceWhenTheStructureBreaks(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		reply string
		adds  int
	}{
		{
			// 2026-09-10 那一种：写到一半没了。
			name:  "断在半截",
			text:  `{"reply":"你说的是上周五那六个桶。把那个数写进去，比「很多」有力得多。","add":[{"text":"食堂每天倒掉的饭`,
			reply: "你说的是上周五那六个桶。把那个数写进去，比「很多」有力得多。",
			adds:  0, // 半个节点宁可不要
		},
		{
			// 2026-09-08 那一种：该收 ] 的地方收了 }。
			name:  "数组收错了括号",
			text:  `{"reply":"先把中心论点定下来。","add":[{"text":"校车该不该装安全带","role":"claim"}}]}`,
			reply: "先把中心论点定下来。",
			adds:  1, // 断点之前那个是完整的，留着
		},
		{
			// reply 在后面、结构在前面坏掉的那一种。捞不到就该照旧报错，
			// 不能因为「前面读过一点」就当成功。
			name:  "前面的数组断了，reply 根本没读到",
			text:  `{"add":[{"text":"食堂`,
			reply: "",
		},
		{
			// 带围栏的坏 JSON 也要能捞 —— 2026-09-11 段落引导那边正是
			// 因为救援喂了没去围栏的原文，一次都没救到过。
			name:  "带围栏",
			text:  "```json\n{\"reply\":\"这一条已经够支撑了，去写吧。\",\"add\":[{\"text\":\"证据一\"",
			reply: "这一条已经够支撑了，去写吧。",
			adds:  0,
		},
		{
			name:  "整份都不是 JSON",
			text:  "抱歉，我没太明白你的意思。",
			reply: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseWritingPlanReply(c.text)
			if c.reply == "" {
				if ok {
					t.Fatalf("捞不到她那句话就该报错，却返回了 %+v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("这一份里 reply 是完整的，应该救回来：%s", c.text)
			}
			if got.Reply != c.reply {
				t.Errorf("reply = %q, want %q", got.Reply, c.reply)
			}
			if len(got.Add) != c.adds {
				t.Errorf("救回 %d 个节点, want %d（%+v）", len(got.Add), c.adds, got.Add)
			}
		})
	}
}

// 好的那一份不能因为多了一条救援路径就变样。
func TestParseWritingPlanReply_HealthyReplyUnchanged(t *testing.T) {
	got, ok := parseWritingPlanReply(
		"```json\n{\"reply\":\"你见过哪一次？\",\"ready\":true," +
			"\"add\":[{\"text\":\"食堂浪费\",\"role\":\"claim\"},{\"text\":\"我数过六个桶\",\"parentId\":\"abc\"}]}\n```")
	if !ok {
		t.Fatal("一份完好的回复没读出来")
	}
	if got.Reply != "你见过哪一次？" || !got.Ready || len(got.Add) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got.Add[1].ParentID != "abc" {
		t.Errorf("parentId 丢了：%+v", got.Add[1])
	}
}
