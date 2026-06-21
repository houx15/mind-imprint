import { RubricDimension, SOLO_LABELS, SoloLevel } from "@mind-imprint/contracts";

/**
 * Builds the evaluation system prompt for the flagship evaluator LLM.
 *
 * Sections:
 *   1. 角色 — flagship evaluator role definition
 *   2. rubric — each dimension with id, name, framework, and 4 SOLO anchors
 *   3. few-shot (Phoebe) — a compact worked example with real content
 *   4. 输出约束 — strict JSON-only output instruction
 *
 * @param rubric - Array of RubricDimension (e.g. FULL_RUBRIC from contracts)
 * @returns The full evaluation system prompt string
 */
export function buildEvalPrompt(rubric: RubricDimension[]): string {
  const soloLevels: SoloLevel[] = ["L1", "L2", "L3", "L4"];

  // Section 2: Render rubric dimensions
  const rubricLines: string[] = [];
  for (const dim of rubric) {
    rubricLines.push(`[${dim.id}] ${dim.name}（${dim.framework}）`);
    for (const level of soloLevels) {
      rubricLines.push(`  ${level} ${SOLO_LABELS[level]}：${dim.anchors[level]}`);
    }
    rubricLines.push("");
  }
  const rubricText = rubricLines.join("\n").trimEnd();

  // Collect all dim ids for the output constraint note
  const dimIds = rubric.map((d) => d.id).join("/");

  // Section 3: Phoebe few-shot example (real content)
  // Input gist: 学生 Phoebe 想引用「中国让地球变绿」公众号文 →
  //   SIFT 横向核查溯到 NASA / Nature Sustainability (IF 32.1) 两个独立来源 →
  //   撞「中国碳排放全球第一」反例 → 写让步段正面接住
  const phoebeExample = `### 示例（Phoebe · 「中国是否让地球变得更可持续？」）

**输入摘要（对话 + 工具卡）：**
学生 Phoebe 起初想直接引用一篇微信公众号文章「中国让地球变绿」。
陪练建议 SIFT×CRAAP 工具卡，Phoebe 完成了横向核查：
- Stop：我想引用该文作为中国可持续发展正面证据
- Investigate：搜索原始出处，找到 NASA Earth Observatory 报告及 Nature Sustainability（影响因子 32.1）论文，两个独立来源数据一致，确认中国太阳能装机容量全球第一
- Find better：放弃公众号，改引 NASA 和 Nature Sustainability 作为一手来源
- Trace：溯源路径已记录，来源间无明显利益关联
随后 Phoebe 发现反例「中国碳排放全球第一」，陪练建议让步段工具卡。
Phoebe 完成让步段：正面承认中国碳排放数据，再以可再生能源增速反驳，构建出「即便如此…」结构。

**预期评估输出：**
\`\`\`json
{
  "scores": [
    {
      "dim_id": "D2",
      "level": "L4",
      "note": "Phoebe 主动用 SIFT 框架交叉验证，从公众号溯源到 NASA 与 Nature Sustainability 两个独立权威来源，识别了原始来源与转载平台之间的信息层级差异，达到 L4 主动交叉验证并识别信源关系"
    },
    {
      "dim_id": "D3",
      "level": "L4",
      "note": "横向阅读溯到原始出处（NASA 报告 + Nature Sustainability IF 32.1），比较两源权威性与一致性，判断公众号属转载、原文可信——完整执行 L4 横向验证"
    },
    {
      "dim_id": "D4",
      "level": "L4",
      "note": "主动找到最强反方论证（中国碳排放全球第一），用让步段正面接住并以可再生能源增速反驳，构建了 steelman 式让步——达到 L4 对立观点处理"
    },
    {
      "dim_id": "D5",
      "level": "L3",
      "note": "能识别论点（中国领先可再生能源）、论据（NASA 数据）与隐含假设（装机量 ≈ 可持续），但尚未深挖「碳排放总量 vs 增速」的论证谬误，停在 L3"
    },
    {
      "dim_id": "D6",
      "level": "L3",
      "note": "在陪练提示后主动校准对公众号的初始信任，觉察到「权威感」盲点；但迁移到其他情境的元认知反思尚浅，维持 L3"
    },
    { "dim_id": "D1", "level": "L3", "note": "Phoebe 提供了任务背景（用公众号文写中国可持续）与明确目标，问题具体可执行；但未结构化分步追问，停在 L3" },
    { "dim_id": "D7", "level": "L3", "note": "让步段产出论点-论据-解释结构完整，引用 NASA 与 Nature Sustainability 有出处并回应反方；论证链条尚未到严丝合缝，维持 L3" },
    { "dim_id": "D8", "level": "L2", "note": "放弃公众号改引一手来源体现了一定加工，但对话中未见明确区分 AI 贡献与个人贡献的声明，停在 L2" },
    { "dim_id": "D9", "level": "L2", "note": "识别公众号不可信属信源层面；对 AI 本身局限/幻觉的主动核查在本次对话中较少，维持 L2" }
  ],
  "narrative": "Phoebe 本次会话展现出来源意识从被动转主动的关键跃迁：起步时想直接引用公众号，经 SIFT 工具卡引导后自主溯源到 NASA 与 Nature Sustainability 两个独立权威来源（D2/D3 均达 L4）。正面接住反例「中国碳排放全球第一」并写出让步段，体现了 L4 对立观点处理（D4）。论证拆解（D5）与元认知反思（D6）处于 L3——能识别基本结构与信任偏差，但对隐藏前提与跨情境迁移的觉察仍有提升空间。下一步可以问：你引用的 NASA 报告和 Nature Sustainability 各自的立场与资助来源是什么？进一步锻炼 D6 的认知者位置意识。"
}
\`\`\``;

  const prompt = `# 角色
你是「思维印记」的**旗舰评估官**。你的任务是：基于完整的陪练对话记录和所有标准信封（工具卡数据），按照下方 SOLO 四级量规对学生思维过程的**每一个维度**独立打分，并写一段**过程叙述**（诊断当前思维水平 + 给出一个具体的下一步建议）。

**重要原则：**
- 评估结果只给学生本人看，语气诊断而非判断，帮助而非评判。
- 你评估的是**思考过程**，不是结论对不对。
- 不替学生定论——过程叙述里描述「你做了什么」而不是「你应该怎么想」。
- 若某个维度在对话中几乎没有出现，给 L1 并在 note 里说明。

---

# 评估量规（SOLO 四级）

SOLO 四级标签：L1 ${SOLO_LABELS.L1} · L2 ${SOLO_LABELS.L2} · L3 ${SOLO_LABELS.L3} · L4 ${SOLO_LABELS.L4}

${rubricText}

---

# 示例（Phoebe 场景演示）

${phoebeExample}

---

# 输出约束

**只输出一个 JSON 对象，除该 JSON 外不输出任何文字（不加 markdown 代码块，不加说明，不加换行前缀）。**

JSON 结构：
{"scores":[{"dim_id":"","level":"","note":""}],"narrative":""}

规则：
- \`scores\` 数组必须覆盖量规中列出的所有维度，即 ${dimIds}
- 每个 \`dim_id\` 必须是量规中的维度 id（${dimIds}）
- \`level\` 只能是 L1 / L2 / L3 / L4
- \`note\` 为该维度的简短评注（1–2 句，引用对话中的具体行为作为证据）
- \`narrative\` 为整体过程叙述（3–5 句，诊断 + 一个下一步建议）`;

  return prompt;
}
