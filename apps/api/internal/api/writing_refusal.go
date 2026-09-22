package api

// writing_refusal.go —— 她请印记替她做那件它不做的事时，说的那句话。
//
// 同事 2026-09-20：
//
//	「对于被禁止的交互，还是回应一句类似『印记不会替代你自己的思考和搜索
//	  行为』的内容，然后再转移下一个问题比较好，现在的有点太生硬了。」
//
// 截图里她说「我懒得搜，你帮我找吧」，印记 直接跳过去开始总结计划 ——
// 边界守住了，可她不知道自己被拒绝了，只看见话题变了。
//
// 🚨 **做成可验的，不是只在散文里要求。** 提示词里的「必须」是模型可以推翻的
// 「必须」（[[prompt-twice-then-make-it-checkable]]）。所以这条路是：
// 检测命中 → 注入那一段 → 校验回复里有没有那句说明 → 缺了重试一次。
//
// 🚨 判据盯的是**真失败本身**（[[detector-must-target-the-real-failure]]）：
// 失败是「一个字都没说就换了话题」，不是「措辞不合我意」。所以校验只问一件事
// —— 这一轮有没有点出这条边界。也**只重试一次**，然后照旧把模型说的话交出去：
// 为了一个检测项让她看见一个死掉的终端，是比生硬更大的毛病。

import "strings"

// writingDoItForMePhrases 是「请你替我做」的那些说法。命中一条就算。
//
// 🚨 **这张表要窄。** 「你帮我看看」「你帮我提点意见」「帮我查一下这份材料
// 靠不靠谱」都是印记该做的事，一个都不许命中 —— 判错的方向不对称：
// 把一次正常的求助判成越界，换来的是一句她没道理挨的说明，
// 而那正是同事说的「生硬」。
var writingDoItForMePhrases = []string{
	// 替我搜
	"你帮我找", "帮我找一下", "帮我找找", "帮我搜", "你帮我搜", "帮我搜一下",
	"帮我查一下资料", "帮我查查资料", "帮我找资料", "帮我找材料", "帮我找个例子",
	"懒得搜", "懒得找", "懒得查",
	// 替我写
	"你帮我写", "帮我写一", "帮我写个", "帮我写篇", "你来写", "你直接写", "你写好",
	"替我写", "代我写", "你写吧", "帮我写完", "懒得写",
	"write it for me", "write this for me", "write that for me",
	"do it for me", "find it for me", "search it for me", "can you write it",
}

// writingAsksUsToDoIt：她这一句是不是在请我们替她做搜索或撰写。
func writingAsksUsToDoIt(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, p := range writingDoItForMePhrases {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// writingRefusalMarkers 是那句说明的痕迹。命中任一条就算她被告知了。
//
// 只看「有没有说」，不判措辞：判措辞就会变成对着判据改提示词，
// 而 2026-09-14 那次实测里，对着判据改 prompt 把犯规数清零、教学质量反而降了
// （[[optimizing-a-detector-made-coaching-worse]]）。
var writingRefusalMarkers = []string{
	"不替你", "不会替你", "不能替你", "不替学生", "不替她",
	"替你写", "替你搜", "替你找", "替你查",
	"得你自己", "要你自己", "由你自己", "你自己来", "得自己",
	"i can't write", "i won't write", "can't search", "won't search", "for you",
}

// writingReplyOwnsTheRefusal：这一轮有没有点出那条边界。
func writingReplyOwnsTheRefusal(reply string) bool {
	t := strings.ToLower(reply)
	for _, m := range writingRefusalMarkers {
		if strings.Contains(t, m) {
			return true
		}
	}
	return false
}
