import { describe, expect, it } from "vitest";
import { prosePendingLabel } from "./ItemPage";

describe("prosePendingLabel", () => {
  it("shows nothing once the prose is not pending", () => {
    expect(prosePendingLabel(false, false)).toBeNull();
    expect(prosePendingLabel(false, true)).toBeNull();
  });

  it("says it's still generating before the one retry has run", () => {
    expect(prosePendingLabel(true, false)).toBe("报告文字生成中");
  });

  it("gives up honestly once the one retry has run and it's still pending", () => {
    expect(prosePendingLabel(true, true)).toBe("报告文字暂未生成");
  });
});
