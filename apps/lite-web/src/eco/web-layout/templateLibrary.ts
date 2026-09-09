import type { SiteLayout, SitePalette } from "../../site/types";
import type { DesignDirection, SiteArchetype, WebBrief } from "./types";

export interface TemplatePreset {
  id: string;
  name: string;
  tag: string;
  layout: SiteLayout;
  archetypes: SiteArchetype[];
  keywords: string[];
  palette: SitePalette;
}

const palette = (label: string, why: string, paper: string, ink: string, accent: string): SitePalette => ({
  label,
  why,
  paper,
  ink,
  accent,
});

/**
 * 这些不是第三方主题的复刻，而是从个人博客、GitHub 主页和设计师作品集中提炼的
 * 信息架构。每种网站类型都有三个真正不同的骨架，继续复用正式主页的三种渲染器。
 */
export const TEMPLATE_LIBRARY: TemplatePreset[] = [
  {
    id: "personal-editorial",
    name: "编辑式作品集",
    tag: "大字自述 · 留白 · 案例叙事",
    layout: "essay",
    archetypes: ["personal"],
    keywords: ["设计师", "摄影", "艺术", "作品集", "portfolio", "极简", "个人品牌", "故事"],
    palette: palette("暖纸朱砂", "让文字和作品说明更接近独立出版物。", "#F2EEE5", "#1E1C19", "#C14F32"),
  },
  {
    id: "personal-gallery",
    name: "精选项目画廊",
    tag: "项目先行 · 图像网格 · 清晰导航",
    layout: "magazine",
    archetypes: ["personal"],
    keywords: ["设计", "插画", "摄影", "视觉", "作品", "案例", "创作者", "工作室"],
    palette: palette("画廊蓝", "中性背景承托作品，以单一蓝色标记交互。", "#F7F7F4", "#171717", "#2457D6"),
  },
  {
    id: "personal-github",
    name: "开发者档案",
    tag: "GitHub 式索引 · 项目与技能 · 深色",
    layout: "ledger",
    archetypes: ["personal"],
    keywords: ["程序员", "开发者", "工程师", "github", "代码", "开源", "技术", "项目经历"],
    palette: palette("代码深色", "用等宽索引呈现项目、年份和技术关键词。", "#0D1117", "#E6EDF3", "#3FB950"),
  },
  {
    id: "personal-visual-index",
    name: "作品索引",
    tag: "年份与类别 · 高密度 · 视觉档案",
    layout: "ledger",
    archetypes: ["personal"],
    keywords: ["设计师", "摄影", "艺术", "作品集", "品牌", "视觉", "案例", "档案"],
    palette: palette("展签黑白", "像展览清单一样按年份与类别组织作品。", "#F2F1ED", "#181817", "#E14C31"),
  },
  {
    id: "personal-resume",
    name: "个人履历",
    tag: "经历时间线 · 技能 · 教育与奖项",
    layout: "ledger",
    archetypes: ["personal"],
    keywords: ["简历", "求职", "经历", "技能", "教育", "奖项", "实习", "履历"],
    palette: palette("简历深蓝", "用紧凑索引清楚呈现经历、技能和时间。", "#F5F7FA", "#18212F", "#2563EB"),
  },
  {
    id: "blog-reading",
    name: "安静阅读",
    tag: "Hexo 极简 · 单栏 · 长文优先",
    layout: "essay",
    archetypes: ["knowledge"],
    keywords: ["随笔", "阅读", "生活", "写作", "长文", "极简", "日记", "思考"],
    palette: palette("米白墨色", "降低界面噪声，让长文标题和摘要成为主角。", "#F6F3EC", "#25231F", "#A2462E"),
  },
  {
    id: "blog-cards",
    name: "主题卡片博客",
    tag: "Hexo 卡片 · 分类入口 · 内容发现",
    layout: "magazine",
    archetypes: ["knowledge"],
    keywords: ["博客", "教程", "文章", "分类", "标签", "专栏", "知识", "更新"],
    palette: palette("清爽靛蓝", "适合文章卡片、分类与作者侧栏的清晰层级。", "#F5F7FB", "#18202B", "#4F46E5"),
  },
  {
    id: "blog-archive",
    name: "技术日志索引",
    tag: "Hexo 归档 · 高密度 · 时间线",
    layout: "ledger",
    archetypes: ["knowledge"],
    keywords: ["技术", "开发", "日志", "归档", "笔记", "代码", "年份", "目录"],
    palette: palette("终端灰绿", "高密度时间索引方便快速定位文章。", "#111513", "#DDE7E1", "#63D297"),
  },
  {
    id: "project-case-study",
    name: "案例研究长页",
    tag: "问题开场 · 过程证据 · 结论",
    layout: "essay",
    archetypes: ["project"],
    keywords: ["研究", "pbl", "过程", "问题", "调研", "案例", "结论", "反思"],
    palette: palette("研究纸张", "以长页叙事保持问题、证据和结论的连续性。", "#F3EFE6", "#20201E", "#A7432D"),
  },
  {
    id: "project-evidence",
    name: "证据档案",
    tag: "数据索引 · 时间与来源 · 可核对",
    layout: "ledger",
    archetypes: ["project"],
    keywords: ["数据", "证据", "实验", "观察", "档案", "记录", "来源", "时间线"],
    palette: palette("实验室深绿", "让日期、来源和结论形成可核对的研究索引。", "#101513", "#E1E7E3", "#65C895"),
  },
  {
    id: "project-visual-report",
    name: "成果视觉报道",
    tag: "成果头图 · 重点卡片 · 行动入口",
    layout: "magazine",
    archetypes: ["project"],
    keywords: ["成果", "展示", "行动", "照片", "采访", "社区", "报告", "传播"],
    palette: palette("报道蓝红", "以高对比标题和图像卡片突出行动成果。", "#FAFAF8", "#151515", "#D8472F"),
  },
  {
    id: "organization-manifesto",
    name: "团队宣言",
    tag: "使命开场 · 成员故事 · 单页",
    layout: "essay",
    archetypes: ["organization"],
    keywords: ["使命", "理念", "团队故事", "公益", "社团", "倡议", "文化"],
    palette: palette("人文棕红", "用克制的叙事表达团队目标与共同价值。", "#F2EEE7", "#24201C", "#A84632"),
  },
  {
    id: "organization-directory",
    name: "成员与项目目录",
    tag: "成员索引 · 项目状态 · 高密度",
    layout: "ledger",
    archetypes: ["organization"],
    keywords: ["成员", "部门", "项目列表", "团队", "组织", "目录", "进度"],
    palette: palette("协作深蓝", "适合同时呈现成员、职责与多个进行中项目。", "#10141C", "#E6EBF2", "#66A3FF"),
  },
  {
    id: "organization-studio",
    name: "工作室主页",
    tag: "代表项目 · 服务卡片 · 加入入口",
    layout: "magazine",
    archetypes: ["organization"],
    keywords: ["工作室", "服务", "客户", "项目", "品牌", "成员", "加入", "合作"],
    palette: palette("工作室黑蓝", "中性画布让项目图像和合作入口更突出。", "#F7F7F5", "#171717", "#3159D8"),
  },
  {
    id: "event-poster",
    name: "活动宣言页",
    tag: "一句主题 · 强日期 · 单一行动",
    layout: "essay",
    archetypes: ["event"],
    keywords: ["发布会", "演讲", "展览", "主题", "海报", "报名", "倒计时"],
    palette: palette("展览橙黑", "大标题与单一强调色适合短周期活动传播。", "#F3EFE5", "#1B1916", "#E0522D"),
  },
  {
    id: "event-schedule",
    name: "日程清单",
    tag: "时间优先 · 场次索引 · 信息密集",
    layout: "ledger",
    archetypes: ["event"],
    keywords: ["日程", "会议", "比赛", "场次", "议程", "地点", "嘉宾", "时间"],
    palette: palette("夜场荧光", "等宽时间表让多场次活动更易扫描。", "#11120F", "#EEF0E8", "#C7F36B"),
  },
  {
    id: "event-campaign",
    name: "活动报名页",
    tag: "视觉头图 · 亮点卡片 · 报名入口",
    layout: "magazine",
    archetypes: ["event"],
    keywords: ["招募", "报名", "市集", "音乐", "活动", "参与", "嘉宾", "体验"],
    palette: palette("活力紫红", "用强视觉卡片组织亮点、嘉宾和报名信息。", "#FFF8F4", "#211A22", "#C83E73"),
  },
];

