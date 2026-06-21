import { Catalog, CatalogEntry } from "@mind-imprint/contracts";
import { ChatTool } from "../llm/types";

export const DEMO_TWINS = ["sift", "craap", "steelman"] as const;

export function buildCatalogText(catalog: Catalog): string {
  // Group entries by category
  const groups = new Map<string, CatalogEntry[]>();
  for (const entry of catalog) {
    const existing = groups.get(entry.category);
    if (existing) {
      existing.push(entry);
    } else {
      groups.set(entry.category, [entry]);
    }
  }

  const lines: string[] = [];
  for (const [category, entries] of groups) {
    lines.push(`【${category}】`);
    for (const e of entries) {
      const tier = e.disclosure_tier ?? "-";
      const prio = e.priority ?? "-";
      lines.push(`· ${e.id} — ${e.name} [${tier}·${prio}]：${e.trigger_condition}`);
    }
  }
  return lines.join("\n");
}

export function buildSystemPrompt(catalog: Catalog): string {
  const catalogText = buildCatalogText(catalog);
  const PROMPT_TEMPLATE = `# 角色
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
- **善用排版。** 用 Markdown 让重点一眼可见：\`**加粗**\`关键词，必要时配小标题 / 列表 / \`>\` 引用。突出重点，但整体仍简短。

# 工具卡（按需，不是每次）
工具卡只是你众多手段中的一种，**不是默认动作**。绝大多数轮次，普通陪练就够了。
- 只有当学生此刻的处境**正好命中**某张卡的适用情形时，才用 \`summon_card\` 提议——一次最多一张；拿不准、不够贴合，就**别提议**，继续正常陪练。
- 先按**分类**判断他现在卡在哪一类问题上，再在该类里挑最贴合的那一张。若多张卡都贴合，优先更**综合 / 更贴合当前任务**的那张。
- \`reason\` 写给系统看（为什么此刻贴合）；\`nudge_text\` 写给学生看（一句自然、邀请式、不命令的话）。
- 学生**婉拒 / 跳过**一张卡时，尊重他，继续陪练，**不要反复弹**同一张卡。
- 学生**提交**一张卡后，你会拿到他填写内容的结构化结果。基于他**自己写下的**东西继续——先接住他的思考，再就其中**一处**往前推一步。

# 可用的思维工具卡目录（按分类）
{{catalog}}`;

  return PROMPT_TEMPLATE.replace("{{catalog}}", catalogText);
}

export function demoCatalog(full: Catalog): Catalog {
  return full.filter((c) => !(DEMO_TWINS as readonly string[]).includes(c.id));
}

export function summonCardTool(catalog: Catalog): ChatTool {
  return {
    name: "summon_card",
    description: "当且仅当此刻命中某卡适用情形时提议一张卡，绝大多数轮次不需要调用。",
    parameters: {
      type: "object",
      properties: {
        card_id: {
          type: "string",
          enum: catalog.map((c) => c.id),
        },
        reason: {
          type: "string",
        },
        nudge_text: {
          type: "string",
        },
      },
      required: ["card_id", "reason", "nudge_text"],
    },
  };
}
