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
		// 引文和落点段一个字都不动；段号是校验器**补上**的（stampOptionWhere，
		// 2026-09-17）—— 她看得懂的那个写法，服务端数的，模型给不了。
		if got.Options[0] != (coachCardOption{BlockID: "b1", Quote: "白天吸热、夜里放热", Where: "第1段"}) {
			t.Fatalf("option 0 changed: %+v", got.Options[0])
		}
		if got.Options[1] != (coachCardOption{BlockID: "b2", Quote: "空调外机把热量排到室外", Where: "第2段"}) {
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

	t.Run("两头都不在边界上的片段，贴回它自己那一句", func(t *testing.T) {
		// 🚨 这一条以前是「整张卡丢掉」。改成贴回去的理由在 snapQuoteToArticle：
		// 丢掉的代价她全担着 —— 屏幕上只剩一个输入框，而她不知道该打什么。
		// 真正要守的不变量没有变：**最终上卡的那句话，一定是原文里一段
		// 落在从句边界上的字面子串**。下面就是照着这条查的。
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				// 「白天吸|热、夜里|放热」—— 前后都是字，不是标点
				{BlockID: "b1", Quote: "吸热、夜里"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("这一句贴得回去，不该整张丢掉：%+v", got)
		}
		byID := map[string]string{}
		for _, b := range blocks {
			byID[b.ID] = b.Text
		}
		for _, o := range got.Options {
			if o.Quote == "吸热、夜里" {
				t.Errorf("半截片段原样上了卡片：%+v", got.Options)
			}
			if !coachCardQuoteIsClause(byID[o.BlockID], o.Quote) {
				t.Errorf("上卡的这一句没落在从句边界上：%q", o.Quote)
			}
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
				// R4：两条都取自 b2 的时候，这张卡片就是「b2 被剁开」——
				// 换成另一段的一句，被测的性质（引文带着句号）一个字没变。
				{BlockID: "b1", Quote: "白天吸热、夜里放热。"},
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
				// R4：同上，换一段取第二条；被测的性质（引文不带句号）不变。
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
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
		// R4：原来这两条都取自同一个 block，那正是「把一段剁开」。拆成两段之后，
		// 换行仍然被两头各考了一次：n1 那条前面紧挨着换行，n2 那条后面紧挨着换行。
		nl := []Block{
			{ID: "n1", Text: "城市越来越热\n夜里也降不下来"},
			{ID: "n2", Text: "空调开得更久\n电费也更高"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "n1", Quote: "夜里也降不下来"},
				{BlockID: "n2", Quote: "空调开得更久"},
			},
		}, nl)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a newline must count as a clause boundary, got %+v", got)
		}
	})

	t.Run("半角标点也是边界", func(t *testing.T) {
		// R4：两条都取自 e1 就是「把一段剁开」；第二段照样是英文，半角标点仍然
		// 被考到两次（e2 那条也跟在一个 ". " 后面）。
		en := []Block{
			{ID: "e1", Text: "Cities absorb heat by day. They release it at night, slowly."},
			{ID: "e2", Text: "Parks are cooler. Rooftops stay hot until dawn."},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "pick one",
			Options: []coachCardOption{
				// 句号后面隔着一个空格 —— 空格本身不是边界字符，但它也不该挡住这一句
				{BlockID: "e1", Quote: "They release it at night"},
				{BlockID: "e2", Quote: "Rooftops stay hot until dawn."},
			},
		}, en)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("half-width punctuation must count as a boundary, got %+v", got)
		}
	})

	t.Run("同一句话在段里出现两次时按合格的那一次算", func(t *testing.T) {
		// 第一次出现是从词中间切的，第二次出现落在边界上 —— 有一次合格就算合格。
		// R4：被测的是 d1 里那句出现两次的话；陪跑的第二条换到另一段去，
		// 免得整张卡片就是 d1 本身。
		dup := []Block{
			{ID: "d1", Text: "夜里放热的地表让城市更热。夜里放热，是热岛的根。"},
			{ID: "d2", Text: "绿地和水面能把这股热压下去一点。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "d1", Quote: "夜里放热"},
				{BlockID: "d2", Quote: "绿地和水面能把这股热压下去一点。"},
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
		// R4：被测的是引号包住的那一句；陪跑的一条换到 q2，整张卡片才不是 q1 自己。
		q := []Block{
			{ID: "q1", Text: "他说：“城市在夜里更热。”这句话有数据支撑。"},
			{ID: "q2", Text: "气象站的记录也是这么写的。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "q1", Quote: "城市在夜里更热。"},
				{BlockID: "q2", Quote: "气象站的记录也是这么写的。"},
			},
		}, q)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a sentence wrapped in 「“ ”」 must pass, got %+v", got)
		}
	})

	t.Run("括号包住的整句通过", func(t *testing.T) {
		// R4：同上，陪跑的一条换到 p2。
		p := []Block{
			{ID: "p1", Text: "夜里的温度更高。（数据来自城区的气象站。）后来又测了一次。"},
			{ID: "p2", Text: "第二次的结果差不多。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "p1", Quote: "数据来自城区的气象站。"},
				{BlockID: "p2", Quote: "第二次的结果差不多。"},
			},
		}, p)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a sentence wrapped in 「（ ）」 must pass, got %+v", got)
		}
	})

	t.Run("单引号包住的整句通过", func(t *testing.T) {
		// R4：同上，陪跑的一条换到 s2。
		s := []Block{
			{ID: "s1", Text: "她问：‘城市为什么在夜里更热’，没人答得上来。"},
			{ID: "s2", Text: "这个问题后来被写进了课本。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				// 收尾的 ’ 也要跳过：它后面那个 ，才是真正的边界。
				{BlockID: "s1", Quote: "城市为什么在夜里更热"},
				{BlockID: "s2", Quote: "这个问题后来被写进了课本。"},
			},
		}, s)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("a sentence wrapped in 「‘ ’」 must pass, got %+v", got)
		}
	})

	t.Run("英文引号包住的整句通过", func(t *testing.T) {
		// R4：同上，陪跑的一条换到 e3。
		e := []Block{
			{ID: "e2", Text: `He said, "Cities are hotter at night." Nobody disagreed.`},
			{ID: "e3", Text: "The data came from the airport station."},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "pick one",
			Options: []coachCardOption{
				{BlockID: "e2", Quote: "Cities are hotter at night."},
				{BlockID: "e3", Quote: "The data came from the airport station."},
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

	// —— 包含式去重比的是「会活到最后的那批」（Task 5c (1)）——
	// 拿整个未截断的候选池当参照会两头落空：短的因为「池子里有更完整的那条」
	// 被丢掉，而那条更完整的排在截断线之后，根本没被走到。她看到的卡片上两条都没有。

	t.Run("包含它的那条排在截断线之后：不能两条都消失", func(t *testing.T) {
		// R4：候选原来全在 b1 里——那本身就是「一段被剁开」的形状，新规则会
		// 整张丢掉。把其中一条搬进 b2 就够了：A 和 G 仍然同段、仍然是包含关系，
		// 被测的「顶替 + 截断」一步没变，而 b2 那条必须落在**截断线之前**——
		// 落在后面的话，最终 4 条又全在 b1，跨段落检查（在截断之后跑）照样毙掉它。
		wide := []Block{
			{ID: "b1", Text: "夜里放热。建筑密集阻碍散热。空调外机排热到室外。绿地和水面能降温。白天吸热、夜里放热。"},
			{ID: "b2", Text: "城市地表以沥青为主。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "夜里放热"}, // A：被最后那条包含
				{BlockID: "b2", Quote: "城市地表以沥青为主"},
				{BlockID: "b1", Quote: "建筑密集阻碍散热"},
				{BlockID: "b1", Quote: "空调外机排热到室外"},
				{BlockID: "b1", Quote: "绿地和水面能降温"},
				{BlockID: "b1", Quote: "白天吸热、夜里放热"}, // G：包含 A，排在第 6 位
			},
		}, wide)
		if got == nil || len(got.Options) != 4 {
			t.Fatalf("expected 4 options, got %+v", got)
		}
		var sawShort, sawLong bool
		for _, o := range got.Options {
			if o.Quote == "夜里放热" {
				sawShort = true
			}
			if o.Quote == "白天吸热、夜里放热" {
				sawLong = true
			}
		}
		if !sawShort && !sawLong {
			t.Fatalf("A 和 G 一起消失了：丢掉短的那条理由指向了卡片上不存在的东西，got %+v", got.Options)
		}
		if !sawLong {
			t.Fatalf("留的应该是更完整的那条，got %+v", got.Options)
		}
	})

	// —— 破折号和省略号也是边界（Task 5c (2)）——
	// `——` / `……` 在中学生读的说明文里很常见；不认它们，破折号后面起头的那一句
	// 会被静默误杀，而误杀看起来就像 印记 这一轮没想出卡片。

	t.Run("破折号后面起头的句子通过", func(t *testing.T) {
		// R4：被测的是破折号后面起头的那一句；陪跑的一条换到 d2。
		dash := []Block{
			{ID: "d1", Text: "他说了一件事——城市在夜里更热。夏天尤其明显。"},
			{ID: "d2", Text: "冬天的差距要小一些。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "d1", Quote: "城市在夜里更热。"},
				{BlockID: "d2", Quote: "冬天的差距要小一些。"},
			},
		}, dash)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("破折号后面那一句被误杀了，got %+v", got)
		}
	})

	t.Run("省略号后面起头的句子通过", func(t *testing.T) {
		// R4：被测的是省略号后面起头的那一句；陪跑的一条换到 e2。
		dots := []Block{
			{ID: "e1", Text: "他数了很久……城市在夜里更热。冬天也一样。"},
			{ID: "e2", Text: "他把这些数字记了整整一年。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "e1", Quote: "城市在夜里更热。"},
				{BlockID: "e2", Quote: "他把这些数字记了整整一年。"},
			},
		}, dots)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("省略号后面那一句被误杀了，got %+v", got)
		}
	})

	t.Run("段里有破折号也不放行从词中间截的窗口", func(t *testing.T) {
		dash := []Block{{ID: "d1", Text: "他说了一件事——城市地表以沥青和混凝土为主，白天吸热。"}}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "d1", Quote: "表以沥青和混凝土"},
				{BlockID: "d1", Quote: "沥青和混凝土为"},
			},
		}, dash)
		if got != nil {
			t.Fatalf("破折号不该把从句中间截的窗口一起放行，got %+v", got)
		}
	})

	// —— 选项必须跨段落（Task R4 (1)）——
	// 逐字校验保证她**看**了文章；它保证不了她**读懂**了。真实走查里那张
	// 「第三段里，哪一句让你最清楚地看到钱去了哪里？」，三个选项就是第三段的
	// 全部三句、按原文顺序排下来——每一条都过了上面所有的检查，而她一段都不用读，
	// 因为选项**就是**那一段。这一条是唯一一条看**选项集**的规则。

	t.Run("选项全部来自同一段：整张卡片被丢掉", func(t *testing.T) {
		// 走查里那张卡的形状：一段被剁成它的全部句子。
		one := []Block{
			{ID: "b1", Text: "先说钱去了哪里。"},
			{ID: "b2", Text: "一部分用来养这套系统本身。另一部分是平台的利润。规则越精细，成本越高。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "哪一句让你最清楚地看到钱去了哪里？",
			Options: []coachCardOption{
				{BlockID: "b2", Quote: "一部分用来养这套系统本身。"},
				{BlockID: "b2", Quote: "另一部分是平台的利润。"},
				{BlockID: "b2", Quote: "规则越精细，成本越高。"},
			},
		}, one)
		if got != nil {
			t.Fatalf("三个选项就是第二段本身，她一段都不用读；整张卡片必须丢掉，got %+v", got)
		}
	})

	t.Run("选项跨两个段落：通过", func(t *testing.T) {
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "作者是怎么让你相信这笔账划算的？",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, blocks)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("两段各取一句正是这条规则要的形状，不许误杀：%+v", got)
		}
	})

	t.Run("跨段落判定在截断之后：被切掉第二段就不算跨段", func(t *testing.T) {
		// 🚨 这一条钉的是**判定的位置**。放在截断之前判，这张卡片会「合法」地
		// 发出去，而她屏幕上的 4 条全在 b1——正是这条规则要挡的那个东西。
		wide := []Block{
			{ID: "b1", Text: "夜里放热。建筑密集阻碍散热。空调外机排热到室外。绿地和水面能降温。"},
			{ID: "b2", Text: "城市地表以沥青为主。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "夜里放热"},
				{BlockID: "b1", Quote: "建筑密集阻碍散热"},
				{BlockID: "b1", Quote: "空调外机排热到室外"},
				{BlockID: "b1", Quote: "绿地和水面能降温"},
				// 唯一的第二段落在第 5 位 —— 截断到 4 之后它就不在卡片上了。
				{BlockID: "b2", Quote: "城市地表以沥青为主"},
			},
		}, wide)
		if got != nil {
			t.Fatalf("截断把唯一的另一段切掉了，剩下的 4 条就是 b1 本身，got %+v", got)
		}
	})

	t.Run("包含式顶替之后仍然跨段落：照常通过", func(t *testing.T) {
		// 顶替发生在 b1 内部，b2 那条一直站着 —— 跨段落这条不该跟着受牵连。
		mix := []Block{
			{ID: "b1", Text: "白天吸热、夜里放热。"},
			{ID: "b2", Text: "空调外机把热量排到室外。"},
		}
		got := validateCoachCard(&coachCard{
			Type:   "choose_span",
			Prompt: "挑一句",
			Options: []coachCardOption{
				{BlockID: "b1", Quote: "夜里放热"},
				{BlockID: "b1", Quote: "白天吸热、夜里放热"},
				{BlockID: "b2", Quote: "空调外机把热量排到室外"},
			},
		}, mix)
		if got == nil || len(got.Options) != 2 {
			t.Fatalf("expected 2 options, got %+v", got)
		}
		if got.Options[0].Quote != "白天吸热、夜里放热" || got.Options[1].BlockID != "b2" {
			t.Fatalf("顶替之后的选项集变形了：%+v", got.Options)
		}
	})

	t.Run("pick_in_article / short_text 不受跨段落约束", func(t *testing.T) {
		// 这条规则管的是「选项集就是一段话」这件事；没有选项的卡片没有这个问题。
		for _, typ := range []string{"pick_in_article", "short_text"} {
			got := validateCoachCard(&coachCard{Type: typ, Prompt: "去文章里点出最站不住的那一句"}, blocks)
			if got == nil {
				t.Fatalf("%s 没有 options，跨段落规则不该碰它", typ)
			}
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
		"哪一句的判断需要更多证据",       // the ruling's own example
		"reply 就不要再把它复述一遍", // the card carries the instruction now
	} {
		if !strings.Contains(readingCoachSystem, want) {
			t.Errorf("readingCoachSystem no longer carries %q", want)
		}
	}
}

// TestCardAnswerQuotesEveryLine —— 这条测试存在的理由是一次已经发生过的事故。
//
// 2026-08-29：ReadingCoachPanel 只给引用的**第一行**加了 `> `。SplitBlocks 只按
// 空行切段，所以一段硬换行的原文（PDF 粘贴、诗、对白）是**一个带换行的 block**；
// 多行引用的后几行进了 role='student' 的行里、身上没有前缀，穿过
// stripQuotedLines，成了可以印在她署名的图片上的「她自己的话」。
// 卡片选项就是文章原句 —— 同一颗地雷的第二次机会。
func TestCardAnswerQuotesEveryLine(t *testing.T) {
	// 一段硬换行的原文 —— SplitBlocks 只按空行切，所以这是「一个 block 里带换行」
	quote := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"
	got := composeCardAnswerMessage("哪一句你读着最不服气？", quote, "")
	for _, line := range strings.Split(quote, "\n") {
		if !strings.Contains(got, "> "+line) {
			t.Fatalf("line not quoted: %q\nfull message:\n%s", line, got)
		}
	}
}

func TestComposeCardAnswerMessage(t *testing.T) {
	quote := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"

	t.Run("文章的每一行都被 stripQuotedLines 拿走", func(t *testing.T) {
		// R4 的真实执行点：报告语料只留 stripQuotedLines 之后剩下的东西。
		// 引用的任何一行漏进这里，就是可以印在她署名的图片上的原文。
		got := composeCardAnswerMessage("哪一句你读着最不服气？", quote, "")
		if left := stripQuotedLines(got); left != "" {
			t.Fatalf("article text survived into her own corpus: %q", left)
		}
	})

	t.Run("卡片的问题也不是她的话", func(t *testing.T) {
		// 提问是 印记 写的。它跟原文一样不能算进「她自己的话」。
		got := composeCardAnswerMessage("哪一句你读着最不服气？", quote, "")
		if !strings.Contains(got, "> 【印记问】哪一句你读着最不服气？") {
			t.Fatalf("the card's question is not quoted:\n%s", got)
		}
	})

	t.Run("她自己写的那句不加前缀", func(t *testing.T) {
		// 反方向同样要命：给她的话加上 `> `，等于把她从她自己的报告里抹掉。
		got := composeCardAnswerMessage("你读着最不服气的是哪一句？", "", "我觉得作者只算了成本，没算住在那儿的人。")
		if strings.Contains(got, "> 我觉得") {
			t.Fatalf("her own words were quoted away:\n%s", got)
		}
		if stripQuotedLines(got) != "我觉得作者只算了成本，没算住在那儿的人。" {
			t.Fatalf("her own words did not survive: %q", stripQuotedLines(got))
		}
	})

	t.Run("既点了又打字：原文进引用，她的话留下", func(t *testing.T) {
		got := composeCardAnswerMessage("哪一句你读着最不服气？", quote, "这句话把人当成了温度计。")
		for _, line := range strings.Split(quote, "\n") {
			if !strings.Contains(got, "> "+line) {
				t.Fatalf("line not quoted: %q\n%s", line, got)
			}
		}
		if stripQuotedLines(got) != "这句话把人当成了温度计。" {
			t.Fatalf("her own words did not survive: %q", stripQuotedLines(got))
		}
	})

	t.Run("什么都没有就是空字符串", func(t *testing.T) {
		// 空内容不写行 —— 一条空的 student 消息比没有更糟。
		if got := composeCardAnswerMessage("", "", ""); got != "" {
			t.Fatalf("want empty, got %q", got)
		}
	})

	t.Run("点了就是指了：合成出来的消息认得出来", func(t *testing.T) {
		// hunt 步的 F3 守门人靠 `> ` 行回读文章。她点卡片选项也是「指」，
		// 合成的消息必须过得了这一关，否则她指了却推不动这一步。
		blocks := []Block{{ID: "b1", Text: quote}}
		got := composeCardAnswerMessage("哪一句你读着最不服气？", quote, "")
		if !quotedLinesCiteArticle(got, blocks) {
			t.Fatalf("a tapped answer does not read as pointing:\n%s", got)
		}
	})

	// 🚨 这一条是这次改动的核心断言。判定过去是「整段一刀切」：整个字符串
	// 匹配不上某个 block，**所有行**就都裸着落进 role='student' 的行里。
	// 一条消息里既有文章原文的行、又有她自己写的行时，那就是原文裸奔。
	t.Run("按行判定：文章的行带前缀，她自己的行裸着", func(t *testing.T) {
		art1 := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"
		art2 := "树冠能挡掉一部分直射，也能把水汽送回空气里。"
		blocks := []Block{{ID: "b1", Text: art1}, {ID: "b2", Text: art2}}
		hers := "我觉得作者只算了成本，没算住在那儿的人。"
		// 跨段落的选择整段匹配不上（block 按构造不含空行），于是它落到
		// 「她自己的话」那一半 —— 混着她真正写的那句一起。
		own := art1 + "\n\n" + art2 + "\n\n" + hers
		got := composeCardAnswerMessage("", "", own, blocks...)
		for _, line := range append(strings.Split(art1, "\n"), art2) {
			if !strings.Contains(got, "> "+line) {
				t.Fatalf("article line reached the transcript bare: %q\nfull message:\n%s", line, got)
			}
		}
		if left := stripQuotedLines(got); left != hers {
			t.Fatalf("only her own words may survive stripQuotedLines, got %q", left)
		}
	})

	t.Run("她的话即使夹在原文中间也不会被前缀掉", func(t *testing.T) {
		art1 := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"
		blocks := []Block{{ID: "b1", Text: art1}}
		hers := "我觉得作者只算了成本。"
		got := composeCardAnswerMessage("", "", "白天吸热、夜里放热。\n"+hers, blocks...)
		if !strings.Contains(got, "\n"+hers) && !strings.HasPrefix(got, hers) {
			t.Fatalf("her own line was quoted away:\n%s", got)
		}
		if left := stripQuotedLines(got); left != hers {
			t.Fatalf("her own words did not survive: %q", left)
		}
	})
}

// TestCardAnswerChoiceParts —— 「她点的这句是文章的还是她自己的」这个判断，
// 以及它带来的 pick。判错的代价是文章原文裸着进 role='student' 的行。
func TestCardAnswerChoiceParts(t *testing.T) {
	art1 := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"
	art2 := "树冠能挡掉一部分直射，也能把水汽送回空气里。"
	blocks := []Block{{ID: "b1", Text: art1}, {ID: "b2", Text: art2}}

	// (1) SplitBlocks 把正文归一成 LF，判定器却不归一化 —— 带 \r\n 的多行引用
	// 于是永远匹配不上：两行原文全部裸着落地，而且她点了却不算点。
	t.Run("CRLF 的多行选择仍然是文章原文，而且算点了", func(t *testing.T) {
		choice := strings.ReplaceAll(art1, "\n", "\r\n")
		quote, own, picks := cardAnswerChoiceParts(choice, "b1", blocks)
		if own != "" {
			t.Fatalf("CRLF quote was mistaken for her own words: %q", own)
		}
		if quote == "" {
			t.Fatal("CRLF quote was not recognized as article text")
		}
		if len(picks) == 0 {
			t.Fatal("she pointed, but the hunt step will never settle on it")
		}
		got := composeCardAnswerMessage("", quote, "", blocks...)
		for _, line := range strings.Split(art1, "\n") {
			if !strings.Contains(got, "> "+line) {
				t.Fatalf("article line reached the transcript bare: %q\n%s", line, got)
			}
		}
		if left := stripQuotedLines(got); left != "" {
			t.Fatalf("article text survived into her own corpus: %q", left)
		}
	})

	// (2) block 按构造永远不含空行，所以任何跨段落的选择整段一律判成
	// 「她自己的话」—— 一个 `> ` 都没有。按行判定才接得住。
	t.Run("跨段落的选择：每一行原文都带前缀", func(t *testing.T) {
		quote, own, _ := cardAnswerChoiceParts(art1+"\n\n"+art2, "b1", blocks)
		got := composeCardAnswerMessage("", quote, own, blocks...)
		for _, line := range append(strings.Split(art1, "\n"), art2) {
			if !strings.Contains(got, "> "+line) {
				t.Fatalf("article line reached the transcript bare: %q\n%s", line, got)
			}
		}
		if left := stripQuotedLines(got); left != "" {
			t.Fatalf("article text survived into her own corpus: %q", left)
		}
	})

	// (3) 「这算点了」没有长度下限：两个字碰巧是文章的子串，就被提升成 pick，
	// hasHuntPickEvidence 于是认为她指了 —— 正是系统 prompt F3 那条要防的事。
	t.Run("两个字的回答不会被提升进 picks", func(t *testing.T) {
		_, _, picks := cardAnswerChoiceParts("吸热", "b1", blocks)
		if len(picks) != 0 {
			t.Fatalf("a two-character answer counted as pointing at the article: %+v", picks)
		}
	})

	t.Run("她自己写的一段是她的话", func(t *testing.T) {
		hers := "我觉得作者只算了成本，没算住在那儿的人。"
		quote, own, picks := cardAnswerChoiceParts(hers, "b1", blocks)
		if quote != "" || own != hers || len(picks) != 0 {
			t.Fatalf("her own sentence was mistaken for the article's: quote=%q own=%q picks=%+v", quote, own, picks)
		}
	})
}

// TestCardAnswerDedupesPicks —— 同一句可以两条路一起到：面板把每个划选内联进
// picks，点中的那句又被提升一次。不去重，【她在文章里点出来的句子】就列两遍。
func TestCardAnswerDedupesPicks(t *testing.T) {
	p := readingPick{BlockID: "b1", Quote: "白天吸热、夜里放热。"}
	got := dedupeReadingPicks([]readingPick{p, p, {BlockID: "b2", Quote: "树冠能挡掉一部分直射"}})
	if len(got) != 2 {
		t.Fatalf("duplicate pick survived: %+v", got)
	}
	if got[0] != p {
		t.Fatalf("dedupe reordered the picks: %+v", got)
	}
}

// TestCardAnswerPromptIsNotHerPointing —— prompt 是没校验的客户端输入。
// 它每行都会被 `> ` 掉，所以不泄漏语料；但一条正好等于文章句子的 prompt 会让
// quotedLinesCiteArticle 为真 —— 下一轮的 hasHuntPickEvidence 就会把 印记
// 自己的问题读成「她点过了」。
func TestCardAnswerPromptIsNotHerPointing(t *testing.T) {
	art1 := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"
	blocks := []Block{{ID: "b1", Text: art1}}

	t.Run("印记 自己的问题不算她指了文章", func(t *testing.T) {
		// 折行之后，第二行**正好**是文章里的一句话 —— 探针 K 的形状。
		prompt := "这一句你服气吗：\n白天吸热、夜里放热。"
		got := composeCardAnswerMessage(prompt, "", "服气一半。", blocks...)
		if quotedLinesCiteArticle(got, blocks) {
			t.Fatalf("the coach's own question reads as her pointing at the article:\n%s", got)
		}
		if left := stripQuotedLines(got); left != "服气一半。" {
			t.Fatalf("only her own words may survive stripQuotedLines, got %q", left)
		}
	})

	t.Run("超长的 prompt 是编出来的，丢掉", func(t *testing.T) {
		// validateCoachCard 卡在 60 runes；点击回答这条路上一个字都没校验。
		long := strings.Repeat("很", coachCardPromptMaxRunes+1)
		got := composeCardAnswerMessage(long, "", "服气一半。", blocks...)
		if strings.Contains(got, long) {
			t.Fatalf("an over-long client-sent prompt reached the transcript:\n%s", got)
		}
		if got != "服气一半。" {
			t.Fatalf("her own words are the whole message, got %q", got)
		}
	})
}

// TestCardAnswerPayloadIgnoresEmptyAnswer —— `{"answer":{}}`（字段都是
// omitempty）会让重渲染逻辑把一条普通打字消息当成卡片回答。
func TestCardAnswerPayloadIgnoresEmptyAnswer(t *testing.T) {
	if raw := coachCardAnswerPayload(&coachCardAnswer{}); raw != nil {
		t.Fatalf("an empty answer must store SQL NULL, got %s", raw)
	}
	if raw := coachCardAnswerPayload(&coachCardAnswer{BlockID: "b1"}); raw != nil {
		t.Fatalf("an answer with neither type nor choice is not an answer, got %s", raw)
	}
	if coachCardAnswerPayload(&coachCardAnswer{Type: coachCardShortText, Prompt: "你怎么看？"}) == nil {
		t.Fatal("a short_text card answered by typing must still be stored")
	}
}

// TestCardAnswerSurvivesWithoutTheSecondNet —— 最狠的一条。
//
// 第二道网 stripArticleLines 自己的注释就写着：源数据行没了的时候它退化成
// no-op（atom_report.go）。所以第一道网必须自己站得住 —— 把合成出来的消息
// 只喂给 stripQuotedLines，文章原文一个字都不许活下来。
func TestCardAnswerSurvivesWithoutTheSecondNet(t *testing.T) {
	art1 := "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"
	art2 := "树冠能挡掉一部分直射，也能把水汽送回空气里。"
	blocks := []Block{{ID: "b1", Text: art1}, {ID: "b2", Text: art2}}
	hers := "我觉得作者只算了成本，没算住在那儿的人。"

	// handler 的拼法：跨段落的选择 + 她同一轮打的字。
	quote, ownFromChoice, _ := cardAnswerChoiceParts(art1+"\n\n"+art2, "b1", blocks)
	own := hers
	if ownFromChoice != "" {
		own = ownFromChoice + "\n\n" + hers
	}
	got := composeCardAnswerMessage("哪一句你读着最不服气？", quote, own, blocks...)

	left := stripQuotedLines(got)
	if left != hers {
		t.Fatalf("net 1 alone let something through: %q", left)
	}
	for _, blk := range blocks {
		for _, line := range strings.Split(blk.Text, "\n") {
			if strings.Contains(left, line) {
				t.Fatalf("article text survived net 1 alone: %q\nfull message:\n%s", line, got)
			}
		}
	}
	// 第二道网退化成 no-op（blocks == nil）时结果必须一样。
	if after := stripArticleLines(left, nil); after != hers {
		t.Fatalf("with the second net degraded the corpus changed: %q", after)
	}
}

// TestQuoteIsArticleText —— 「这段字是不是文章的」这个判断必须去看文章，
// 不能信客户端捎来的 type 字段。
func TestQuoteIsArticleText(t *testing.T) {
	blocks := []Block{
		{ID: "b1", Text: "城市地表以沥青和混凝土为主，\n白天吸热、夜里放热。"},
		{ID: "b2", Text: "树冠能挡掉一部分直射。"},
	}
	t.Run("跨行的原句也是原句", func(t *testing.T) {
		if !quoteIsArticleText("为主，\n白天吸热", blocks) {
			t.Fatal("a quote spanning a hard line break was not recognized as article text")
		}
	})
	t.Run("她自己的话不是原句", func(t *testing.T) {
		if quoteIsArticleText("我觉得作者只算了成本。", blocks) {
			t.Fatal("her own sentence was mistaken for the article's")
		}
	})
	t.Run("空的不算", func(t *testing.T) {
		if quoteIsArticleText("   ", blocks) {
			t.Fatal("blank counted as article text")
		}
	})
}

// TestCoachCardAnswerPayload —— 她的答案跟着那条 student 消息一起存下来，
// 这样刷新之后房间知道那张卡片她已经答过了。
func TestCoachCardAnswerPayload(t *testing.T) {
	if coachCardAnswerPayload(nil) != nil {
		t.Fatal("a nil answer must store SQL NULL, not an empty shell")
	}
	raw := coachCardAnswerPayload(&coachCardAnswer{
		Type:    coachCardChooseSpan,
		Prompt:  "哪一句你读着最不服气？",
		Choice:  "白天吸热、夜里放热。",
		BlockID: "b1",
	})
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("payload is not JSON: %v (%s)", err, raw)
	}
	if _, hasCard := back["card"]; hasCard {
		t.Errorf("an answer payload must not carry a card key: %s", raw)
	}
	answer, isObject := back["answer"].(map[string]any)
	if !isObject {
		t.Fatalf("payload[\"answer\"] is %T, want a JSON object", back["answer"])
	}
	if answer["choice"] != "白天吸热、夜里放热。" || answer["blockId"] != "b1" {
		t.Errorf("the answer did not survive: %+v", answer)
	}
}

