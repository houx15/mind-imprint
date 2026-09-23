package api

import (
	"strings"

	"mindimprint/api/internal/guidance"
)

// writing_symptoms.go —— 反馈的两张闭表：**先说哪一层**，和**这是哪一种毛病**。
//
// # 为什么要有层
//
// 四份互不相干的材料收敛到同一句话（`docs/2026-09-11-writing-guidance-redesign.md`）：
//
//	qifeng `technique-router.md`      高层问题没有解决时，低层润色必须让位。
//	master-writing `scoring-rubric.md` 先看观点和材料，最后才看语言。
//	                                   观点没立住的时候去改句子，是在给房子刷漆。
//	cap-writing-coach                  題意 → 材料 → 段落 → 字句，
//	                                   而且明写「是回饋順序，不是官方配分」。
//	draft-review-kit `revision`        developmental → line → copy。
//
// 在这之前这个房间一条优先级都没有：`writingCommentSystem` 让模型
//「从结构、论证、证据、语言这些角度」自己挑几条，于是一篇主张还没立住的文章，
// 收到的第一条意见完全可能是某个词不准。那一刀只有一次，花在第四层就浪费了。
//
// 所以层不是建议，是**代码筛子**：`validateCommentPoints` 只留最上面那一层的
// 问题点，下面几层整批丢掉。见那个函数。
//
// # 为什么症状要闭表
//
// 和 `method_ids` 闭表、阅读室角色格子闭表同一个理由：模型编得出第十九种毛病，
// 而第十九种毛病在优先级里没有位置、在方法库里没有对应的技法。闭表还让
//「它到底在说什么」这件事可以统计——同一个 id 在她整篇里出现几次，是过程数据。
//
// 🚨 每一条都必须有一条**看得见的信号**，不是一个形容词。这是 qifeng 那张表
// 最值钱的地方：「论证像观点清单」的信号是「**段落可以任意换序**」，
// 而不是「论证不够深入」。前者她自己能验，后者只能听着。
//
// # 中英两张表，不是一张
//
// 中文那张来自 qifeng 的诊断路由；英文那张来自 `ielts-writing-coach` 的
// `exercise-contracts.md`（四个维度十三个 id，原文明写
// "Unsupported issues remain in feedback. Do not invent an ID."）。
// 两张表按 `writing.lang` 选一张，**不合并**——英文那 13 条里的
// `article_control`、`subject_verb_agreement` 对中文写作没有意义，
// 中文那张里的「句子太碎、像口号」对第二语言学习者也不是当务之急。
//
// 🚨 英文表里有一条是专门给中文母语者的：`l1_information_structure`——
// 词和内容都对，但整句是按中文语序拼的。它既不是语法错也不是搭配错，
// 我们在这之前没有任何地方说得出这件事。

// 反馈的四层。数字小的先说。
//
// 🚨 这四个数字会进 JSON、进数据库、进统计，**不要重新编号**。
const (
	writingLayerClaim     = 1 // 立意：这篇要说的那句话立住了没有
	writingLayerMaterial  = 2 // 材料：撑着它的东西在不在
	writingLayerStructure = 3 // 结构与段落：顺序、每段在做什么、怎么接
	writingLayerSentence  = 4 // 字句：最后才碰
)

// writingLayerNames 是四层在界面和 prompt 里的说法。
//
// 名词，不是句子（`ui-copy-style` 第 1 条）。
var writingLayerNames = map[int]string{
	writingLayerClaim:     "立意",
	writingLayerMaterial:  "材料",
	writingLayerStructure: "结构",
	writingLayerSentence:  "字句",
}

// writingSymptom 是一种毛病：它属于哪一层，学生读得懂的名字，
// 以及**一条她自己能验的信号**。
type writingSymptom struct {
	ID    string
	Layer int
	Name  string
	// Signal 是「怎么看出来的」，会原样写进 prompt。
	// 🚨 必须是可观察的现象，不能是评价。
	Signal string
}

