// Package agent is the server-side agent brain ported from the TS app:
// the 克制阶梯 system prompt, the summon_card tool, refeed serialization, the
// LLM message-history mapping, and the turn-loop engine. The system prompt and
// refeed logic are byte-for-byte parity with the TS originals (golden fixtures
// in testdata/ pin them).
package agent

import (
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// promptTemplate is the TS PROMPT_TEMPLATE, verbatim, with the {{catalog}}
// placeholder filled by BuildSystemPrompt. Do not paraphrase or reformat — a
// golden-fixture test pins this against the TS source.
const promptTemplate = `# 角色
你是「思维印记」里的思维陪练——更像一位**导师 / 教练**，服务国际课程（IB）方向的学生。学生带着自己真实的任务（论文、项目、课题、阅读）来。你的价值不是当一台答案机，而是在协作中把「思考」交回给他自己，让他离开时比来时更会想。

# 你怎么帮（克制，但不是只会反问）
- **不替他定论、不替他写、不替他判对错好坏。** 该他想的，别替他想完。
- 你有一整套教练手段，按情况挑用，而不是每次都反问：
  - 给一个**提示**，把他往前推一小步；
  - 问一个**引导性问题**，让他自己发现缺口；
  - **指出一个他没注意到的角度**或可能的反例；
  - **肯定**他已经做对的部分，让他知道哪条路走对了；
  - 必要时，**提议一张思维工具卡**（见下，按需，不是默认动作）。
- **聚焦一步。** 一次只推进一个焦点，简短、口语；别一口气抛一堆问题或长篇大论——保护他的思考节奏。
- **善用排版。** 用 Markdown 让重点一眼可见：` + "`**加粗**`" + `关键词，必要时配小标题 / 列表 / ` + "`>`" + ` 引用。突出重点，但整体仍简短。

# 关于链接和外部资料（重要）
你**打不开链接、也看不到网页或文件里的内容**——你只看得到学生在对话里贴出的文字。所以当学生只丢来一个链接（或提到某个网页/PDF）时：
- **别假装读过它**，别凭标题或网址猜测、编造里面的内容。
- 坦诚说明你看不到链接内容，请他把**关键段落 / 数据 / 原话**粘贴进来；或者用一两句话先讲讲他从中看到了什么。
- 这正好是个起点：可以借机和他一起**溯源、核实**这份材料（必要时再提议相应的工具卡）。

# 工具卡（贴合就递，别犹豫）
工具卡是你手里一种有力的手段。当此刻的处境**贴合**某张卡时，把它递给他就是好陪练——别因为「怕越界」就压着不给。
- 目录里每张卡都标了「何时用」(适用情形) 和「能帮他」(这张卡能给学生什么)。当学生此刻的处境贴合某卡这两栏时，就用 ` + "`summon_card`" + ` 提议它——这不违反克制；克制是指不替他定论，**不是把工具藏起来**。
- **一次最多一张**；同时贴合多张时，挑最综合 / 最贴合当前任务的那张。真的没有贴合的卡，就正常陪练，别硬塞。
- 先按**分类**判断他现在卡在哪一类问题上，再在该类里挑最贴合的那一张。
- ` + "`reason`" + ` 写给系统看（为什么此刻贴合）；` + "`nudge_text`" + ` 写给学生看（一句自然、邀请式、不命令的话）。**打开由学生确认**——你只是提议。
- 学生**婉拒 / 跳过**一张卡时，尊重他，继续陪练，**不要反复弹**同一张卡。
- 学生**提交**一张卡后，你会拿到他填写内容的结构化结果。基于他**自己写下的**东西继续——先接住他的思考，再就其中**一处**往前推一步。

# 可用的思维工具卡目录（按分类）
{{catalog}}`

// Material is the agent's view of a task material (see slice-2 material table).
type Material struct {
	ID     string
	Title  string
	Blocks []MaterialBlock
}

// MaterialBlock is one addressable paragraph of a material.
type MaterialBlock struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// BuildMaterialContext renders the task's materials (with block ids) as a
// context block the model can reference when anchoring questions. Empty when
// there are no materials.
func BuildMaterialContext(materials []Material) string {
	if len(materials) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("# 学生正在读/写的材料（可引用其中的 block_id 与原句）\n")
	for _, m := range materials {
		b.WriteString("## 材料：" + m.Title + "\n")
		for _, blk := range m.Blocks {
			b.WriteString("[" + blk.ID + "] " + blk.Text + "\n")
		}
	}
	return b.String()
}

// BuildCatalogText groups catalog entries by category (first-seen insertion
// order) and renders the Chinese lines exactly as the TS buildCatalogText does.
func BuildCatalogText(catalog []cards.Spec) string {
	var order []string
	byCat := map[string][]cards.Spec{}
	for _, e := range catalog {
		if _, seen := byCat[e.Category]; !seen {
			order = append(order, e.Category)
		}
		byCat[e.Category] = append(byCat[e.Category], e)
	}

	var lines []string
	for _, category := range order {
		lines = append(lines, "【"+category+"】")
		for _, e := range byCat[category] {
			kind := ""
			if e.InteractionType != "" {
				kind = "（" + e.InteractionType + "）"
			}
			lines = append(lines, "· "+e.ID+"｜"+e.Name+kind)
			lines = append(lines, "   何时用："+e.TriggerCondition)
			lines = append(lines, "   能帮他："+e.Purpose)
		}
	}
	return strings.Join(lines, "\n")
}

// BuildSystemPrompt fills the {{catalog}} placeholder in the 克制阶梯 template.
func BuildSystemPrompt(catalog []cards.Spec) string {
	return strings.Replace(promptTemplate, "{{catalog}}", BuildCatalogText(catalog), 1)
}

// SummonCardTool builds the summon_card ChatTool whose card_id enum is the
// catalog ids. Mirrors the TS summonCardTool exactly (description + required).
func SummonCardTool(catalog []cards.Spec) gateway.ChatTool {
	ids := make([]string, len(catalog))
	for i, c := range catalog {
		ids[i] = c.ID
	}
	return gateway.ChatTool{
		Name:        "summon_card",
		Description: "当此刻贴合某卡的适用情形时，提议这张卡。一次最多一张。",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"card_id": map[string]any{
					"type": "string",
					"enum": ids,
				},
				"reason": map[string]any{
					"type": "string",
				},
				"nudge_text": map[string]any{
					"type": "string",
				},
			},
			"required": []string{"card_id", "reason", "nudge_text"},
		},
	}
}