// TestCardAnswerQuotesEveryPromptLine —— 提问本身也可能换行。
// `prompt` 是客户端捎上来的，卡片的问题里完全可能引一句原文；问题折到第二行，
// 那一行就会**不带前缀**落进 role='student' 的行里 —— 和 2026-08-29 那次
// 一模一样的形状，只是换了个字段。这条路上除了她自己的话，
// 没有任何一行可以裸着进 transcript。
func TestCardAnswerQuotesEveryPromptLine(t *testing.T) {
	prompt := "作者说「城市地表以沥青和混凝土为主，\n白天吸热、夜里放热」——你服气吗？"
	got := composeCardAnswerMessage(prompt, "", "服气一半。")
	for _, line := range strings.Split(got, "\n") {
		if strings.TrimSpace(line) == "" || line == "服气一半。" {
			continue
		}
		if !strings.HasPrefix(line, "> ") {
			t.Fatalf("prompt line reached the transcript bare: %q\nfull message:\n%s", line, got)
		}
	}
	if stripQuotedLines(got) != "服气一半。" {
		t.Fatalf("only her own words may survive stripQuotedLines, got %q", stripQuotedLines(got))
	}
}

// 🚨 引文差一点点对不上，贴回原文那一句，而不是把整张卡丢掉。
//
// 实测那一幕：日志写「fewer than 2 options survived the article check」，
// 她那边 印记 说「我们集中看第 2 段」然后什么都没给 ——
// 「只有一个输入框，不知道该往里面打什么字」。
func TestSnapQuoteToArticle(t *testing.T) {
	body := "Aid groups said the blockade had made every delivery slower. " +
		"Officials cautioned that the figure could not be independently verified."
	cases := []struct{ name, quote, want string }{
		{
			"少了收尾的句号",
			"Aid groups said the blockade had made every delivery slower",
			"Aid groups said the blockade had made every delivery slower.",
		},
		{
			"从半句中间起头",
			"the figure could not be independently verified",
			"Officials cautioned that the figure could not be independently verified.",
		},
		{
			"说的是别的句子 —— 宁可丢掉，也不要贴错一句让她去文章里找",
			"完全无关的一句中文，和这一段没有任何关系。",
			"",
		},
	}
	for _, c := range cases {
		if got := snapQuoteToArticle(body, c.quote); got != c.want {
			t.Errorf("%s：snapQuoteToArticle(%q) = %q，想要 %q", c.name, c.quote, got, c.want)
		}
	}
}

