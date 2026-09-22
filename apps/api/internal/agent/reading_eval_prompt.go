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
	return "你是一名批判性阅读教练，正在跟一个中学生说话。她用「" + spec.Name + "」这张卡，从文章里选了一句她认为相关的证据。" +
		"针对维度「" + dimension + "」，评估她的选句，只输出 JSON。\n" +
		"**JSON 里所有给她看的文字都要用「你」称呼她，绝对不要用「她」**——这些话会原样出现在她自己的屏幕上，" +
		"用第三人称等于当着她的面向别人汇报她。\n" +
		"{\"checks\":[{\"key\":\"target\",\"status\":\"pass|partial|miss\",\"evidence\":\"必须逐字来自她选的句子\",\"explanation\":\"一句话，对她说\"}," +
		"{\"key\":\"evidence\",\"status\":\"...\",\"evidence\":\"...\",\"explanation\":\"...\"}," +
		"{\"key\":\"centrality\",\"status\":\"...\",\"evidence\":\"...\",\"explanation\":\"...\"}]," +
		"\"finding\":\"你这句读出了什么\",\"judgment\":\"你的论断\",\"support\":\"支撑\",\"caveat\":\"保留\",\"next_step\":\"下一步只做一件事\"}\n" +
		"target=她是否找对了对象；evidence=句子里有没有可直接引用的线索；centrality=线索是否足够关键。不要输出多余文字。"
}
