package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateCoachCard(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "城市地表以沥青和混凝土为主，白天吸热、夜里放热。"},
		{ID: "b2", Text: "建筑密集阻碍了夜间散热。空调外机把热量排到室外。"},
	}

	t.Run("编造的句子整张卡片被丢掉", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "哪一句你读着最不服气？",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "城市地表以沥青和混凝土为主"},
				{BlockID: "b2", Quote: "这句话文章里根本没有"},
			},
		}, blocks)
		// 只剩 1 个合格选项 < 2 → 整张丢掉
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("选项必须是它自己那个 block 的子串", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				// b1 的句子挂在 b2 上 —— 不算
				{BlockID: "b2", Quote: "城市地表以沥青和混凝土为主"},
				{BlockID: "b2", Quote: "建筑密集阻碍了夜间散热"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("合格的卡片原样通过", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "哪一句你读着最不服气？",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("expected 2 options, got %+v", got)
		}
		if got.Prompt != "哪一句你读着最不服气？" {
			t.Fatalf("prompt changed: %q", got.Prompt)
		}
		if got.Options[0] != (coachCardOption{BlockID: "b1", Quote: "白天吸热、夜里放热"}) {
			t.Fatalf("option 0 changed: %+v", got.Options[0])
		}
		if got.Options[1] != (coachCardOption{BlockID: "b2", Quote: "空调外机把热量排到室外"}) {
			t.Fatalf("option 1 changed: %+v", got.Options[1])
		}
	})

	t.Run("超过 4 个选项截断到 4", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "城市地表以沥青和混凝土为主"},
				{BlockID: "b1", Quote: "白天吸热"},
				{BlockID: "b1", Quote: "夜里放热"},
				{BlockID: "b2", Quote: "建筑密集阻碍了夜间散热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 4 {
			t.Fatalf("expected 4 options, got %+v", got)
		}
		if got.Options[3].Quote != "建筑密集阻碍了夜间散热" {
			t.Fatalf("expected the FIRST four survivors in order, got %+v", got.Options)
		}
	})

	t.Run("重复选项去重", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("expected 2 options after dedupe, got %+v", got)
		}
	})

	t.Run("去重后不足 2 个也丢掉", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("未知 blockId 丢掉", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b9", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("太短的选项不算一句话", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("未知类型丢掉", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "multiple_choice",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("prompt 超长丢掉", func(t *testing.T) {
		long := strings.Repeat("段", 61) // 61 runes, 183 bytes —— 按 rune 数，不是 byte 数
		if got := validateCoachCard(&coachCard{Type: "short_text", Prompt: long}, blocks); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
		ok := strings.Repeat("段", 60)
		if got := validateCoachCard(&coachCard{Type: "short_text", Prompt: ok}, blocks); got == nil {
			t.Fatalf("60 runes should pass")
		}
	})

	t.Run("prompt 为空丢掉", func(t *testing.T) {
		if got := validateCoachCard(&coachCard{Type: "short_text", Prompt: "  \n "}, blocks); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("pick_in_article 不需要 options", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:    "pick_in_article",
			Prompt:  "去文章里点出最站不住的那一句",
			Options: []coachCardOption{{BlockID: "b9", Quote: "文章里根本没有这句"}},
		}, blocks)
		if got == nil {
			t.Fatalf("expected the card to survive")
		}
		if len(got.Options) != 0 {
			t.Fatalf("expected options to be dropped, got %+v", got.Options)
		}
	})

	t.Run("short_text 不需要 options", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "short_text",
			Prompt: "用你自己的话说说这一段在讲什么",
		}, blocks)
		if got == nil {
			t.Fatalf("expected the card to survive")
		}
		if len(got.Options) != 0 {
			t.Fatalf("expected no options, got %+v", got.Options)
		}
	})

	t.Run("nil 卡片", func(t *testing.T) {
		if got := validateCoachCard(nil, blocks); got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("原卡片不被就地改写", func(t *testing.T) {
		in := &coachCard{
			Type:   "pick_in_article",
			Prompt: " 去文章里点一句 ",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
			},
		}
		got := validateCoachCard(in, blocks)
		if got == nil {
			t.Fatalf("expected the card to survive")
		}
		if got == in {
			t.Fatalf("expected a fresh card, not the caller's own pointer")
		}
		if len(in.Options) != 1 || in.Prompt != " 去文章里点一句 " {
			t.Fatalf("input was mutated: %+v", in)
		}
	})
}

