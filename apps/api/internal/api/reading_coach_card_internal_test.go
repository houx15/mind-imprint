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

	// —— 从句边界（Task 4b）——
	// ≥4 runes 管的是**长度**，不是「是不是一句话」。一个从句子中间截出来的窗口
	// 确确实实是原文的子串，但它是靠扫几个字凑出来的；一句话必须被当作一个整体
	// 读过。下面这一组把「选项是一句话」这件事钉住 —— 反例被丢掉，
	// 而**正例一个都不许被误杀**：误杀是看不见的（卡片静悄悄消失，
	// 看起来就像模型这一轮不想出卡片）。

	t.Run("从词中间截出来的窗口被丢掉", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				// 真原文子串，但两头都不在标点边界上（「地|表以沥青和混凝土|为主」）
				{BlockID: "b1", Quote: "表以沥青和混凝土"},
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("expected the two clause-aligned options to survive, got %+v", got)
		}
		for _, o := range got.Options {
			if o.Quote == "表以沥青和混凝土" {
				t.Fatalf("a mid-word window survived: %+v", got.Options)
			}
		}
	})

	t.Run("两头都不在边界上的片段被丢掉", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				// 「白天吸|热、夜里|放热」—— 前后都是字，不是标点
				{BlockID: "b1", Quote: "吸热、夜里"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("expected nil (only 1 survivor), got %+v", got)
		}
	})

	t.Run("同一句话的重叠片段整张卡片被丢掉", func(t *testing.T) {
		// 三个互不相同的字符串，「按可见文本去重」永远合并不掉它们；
		// 但没有一个落在从句边界上 —— 存活 0 < 2 → 整张丢掉。
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "表以沥青和混凝土"},
				{BlockID: "b1", Quote: "以沥青和混凝土为"},
				{BlockID: "b1", Quote: "沥青和混凝土为主"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("正常的完整句子照样通过（带尾部句号）", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b2", Quote: "建筑密集阻碍了夜间散热。"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外。"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("模型把句号一起引进来是常态，不许因此丢卡片: %+v", got)
		}
	})

	t.Run("正常的完整句子照样通过（不带尾部句号）", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b2", Quote: "建筑密集阻碍了夜间散热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("expected 2 options, got %+v", got)
		}
	})

	t.Run("block 开头的第一句通过", func(t *testing.T) {
		// 「城市地表…」前面没有边界字符 —— 它就在 block 的开头。
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "城市地表以沥青和混凝土为主"},
				{BlockID: "b2", Quote: "建筑密集阻碍了夜间散热"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a block's first sentence must pass, got %+v", got)
		}
	})

	t.Run("block 结尾的最后一句通过", func(t *testing.T) {
		// 「…排到室外。」后面已经是 block 的结尾。
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b2", Quote: "空调外机把热量排到室外。"},
				{BlockID: "b1", Quote: "白天吸热、夜里放热。"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a block's last sentence must pass, got %+v", got)
		}
	})

	t.Run("以顿号分隔的从句通过", func(t *testing.T) {
		// 「白天吸热、|夜里放热|。」—— 从句也是合法的引用单位。
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a 、-delimited clause is a legitimate quote, got %+v", got)
		}
	})

	t.Run("换行也是边界", func(t *testing.T) {
		// block 文本里带换行时，换行两边各自是一句话 —— 不许因为「前一个字不是标点」误杀。
		nl := []Block{{ID: "n1", Text: "城市越来越热\n夜里也降不下来\n空调开得更久"}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "n1", Quote: "夜里也降不下来"},
				{BlockID: "n1", Quote: "空调开得更久"},
			},
		}, nl)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a newline must count as a clause boundary, got %+v", got)
		}
	})

	t.Run("半角标点也是边界", func(t *testing.T) {
		en := []Block{{ID: "e1", Text: "Cities absorb heat by day. They release it at night, slowly."}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "pick one",
			Options: []coachCardOption{
				{BlockID: "e1", Quote: "Cities absorb heat by day."},
				// 句号后面隔着一个空格 —— 空格本身不是边界字符，但它也不该挡住这一句
				{BlockID: "e1", Quote: "They release it at night"},
			},
		}, en)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("half-width punctuation must count as a boundary, got %+v", got)
		}
	})

	t.Run("同一句话在段里出现两次时按合格的那一次算", func(t *testing.T) {
		// 第一次出现是从词中间切的，第二次出现落在边界上 —— 有一次合格就算合格。
		dup := []Block{{ID: "d1", Text: "夜里放热的地表让城市更热。夜里放热，是热岛的根。"}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "d1", Quote: "夜里放热"},
				{BlockID: "d1", Quote: "是热岛的根"},
			},
		}, dup)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("one clause-aligned occurrence is enough, got %+v", got)
		}
	})

	// —— 引号/括号包住的句子（Task 5b (c)）——
	// 一句被引号或括号包起来的话，它的边界标点落在引号**外面**。把成对的
	// 引号/括号当作跳过不计的字符（和空白同一条路），否则这一整类正常引文
	// 会被静默误杀 —— 选项被丢 → 存活不足 2 → 整张卡片消失，看起来就像
	// 印记 这一轮没想出卡片。

	t.Run("中文引号包住的整句通过", func(t *testing.T) {
		q := []Block{{ID: "q1", Text: "他说：“城市在夜里更热。”这句话有数据支撑。"}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "q1", Quote: "城市在夜里更热。"},
				{BlockID: "q1", Quote: "这句话有数据支撑。"},
			},
		}, q)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a sentence wrapped in 「“ ”」 must pass, got %+v", got)
		}
	})

	t.Run("括号包住的整句通过", func(t *testing.T) {
		p := []Block{{ID: "p1", Text: "夜里的温度更高。（数据来自城区的气象站。）后来又测了一次。"}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "p1", Quote: "数据来自城区的气象站。"},
				{BlockID: "p1", Quote: "后来又测了一次。"},
			},
		}, p)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a sentence wrapped in 「（ ）」 must pass, got %+v", got)
		}
	})

	t.Run("单引号包住的整句通过", func(t *testing.T) {
		s := []Block{{ID: "s1", Text: "她问：‘城市为什么在夜里更热’，没人答得上来。"}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				// 收尾的 ’ 也要跳过：它后面那个 ，才是真正的边界。
				{BlockID: "s1", Quote: "城市为什么在夜里更热"},
				{BlockID: "s1", Quote: "没人答得上来"},
			},
		}, s)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a sentence wrapped in 「‘ ’」 must pass, got %+v", got)
		}
	})

	t.Run("英文引号包住的整句通过", func(t *testing.T) {
		e := []Block{{ID: "e2", Text: `He said, "Cities are hotter at night." Nobody disagreed.`}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "pick one",
			Options: []coachCardOption{
				{BlockID: "e2", Quote: "Cities are hotter at night."},
				{BlockID: "e2", Quote: "Nobody disagreed."},
			},
		}, e)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf(`a sentence wrapped in " " must pass, got %+v`, got)
		}
	})

	t.Run("引号里从句子中间截的窗口仍然被丢掉", func(t *testing.T) {
		// 跳过引号**不等于**放行从句子中间切的窗口：「城|市在夜里更热|。」
		// 前面紧挨着的是「城」，不是标点、不是空白、也不是引号。
		q := []Block{{ID: "q1", Text: "他说：“城市在夜里更热。”这句话有数据支撑。"}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "q1", Quote: "市在夜里更热"},
				{BlockID: "q1", Quote: "这句话有数据支撑。"},
			},
		}, q)
		if got != nil {
			t.Fatalf("skipping quote marks must not let a mid-clause window through, got %+v", got)
		}
	})

	// —— 包含式去重（Task 5b (d)）——
	// 边界规则合并不掉两个**各自都对齐**的重叠片段：「白天吸热、夜里放热」和
	// 「夜里放热」今天都合法（后者还是一条必须通过的正例）。给她四个选项、
	// 其中两个是套娃，会让「挑一句」这件事变得莫名其妙。长的信息更完整，留长的。

	t.Run("一个选项是另一个的子串时丢掉短的", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b1", Quote: "夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("expected 2 options after containment dedupe, got %+v", got)
		}
		for _, o := range got.Options {
			if o.Quote == "夜里放热" {
				t.Fatalf("the shorter, contained option survived: %+v", got.Options)
			}
		}
	})

	t.Run("短的先来也是丢短的，不是丢先来的", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "夜里放热"},
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("expected 2 options, got %+v", got)
		}
		if got.Options[0].Quote != "白天吸热、夜里放热" {
			t.Fatalf("the longer option must be the survivor, got %+v", got.Options)
		}
	})

	t.Run("包含式去重后只剩 1 个：整张卡片丢掉", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b1", Quote: "夜里放热"},
			},
		}, blocks)
		if got != nil {
			t.Fatalf("one option left after dedupe is no choice at all, got %+v", got)
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

	// —— 一轮只给一件教具（Task 5b (a)）——
	// 铁律③「一次只问一个」。透镜和卡片都是把这一步交回她手上；一轮里同时弹出
	// 两个，她不知道该做哪一个。丢卡片、留透镜：透镜是更重的器械，有召唤→选句→
	// 评估→发现一整套流程，而且已经落进 atom_card 并受 atom_card_one_open_idx
	// 约束；卡片是轻的，下一轮再给一张没有任何损失。
	allowLens := func(string) bool { return true }

	t.Run("同一轮既有透镜又有卡片：丢卡片、留透镜，turn 照常成功", func(t *testing.T) {
		const raw = `{"reply":"我先做一遍给你看。","advance":"","focusBlock":"b1","lens":"craap",
		  "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？","options":[
		    {"blockId":"b1","quote":"白天吸热、夜里放热"},
		    {"blockId":"b2","quote":"建筑密集阻碍了夜间散热"}]}}`
		got, ok := parseReadingCoachReply(raw, blocks, "zh", allowLens)
		if !ok {
			t.Fatalf("two instruments in one turn must not fail the turn")
		}
		if got.Lens != "craap" {
			t.Errorf("lens = %q, want it kept — it is the heavier instrument", got.Lens)
		}
		if got.Card != nil {
			t.Errorf("card = %+v, want nil — 一次只问一个", got.Card)
		}
		if got.Reply != "我先做一遍给你看。" {
			t.Errorf("reply = %q, want it kept intact", got.Reply)
		}
	})

	t.Run("只有卡片：照常存活（上一条不是把卡片一律干掉）", func(t *testing.T) {
		const raw = `{"reply":"来点一句。","advance":"","focusBlock":"b1","lens":"",
		  "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？","options":[
		    {"blockId":"b1","quote":"白天吸热、夜里放热"},
		    {"blockId":"b2","quote":"建筑密集阻碍了夜间散热"}]}}`
		got, ok := parseReadingCoachReply(raw, blocks, "zh", allowLens)
		if !ok {
			t.Fatalf("parse failed")
		}
		if got.Card == nil {
			t.Fatalf("a card with no lens beside it must survive")
		}
		if got.Lens != "" {
			t.Errorf("lens = %q, want empty", got.Lens)
		}
	})

	t.Run("只有透镜：和今天完全一致", func(t *testing.T) {
		got, ok := parseReadingCoachReply(
			`{"reply":"看这一句。","advance":"","focusBlock":"b1","lens":"craap"}`,
			blocks, "zh", allowLens)
		if !ok {
			t.Fatalf("parse failed")
		}
		if got.Lens != "craap" {
			t.Errorf("lens = %q, want craap", got.Lens)
		}
		if got.Card != nil {
			t.Errorf("card = %+v, want nil", got.Card)
		}
	})

	t.Run("透镜被拒时卡片不该跟着陪葬", func(t *testing.T) {
		// lens 没有 focusBlock → 透镜自己被丢掉；这一轮就只剩卡片一件教具，
		// 「一次只问一个」已经满足了，没有理由再把卡片也拿走。
		const raw = `{"reply":"来点一句。","advance":"","focusBlock":"","lens":"craap",
		  "card":{"type":"choose_span","prompt":"哪一句你读着最不服气？","options":[
		    {"blockId":"b1","quote":"白天吸热、夜里放热"},
		    {"blockId":"b2","quote":"建筑密集阻碍了夜间散热"}]}}`
		got, ok := parseReadingCoachReply(raw, blocks, "zh", allowLens)
		if !ok {
			t.Fatalf("parse failed")
		}
		if got.Lens != "" {
			t.Errorf("lens = %q, want it dropped (un-aimed)", got.Lens)
		}
		if got.Card == nil {
			t.Fatalf("an un-aimed lens must not take the card down with it")
		}
	})
}

// TestReadingCoachSystem_OneInstrumentPerTurnAndQuoteShape — two rulings the
// prompt has to carry itself, because the server-side half of each is SILENT.
//
//   - 一轮只给一件教具: the server drops the card whenever a lens survives the
//     same turn (铁律③). If the prompt never says so, every such turn quietly
//     loses a card the model meant to hand her.
//   - 引文从标点后面开始、到标点为止: a misaligned quote is dropped, and if fewer
//     than two options survive the whole card vanishes — no error, no log.
//     Task 4b found that even a careful HUMAN writing a natural-looking quote by
//     hand produced a mid-clause one without noticing (Task 5's own fixture);
//     the model will do it more often. So the loss is headed off on the model's
//     side too, not left entirely to the validator to clean up after.
func TestReadingCoachSystem_OneInstrumentPerTurnAndQuoteShape(t *testing.T) {
	for _, want := range []string{
		"这一轮已经给了 lens，就不要再给卡片",
		"从一个标点后面开始、到一个标点为止",
	} {
		if !strings.Contains(readingCoachSystem, want) {
			t.Errorf("readingCoachSystem no longer carries %q", want)
		}
	}
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
