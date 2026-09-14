package api

import "strings"

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
	writingLayerClaim    = 1 // 立意：这篇要说的那句话立住了没有
	writingLayerMaterial = 2 // 材料：撑着它的东西在不在
	writingLayerStructure = 3 // 结构与段落：顺序、每段在做什么、怎么接
	writingLayerSentence = 4 // 字句：最后才碰
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
	{"claim_not_stated", writingLayerClaim, "主张没有说出来",
		"通篇在介绍情况，找不到一句话是你自己的判断。"},
	{"claim_too_safe", writingLayerClaim, "主张两边都站",
		"正反都说了一遍，最后没有落在任何一边；换成谁来写都成立。"},

	// —— 第二层 · 材料 ——
	{"claim_without_evidence", writingLayerMaterial, "有判断，没有证据",
		"判断下得很重，后面跟的是另一个判断，不是事实、例子或数据。"},
	{"abstract_and_dry", writingLayerMaterial, "太抽象",
		"名词和结论密集，没有人、没有动作、没有具体的东西和场景。"},
	{"evidence_not_explained", writingLayerMaterial, "举了例子，没有解释",
		"例子摆在那里就过去了，没有一句话说清它凭什么支持你的判断。"},
	{"story_without_meaning", writingLayerMaterial, "有事情，没有意义",
		"读者知道发生了什么，不知道你为什么要讲这件事。"},
	{"flat_character", writingLayerMaterial, "人物扁平",
		"靠「勇敢」「努力」「善良」这类形容词撑着；人物没有做过选择，也没有为难过。"},

	// —— 第三层 · 结构与段落 ——
	{"loose_whole", writingLayerStructure, "全文松散",
		"每一段都相关，但把中间两段对调，读起来一样通顺。"},
	{"claim_list_no_evidence", writingLayerStructure, "论证像观点清单",
		"每段都是一个判断，段落之间可以任意换序。"},
	{"slow_opening", writingLayerStructure, "开头进入太慢",
		"背景、定义、客套占了很长，真正要谈的那件事迟迟不出现。"},
	{"hook_not_carried", writingLayerStructure, "开头接不住",
		"第一段很有意思，第二段换了个题目，两者没有关系。"},
	{"middle_collapse", writingLayerStructure, "中部塌陷",
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
	{"ornate_but_weak", writingLayerSentence, "有文采，没有力量",
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
		"从主张直接跳到结果，中间「为什么会这样」那一环没有写出来。"},
	{"development_relevance", writingLayerMaterial, "解释和例子跑题",
		"例子本身没问题，但它支持的不是这一段要说的那句话。"},

	// —— CC 连贯 ——
	{"paragraph_function_order", writingLayerStructure, "句子的角色和顺序乱了",
		"一段里主张、证据、解释的先后颠倒，或者一段同时在做两件事。"},
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

// writingSymptomTable 按语言选表。
//
// `writing.lang` 只有 zh / en 两个值（`setWritingSetup` 拦住了别的），
// 所以 default 走中文而不是报错。
func writingSymptomTable(lang string) []writingSymptom {
	if lang == "en" {
		return writingSymptomsEN
	}
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
	for _, s := range writingSymptomTable(lang) {
		if s.ID == id {
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
func writingSymptomCatalog(lang string) string {
	var b strings.Builder
	table := writingSymptomTable(lang)
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
