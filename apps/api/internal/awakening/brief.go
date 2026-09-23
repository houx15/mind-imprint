package awakening

import (
	"fmt"
	"sort"
	"strings"

	"mindimprint/api/internal/interests"
)

// brief.go —— 她树上已经有什么。
//
// # 这个文件存在的理由
//
// 旧的兴趣测试是一条单行道：测试往树上写词，树从不回头影响测试。后果是**第二次
// 做和第一次做问的是同样的问题** —— 一个树上已经有「游戏」强度 4、「心理」
// 强度 3 的学生，第一问仍然是「最近有什么让你主动点开」。产品忘了她。
//
// TreeBrief 把树的现状算成一小段文字，进终端的 system 上文。它让第二趟从她
// 已经有的东西出发。
//
// # 服务端算，不让模型数
//
// 强度、哪几根枝是空的、上次隔了多久 —— 这些都是服务端一次查询就有的事实。
// 让模型每轮自己从一串词里数，是它每轮都可能数错的事
// （memory: hardcoded-thresholds-vs-user-set-scale-2026-09-12）。

// fieldZh 是七根主枝的中文名。
//
// 和 interests.json 的 field 取值一一对应。写在这里而不是查表，因为
// 「她哪几根枝是空的」要对**全部七根**发问，而词表里只有她有词的那几根。
var fieldZh = map[string]string{
	"formal":     "数学与形式",
	"science":    "科学与自然",
	"making":     "技术与创造",
	"society":    "社会与世界",
	"humanities": "人文与写作",
	"arts":       "艺术与表达",
	"self":       "自我与成长",
}

// allFields 是七根主枝，顺序固定 —— 一份每次顺序都不同的简报会让两次调用的
// prompt 不一样，而我们要的是可比较。
var allFields = []string{
	"formal", "science", "making", "society", "humanities", "arts", "self",
}

// Known 是她树上的一个词，只取简报要用的那几列。
type Known struct {
	InterestID string
	Zh         string
	Field      string
	Strength   int
}

// TreeBrief 是喂给终端的那份简报。
type TreeBrief struct {
	// Top 是强度最高的若干个词，降序。
	Top []Known
	// EmptyFields 是她一个词都没有的主枝（存 field id）。
	//
	// 它是这张图上**最有用的一条信息**：她还没走过的方向。终端在她的话
	// 碰到这些方向时可以顺着追一句，报告里也会把它们列成「还空着的枝」。
	EmptyFields []string
	// Total 是她树上一共有多少个词。
	Total int
	// AttemptNo 是这一趟是她的第几次（从 1 起）。
	AttemptNo int
	// DaysSinceLast 是距上一趟走完过了多少天。第一次为 0。
	DaysSinceLast int
}

// briefTopN 是简报里最多列几个词。
//
// 十个够用而且有上限：一棵长了三年的树可能有八十个词，整份塞进 prompt 既贵
// 又会把当前这一问淹掉。按强度降序取前十，她最认同的那几个一定在里面。
const briefTopN = 10

// BuildBrief 从她树上的全部词算出一份简报。
//
// known 可以是空的（一个刚注册的学生），那时 EmptyFields 是全部七根。
func BuildBrief(known []Known, attemptNo, daysSinceLast int) TreeBrief {
	b := TreeBrief{
		Total:         len(known),
		AttemptNo:     attemptNo,
		DaysSinceLast: daysSinceLast,
	}

	has := make(map[string]bool, len(allFields))
	for _, k := range known {
		has[k.Field] = true
	}
	for _, f := range allFields {
		if !has[f] {
			b.EmptyFields = append(b.EmptyFields, f)
		}
	}

	// 强度降序，同强度按中文名排 —— 一个确定的顺序，两次调用才可比较。
	sorted := append([]Known(nil), known...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Strength != sorted[j].Strength {
			return sorted[i].Strength > sorted[j].Strength
		}
		return sorted[i].Zh < sorted[j].Zh
	})
	if len(sorted) > briefTopN {
		sorted = sorted[:briefTopN]
	}
	b.Top = sorted
	return b
}

// IsFirstTime 报告这是不是她第一次走。
//
// 判据是**树上有没有词**，不是 attempt_no。一个做过一次、但一个词都没长出来
// 的学生，第二次仍然该从头问起 —— 让终端说「上次你说的是……」而后面接不上
// 任何东西，比重新问一遍糟。
func (b TreeBrief) IsFirstTime() bool { return b.Total == 0 }

// Text 把简报拼成进 prompt 的那一段。
//
// 第一次返回空串：没有树的时候，一段「她的树：（空）」只是在告诉模型一件它
// 不需要知道的事，还会诱导它替她解释为什么是空的。
func (b TreeBrief) Text() string {
	if b.IsFirstTime() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("学生的兴趣树上已经有这些词（括号里是强度，1 到 5，由出现过几次推出来）：\n")
	for _, k := range b.Top {
		zh := k.Zh
		if zh == "" {
			// 词表是真相源：库里那一列只是它的副本，对不上时以词表为准。
			if it, ok := interests.ByID(k.InterestID); ok {
				zh = it.Zh
			}
		}
		fmt.Fprintf(&sb, "- %s（%d）\n", zh, k.Strength)
	}
	if n := b.Total - len(b.Top); n > 0 {
		fmt.Fprintf(&sb, "（另外还有 %d 个强度更低的词。）\n", n)
	}

	if len(b.EmptyFields) > 0 {
		names := make([]string, 0, len(b.EmptyFields))
		for _, f := range b.EmptyFields {
			names = append(names, fieldZh[f])
		}
		fmt.Fprintf(&sb, "学生一个词都还没有的方向：%s。\n", strings.Join(names, "、"))
	}

	if b.DaysSinceLast > 0 {
		fmt.Fprintf(&sb, "距离学生上一次完成兴趣探询，过了 %d 天。\n", b.DaysSinceLast)
	}

	sb.WriteString(`怎么用这份简报：
- 第一问从学生已经有的词出发，了解学生目前是否仍关注这个方向，以及想法是否有变化。不要重新问「你最近喜欢什么」。
- 学生提到上面没有的方向时，围绕新方向提问，不要求学生继续讨论已有关键词。
- 将这些词作为理解学生兴趣的背景，不逐项复述或评价兴趣树。记录中没有某个领域，不代表学生对该领域没有兴趣。
`)
	return sb.String()
}

// OpeningAsk 是终端第一屏显示的那个问题。
//
// 第一次用 NODE 01 的默认问法；第二次起换成从她最强的那个词出发。这一句由
// **服务端**生成而不是模型生成：她刚进终端、还没说过一句话，这时的第一问
// 完全由我们已知的事实决定，没有理由为它花一次调用，也没有理由让它有可能
// 编出一个她没有的词。
func (b TreeBrief) OpeningAsk() string {
	if b.IsFirstTime() || len(b.Top) == 0 {
		return Nodes[0].Ask
	}
	top := b.Top[0]
	zh := top.Zh
	if zh == "" {
		if it, ok := interests.ByID(top.InterestID); ok {
			zh = it.Zh
		}
	}
	if zh == "" {
		return Nodes[0].Ask
	}
	return fmt.Sprintf(
		"上次的兴趣测试在你的树上记下的是「%s」。这段时间它还在吗？请说一件最近和它有关、你主动去做或去看的具体的事；如果你现在追的是别的，就直接说那一件。",
		zh,
	)
}