func TestCardSurvivesANearMissQuote(t *testing.T) {
	blocks := SplitBlocks("Aid groups said the blockade had made every delivery slower. " +
		"Officials cautioned that the figure could not be independently verified." +
		"\n\n第二段在讲别的事情，句子也够长，能上卡片。")
	c := &coachCard{
		Type:   coachCardChooseSpan,
		Prompt: "哪一句更像主张？",
		Options: []coachCardOption{
			// 两句都差一点：一句少了句号，一句从半句中间起头。
			// choose_span 的选项要来自不同段落（同段的一对没法靠扫读分辨）。
			{BlockID: "b1", Quote: "Aid groups said the blockade had made every delivery slower"},
			{BlockID: "b2", Quote: "第二段在讲别的事情，句子也够长"},
		},
	}
	got, why := validateCoachCardWhy(c, blocks)
	if got == nil {
		t.Fatalf("整张卡被丢掉了，理由 %q —— 两句都贴得回去", why)
	}
	byID := map[string]string{}
	for _, b := range blocks {
		byID[b.ID] = b.Text
	}
	for _, o := range got.Options {
		if !strings.Contains(byID[o.BlockID], o.Quote) {
			t.Errorf("贴回去之后这一句还是不在 %s 里：%q", o.BlockID, o.Quote)
		}
	}
}

