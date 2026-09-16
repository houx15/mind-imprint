import { describe, expect, it } from "vitest";
import { errorMarkdown, normalizeSayMarkdown } from "./Says";

it("separates legacy error excerpts without rewriting multiline code or ordinary punctuation", () => {
  expect(errorMarkdown("保存失败：未保存：## 修改意见\n正文")).toBe("保存失败：未保存：\n\n## 修改意见\n正文");
  for (const text of ["错误：HTTP 500", "错误：\n\n## 正文", "错误：\n```\n内容：## 原样\n```", "错误：#标签"]) {
    expect(errorMarkdown(text)).toBe(text);
  }
});

describe("Chinese list compatibility", () => {
  it("renders leading Chinese colon labels without changing code or literal inline markers", () => {
    expect(normalizeSayMarkdown("**第一项：**到达现场\n- **记录：**看不清"))
      .toBe("**第一项**：到达现场\n- **记录**：看不清");
    expect(normalizeSayMarkdown("**第一，数据对不上你说的结论。**12个盘子\n**为什么？**请检查"))
      .toBe("**第一，数据对不上你说的结论**。12个盘子\n**为什么**？请检查");
    for(const text of ["`**第一项：**到达现场`", "    **第一项：**到达现场", "```text\n**第一项：**到达现场\n```", "\\**第一项：**到达现场", "正文 **第一项：**到达现场", "**第一项：** 到达现场"]) {
      expect(normalizeSayMarkdown(text)).toBe(text);
    }
  });
  it("normalizes existing markers without changing decimal numbers or inline formatting", () => {
    expect(normalizeSayMarkdown("· **要点**\n2、下一步\n3.5 倍\n- 常规列表"))
      .toBe("- **要点**\n2. 下一步\n3.5 倍\n- 常规列表");
  });
  it("preserves fenced and indented code, including shorter fence content", () => {
    const code = "````text\n· 原样\n```\n2、原样\n````\n    · 缩进代码\n• 正文";
    expect(normalizeSayMarkdown(code)).toBe(code.replace("• 正文", "- 正文"));
  });
  it("does not end a tilde fence on a backtick fence", () => {
    expect(normalizeSayMarkdown("~~~\n```\n· 原样\n~~~\n1、正文"))
      .toBe("~~~\n```\n· 原样\n~~~\n1. 正文");
  });
});