// writingSymptomsZH —— 中文写作的诊断表，来自 qifeng `technique-router.md`。
//
// 原表有 17 条，这里去掉了「文字清楚，但没有余味」：它要的是余味，
// 而 qifeng 自己给这条划了边界（「知识说明、操作步骤和关键事实不能为了余味
// 牺牲清楚」），对中学生的议论文和说明文，追求余味是把她往文学腔上推，
// 和我们的文案规矩第 10 条正面冲突。
var writingSymptomsZH = []writingSymptom{
	// —— 第一层 · 立意 ——
	{"topic_without_question", writingLayerClaim, "只有主题，没有问题",
		"材料围绕一个大词堆积；读完不知道你想说的是哪一件事。"},
	{"claim_not_stated", writingLayerClaim, "观点未明确表达",
		"通篇在介绍情况，找不到一句话是你自己的判断。"},
	{"claim_too_safe", writingLayerClaim, "立场不明确",
		"列出了正反观点，但未说明自己的结论或作出判断的条件。"},

	// —— 第二层 · 材料 ——
	{"claim_without_evidence", writingLayerMaterial, "有判断，没有证据",
		"判断下得很重，后面跟的是另一个判断，不是事实、例子或数据。"},
	{"abstract_and_dry", writingLayerMaterial, "太抽象",
		"名词和结论密集，没有人、没有动作、没有具体的东西和场景。"},
	{"evidence_not_explained", writingLayerMaterial, "举了例子，没有解释",
		"例子摆在那里就过去了，没有说明例子如何支持文中的判断。"},
	{"story_without_meaning", writingLayerMaterial, "事件与主题的联系不明确",
		"读者知道发生了什么，不知道你为什么要讲这件事。"},
	{"flat_character", writingLayerMaterial, "人物扁平",
		"主要使用「勇敢」「努力」「善良」等形容词评价人物，未描述体现这些特点的具体行为。"},

	// —— 第三层 · 结构与段落 ——
	{"loose_whole", writingLayerStructure, "全文松散",
		"每一段都相关，但把中间两段对调，读起来一样通顺。"},
	{"claim_list_no_evidence", writingLayerStructure, "论证像观点清单",
		"每段都是一个判断，段落之间可以任意换序。"},
	{"slow_opening", writingLayerStructure, "开头进入太慢",
		"背景、定义、客套占了很长，真正要谈的那件事迟迟不出现。"},
	{"hook_not_carried", writingLayerStructure, "开头与后文脱节",
		"第一段很有意思，第二段换了个题目，两者没有关系。"},
	{"middle_collapse", writingLayerStructure, "主体展开不足",
		"开头和结尾都完整，中间变成资料堆或者同一个论点说了几遍。"},
	{"ending_only_summary", writingLayerStructure, "结尾只是重复",
		"结尾把前面说过的再说一遍，或者突然喊一句口号，或者抛出一个没有展开的大问题。"},
	{"paragraph_jump", writingLayerStructure, "段落之间跳跃",
		"连接词是有的，但读者仍然不知道前后两段是什么关系；「这」「那」指向不清。"},
	{"flat_chronicle", writingLayerStructure, "流水账",
		"事情按时间一件件发生，没有选择、没有阻力、没有代价、没有变化。"},

	// —— 第四层 · 字句 ——
	{"sentence_bloat", writingLayerSentence, "句子拖沓",
		"主干被「进行」「实现」「对于」「在……方面」盖住；修饰层次套了三层以上。"},
	{"sentence_choppy", writingLayerSentence, "句子太碎",
		"连着一串短句，一行一句，本来有关系的两件事被切断了。"},
	{"ornate_but_weak", writingLayerSentence, "修辞多于实质内容",
		"形容词、比喻、排比很多，事实、判断和结构很少。"},
}

