import type { SiteContent, SiteLayout } from "../../site/types";
import type { DesignDirection, PageBlock, PageDocument, Recommendation, SiteArchetype, WebBrief } from "./types";
import { attachTemplates } from "./templateLibrary";

export const INITIAL_REQUIREMENT =
  "为我的社区微气候调研项目制作网页，让同学和老师快速理解问题、研究过程与行动成果。页面需要清楚，但不能像普通作业展示。";

export const DECIDED_CONTEXT = [
  { label: "受众", value: "同学、老师、社区伙伴" },
  { label: "关键词", value: "可信、克制、过程可见" },
  { label: "结构", value: "问题 → 调研 → 发现 → 行动" },
  { label: "视觉", value: "纸张底色、深墨文字、朱砂强调" },
];

const DEMO_BRIEF: WebBrief = {
  kind: "项目成果页",
  style: "克制、清晰、过程可见",
  topic: "社区微气候调研",
  goal: "让访客理解发现并查看行动成果",
  audience: ["同学", "老师", "社区伙伴"],
  requiredContent: ["问题背景", "调研过程", "关键发现", "行动成果"],
  avoid: ["普通作业模板", "空泛口号"],
};

const DEMO_DIRECTIONS = [
  {
    key: "story",
    name: "研究叙事",
    reason: "从问题现场开始，按研究发生的顺序呈现证据和行动。",
    headline: "一条街的阴影，能改变夏天吗？",
    description: "我们记录温度、树荫和人的停留，寻找社区降温的具体线索。",
    cta: "查看调研过程",
  },
  {
    key: "evidence",
    name: "证据索引",
    reason: "让数据、观察和结论在一屏内形成可核对的研究档案。",
    headline: "社区微气候观察档案",
    description: "12 个测量点、4 个时段，以及我们从现场得到的三条发现。",
    cta: "打开证据",
  },
  {
    key: "action",
    name: "行动报道",
    reason: "用最重要的行动成果开场，再回到它背后的研究过程。",
    headline: "把最热的路口，变成可以停留的地方",
    description: "一次由学生完成的测量、判断与社区微更新提案。",
    cta: "查看行动方案",
  },
];

export const DEMO_RECOMMENDATION: Recommendation = {
  archetype: "project",
  sourceRequirement: INITIAL_REQUIREMENT,
  brief: DEMO_BRIEF,
  directions: attachTemplates(DEMO_DIRECTIONS, INITIAL_REQUIREMENT, DEMO_BRIEF, "project"),
  provider: "demo",
  model: "scenario-data",
};

export function pageFromDirection(direction: DesignDirection, recommendation: Recommendation): PageDocument {
  const siteName = siteNameFor(recommendation.archetype, recommendation.brief, recommendation.sourceRequirement);
  return {
    layout: direction.layout,
    templateId: direction.templateId,
    palette: direction.palette,
    title: direction.name,
    siteName,
    navItems: navFor(recommendation.archetype),
    blocks: blocksFor(direction, recommendation.archetype, recommendation.brief),
  };
}

function hero(direction: DesignDirection, archetype: SiteArchetype): PageBlock {
  return {
    id: "site.hero",
    name: archetype === "personal" ? "个人首屏" : "首屏",
    items: [
      { id: "site.hero.headline", type: "heading", text: direction.headline, span: 8 },
      { id: "site.hero.image", type: "image", text: "", span: 4, height: 220 },
      { id: "site.hero.lead", type: "text", text: direction.description, span: 8 },
      { id: "site.hero.cta", type: "button", text: direction.cta, span: 4 },
    ],
  };
}

function contentCards(brief: WebBrief, fallback: string[]): string[] {
  return brief.requiredContent.filter(Boolean).slice(0, 4).length > 0
    ? brief.requiredContent.filter(Boolean).slice(0, 4)
    : fallback;
}

