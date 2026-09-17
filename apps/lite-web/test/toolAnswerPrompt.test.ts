import { describe, expect, it } from "vitest";
import { toolAnswerPrompt } from "../src/readings/BlockToolsPanel";

/**
 * 交给 印记 的那一行说明。🚨 服务端对卡片题目有 60 个字的上限，超过的**整行
 * 丢掉**（reading_coach.go 的 collapseCardPrompt）—— 那时 印记 看到的是她
 * 一段没头没尾的话，不知道她在答哪件工具。所以长度是这里要守的不变量。
 */
describe("toolAnswerPrompt", () => {
  it("仿写：去掉加粗的那个标签，留下写法本身", () => {
    const body = "**这一段的写法**：先给一个日常场景，再解释背后的原理\n\n换个话题，你也这样写一段：\n- 为什么冬天的铁栏杆摸起来比木头冷";
    expect(toolAnswerPrompt("仿写", 3, body)).toBe("仿写 · 第3段：先给一个日常场景，再解释背后的原理");
  });

  it("想一想：用第一个问题，去掉列表记号", () => {
    const body = "- 为什么作者先讲郊区，再讲城市？\n- 第二句的数据从哪里来？";
    expect(toolAnswerPrompt("想一想", 5, body)).toBe("想一想 · 第5段：为什么作者先讲郊区，再讲城市？");
  });

  it("再长也不超过 60 个字", () => {
    const body = "- " + "很".repeat(200) + "？";
    const got = toolAnswerPrompt("想一想", 12, body);
    expect(Array.from(got).length).toBeLessThanOrEqual(60);
    expect(got.startsWith("想一想 · 第12段：")).toBe(true);
    expect(got.endsWith("…")).toBe(true);
  });

  it("不知道段号就不写段号，不写「第0段」", () => {
    expect(toolAnswerPrompt("仿写", undefined, "**这一段的写法**：先抑后扬")).toBe("仿写：先抑后扬");
    expect(toolAnswerPrompt("仿写", 0, "**这一段的写法**：先抑后扬")).toBe("仿写：先抑后扬");
  });
});
