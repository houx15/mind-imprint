package agent

// Prompt assembly for reading_router.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"fmt"
	"strings"
)

func buildRouterPrompt(in ReadingRouteInput) string {
	var b strings.Builder
	b.WriteString("你是一名批判性阅读教练，正在和学生一起读一篇文章。基于学生此刻的表达和她正在看的原文，先给她一个简短的正常回答，再判断是否要请出一张\"思维卡\"，帮助她更深入地读这篇（不是替她下结论）。\n")
	b.WriteString("克制阶梯：多数时候只需正常回答(respond)；表达和某张卡有合理联系但意图还不明确时给轻提示(hint)；表达清楚、且能在原文里找到一处示范句时才正式请出(summon)。一次只请一张。\n")
	b.WriteString("当学生的问题明显对应某副卡、但你还不确定她此刻是否想用时，优先给 hint 并在 card_id 填上那张卡——这会变成一个「要不要用它看看」的邀请，由她决定，而不是干脆不提。\n")
	b.WriteString("reply 字段是你必须始终填写的——用不超过三句话、口语化、温暖的中文，紧扣文章内容回答学生，绝不替她下结论、只引导她自己往下想；一次只问一个问题。decision 是 respond 时 reply 就是这一轮的全部回答；decision 是 summon/hint 时 reply 是请卡前的简短过渡语。\n")
	b.WriteString("如果给了「阅读目的」，你的每一次追问都要服务这个目的（是在找反驳、印证，还是背景）。\n")
	b.WriteString("可用的卡（只能从这些里选）：\n")
	for _, c := range in.Catalog {
		b.WriteString("- " + c.CardID + "（" + c.Name + "）：" + c.Trigger + "\n")
	}
	b.WriteString("\n只输出 JSON：{\"decision\":\"respond|hint|summon\",\"card_id\":\"...\",\"reason\":\"给学生看的一句话，说明为什么此刻值得看这张卡\",\"reply\":\"始终填写：给学生的简短正常回答，扣紧文章\",\"example_block_id\":\"summon时给出示范句所在的block id\",\"example_quote\":\"summon时给出该block里的一句原文（必须逐字来自原文）\",\"example_why\":\"用不超过两句话解释这句为什么适合这张卡\",\"followup_plan\":[\"最多两张后续卡的id\"]}。respond/hint 时 card_id 可留空、example 字段留空；reply 永远不留空。不要输出任何多余文字。")
	return b.String()
}

// buildReadingRouteUserPrompt assembles the router's user message. Pure, no
// I/O — this is what makes the brief-in behavior (S2 §6) testable without a
// model call. Preserves the pre-brief prompt content exactly; the brief block
// is only added when at least one field is non-empty, so a zero ReadingBrief
// degrades to today's prompt byte-for-byte.
func buildReadingRouteUserPrompt(in ReadingRouteInput) string {
	var b strings.Builder
	if br := in.Brief; strings.TrimSpace(br.Reason+br.Focus+br.PhaseTag+br.ProposalSnap) != "" {
		fmt.Fprintf(&b, "【阅读目的】\n")
		if br.Reason != "" {
			fmt.Fprintf(&b, "- 为什么读这篇：%s\n", br.Reason)
		}
		if br.PhaseTag != "" {
			fmt.Fprintf(&b, "- 服务于：%s\n", br.PhaseTag)
		}
		if br.ProposalSnap != "" {
			fmt.Fprintf(&b, "- 当前论点：%s\n", br.ProposalSnap)
		}
		if br.Focus != "" {
			fmt.Fprintf(&b, "- 学生关注：%s\n", br.Focus)
		}
		b.WriteString("让你的追问围绕这个目的，而不是泛泛而读。\n\n")
	}
	if in.Article != "" {
		b.WriteString("文章：\n" + in.Article + "\n")
	}
	if in.StudentText != "" {
		b.WriteString("学生说：" + in.StudentText + "\n")
	}
	if len(in.FocusedSpans) > 0 {
		b.WriteString("她正在看的原文：\n")
		for _, s := range in.FocusedSpans {
			b.WriteString("[" + s.BlockID + "] " + s.Quote + "\n")
		}
	}
	if len(in.RecentTurns) > 0 {
		b.WriteString("最近对话：\n" + strings.Join(in.RecentTurns, "\n") + "\n")
	}
	return b.String()
}
