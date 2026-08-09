import { describe, it, expect, vi } from "vitest";
import { appendFrameworkVerdict } from "../../src/workspace/WorkspaceContainer";
import type { StudioChatMsg } from "../../src/studio/ai/StudioChatContext";

// appendFrameworkVerdict turns a framework-readiness verdict into a 印记 bubble.
function collect(verdict: Parameters<typeof appendFrameworkVerdict>[0]): StudioChatMsg[] {
  let msgs: StudioChatMsg[] = [];
  const setMessages = vi.fn((updater: (c: StudioChatMsg[]) => StudioChatMsg[]) => {
    msgs = updater(msgs);
  });
  appendFrameworkVerdict(verdict, setMessages as never);
  return msgs;
}

describe("appendFrameworkVerdict", () => {
  it("renders why + suggestions as one 印记 markdown bubble", () => {
    const msgs = collect({ ready: true, why: "四点都扎实。", suggestions: ["资源那条更具体", "补一个反例"] });
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.role).toBe("ai");
    expect(msgs[0]!.text).toContain("我读了一遍你的研究框架");
    expect(msgs[0]!.text).toContain("四点都扎实。");
    expect(msgs[0]!.text).toContain("- 资源那条更具体");
    expect(msgs[0]!.text).toContain("- 补一个反例");
  });

  it("omits the suggestions block when there are none", () => {
    const msgs = collect({ ready: true, why: "很扎实，没什么要改的。", suggestions: [] });
    expect(msgs).toHaveLength(1);
    expect(msgs[0]!.text).not.toContain("可以再打磨");
  });

  it("is a no-op without a verdict", () => {
    expect(collect(null)).toHaveLength(0);
    expect(collect(undefined)).toHaveLength(0);
  });
});
