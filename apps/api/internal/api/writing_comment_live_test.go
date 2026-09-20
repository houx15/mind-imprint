package api

// writing_comment_live_test.go —— 新的反馈契约，发给**真模型**跑一遍。
//
//	set -a; . .deploy-local/env.prod; set +a
//	LIVE_LLM=1 go test ./internal/api -run TestLiveWritingComment -v -count=1
//
// # 为什么这个非跑不可
//
// [[prompt-output-must-be-verifiable-2026-09-03]]：stub 测试只证明解析器读得懂
// **我写的** JSON。2026-09-11 这次改动把一整套新规矩写进了 prompt：
// 挑一个闭表里的 symptom、只说最上面那一层、每条都要给一句祈使、
// 不许给可以直接粘贴的句子。这些全是「只有模型能遵守」的那一类，
// 而单元测试对它们永远是绿的。
//
// 这里跑两篇：一篇中文、一篇英文。英文那篇尤其要跑 —— symptom 表是按语言
// 选的，一篇英文稿子必须拿到 IELTS 那 13 个 id，而不是中文那 18 条。
//
// 🚨 断言只落在**结构性**的东西上（id 在不在闭表里、action 空不空、
// 层是不是同一层），不断言模型说得好不好 —— 那是走查该看的，不是测试。

import (
	"context"
	"strings"
	"testing"
	"time"

	"mindimprint/api/internal/gateway"
	"mindimprint/api/internal/store/sqlc"
)

// 一段真的中学生作文：有判断、有例子，**没有一句解释**，而且句子偏碎。
// 也就是说第 2 层和第 4 层都有毛病 —— 正好用来看它会不会只说上面那一层。
const liveZHParagraph = `学校食堂每天倒掉的饭特别多。上周五我数了一下，回收桶里有六个桶是满的。
光是那一天，倒掉的米饭大概能装满两个洗菜盆。这很浪费。这不应该。我觉得这件事值得写。`

// 一段中国学生写的英文：意思清楚，但整句按中文语序拼，而且比较没有比完。
const liveENParagraph = `Because of the food waste problem is very serious in our school, I think we should do something.
Every day the canteen throw away many rice. Last Friday I counted six bins are full.
This is more serious than before.`

func liveReview(t *testing.T) (gateway.Provider, gateway.Resolved) {
	t.Helper()
	return liveClass(t, gateway.ClassReview)
}

// oneCommentTurn 跑一次真的 block comment，并把服务端那套校验原样走一遍。
func oneCommentTurn(t *testing.T, lang, source string) []CommentPoint {
	t.Helper()
	prov, resolved := liveReview(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	wr := sqlc.Writing{Title: "食堂浪费", Lang: lang}
	res, err := gateway.Collect(ctx, prov, resolved, gateway.ChatRequest{
		Messages: []gateway.ChatMessage{
			{Role: gateway.RoleSystem, Content: buildWritingCommentSystem(lang, writingBlockCommentMaxIssues, "")},
			{Role: gateway.RoleUser, Content: buildWritingCommentPrompt(wr, "她写的这一段", source, "")},
		},
	})
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	t.Logf("模型原样回的：\n%s", res.Text)

	parsed, ok := parseWritingComment(res.Text)
	if !ok {
		t.Fatalf("解析不了模型的回复 —— 这就是线上会静默丢掉整条意见的那一种：\n%s", res.Text)
	}
	points := validateCommentPoints(parsed.Points, source, lang, writingBlockCommentMaxIssues)
	t.Logf("校验之后还剩 %d 条（模型给了 %d 条）", len(points), len(parsed.Points))
	for _, p := range points {
		t.Logf("  [%s] layer=%d symptom=%s\n    说：%s\n    做：%s\n    引：%s",
			p.Kind, p.Layer, p.Symptom, p.Text, p.Action, p.Quote)
	}
	return points
}

// 中文：至少要活下来一条，而且 issue 必须带着一句祈使。
//
// 🚨 「至少活下来一条」本身就是一条真断言：校验器丢得太狠的话，
// 学生按一次「请印记看看这一段」会得到一张空白的卡 —— 那是比没有更糟的东西。
func TestLiveWritingComment_ZH(t *testing.T) {
	points := oneCommentTurn(t, "zh", liveZHParagraph)
	if len(points) == 0 {
		t.Fatal("一条都没活下来 —— 她按下按钮会看到一张空卡")
	}
	assertCommentContract(t, "zh", points)
}

// 英文：同样的契约，而且 symptom 必须来自**英文那张表**。
func TestLiveWritingComment_EN(t *testing.T) {
	points := oneCommentTurn(t, "en", liveENParagraph)
	if len(points) == 0 {
		t.Fatal("一条都没活下来 —— 英文这一侧等于没有反馈")
	}
	assertCommentContract(t, "en", points)

	// 🚨 表是按语言选的。一篇英文稿子拿到「流水账」这种中文诊断，
	// 说明选表那一步没生效。
	for _, p := range points {
		if p.Kind != "issue" {
			continue
		}
		if _, ok := lookupWritingSymptom("en", p.Symptom); !ok {
			t.Fatalf("英文稿子上出现了一个不在英文表里的 symptom：%q", p.Symptom)
		}
	}
}

// assertCommentContract 是两篇共用的那几条结构性断言。
func assertCommentContract(t *testing.T, lang string, points []CommentPoint) {
	t.Helper()

	var layers []int
	for _, p := range points {
		switch p.Kind {
		case "good":
			if strings.TrimSpace(p.Quote) == "" {
				t.Error("肯定那一条也必须挂在她真写过的一句话上")
			}
		case "issue":
			if _, ok := lookupWritingSymptom(lang, p.Symptom); !ok {
				t.Errorf("symptom %q 不在 %s 那张闭表里", p.Symptom, lang)
			}
			if strings.TrimSpace(p.Action) == "" {
				t.Error(`issue 没有给下一步 —— 这正是 "Diagnostic Without Return"`)
			}
			layers = append(layers, p.Layer)
		default:
			t.Errorf("kind 只能是 good/issue，拿到 %q", p.Kind)
		}
	}

	// 一轮只说一层。
	for i := 1; i < len(layers); i++ {
		if layers[i] != layers[0] {
			t.Errorf("同一条回复里出现了两层：%v —— 优先级那道筛子漏了", layers)
			break
		}
	}

	// 一段只给一条要改的。
	var issues int
	for _, p := range points {
		if p.Kind == "issue" {
			issues++
		}
	}
	if issues > writingBlockCommentMaxIssues {
		t.Errorf("一段给了 %d 条要改的，上限是 %d", issues, writingBlockCommentMaxIssues)
	}
}