// 🚨 教它换一种说法的时候，认「它是不是在指着一块板说话」的那张表要跟着换。
//
// 实测：上一条 prompt 教它改口说「把这几句各自放进它的角色里」（不要自己编
// 格子名），而 cardPromiseWords 里一个字都没对上 —— 于是服务端没有补板，
// 她看到的是那句话加上一片空白：「屏幕上看不到任何格子、板子或者可以拖拽的
// 地方，我不知道该把句子放到哪里去。」
func TestPromiseWordsCoverThePhrasingWeTeachIt(t *testing.T) {
	for _, reply := range []string{
		"现在把这几句各自放进它的角色里。",
		"看看第 2 段这三句，各自放进哪个角色。",
		"把它们归到各自的角色里。",
	} {
		if !replyPromisesACard(reply) {
			t.Errorf("这是在指着一块板说话，没认出来：%q", reply)
		}
	}
	// 只是提到「角色」不算 —— 讲解里常常出现这个词。
	if replyPromisesACard("这一句在论证里的角色是证据。") {
		t.Error("讲解里提到角色被误判成了在指板说话")
	}
}

// 🚨 题目里另起一套格子名的，把题目换回标准那一句。
//
// 格子是闭表、由服务端填，但**题目**不是 —— 于是屏幕上是「把卡片放进
// 『进不去/动不了/快撑不住了』三个格子」，而下面摆着的是主张/证据/限制/背景/
// 对比。她逐字报的：「名字完全不一样，我不知道哪个对应哪个，没法往下做。」
func TestLabelPromptInventsBins(t *testing.T) {
	for prompt, want := range map[string]bool{
		"把这几句放进「进不去/动不了/快撑不住了」三个格子":  true,
		"放进进不去/动不了/快撑不住了":            true,
		"这几句在作者的论证里各自扮演什么角色？":        false,
		"把这几句各自放进它的角色里":              false,
		// 闭表里的名字照说不算编。
		"哪一句是「主张」，哪一句是「证据」？":         false,
		"分成主张/证据两类":                  false,
	} {
		if got := labelPromptInventsBins(prompt); got != want {
			t.Errorf("labelPromptInventsBins(%q) = %v，想要 %v", prompt, got, want)
		}
	}
}