// TestParseReadingCoachReply_Card — the wiring, from the model's JSON to the
// parsed turn. The load-bearing half is the FAILURE case: a card whose
// options were invented reads exactly like a good one to anybody but the
// validator, and dropping it must not take the turn down with it. She gets
// the coach's words; she just doesn't get a card built on sentences that
// aren't in her article.
func TestParseReadingCoachReply_Card(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "城市地表以沥青和混凝土为主，白天吸热、夜里放热。"},
		{ID: "b2", Text: "建筑密集阻碍了夜间散热。空调外机把热量排到室外。"},
	}
	lensOK := func(string) bool { return false }

	t.Run("编造的选项：卡片丢掉，这一轮照常成功", func(t *testing.T) {
		const raw = `{"reply":"你说的这点我接住了。","advance":"","focusBlock":"b1",
		  "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？","options":[
		    {"blockId":"b1","quote":"城市其实并没有那么热"},
		    {"blockId":"b2","quote":"这句话文章里根本不存在"}]}}`
		got, ok := parseReadingCoachReply(raw, blocks, "zh", lensOK)
		if !ok {
			t.Fatalf("a bad card must not fail the turn")
		}
		if got.Reply != "你说的这点我接住了。" {
			t.Errorf("reply = %q, want it kept intact", got.Reply)
		}
		if got.Card != nil {
			t.Errorf("card = %+v, want nil — its options are not in the article", got.Card)
		}
	})

	t.Run("合格卡片一路走到 parsed", func(t *testing.T) {
		const raw = `{"reply":"读完了就好，来点一句。","advance":"","focusBlock":"b1",
		  "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？","options":[
		    {"blockId":"b1","quote":"白天吸热、夜里放热"},
		    {"blockId":"b2","quote":"建筑密集阻碍了夜间散热"}]}}`
		got, ok := parseReadingCoachReply(raw, blocks, "zh", lensOK)
		if !ok {
			t.Fatalf("parse failed")
		}
		if got.Card == nil {
			t.Fatalf("card was dropped; both quotes are literal substrings of their own block")
		}
		if got.Card.Type != coachCardChooseSpan || len(got.Card.Options) != 2 {
			t.Fatalf("card came through wrong: %+v", got.Card)
		}
		if got.Card.Prompt != "哪一句你读着最不服气？" {
			t.Errorf("prompt = %q", got.Card.Prompt)
		}
	})

	t.Run("没有 card 键就是没有卡片", func(t *testing.T) {
		got, ok := parseReadingCoachReply(
			`{"reply":"先整体读一遍。","advance":"","focusBlock":""}`, blocks, "zh", lensOK)
		if !ok {
			t.Fatalf("parse failed")
		}
		if got.Card != nil {
			t.Errorf("card = %+v, want nil", got.Card)
		}
	})
}

// TestCoachCardPayload_IsAnEnvelopeNotBase64 — the payload column is []byte,
// and encoding/json turns a []byte into a base64 STRING. So the bytes handed
// to the column must be real JSON, and the envelope must nest the card under
// "card" so a later payload (her answer to one) stays distinguishable from it.
func TestCoachCardPayload_IsAnEnvelopeNotBase64(t *testing.T) {
	if got := coachCardPayload(nil); got != nil {
		t.Fatalf("a turn with no card must store SQL NULL, got %q", got)
	}
	b := coachCardPayload(&coachCard{
		Type:    coachCardChooseSpan,
		Prompt:  "哪一句你读着最不服气？",
		Options: []coachCardOption{{BlockID: "b1", Quote: "白天吸热"}},
	})
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("payload is not JSON: %v — %q", err, b)
	}
	card, isObject := back["card"].(map[string]any)
	if !isObject {
		t.Fatalf("payload[\"card\"] is %T, want a JSON object", back["card"])
	}
	if card["prompt"] != "哪一句你读着最不服气？" {
		t.Errorf("prompt did not survive: %+v", card)
	}
}

// TestReadingCoachSystem_CardRulings — the card section is a product surface,
// not prose. These clauses are the ones that make the card worth building:
// verbatim options (the guarantee she must read the article), a question with
// no single right answer (we are not an exam), and no restating of the step.
func TestReadingCoachSystem_CardRulings(t *testing.T) {
	for _, want := range []string{
		"card",             // the field is documented in the output contract
		"choose_span",      // and so are the three shapes
		"pick_in_article",  //
		"short_text",       //
		"逐字抄自文章",           // the verbatim rule
		"不能有唯一正解",          // 🚨 the product ruling: a ladder, not a test
		"哪一句你读着最不服气",       // the ruling's own example
		"reply 就不要再把它复述一遍", // the card carries the instruction now
	} {
		if !strings.Contains(readingCoachSystem, want) {
			t.Errorf("readingCoachSystem no longer carries %q", want)
		}
	}
}