// writingSymptomsEN —— 英文写作的诊断表。
//
// 来自 `ielts-writing-coach` 的 `references/exercise-contracts.md`：
// 四个维度十三个 id。这里保留它的 id 原文（英文写作的术语就该是英文），
// 名字和信号换成中文——**教学语言和写作语言是两回事**，
// 这一条是整份英文调研里最要紧的一个分辨：她可以用中文想清楚，再用英文写出来。
//
// 层的归属是我们定的：任务回应（TR）属于立意和材料，连贯（CC）属于结构，
// 语法（GRA）和词汇（LR）属于字句。这样「先看观点和材料，最后才看语言」
// 在英文这一侧同样成立。
var writingSymptomsEN = []writingSymptom{
	// —— TR 任务回应 ——
	{"task_instruction_coverage", writingLayerClaim, "没有回答题目问的那件事",
		"题目要求的动作（讨论／比较／给出立场）有一半没做，或者答的是另一个范围。"},
	{"weighing_qualification", writingLayerClaim, "没有权衡，也没有限定",
		"只把一边说完；没有交代在什么条件下成立、什么条件下不成立。"},
	{"mechanism_chain", writingLayerMaterial, "缺中间那一步",
		"从观点直接写到结果，中间「为什么会这样」那一环没有写出来。"},
	{"development_relevance", writingLayerMaterial, "解释和例子跑题",
		"例子本身没问题，但它支持的不是这一段要说的那句话。"},

	// —— CC 连贯 ——
	{"paragraph_function_order", writingLayerStructure, "句子的角色和顺序乱了",
		"一段里观点、证据、解释的先后颠倒，或者一段同时在做两件事。"},
	{"reference_linking", writingLayerStructure, "指代和连接对不上",
		"this / it / such 指向不明；连接词用的关系和句子真实的关系不一致。"},

	// —— LR 词汇 ——
	{"collocation_perspective", writingLayerSentence, "搭配不自然",
		"语法挑不出错，但英语里不这么搭；或者视角是中文的，英文习惯换一个主语。"},
	{"word_form_precision", writingLayerSentence, "词性或词义不准",
		"词选对了大意，但词性不对，或者比你要说的意思宽／窄。"},

	// —— GRA 语法 ——
	{"l1_information_structure", writingLayerSentence, "整句按中文语序拼的",
		"每个词都对，但信息的先后是中文的排法，英文读者要回头读第二遍。"},
	{"sentence_boundary", writingLayerSentence, "句子边界",
		"残句、逗号连句、或者两个完整句子之间只有一个逗号。"},
	{"complete_comparison", writingLayerSentence, "比较没有比完",
		"说了 more / better，但没有说比什么；或者比较的两样东西不是同类。"},
	{"verb_form_trigger", writingLayerSentence, "动词形式",
		"介词、不定式这些位置后面的动词形式不对。"},
	{"subject_verb_agreement", writingLayerSentence, "主谓一致",
		"主语和动词的单复数对不上。"},
	{"article_control", writingLayerSentence, "冠词和可数",
		"a / the / 零冠词的选择，或者把不可数名词当可数用。"},
}

// writingSymptomsNarrativeZH —— 记叙文特有的几条（R4，2026-09-21）。
//
// 上面那张表是按议论文写的：「有判断没有证据」「举了例子没有解释」在一篇
// 写风雨夜父亲来接我的文章里都不成立。这四条来自产品负责人拿来的两份
// 记叙文讲义（细节描写、抑扬转情法）。
//
// 🚨 **是补一张小表，不是改那张大表。** 中文那 16 条里有一多半
// （句子太碎、只有主题没有问题、字句层的每一条）记叙文照样用得上。
// 两张合起来给记叙文，议论文那一篇一条都不多拿。
var writingSymptomsNarrativeZH = []writingSymptom{
	// 讲义：「细节是作文的灵魂……多写动作、神态、语言，少写空洞的感受。」
	{"detail_vague", writingLayerMaterial, "只有感受，没有细节",
		"写着「很感动」「特别好」这一类直接说出来的感受，没有动作、神态、说过的话。"},
	// 讲义：「精准的动词 + 恰当的修饰词」。
	{"verb_generic", writingLayerSentence, "动词太笼统",
		"动作只用「走过去」「拿着」「看了看」等笼统词语，未交代与人物和情境相关的细节。"},
	// 讲义（抑扬转情法）：「制造波澜」。
	{"no_turn", writingLayerStructure, "从头到尾一个调子",
		"平铺直叙，读者的感受从第一句到最后一句没有变过。"},
	// 讲义：「刚写完讨厌这个人，下一段就突然写我发现他很好，特别生硬。」
	{"turn_abrupt", writingLayerStructure, "情感转得太突然",
		"前一段还在写不满，后一段直接写感动，中间没有过渡，也没有触发的那件事。"},
	// 讲义：结尾要「从这件事里领悟到了什么」，而不是一句放哪儿都成立的话。
	{"feeling_unearned", writingLayerClaim, "感悟缺少事件依据",
		"结尾提出了新的认识，但没有说明前文事件如何支持这一认识。"},
}