function sourceText(requirement: string, brief: WebBrief, direction?: Omit<DesignDirection, "layout" | "templateId" | "templateName" | "templateTag" | "palette">): string {
  return [requirement, brief.kind, brief.style, brief.topic, brief.goal, ...brief.audience, ...brief.requiredContent, direction?.name, direction?.reason]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

export function templatesFor(
  requirement: string,
  brief: WebBrief,
  archetype: SiteArchetype,
): TemplatePreset[] {
  const text = sourceText(requirement, brief);
  const candidates = TEMPLATE_LIBRARY.filter((item) => item.archetypes.includes(archetype));
  return [...candidates].sort((a, b) => {
    const score = (item: TemplatePreset) => item.keywords.reduce((sum, keyword) => sum + (text.includes(keyword) ? 3 : 0), 0);
    return score(b) - score(a);
  });
}

export function attachTemplates(
  directions: Array<Omit<DesignDirection, "layout" | "templateId" | "templateName" | "templateTag" | "palette">>,
  requirement: string,
  brief: WebBrief,
  archetype: SiteArchetype,
): DesignDirection[] {
  const ranked = templatesFor(requirement, brief, archetype);
  const layouts: SiteLayout[] = ["essay", "ledger", "magazine"];
  const permutations: SiteLayout[][] = [
    ["essay", "ledger", "magazine"],
    ["essay", "magazine", "ledger"],
    ["ledger", "essay", "magazine"],
    ["ledger", "magazine", "essay"],
    ["magazine", "essay", "ledger"],
    ["magazine", "ledger", "essay"],
  ];
  const layoutFit = (direction: (typeof directions)[number], layout: SiteLayout): number => {
    const name = `${direction.key} ${direction.name}`.toLowerCase();
    const detail = `${direction.reason} ${direction.headline} ${direction.description}`.toLowerCase();
    const patterns: Record<SiteLayout, RegExp> = {
      essay: /(叙事|长页|故事|宣言|大字|留白|单栏|阅读|editorial|essay)/i,
      ledger: /(索引|档案|目录|清单|时间|年份|履历|技能|证据|数据|archive|index|ledger)/i,
      magazine: /(画廊|图像|大图|卡片|杂志|项目先行|网格|视觉|海报|gallery|magazine|poster)/i,
    };
    return (patterns[layout].test(name) ? 12 : 0) + (patterns[layout].test(detail) ? 4 : 0);
  };
  const assignment = permutations.reduce((best, candidate) => {
    const total = candidate.reduce((sum, layout, index) => sum + layoutFit(directions[index]!, layout), 0);
    return total > best.total ? { layouts: candidate, total } : best;
  }, { layouts, total: -1 });

  return directions.map((direction, index) => {
    const layout = assignment.layouts[index] ?? layouts[index] ?? "essay";
    const directionText = sourceText(requirement, brief, direction);
    const template = ranked
      .filter((item) => item.layout === layout)
      .sort((a, b) => {
        const score = (item: TemplatePreset) => item.keywords.reduce((sum, keyword) => sum + (directionText.includes(keyword) ? 3 : 0), 0);
        return score(b) - score(a);
      })[0] ?? ranked[index] ?? TEMPLATE_LIBRARY[index]!;
    return {
      ...direction,
      layout,
      templateId: template.id,
      templateName: template.name,
      templateTag: template.tag,
      palette: template.palette,
    };
  });
}

export const TEMPLATE_GUIDANCE = `请让三个方向分别体现：叙事长页、索引档案、图像或卡片主页。个人主页要根据身份在设计师作品集、开发者项目档案和个人叙事之间取舍；博客要根据内容在安静阅读、分类卡片和时间归档之间取舍；项目页要在案例过程、证据档案和成果报道之间取舍。标题、说明和行动按钮必须与用户提供的内容相关。key 字段仍须严格使用 course、poster、gallery、editorial、future 五个允许值之一，不能使用 index、archive、portfolio 等新值。`;