func TestInventedBinsGetTheStandardPrompt(t *testing.T) {
	blocks := SplitBlocks("第一段这句话足够长，可以上板使用。\n\n第二段这句话也足够长，同样可以上板。")
	c := &coachCard{
		Type:   coachCardLabelRoles,
		Prompt: "把这几句放进「进不去/动不了/快撑不住了」三个格子",
		Options: []coachCardOption{
			{BlockID: "b1", Quote: "第一段这句话足够长，可以上板使用。"},
			{BlockID: "b2", Quote: "第二段这句话也足够长，同样可以上板。"},
		},
	}
	got, why := validateCoachCardWhy(c, blocks)
	if got == nil {
		t.Fatalf("不该丢卡，丢了她这一步什么都没有：%q", why)
	}
	if got.Prompt != coachLabelBoardPrompt {
		t.Fatalf("题目没换回标准那一句：%q", got.Prompt)
	}
	// 格子仍然是闭表里的那一套。
	if len(got.Labels) != len(coachArgueBinsBasic) {
		t.Fatalf("格子被改了：%+v", got.Labels)
	}
}

// 🚨 题目就是那道题：不写怎么操作，也不写有几句。
//
// 产品负责人 2026-09-12 逐字指过这一张：「这三句各自在算账的哪一步？拖到角色
// 各自里。」—— 板上只有两句，而且这不像一道题。
func TestPromptTellsHerHowToDrag(t *testing.T) {
	for prompt, want := range map[string]bool{
		"这三句各自在算账的哪一步？拖到角色各自里。": true,
		"把它们拖进对应的格子。":            true,
		"分析下列句子，判断它们各自属于哪一类论证成分。": false,
	} {
		if got := promptTellsHerHowToDrag(prompt); got != want {
			t.Errorf("promptTellsHerHowToDrag(%q) = %v，想要 %v", prompt, got, want)
		}
	}
}