// writingSymptomsNarrativeEN —— 英文记叙文特有的几条（phase 4，2026-09-23）。
//
// 🚨 不是把 `writingSymptomsNarrativeZH` 那 5 条直译过来。中文母语者写
// 英文记叙文会撞上的是英语这门语言自己的机关——时态、对话标点、
// 「直接说感受」这几件事在英文里有专门的名字和专门的信号，跟中文记叙文
// 那五条（细节、动词、抑扬转情）不是同一批问题。
//
// 和 `writingSymptomsNarrativeZH` 同一个规矩：这张是**加在**
// `writingSymptomsEN` 上面的，议论文那 13 条一条都不少拿。
var writingSymptomsNarrativeEN = []writingSymptom{
	// show, don't tell —— 结果和情绪被形容词直接说出来，没有画面撑着。
	{"showing_vs_telling", writingLayerMaterial, "只说结论，没有画面",
		"情绪或结果被形容词直接说出来（very happy / so scared），没有动作、场景、感官细节让读者自己看到。"},
	// 记叙文默认过去时，中文没有时态变化，转换到英文时最容易滑掉。
	{"tense_drift", writingLayerSentence, "时态在叙述中途跳变",
		"整段在讲过去发生的事，动词忽然滑进现在时，或者一句话里两种时态混用。"},
	{"filtering_distance", writingLayerSentence, "隔着一层的叙述",
		"反复用 I saw / I felt / I noticed / I heard 这类词把读者放在你和事件中间，而不是直接写那件事本身。"},
	{"dialogue_mechanics", writingLayerSentence, "对话标点和提示语不对",
		"引号里该有的标点漏在引号外面，换人说话没有换行，或者提示语只会用 said 不带动作。"},
	{"connector_monotony", writingLayerStructure, "只靠 and then 串事情",
		"事件全靠 and then / after that 一件件接起来，重要时刻和次要时刻用的笔墨一样多。"},
}

// writingSymptomTable —— 这一次摆给模型看的是哪张毛病表。
//
// 2026-09-22：挑哪一张由 internal/guidance 那条共用规则决定。行为不变。
//
// 英文记叙（`writingSymptomsNarrativeEN`）和英文议论文一样，都补在
// 2026-09-23（phase 4）——英文以前只有一张不分文体的表，`Lang(8)+Genre(4)=28`
// 压过只定语言的那一行 `{en}`(24)，两种体裁从此各拿各的表。
func writingSymptomTable(lang string, genre string) []writingSymptom {
	narrativeZH := make([]writingSymptom, 0, len(writingSymptomsZH)+len(writingSymptomsNarrativeZH))
	narrativeZH = append(narrativeZH, writingSymptomsZH...)
	narrativeZH = append(narrativeZH, writingSymptomsNarrativeZH...)

	narrativeEN := make([]writingSymptom, 0, len(writingSymptomsEN)+len(writingSymptomsNarrativeEN))
	narrativeEN = append(narrativeEN, writingSymptomsEN...)
	narrativeEN = append(narrativeEN, writingSymptomsNarrativeEN...)

	rows := []guidance.Row[[]writingSymptom]{
		// 先登记的在平局时胜出，所以更具体的那几行要排在前面。
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite, Lang: "zh",
			Genres: []string{genreNarrative}}, Value: narrativeZH},
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite, Lang: "en",
			Genres: []string{genreNarrative}}, Value: narrativeEN},
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite, Lang: "zh"},
			Value: writingSymptomsZH},
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite, Lang: "en"},
			Value: writingSymptomsEN},
		// 语言认不出来（老数据、空串）时的记叙文：和 2026-09-22 之前一样给
		// 中文合表。没有 Lang，所以它比上面任何一行都宽（specificity 20），
		// 只有在 zh / en 两行都不匹配时才轮得到它。
		{Scope: guidance.Scope{Surface: guidance.SurfaceWrite,
			Genres: []string{genreNarrative}}, Value: narrativeZH},
	}
	k := guidance.Key{Surface: guidance.SurfaceWrite, Lang: lang, Genre: genre}
	if got, ok := guidance.Pick(k, rows); ok {
		return got
	}
	// 语言认不出来、体裁也不是记叙文时按中文通用表办；语言认不出来但是
	// 记叙文时上面那一行接住 —— 两种情况合起来和 2026-09-22 之前完全一致。
	return writingSymptomsZH
}

