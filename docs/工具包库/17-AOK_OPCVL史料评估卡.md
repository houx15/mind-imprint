---
id: opcvl
name_zh: OPCVL 史料评估卡
name_en: OPCVL Source Evaluation
category: AOK
priority: P0
disclosure_tier: tier-2
age_band: [DP]
trigger_keywords: [史料, 来源分析, OPCVL, 这张海报, 历史材料, 价值和局限]
trigger_context: 学生在历史任务中评估一份/两份史料（IB 史料分析、首发 Demo 场景）
interaction_type: 步骤引导卡
rubric_dims: [OPCVL D1, OPCVL D2, OPCVL D3, OPCVL D4, OPCVL D5, OPCVL D6, OPCVL D12]
related: [source-map, fact-opinion-value, ai-boundary]
---

# OPCVL 史料评估卡 · Origin / Purpose / Content / Value / Limitation

## 一句话定位
用 IB 历史的 OPCVL 框架评估史料：来源、目的、内容、价值、局限——并升到"双源对照"与"史观自觉"。这是产品的**首个落地 Demo 场景**。

## 工具包解释
OPCVL 是 IB 历史官方的史料评估法：**O**rigin 来源、**P**urpose 目的、**C**ontent 内容、**V**alue 价值、**L**imitation 局限。评估平台把它扩成 12 维：五维核心 + 多源层（对照/脉络/共情）+ 推理层（因果与变迁、历史意义）+ 输出层（综合论证、**D12 史观自觉**——觉察自己作为分析者的位置，是框架的灵魂，直接对应 TOK"认知者与知识"）。锚点：1915 征兵海报 ×2018 戏仿双源对照；哥伦布多源。Demo 的精髓是**让 AI 暴露自己的边界与幻觉**，成为学生练查证的最佳时刻（与"AI 边界与幻觉核查卡"配合）。

## 何时被触发
- **情境**：历史任务、史料分析、OPCVL/EE 历史方向。
- **可识别信号**：`trigger_keywords`；任务含史料图像/文本；IB 历史课。
- **谁可触发**：教师在历史任务中预置；系统在史料任务中浮现。

## 用来做什么
- 目标：完整评估史料并形成有据论证，最终升到史观自觉。
- 学生收获：会用 OPCVL、会双源对照、会觉察"我带着什么史观在看"。

## 如何交互
**界面元素**：史料展示区（单/双源）+ OPCVL 五步引导 + 多源对照表 + 史观自觉收口位。

**分步脚本**
1. 学生看史料 → 先自答 O/P/C/V/L，AI 一次只问一维、不替答。
2. AI 在适当处**暴露边界**（如"某报背景我不确定"）→ 提示学生查证（接 AI 边界卡）。
3. 双源时进入对照表：两份在内容/视角/价值上有何异同？
4. 收口 D12："你自己带着什么立场/史观在分析这两张图？"
5. 形成综合论证段（接 PEE）。

**就地小讲解**："价值和局限怎么分？"→ "价值=因为它的来源/目的，它特别能告诉我们什么；局限=同样因为来源/目的，它看不到/会偏向什么。"

## AI 克制红线
- 可以：逐维提问、暴露自身边界、引导对照与反思、检查论证结构。
- 绝不能：替学生写 OPCVL 分析、伪装成无所不知的权威、把幻觉当事实给出。

## 过程留痕
- 记录：各维自答质量、是否识别 AI 幻觉、双源对照深度、史观自觉水平、独立表达。
- 对应维度：OPCVL D1–D6、D12（及推理/输出层）。

## 渲染要点
```json
{ "type":"opcvl", "sources":[{"img":"","caption":""}],
  "steps":["origin","purpose","content","value","limitation"],
  "compare":{"enabled":true,"dims":["内容","视角","价值"]},
  "reflexivity_prompt":"你带着什么史观在看？", "expose_ai_boundary":true }
```

## 示例片段
> （展示 1915 征兵海报）卡片：先看 Origin——这是谁、什么时候、为什么做的？
> 学生：一战时英国的征兵宣传。
> AI：好。Purpose 呢？顺便说明一句：这张海报的确切发行机构我不能 100% 确定，建议你查证——这正好是你练查证的地方。