func TestPromptCountMismatch(t *testing.T) {
	// 说三句、板上两句 —— 她数得出来。
	if !promptCountMismatch("这三句各自在算账的哪一步？", 2) {
		t.Error("说了三句、只有两句，应该算对不上")
	}
	if promptCountMismatch("这三句各自在算账的哪一步？", 3) {
		t.Error("数目对得上，不该判")
	}
	// 没写数目是对的写法。
	if promptCountMismatch("分析下列句子，判断它们各自属于哪一类论证成分。", 2) {
		t.Error("题目里没写数目，不该判")
	}
}

func TestAwkwardLabelPromptGetsTheStandardOne(t *testing.T) {
	blocks := SplitBlocks("第一段这句话足够长，可以上板使用。\n\n第二段这句话也足够长，同样可以上板。")
	c := &coachCard{
		Type:   coachCardLabelRoles,
		Prompt: "这三句各自在算账的哪一步？拖到角色各自里。",
		Options: []coachCardOption{
			{BlockID: "b1", Quote: "第一段这句话足够长，可以上板使用。"},
			{BlockID: "b2", Quote: "第二段这句话也足够长，同样可以上板。"},
		},
	}
	got, why := validateCoachCardWhy(c, blocks)
	if got == nil {
		t.Fatalf("不该丢卡：%q", why)
	}
	if got.Prompt != coachLabelBoardPrompt {
		t.Fatalf("题目没换成标准那一句：%q", got.Prompt)
	}
}