// lookupWritingSymptom 在那张表里找一条。第二个返回值是「这个 id 存在吗」。
//
// 🚨 模型给的 id 一律要过这里。编出来的 id 不是「宽容一下渲染出来」，
// 是**整条意见丢掉**——一条挂不上任何技法的诊断，学生拿它没有下一步可做。
func lookupWritingSymptom(lang, id string) (writingSymptom, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return writingSymptom{}, false
	}
	// 🚨 查的是**并集**，不按文体挡。
	//
	// 摆给模型看的那张表按文体过滤（writingSymptomCatalog），但**认**一条
	// id 的时候不过滤：判错的方向不对称 —— 文体是推断出来的，推断翻一下
	// （她往板上加了一张分论点卡），一条本来有效的意见就会整条被丢掉，
	// 而屏幕上只是「印记没说话」。多认一条从来不会伤到她。
	for _, s := range writingSymptomTable(lang, genreNarrative) {
		if s.ID == id {
			return s, true
		}
	}
	return writingSymptom{}, false
}

// resolveWritingSymptom 认一个 id **或**一条毛病的显示名，答回那一行。
//
// 🚨 为什么要认名字：批改那一路的 content 会**来回**一趟 —— 服务端发给老师
// 的批改卡是给人看的（名字），她按保存时客户端把整份内容原样送回来。
// 只认 id 的话，第二趟一定认不出第一趟写下的东西，于是「对应毛病」每保存
// 一次就被清空一次（2026-09-23 实测）。认两种写法、**一律答回 id**，
// 这条路就是幂等的：litegrade.SanitizeProvenance 存 id，
// gradingContentForView 渲染时才换成名字。
//
// 查的是并集、不按文体挡，理由同 lookupWritingSymptom。
func resolveWritingSymptom(lang, idOrName string) (writingSymptom, bool) {
	v := strings.TrimSpace(idOrName)
	if v == "" {
		return writingSymptom{}, false
	}
	table := writingSymptomTable(lang, genreNarrative)
	for _, s := range table {
		if s.ID == v {
			return s, true
		}
	}
	for _, s := range table {
		if s.Name == v {
			return s, true
		}
	}
	return writingSymptom{}, false
}

// writingSymptomCatalog 把那张表渲染进 prompt。
//
// 按层分组，层内按表里的顺序。渲染成
//
//	【第 1 层 · 立意】
//	- topic_without_question（只有主题，没有问题）：材料围绕一个大词堆积；……
//
// id 写在最前面是故意的：模型要回填的就是那个 id，把它放在每一行的开头，
// 比让它从一句中文里反推一个英文 id 可靠得多。
func writingSymptomCatalog(lang string, genre string) string {
	var b strings.Builder
	table := writingSymptomTable(lang, genre)
	for layer := writingLayerClaim; layer <= writingLayerSentence; layer++ {
		var rows []writingSymptom
		for _, s := range table {
			if s.Layer == layer {
				rows = append(rows, s)
			}
		}
		if len(rows) == 0 {
			continue
		}
		b.WriteString("【第 ")
		b.WriteByte(byte('0' + layer))
		b.WriteString(" 层 · " + writingLayerNames[layer] + "】\n")
		for _, s := range rows {
			b.WriteString("- " + s.ID + "（" + s.Name + "）：" + s.Signal + "\n")
		}
	}
	return b.String()
}
