import { describe, expect, it } from "vitest";
import { splitBroughtFile } from "./broughtFile";

describe("splitBroughtFile", () => {
  it("drops the title line from the body when the file's first line is the title", () => {
    const out = splitBroughtFile(
      { title: "Working in Groups", text: "Working in Groups\n\nNowadays, many teachers..." },
      "essay.txt",
    );
    expect(out).toEqual({ title: "Working in Groups", body: "Nowadays, many teachers..." });
  });

  it("keeps a body whose first line only looks like the title", () => {
    const out = splitBroughtFile({ title: "手机", text: "手机让生活更好。\n\n第二段。" }, "a.docx");
    expect(out.body).toBe("手机让生活更好。\n\n第二段。");
  });

  it("keeps a one-line body even when it equals the title", () => {
    expect(splitBroughtFile({ title: "Hi", text: "Hi" }, "a.txt").body).toBe("Hi");
  });

  it("falls back to a readable file name", () => {
    expect(splitBroughtFile({ title: "", text: "Body." }, "en-toefl_groups.txt").title).toBe("en toefl groups");
  });

  it("strips a markdown heading mark before comparing", () => {
    expect(splitBroughtFile({ title: "标题", text: "# 标题\n\n正文。" }, "a.md").body).toBe("正文。");
  });
});
