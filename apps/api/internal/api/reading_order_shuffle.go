package api

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"
)

// reading_order_shuffle.go —— 排序板发出去之前，先把选项打乱。
//
// # 为什么存在（2026-09-23，产品负责人第 4 条）
//
// 她截的那张图里，「排出事件时间线」的四个选项是这样摆的：
//
//	1  第3段  The woman bought …
//	2  第6段  The woman decided to hang …
//	3  第11段 Some commenters … recognized …
//	4  第14段 It was sold at an auction …
//
// 段号一路递增 —— 板子一摆出来就已经是答案。原话：「默认选项就是正确答案。
// 需要优化成无答案或者打乱顺序。」
//
// 两条来路都会这样：
//   - buildOrderBoard（服务端兜底那一块）按 blocks 的下标取句子，取出来自然
//     是原文顺序；
//   - 模型自己给的那一块，它是照着文章从上往下读的，列出来同样是原文顺序。
//
// 提示词里那句「给板的那一轮不要说出正确的先后」只管住了**印记的嘴**，管不住
// 板子的摆法。所以这件事不归提示词管，归这里管
//（memory：hardcoded-thresholds-vs-user-set-scale —— 提示词里的软话跨不过
// 代码里的硬判据；这一次是反过来的同一件事）。
//
// # 判据是「不等于原来那个顺序」，不是「随机」
//
// 随机可以随出原顺序来，而那正是要挡的那一种。所以这里要的是一个**保证不同**
// 的排列：打乱之后如果还等于原来那一串，就再换一种摆法。
//
// # 为什么是确定性的
//
// 这块板会落进 atom_message 的 payload，之后每次打开都从库里读同一份 —— 所以
// 乱序只需要发生一次。用确定性的种子是为了**测试能钉住它**，也为了同一块板不会
// 因为重算而换一副样子。种子取自选项本身（段号 + 原句），不取时间、不取随机数：
// 同一块板永远打乱成同一个样子。
//
// # 只动 order_events
//
// 别的卡片没有「顺序就是答案」这回事：label_roles 的格子、word_bank 的词、
// choose_span 的备选，顺序都不承载对错。

// shuffleOrderOptions 返回一块选项被打乱过的排序板。
//
// 不是排序板、或者选项少于两条（一条排不出顺序）时原样返回。
func shuffleOrderOptions(c *coachCard) *coachCard {
	if c == nil || c.Type != coachCardOrderEvents || len(c.Options) < 2 {
		return c
	}
	out := *c
	out.Options = orderShufflePermutation(c.Options)
	return &out
}

// orderShufflePermutation 把选项重排成一个**保证不等于原序**的排列。
//
// 做法：用一个由选项内容算出来的种子跑 Fisher–Yates；万一排出来还是原序
// （n 很小的时候真的会），就把头两条对调 —— 那一下必然改变顺序，而且不会把
// 它变成「总是原序的某一种固定变形」，因为前面已经打乱过了。
func orderShufflePermutation(in []coachCardOption) []coachCardOption {
	out := append([]coachCardOption(nil), in...)
	rnd := orderShuffleSeed(in)
	for i := len(out) - 1; i > 0; i-- {
		rnd = rnd*6364136223846793005 + 1442695040888963407 // LCG，够用且没有依赖
		j := int(rnd >> 33 % uint64(i+1))
		out[i], out[j] = out[j], out[i]
	}
	if sameOrder(in, out) {
		out[0], out[1] = out[1], out[0]
	}
	return out
}

// orderShuffleSeed —— 种子只看选项的内容，所以同一块板永远打乱成同一个样子。
func orderShuffleSeed(in []coachCardOption) uint64 {
	var b strings.Builder
	for _, o := range in {
		b.WriteString(o.BlockID)
		b.WriteByte('\x00')
		b.WriteString(o.Quote)
		b.WriteByte('\x00')
	}
	sum := sha256.Sum256([]byte(b.String()))
	// 不让种子落在 0 上：LCG 从 0 起步照样能用，但一个固定的哨兵值更好读。
	if s := binary.BigEndian.Uint64(sum[:8]); s != 0 {
		return s
	}
	return 0x9e3779b97f4a7c15
}

func sameOrder(a, b []coachCardOption) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].BlockID != b[i].BlockID || a[i].Quote != b[i].Quote {
			return false
		}
	}
	return true
}
