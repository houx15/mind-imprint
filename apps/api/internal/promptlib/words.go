package promptlib

import (
	"regexp"
	"strconv"
)

// 从「不少于800字」「at least 250 words」「120-150词」里取出目标字数。
//
// # 为什么要取
//
// 产品负责人 2026-09-21：从题库开出来的那一篇，「开始之前」那个弹窗里的
// 语言和目标字数**应该是定好的**，不该再问她一遍 —— 题目自己就写着
// 「不少于800字」，让她再填一次，是把已经知道的事又问一遍。
//
// 语言本来就跟着题走（`langOf`）。字数在源数据里是一句人话，所以要解出来。
//
// # 🚨 取不到就返回 0，**不要猜一个**
//
// 目标字数会写进这一篇、陪练会照着它算「还差多少字」。猜错了，她会被一个
// 题目里根本不存在的要求追着跑 —— 而 铁律② 说得很清楚：
// 篇幅从来不是一道门槛。宁可空着。
//
// 区间取**上限**：「120-150词」要求的是能写到 150 那一档。

var wordsRe = regexp.MustCompile(`\d{1,6}`)

// 太小的数字不是字数要求：「二选一」里的 2、「共3题」里的 3、
// 「满分60分」里的 60（分值不是字数）。
//
// 20：比这更少的字数要求在这批题里一条都没有，而 10 以下的数字在题面里
// 到处都是（题号、小问编号）。
const (
	wordsMin = 20
	wordsMax = 100000
)

// TargetWords 解出目标字数；解不出来返回 0。
func TargetWords(wordLimit string) int {
	best := 0
	for _, m := range wordsRe.FindAllString(wordLimit, -1) {
		n, err := strconv.Atoi(m)
		if err != nil || n < wordsMin || n > wordsMax {
			continue
		}
		if n > best {
			best = n
		}
	}
	return best
}
