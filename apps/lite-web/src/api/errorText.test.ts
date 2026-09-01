import { describe, expect, it } from "vitest";
import { ApiError } from "./client";
import { apiErrorText } from "./errorText";

describe("apiErrorText", () => {
  // 服务端说的那句话必须原样到她眼前——那句话通常是她**能自己改**的东西。
  it("carries the server's own sentence through", () => {
    // ApiError(code, message, status) —— message 才是给人看的那一句。
    const err = new ApiError("no_reason", "写一句为什么——收下也好，退回也好", 400);
    expect(apiErrorText(err)).toContain("写一句为什么——收下也好，退回也好");
    // 🚨 给她看的是那句话，不是 no_reason 这种代号。
    expect(apiErrorText(err)).not.toContain("no_reason");
  });

  // 🚨 我们自己的代码崩了，也要照实说。以前这里会显示"再试一次"，而她再试
  // 一百次也没用——被盖掉的恰好是唯一能查出问题的那部分。
  it("does not disguise a bug in our own code as something worth retrying", () => {
    const text = apiErrorText(new TypeError("x.map is not a function"));
    expect(text).toContain("x.map is not a function");
    expect(text).not.toContain("再试一次");
  });

  // 网络断了抛出来的东西不一定是 Error。
  it("handles a thrown string", () => {
    expect(apiErrorText("Failed to fetch")).toContain("Failed to fetch");
  });

  // 什么都问不出来时，也不要假装知道原因。
  it("says it has nothing rather than inventing a cause", () => {
    expect(apiErrorText(undefined)).toBe("后台错误：没有更多信息");
    expect(apiErrorText(new Error("   "))).toBe("后台错误：没有更多信息");
  });

  it("always marks the text as coming from the backend", () => {
    for (const e of [new Error("boom"), "boom", 42, null]) {
      expect(apiErrorText(e).startsWith("后台错误：")).toBe(true);
    }
  });
});
