package api

import "mindimprint/api/internal/store/sqlc"

// reading_disclosure.go — 每一轮只展开这一步真正要读的那几段。
//
// 为什么：线上十天的账拆开之后，`reading_coach` 一个调用点占掉一次阅读成本的
// 82.8%、整个 lite 的 56%，平均每次带 **11,035 个输入 token**，而其中九成是
// 整篇文章 —— 每一轮重发一遍。缓存把它打到两折，但它体量太大，即便两折仍是
// 一轮里最大的一笔（实测 ¥0.0291 一轮里，命中的输入占 ¥0.0172）。
//
// 而**服务端本来就知道这一步只看哪几段**：`outline.Parts` 就是为此算出来的，
// prompt 里甚至已经写着「当前这一步只管它标明的那几段」。既然如此，没必要把
// 另外几部分的正文也铺开。
//
// 🚨 只对**明确按段落划定范围**的两种步骤收窄：通读（一步一个部分）和精读
// （一步一段）。别的步骤（提问、连线、收尾）本来就要通观全文，收窄它们等于
// 拿掉它们的依据。宁可省得少一点，也不要让某一步失去它需要的东西。
//
// 收起来的段落**仍然点名**（「这一段属于第三部分，这一步不看它」），
// 和超预算时写「（这一段没放进来，但它存在）」是同一套做法：她的文章没有被
// 悄悄改短，模型知道那里还有东西，只是这一轮不展开。

// disclosureScope 说的是这一轮哪些段落要给全文。nil = 全给（不收窄）。
type disclosureScope map[string]bool

// readingDisclosureMinRunes 是收窄的门槛。低于它整篇都给。
//
// 线上一次阅读陪练平均带 11,035 个输入 token，绝大部分是正文；真正值得收窄的
// 是那种文章。三千字以下省不到什么，而风险是实打实的（见上面那段）。
const readingDisclosureMinRunes = 3000

// readingDisclosureScope 算出这一轮该展开的段落集合。
//
// 返回 nil 表示「不收窄，照旧全给」——没有分部分、当前这一步不是按段落划定
// 范围的那两种、或者算出来的范围是空的，都走这条。
func readingDisclosureScope(
	blocks []Block,
	outline readingOutline,
	tasks []sqlc.ReadingTask,
	picks []readingPick,
	msgs []sqlc.AtomMessage,
) disclosureScope {
	current := currentReadingTask(tasks)
	if current == nil || len(outline.Parts) == 0 || current.BlockID == "" {
		return nil
	}
	// 🚨 短文章不收窄。收窄只在文章大的时候才省得到钱，而在短文章上它只剩风险：
	// 六段的文章里，解释性的那一段可能就是她这一步要用的依据。
	// 省不到的地方不要冒险。
	total := 0
	for _, b := range blocks {
		total += len([]rune(b.Text))
	}
	if total < readingDisclosureMinRunes {
		return nil
	}
	switch current.Kind {
	case string(taskRead), string(taskFocusBlock):
	default:
		// 这一步没有划定段落范围（提问、收尾之类），它要看全文。
		return nil
	}

	ord := make(map[string]int, len(blocks))
	for i, b := range blocks {
		ord[b.ID] = i + 1
	}
	keep := disclosureScope{}

	switch current.Kind {
	case string(taskFocusBlock):
		// 精读：这一段，外加前后各一段做上下文 —— 一段话很少能脱离它的邻居读懂。
		if n, ok := ord[current.BlockID]; ok {
			for i, b := range blocks {
				if abs(i+1-n) <= 1 {
					keep[b.ID] = true
				}
			}
		}
	case string(taskRead):
		// 通读：这一步就是一个部分，把那个部分整段给全。
		for _, p := range outline.Parts {
			from, okF := ord[p.From]
			to, okT := ord[p.To]
			if !okF || !okT {
				continue
			}
			if n, ok := ord[current.BlockID]; ok && n >= from && n <= to {
				for i, b := range blocks {
					if i+1 >= from && i+1 <= to {
						keep[b.ID] = true
					}
				}
			}
		}
	}

	// 她自己点出来的句子、和她屏幕上那张卡片指着的段落，无论属于哪一部分都要
	// 展开：印记正要跟她谈的就是这些，收起来它就只能干说。
	for _, p := range picks {
		if p.BlockID != "" {
			keep[p.BlockID] = true
		}
	}
	for _, id := range openCardBlockIDs(msgs) {
		keep[id] = true
	}

	// 🚨 承重段永远展开，不管它属于哪一部分。
	//
	// 走查里撞出来的：那篇文章的最后一段写着「装机容量衡量的是发电能力，而不是
	// 实际发电量」—— 而她这一步（精读第 3 段）要说的正是这个区别。按「这一段
	// 加左右各一段」收窄会把它收起来，于是印记 失去了她这一步真正要用的依据。
	// 承重段是排读法时就标好的（outline.Load 的 core），它们是这篇文章的骨架，
	// 省它们省不到多少，丢它们丢的是这一步的根据。
	for id, load := range outline.Load {
		if load == loadCore {
			keep[id] = true
		}
	}

	if len(keep) == 0 {
		return nil
	}
	return keep
}

// openCardBlockIDs 是她屏幕上那张卡片指着的段落。那张卡的每个选项都是文章里的
// 一句话 —— 印记 这一轮要跟她谈的就是它们，收起来它就只能干说。
func openCardBlockIDs(msgs []sqlc.AtomMessage) []string {
	tail := msgs
	if len(tail) > readingCoachTurnsWindow {
		tail = tail[len(tail)-readingCoachTurnsWindow:]
	}
	card := lastOpenCard(tail)
	if card == nil {
		return nil
	}
	var out []string
	for _, o := range card.Options {
		if o.BlockID != "" {
			out = append(out, o.BlockID)
		}
	}
	for _, w := range card.Words {
		if w.BlockID != "" {
			out = append(out, w.BlockID)
		}
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