function blocksFor(direction: DesignDirection, archetype: SiteArchetype, brief: WebBrief): PageBlock[] {
  const first = hero(direction, archetype);
  if (archetype === "personal") {
    const workTitle = direction.layout === "ledger" ? "经历与能力索引" : direction.layout === "magazine" ? "精选作品" : "我是谁，以及我在做什么";
    return [
      first,
      {
        id: "site.about",
        name: "关于我",
        items: [
          { id: "site.about.heading", type: "heading", text: "关于我", span: 5 },
          { id: "site.about.text", type: "text", text: brief.goal || "介绍你的关注方向、经历和正在做的事。", span: 7 },
        ],
      },
      {
        id: "site.work",
        name: "作品与经历",
        items: [
          { id: "site.work.heading", type: "heading", text: workTitle, span: 8 },
          { id: "site.work.cards", type: "cards", text: contentCards(brief, ["代表作品", "学习经历", "技能与兴趣"]), span: 12 },
        ],
      },
      {
        id: "site.contact",
        name: "联系",
        items: [
          { id: "site.contact.heading", type: "heading", text: "保持联系", span: 8 },
          { id: "site.contact.text", type: "text", text: "在这里留下适合公开展示的联系方式或合作说明。", span: 8 },
          { id: "site.contact.cta", type: "button", text: direction.cta || "联系我", span: 4 },
        ],
      },
    ];
  }

  const presets: Record<Exclude<SiteArchetype, "personal">, Array<{ id: string; name: string; heading: string; cards: string[] }>> = {
    project: [
      { id: "evidence", name: "研究证据", heading: "从现场到判断", cards: ["问题背景", "调研过程", "关键发现"] },
      { id: "action", name: "行动成果", heading: "让研究进入真实场景", cards: ["行动方案", "成果记录", "下一步"] },
    ],
    organization: [
      { id: "mission", name: "使命与成员", heading: "我们为什么聚在一起", cards: ["共同目标", "成员与角色", "工作方式"] },
      { id: "programs", name: "项目与加入", heading: "正在发生的事情", cards: ["近期项目", "活动记录", "加入我们"] },
    ],
    event: [
      { id: "details", name: "活动信息", heading: "时间、地点与参与方式", cards: ["活动日程", "地点交通", "参与须知"] },
      { id: "program", name: "内容与报名", heading: "你将在这里经历什么", cards: ["环节介绍", "嘉宾与作品", "立即报名"] },
    ],
    knowledge: [
      { id: "topics", name: "主题导航", heading: "从这里开始阅读", cards: ["核心主题", "最新文章", "推荐阅读"] },
      { id: "archive", name: "内容归档", heading: "持续整理的知识线索", cards: ["文章归档", "资源清单", "订阅更新"] },
    ],
  };
  return [first, ...presets[archetype].map((section, index) => ({
    id: `site.${section.id}`,
    name: section.name,
    items: [
      { id: `site.${section.id}.heading`, type: "heading" as const, text: section.heading, span: 8 },
      {
        id: `site.${section.id}.cards`,
        type: "cards" as const,
        text: index === 0 ? contentCards(brief, section.cards) : section.cards,
        span: 12,
      },
    ],
  }))];
}

export function siteContent(
  direction: DesignDirection,
  layout: SiteLayout = direction.layout,
  recommendation: Recommendation = DEMO_RECOMMENDATION,
): SiteContent {
  const archetype = recommendation.archetype;
  const siteName = siteNameFor(archetype, recommendation.brief, recommendation.sourceRequirement);
  const cards = contentCards(recommendation.brief, archetype === "personal" ? ["作品", "经历", "兴趣"] : ["背景", "过程", "成果"]);
  const palette = direction.palette;
  const plates: Array<[string, string]> = [
    [palette?.accent ?? "#c76c4c", palette?.ink ?? "#5e2f28"],
    [palette?.ink ?? "#23332d", palette?.paper ?? "#e7ded0"],
    [palette?.accent ?? "#1b44d8", palette?.paper ?? "#d9e1f7"],
    [palette?.ink ?? "#111111", palette?.accent ?? "#7fd1a6"],
  ];
  const blurbs = [direction.description, direction.reason, recommendation.brief.goal];
  const projects = archetype === "knowledge" || archetype === "event" ? [] : cards.map((title, index) => ({
    id: `p${index + 1}`,
    year: "",
    kind: recommendation.brief.kind,
    title,
    blurb: blurbs[index % blurbs.length] ?? "",
    plate: plates[index % plates.length]!,
  }));
  const posts = archetype === "knowledge" ? cards.map((title, index) => ({
    id: `post-${index + 1}`,
    date: "",
    title,
    blurb: blurbs[index % blurbs.length] ?? "",
    kind: index === 0 ? "推荐" : "文章",
    words: 0,
    tags: recommendation.brief.audience.slice(0, 2),
  })) : [];
  const eventProjects = archetype === "event" ? cards.map((title, index) => ({
    id: `event-${index + 1}`,
    year: "",
    kind: index === cards.length - 1 ? "参与" : "活动",
    title,
    blurb: blurbs[index % blurbs.length] ?? "",
    plate: plates[index % plates.length]!,
  })) : [];
  return {
    name: siteName,
    role: roleFor(archetype, recommendation.brief),
    headline: direction.headline,
    lead: direction.description,
    now: recommendation.brief.goal,
    motto: cards.slice(0, 4),
    stats: [],
    tags: cards.slice(0, 3),
    projects: [...projects, ...eventProjects],
    posts,
    reads: [],
    about: archetype === "personal" ? [direction.description, recommendation.brief.goal] : [direction.reason],
    nowList: cards.slice(0, 3),
    email: "",
    updated: "",
    seed: 27,
  };
}

function siteNameFor(archetype: SiteArchetype, brief: WebBrief, requirement: string): string {
  const named = requirement.match(/(?:我叫|姓名[：:]?|名字是)([\u4e00-\u9fa5·]{2,8})/)?.[1];
  if (named) return named;
  if (archetype === "personal") return "我的主页";
  return brief.topic || "我的网站";
}

function roleFor(archetype: SiteArchetype, brief: WebBrief): string {
  return {
    personal: "个人介绍 · 作品与经历",
    project: brief.kind || "项目成果",
    organization: brief.kind || "团队与组织",
    event: brief.kind || "活动页面",
    knowledge: brief.kind || "知识与文章",
  }[archetype];
}

function navFor(archetype: SiteArchetype): string[] {
  return {
    personal: ["关于", "作品", "联系"],
    project: ["问题", "过程", "成果"],
    organization: ["使命", "成员", "加入"],
    event: ["信息", "日程", "报名"],
    knowledge: ["主题", "文章", "归档"],
  }[archetype];
}
