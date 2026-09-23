// Package agent is the server-side agent brain ported from the TS app:
// the 克制阶梯 system prompt, the summon_card tool, refeed serialization, the
// LLM message-history mapping, and the turn-loop engine. Golden fixtures
// in testdata/ track intentional changes to the assembled prompt.
package agent

import (
	"strconv"
	"strings"

	"mindimprint/api/internal/cards"
	"mindimprint/api/internal/gateway"
)

// promptTemplate is the legacy coach instruction. BuildSystemPrompt replaces
// {{catalog}} with the registry-derived card directory.
const promptTemplate = `# 角色
你是“思维印记”的学习陪练，帮助国际课程学生理解材料、检验思路并完成自己的学习任务。回复直接展示给学生，用“你”称呼学生，清楚说明当前讨论的内容。

# 提供帮助
- 学生自己作出判断、撰写要提交的正文。你可以解释概念、指出已有论述中的依据或缺口、提供提示和引导问题，不代写正文或替学生作出最终决定。
- 根据学生当前需要选择帮助方式：解释一个概念、提出一个具体问题、提示相关条件或反例，或说明已经完成的某项思考。肯定与建议都应有具体依据。
- 一轮聚焦一个问题，使用自然、完整的句子；需要多项说明时用简短列表。无需在每次解释后追加问题。
- 可用 Markdown 标出重点，避免过多标题和强调。

# 材料范围
你只能依据对话和系统提供的材料，不能自行打开链接或文件。学生只给出链接、标题或文件名时，不假装读过正文，也不推测内容；请学生粘贴与问题相关的段落、数据或原话。讨论来源时区分已提供的信息和仍需核实的内容。

# 工具卡
- 根据目录中的适用情形和用途选择工具卡；当前任务与某张卡相符时，调用 summon_card 提议使用。
- 一次最多提议一张；多张均适用时选择最相关的一张，没有适用卡片时继续对话。
- reason 说明本次选择的依据，供系统使用；nudge_text 直接给学生看，用一句话说明卡片能帮助处理什么问题，并邀请学生使用。
- 工具卡由学生确认打开。学生跳过或拒绝后继续提供帮助，不重复弹出同一张卡。
- 学生提交工具卡后，依据实际填写内容回应，并围绕其中一个需要继续思考的地方提供帮助。

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

// materialAlias returns the prompt-local alias for the i-th material in a
// materials slice: "m0", "m1", … — the index, NOT m.ID. Block ids are only
// "stable within a material" (materialize.Segment), so a project with ≥2
// materials needs SOME qualifier to disambiguate colliding bare [b0] labels.
// The qualifier can't be m.ID itself: on the course path m.ID is the literal
// "m0", but on the studio path (api/studioturn.go's projectMaterials) it is
// the material's full 36-char UUID — and the L1 instruction's own example
// ("m0:b0", buildAnchorPrompt) is only unmistypeable by a beginner student
// AND by the model when the label is this short. blockLookup (anchors.go)
// derives the identical alias from the SAME materials slice in the SAME
// order, so the pair stays in lockstep without a shared table or a migration.
func materialAlias(i int) string {
	return "m" + strconv.Itoa(i)
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
	for i, m := range materials {
		alias := materialAlias(i)
		b.WriteString("## 材料：" + m.Title + "\n")
		for _, blk := range m.Blocks {
			b.WriteString("[" + alias + ":" + blk.ID + "] " + blk.Text + "\n")
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
			lines = append(lines, "   用途："+e.Purpose)
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
