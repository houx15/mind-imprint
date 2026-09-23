package agent

// Prompt assembly for reading_eval.go. Pure: no database or model calls.
// Static teaching text lives in internal/prompts; context selection is separate
// where a feature has a dedicated *_context.go file.

import (
	"mindimprint/api/internal/cards"
)

// buildEvalPrompt — every string this returns is read BY THE STUDENT, on her own
// screen, so it is written TO her: 你, never 她.
//
// It used to ask the model for 「她这句读出了什么」 and the model obliged, so the
// panel reported on her in the third person — 「她读出了幼鸟靠本能…」 — as if
// briefing a teacher about her while she watched. Same defect the lite reports
// hit on 2026-08-29 and fixed there; this is the other surface it lived on.
// (`fallbackEval` below was already second person, so the degraded path was
// more correct than the live one.)
func buildEvalPrompt(spec cards.Spec, dimension string) string {
	return "你是一名阅读老师，正在评价学生用「" + spec.Name + "」选择的原文句子。分析维度是「" + dimension + "」。\n" +
		"所有自然语言字段会直接展示给学生，称呼学生时使用「你」。根据所选原句与分析目标说明判断，不评价学生的能力或态度。\n" +
		"逐项判断 target（选句是否对应分析对象）、evidence（是否包含可引用的依据）、centrality（该线索对当前分析维度是否关键）。" +
		"status 使用 pass / partial / miss，分别表示符合、部分符合、不符合。evidence 只引用学生所选句子中的原文；没有相应依据时留空。explanation 用一句话说明判断理由。\n" +
		"finding 说明这处选句体现的理解；judgment 说明这句原文可以得出的判断，不将模型推论写成学生已经表达的观点；support 说明依据；caveat 说明实际存在的局限，没有则留空；next_step 提出一个具体的后续阅读动作。" +
		"只输出 JSON，不加其他文字：\n" +
		`{"checks":[{"key":"target","status":"pass|partial|miss","evidence":"","explanation":""},{"key":"evidence","status":"pass|partial|miss","evidence":"","explanation":""},{"key":"centrality","status":"pass|partial|miss","evidence":"","explanation":""}],"finding":"","judgment":"","support":"","caveat":"","next_step":""}`
}
