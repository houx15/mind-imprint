package api

// Prompt assembly for writing_turn.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"fmt"
	"strings"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/store/sqlc"
)

// buildWritingCoachProjection assembles the free-text state projection
// agent.ProposeProjectCoachReply's BuildProjectCoachContext renders verbatim
// under "项目当前状态（供你参考，别照搬复述）". Kept pure and separate from
// the handler so the "what does writing have instead of an article" assembly
// can be reasoned about (and tested) on its own, mirroring
// buildReadingRouteInput's split in reading_turn.go.
//
// target_words is reported as either a number or "还没定" (never omitted,
// never invented) — 2026-08-27 product ruling (W-R7): the coach must be able
// to SEE that length is still unsettled while she is in 构思 so she can raise
// it, but nothing here treats it as a precondition.
// draftBody 是成稿那一页上她此刻的正文。空串 = 她还没进成稿（或者那一页还没
// 落过字），这时以段落为准。
func buildWritingCoachProjection(wr sqlc.Writing, outline []sqlc.WritingOutline, snippets []sqlc.WritingSnippet, draftBody string) string {
	var b strings.Builder
	b.WriteString(writingRoomHowTo)
	b.WriteString(writingTopicLine(wr, "题目/想法："))
	// 🚨 The language rule reaches the MAIN coach chat here. It was missing
	// entirely, which is why 印记 kept discussing an English piece as though
	// every artifact it produced should be Chinese. See writing_lang.go.
	b.WriteString(writingLangLine(wr))
	// 🚨 「约 %d 字」 was hard-coded here too. On an English piece this told the
	// coach a 500-word essay was 500 Chinese characters — see writing_lang.go
	// for the advice that came out the other end.
	// 🚨 目标篇幅是稳定的（整篇写作都是那个数），留在前面；
	// **「她现在多少字、还差多少」每一轮都在变，所以它不能排在这儿** ——
	// 它一动，排在它后面的提纲和她那份正文就全部掉出缓存前缀，每轮全价。
	// 它现在由 gap 变量带到这个函数的最后（两条分支都要带上）。
	var gap string
	if wr.TargetWords != nil {
		b.WriteString(writingLengthLine(wr, "目标篇幅"))
		gap = writingLengthGapBlock(wr, draftBody)
	} else {
		b.WriteString("目标篇幅：还没定\n")
	}
	if len(outline) > 0 {
		b.WriteString("已确定的提纲：\n")
		for _, o := range outline {
			indent := strings.Repeat("  ", int(o.Depth))
			fmt.Fprintf(&b, "%s- %s\n", indent, truncateRunes(o.Text, 120))
		}
	}
	// 🚨 **成稿那一页开了之后，她的正文就是那一页，不再是段落那几块。**
	//
	// 这是 2026-09-12 第二十七轮走查里她自己诊断出来的 —— 而且她是对的：
	//
	//	「印记反复让我改一句我正文里根本没有的话（**它在读下面那段只读的旧文本**），
	//	  导致对话死循环」
	//	「印记给的修改意见滞后了，我正文框里的第二段已经是改过时态的版本了，
	//	  它还让我改 go、see、finish eat」
	//
	// 她在成稿里改的是 writing_draft.body；段落那几块 writing_snippet 停在她
	// 进成稿之前的样子（那一栏在屏幕上本来就标着「只读、不会跟着上面变」）。
	// 而这个上文一直只喂 snippets —— 陪练读的**确实**是那份旧的。
	//
	// 前面几轮我一直在治这个的症状（提示词说「以这里为准」、幻引判据、
	// 把截断上限一路调高），都没治到这儿。根因就是这个函数从来没拿过 draft。
	//
	// 一旦有成稿，就**只**给成稿：两份她的文字同时摆在上文里，正是让它挑错
	// 一份的原因。段落那几块此刻是历史，不是她的正文。
	if body := strings.TrimSpace(draftBody); body != "" {
		b.WriteString("正文（**这是学生此刻的正文，以这里为准**；" +
			"上面对话里你早先引过的句子学生可能已经改掉了，不要照着那些再提一遍）：\n")
		b.WriteString(writingProjectionSnippet(body) + "\n")
		// 🚨 这三条在成稿这一支上同样要有。差点漏掉：这一支是后加的、而且
		// 提前 return，而那条测试当时用的是空成稿，绿着也没发现。
		b.WriteString(writingCoachGroundingRules)
		b.WriteString(gap)
		return b.String()
	}

	nonEmpty := 0
	for _, s := range snippets {
		if strings.TrimSpace(s.Text) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 0 {
		// 🚨 **说清这几段是「现在这一版」。**
		//
		// 上文里同时摆着两样东西：她此刻的正文，和最近十二轮对话 —— 而那十二轮
		// 里有印记自己说过的话，引着她**当时**写的句子。模型会顺着自己上一轮
		// 接着说，于是一轮一轮重复一个她早就改掉的毛病。
		//
		// 2026-09-12 第二十三轮，她连着三步在说这件事：
		//
		//	「印记让我删的那句话在正文框里已经不存在了，它还在拿旧版本的问题指导我」
		//	「它一直说我正文里有 huge 和 50 kilogram 要我删，但我正文框里早就没有
		//	  这些词了。我不知道该听它的还是按我现在的正文来」
		//
		// 最后那半句是真正的代价：**她开始怀疑该信屏幕上的哪一个**。
		b.WriteString("已经写好的片段（**这是学生此刻的正文，以这里为准**；" +
			"上面对话里你早先引过的句子学生可能已经改掉了，不要照着那些再提一遍）：\n")
		// 🚨 **带上这一块的标题，别只给一个号。**
		//
		// 第三十六轮中文那一路：「印记说的『第3段最后那两句』跟我现在看到的
		// 第一段最后一句有点像，但不确定它到底在说哪一段，有点乱。」
		// 屏幕上每一块的抬头写的是**结构那一步的标题**，一个数字都没有；
		// 而这里只给了号。于是「第3段」在她那边没有任何落点，只能自己数 ——
		// 而空的块也占位置，数出来常常对不上。
		//
		// 两头一起改：屏幕上把号摆出来（SnippetsStage），这里把标题给它。
		// 有标题就能说「『各地都能开设很好的学校』那一块」，比数字准得多。
		//
		// 2026-09-18：按卡片的顺序列（开头 → 各分论点 → 结尾），标签也取卡片的 ——
		// 虚拟的开头/结尾卡在结构图上没有节点，只能从卡片那里拿到名字。
		for i, c := range writingCards(outline, snippets) {
			if c.Snippet == nil {
				continue
			}
			text := strings.TrimSpace(c.Snippet.Text)
			if text == "" {
				continue
			}
			label := ""
			if l := c.Label(); l != "" {
				label = "「" + l + "」"
			}
			fmt.Fprintf(&b, "  [第%d块]%s %s\n", i+1, label, writingProjectionSnippet(text))
		}
		b.WriteString(writingCoachGroundingRules)
	}
	b.WriteString(gap)
	return b.String()
}

// writingProjectionSnippet 渲染一段正文，**并且在真的截断时说出来**。
//
// 🚨 截断要说出来，不能只留一个「…」。默不作声地切一刀，下游那个读的人
// （这里是模型）会把「被切掉」当成「她没写」，然后去要一件她已经写过的东西。
// 我自己在走查那只眼睛上犯过一模一样的错：`f.value.slice(0, 120)` 不声不响
// 切一刀，学生于是连着五六步在「补全被截掉的那一段」，而她一个字都没丢。
// 同一个毛病，这次在产品里。
func writingProjectionSnippet(text string) string {
	r := []rune(text)
	if len(r) <= writingProjectionSnippetRunes {
		return text
	}
	return fmt.Sprintf("%s……（这一段一共 %d 字，上面只给了前 %d 字，后文未展示，不能据此判断学生未写相关内容）",
		string(r[:writingProjectionSnippetRunes]), len(r), writingProjectionSnippetRunes)
}

// buildWritingTurnHistory windows the raw transcript to writingTurnsWindow
// (never the whole thread — see the constant's comment) and converts it to
// agent.ChatTurn, then appends the CURRENT student turn — mirroring how
// coach.go's postCoachSubagentTurn builds ProposeProjectCoachReply's history
// (load prior, then append the turn under way, so the producer's context
// always ends with what she just said regardless of whether the persist
// below succeeds).
//
// role='system' rows (writing_stage.go's stage-transition trace, e.g. "stage:
// ideate → outline") are deliberately excluded from the conversation itself —
// they are a structural record, not something either side "said", and
// BuildProjectCoachContext has no third bucket for them; folding one in under
// "学生：" would misattribute it to her. They still count against the window
// (it slices the raw tail first, then filters), so a writing thread with many
// stage skips cannot inflate the prompt past the same bound reading enjoys.
func buildWritingTurnHistory(msgs []sqlc.AtomMessage, studentText string) []agent.ChatTurn {
	tail := msgs
	if len(tail) > writingTurnsWindow {
		tail = tail[len(tail)-writingTurnsWindow:]
	}
	history := make([]agent.ChatTurn, 0, len(tail)+1)
	for _, m := range tail {
		switch m.Role {
		case "student":
			history = append(history, agent.ChatTurn{Role: "user", Content: m.Content})
		case "ai":
			history = append(history, agent.ChatTurn{Role: "assistant", Content: m.Content})
		}
	}
	return append(history, agent.ChatTurn{Role: "user", Content: studentText})
}
