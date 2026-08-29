import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ReadingCoachSlot } from "@lite/readings/ReadingRoom";
import type { LiteMessage } from "../src/api/readingRoom";

/**
 * Task 11 — 印记 is allowed to speak with structure.
 *
 * The complaint that started this was that her replies read as "very
 * text-heavy and students won't have patience to read it thoroughly."
 * Markdown was ALREADY rendering (`@/studio/ai/ChatMarkdown`); the flat wall
 * of prose was the model not reaching for it. So the front-end half of this
 * task is small and specific: lite gets its own renderer whose `**bold**`
 * lands in the accent colour, so the one word she is meant to catch actually
 * catches her eye instead of dissolving back into ink.
 *
 * 🚨 The shared component is NOT edited — it styles four pro rooms, and its
 * bold is deliberately ink there. This is lite's variant, and the ONLY
 * difference is `strong`.
 *
 * The third test is the guard rail, and it is the one that would hurt to
 * lose: only 印记's turns are markdown. A student typing a literal `*` or `#`
 * must come back on screen exactly as she typed it, never silently eaten as
 * markup — that rule is written into the shared renderer's own header and it
 * survives here.
 */

const { LiteChatMarkdown } = await import("../src/readings/LiteChatMarkdown");

vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  postReadingCoachTurn: vi.fn(),
}));

const { ReadingCoachPanel } = await import("../src/readings/ReadingCoachPanel");

function slot(): ReadingCoachSlot {
  return { locked: false, quotes: [], removeQuote: () => {}, clearQuotes: vi.fn() };
}

afterEach(cleanup);

describe("LiteChatMarkdown", () => {
  it("puts **bold** in the accent colour, not in ink", () => {
    const { container } = render(<LiteChatMarkdown text="先找到**转折**那一句。" />);
    const strong = container.querySelector("strong");
    expect(strong).not.toBeNull();
    expect(strong!.textContent).toBe("转折");
    // The accent token has to reach the DOM. Asserted on the style attribute
    // rather than on `.style.color`, because a CSS variable is not a colour
    // jsdom's own parser can resolve — the attribute is the honest record of
    // what the browser will be handed.
    expect(strong!.getAttribute("style") ?? "").toContain("--mk-accent-600");
    // …and it is emphasis, not the surrounding ink colour repeated in bold.
    expect(strong!.className).not.toContain("text-mk-ink");
  });

  it("renders a short list as a real list", () => {
    const { container } = render(<LiteChatMarkdown text={"你想先看哪一个：\n\n- 第一段的数字\n- 最后那句反问"} />);
    const items = container.querySelectorAll("ul li");
    expect(items).toHaveLength(2);
    expect(items[0]!.textContent).toBe("第一段的数字");
    expect(items[1]!.textContent).toBe("最后那句反问");
  });

  it("still renders paragraphs and links the way the shared renderer does", () => {
    const { container } = render(<LiteChatMarkdown text="看看 [NASA](https://nasa.gov) 怎么说。" />);
    const a = container.querySelector("a");
    expect(a?.getAttribute("href")).toBe("https://nasa.gov");
    expect(a?.getAttribute("target")).toBe("_blank");
  });
});

describe("ReadingCoachPanel message roles", () => {
  it("renders 印记's turn as markdown", () => {
    const messages: LiteMessage[] = [{ seq: 1, role: "ai", content: "注意**因果**这个词。", createdAt: "" }];
    const { container } = render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={[]}
        initialMessages={messages}
        slot={slot()}
        onTasks={() => {}}
        onFocusBlock={() => {}}
      />,
    );
    const strong = container.querySelector("strong");
    expect(strong?.textContent).toBe("因果");
    expect(strong!.getAttribute("style") ?? "").toContain("--mk-accent-600");
  });

  it("leaves the student's own words alone — her asterisks are hers", () => {
    const messages: LiteMessage[] = [
      { seq: 1, role: "student", content: "我觉得**这句**最重要，# 但我说不清为什么", createdAt: "" },
    ];
    const { container } = render(
      <ReadingCoachPanel
        readingId="r1"
        tasks={[]}
        initialMessages={messages}
        slot={slot()}
        onTasks={() => {}}
        onFocusBlock={() => {}}
      />,
    );
    // Verbatim, asterisks and hash included.
    expect(screen.getByText("我觉得**这句**最重要，# 但我说不清为什么")).toBeTruthy();
    // Nothing of hers was promoted to markup.
    expect(container.querySelector("strong")).toBeNull();
    expect(container.querySelector("h1")).toBeNull();
  });
});
