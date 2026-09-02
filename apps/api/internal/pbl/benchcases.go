package pbl

import (
	"fmt"

	"mindimprint/api/internal/benchcase"
	"mindimprint/api/internal/gateway"
)

// BenchCases are the routing workbench's PBL cases. They live here, not in the
// workbench, because coachSystem is unexported and a copied prompt drifts — and
// a routing decision made on a drifted prompt looks like evidence while being
// worth nothing.
//
// Nothing on a request path calls this. See cmd/routebench.
func BenchCases() []benchcase.Case {
	return []benchcase.Case{{
		ID:    "dialogue/pbl-turn",
		Class: gateway.ClassDialogue,
		Site:  "internal/pbl.Coach (POST /pbl/projects/{id}/turn)",
		Request: gateway.ChatRequest{
			MaxTokens: 16384,
			Messages: []gateway.ChatMessage{
				{Role: gateway.RoleSystem, Content: fmt.Sprintf(coachSystem, toolCatalogue(), produceCatalogue())},
				{Role: gateway.RoleUser, Content: buildCoachContext(benchTurnInput())},
			},
		},
		// The real parser. If it refuses the output, production shows the
		// student an error — no judge needed to call that a failure.
		Validate: func(text string) error {
			_, err := parseCoachOutput(text)
			return err
		},
		Judge: `这是一个中学生项目式学习的 AI 陪练回合。学生刚说她想做校园食堂剩饭的项目，
并且已经报了一个她自己观察到的数字。好的一轮应当：
（1）接住她已经说过的具体内容，而不是重复一遍通用鼓励；
（2)一次只问一个问题；
（3）不替她定论、不直接给结论或计划；
（4）如果给出工具或钩子，说清楚为什么是现在这一个。
按 1–5 打分：5 = 四条都做到；3 = 接住了但问题泛泛；1 = 空洞鼓励或代她做决定。`,
	}, {
		ID:    "assess/pbl-lookback",
		Class: gateway.ClassAssess,
		Site:  "internal/pbl.GenerateLookback (GET /pbl/projects/{id}/lookback)",
		Request: gateway.ChatRequest{
			MaxTokens: 16384,
			Messages: []gateway.ChatMessage{{
				Role:    gateway.RoleSystem,
				Content: fmt.Sprintf(lookbackSystem, lookbackSectionList(), buildLookbackContext(benchLookbackInput())),
			}},
		},
		Validate: func(text string) error {
			_, err := parseLookback(text)
			return err
		},
		Judge: `这是项目结束后的过程回顾。好的回顾要指出学生**实际做过的具体动作**
（改了什么、放弃了什么、被什么数据推翻过），而不是泛泛地夸「你很努力」。
按 1–5 打分：5 = 每一条都能追到过程里的具体一步；3 = 有具体但夹杂套话；
1 = 通篇是可以套在任何学生身上的评价。`,
	}}
}

// benchTurnInput is one representative mid-project turn: she has a plan, she has
// used a tool, and she has just reported a number she gathered herself. This is
// the shape where a weak model gives itself away — it congratulates her instead
// of asking what the number does not yet show.
func benchTurnInput() CoachInput {
	return CoachInput{
		Idea: "我想搞清楚我们学校食堂每天到底浪费了多少饭菜，然后想办法让它少一点。",
		Kind: "调查研究",
		Steps: []string{
			"定义「浪费」到底指什么，怎么量",
			"连续一周在午餐时段称剩饭重量",
			"访谈食堂师傅和同学，问原因",
			"提一个能落地的改动，并说清楚怎么验证",
		},
		Recent: []Turn{
			{Role: "ai", Content: "你打算怎么量「浪费」这件事？是称重量，还是数份数？"},
			{Role: "student", Content: "称重量吧，数份数太不准了，有的人剩半碗有的人剩一口。"},
			{Role: "ai", Content: "那一周下来，你打算在什么时间点称？"},
			{Role: "student", Content: "我这周已经称了三天了，午饭结束后统一称。平均每天大概 47 公斤剩饭。"},
		},
		ToolWork: []string{
			"便签板：她把「浪费」拆成了三类——没打完的、打多了的、不好吃剩下的",
		},
	}
}

// benchLookbackInput is a finished project with a real reversal in it — she
// suspected bad data, found a cause, and changed her conclusion. A lookback that
// cannot find that reversal is a lookback that would fit any student.
func benchLookbackInput() LookbackInput {
	return LookbackInput{
		Name: "校园食堂剩饭调查",
		Idea: "我想搞清楚我们学校食堂每天到底浪费了多少饭菜，然后想办法让它少一点。",
		Steps: []string{
			"定义「浪费」到底指什么，怎么量",
			"连续一周在午餐时段称剩饭重量",
			"访谈食堂师傅和同学，问原因",
			"提一个能落地的改动，并说清楚怎么验证",
		},
		Reframes: []string{
			"一开始想数「剩了几份」，自己推翻了，改成称重量——理由是「有的人剩半碗有的人剩一口」",
			"原本的结论是「学生浪费习惯不好」，在周三那条数据之后改成「菜品和口味的匹配度影响很大」",
		},
		Decisions: []string{
			"周三只有 31 公斤（平常 47），她一开始判断是数据错了；访谈后发现周三供应的是学生投票选出来的菜",
		},
		Artifacts: []string{
			"五天剩饭重量记录表",
			"两位食堂师傅的访谈笔记",
			"改动方案：每月一次菜单投票，并继续称重比较投票日与非投票日",
		},
		Keeps: []string{
			"她自己说的困难：访谈的时候不知道该问什么，问了两个问题就没话了",
		},
	}
}
