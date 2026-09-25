import { describe, expect, it } from "vitest";
import { attachTemplates, templatesFor } from "./templateLibrary";
import type { WebBrief } from "./types";

const brief = (kind: string, style: string, requiredContent: string[]): WebBrief => ({
  kind,
  style,
  topic: kind,
  goal: "清楚展示内容",
  audience: ["访客"],
  requiredContent,
  avoid: [],
});

const directions = [
  { key: "editorial", name: "个人叙事", reason: "用大字介绍自己", headline: "你好，我是林澄", description: "设计与观察", cta: "了解我" },
  { key: "gallery", name: "作品画廊", reason: "让项目图片优先", headline: "精选作品", description: "三个近期项目", cta: "查看作品" },
  { key: "future", name: "项目档案", reason: "按年份整理经历", headline: "项目与经历", description: "持续更新", cta: "打开项目" },
];

describe("template library matching", () => {
  it("优先为开发者个人主页选择 GitHub 式项目档案", () => {
    const result = templatesFor(
      "为开源开发者制作个人主页，重点展示 GitHub、代码和技术项目",
      brief("个人主页", "深色技术风格", ["开源项目", "技能", "GitHub"]),
      "personal",
    );
    expect(result[0]?.id).toBe("personal-github");
  });

  it("为设计师作品集保留三种不同骨架并绑定模板信息", () => {
    const result = attachTemplates(
      directions,
      "设计师个人作品集，展示视觉案例、摄影和个人故事",
      brief("设计师作品集", "极简编辑感", ["精选作品", "案例过程", "关于我"]),
      "personal",
    );
    expect(new Set(result.map((item) => item.layout))).toEqual(new Set(["essay", "ledger", "magazine"]));
    expect(result.map((item) => item.layout)).toEqual(["essay", "magazine", "ledger"]);
    expect(result.every((item) => item.templateId?.startsWith("personal-"))).toBe(true);
    expect(result[2]?.templateId).toBe("personal-visual-index");
    expect(result.every((item) => item.palette?.paper && item.templateTag)).toBe(true);
  });

  it("技术教程博客优先使用支持分类发现的卡片模板", () => {
    const result = templatesFor(
      "制作 Hexo 技术博客，发布前端教程并按分类和标签查找文章",
      brief("技术博客", "清晰现代", ["教程", "分类", "标签", "最新文章"]),
      "knowledge",
    );
    expect(result[0]?.id).toBe("blog-cards");
  });
});
