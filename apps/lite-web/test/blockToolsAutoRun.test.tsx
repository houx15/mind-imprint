import { render, screen, cleanup, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * When 印记 reaches for a tool, the tool RUNS.
 *
 * The product call: *"for the 仿写 or questions, these are tools and AI can
 * decide how to guide students to read deeper."* Deciding is only half of it —
 * if the coach picks 仿写 and the panel merely lights up a button she has to
 * find, the coach has made a suggestion, not taught anything.
 *
 * These tests pin the two halves of that: the chosen tool fires exactly once
 * (it is a metered model call, not a render effect), and a tool the panel
 * cannot open releases the latch instead of stranding it — otherwise the next
 * paragraph 印记 opens would arrive carrying a tool that never clears.
 */

const explain = vi.fn();
vi.mock("../src/api/readingRoom", async (orig) => ({
  ...(await orig<Record<string, unknown>>()),
  explainReadingBlock: (...args: unknown[]) => explain(...args),
}));

const { BlockToolsPanel } = await import("../src/readings/BlockToolsPanel");

const TOOLS = [
  { id: "translate", label: "翻译" },
  { id: "imitate", label: "仿写" },
];

function mount(autoTool: string | null, onConsumed = vi.fn()) {
  // The tool buttons live in the floating bar, and the bar pins itself to the
  // paragraph — so a mount with no paragraph renders no buttons at all.
  const paragraph = document.createElement("p");
  paragraph.setAttribute("data-block-id", "b2");
  document.body.appendChild(paragraph);
  render(
    <BlockToolsPanel
      readingId="r1"
      blockId="b2"
      anchorEl={paragraph}
      pointerX={200}
      tools={TOOLS}
      notes={[]}
      autoTool={autoTool}
      onAutoToolConsumed={onConsumed}
      onNote={() => {}}
      onClose={() => {}}
    />,
  );
  return onConsumed;
}

beforeEach(() => {
  explain.mockReset();
  explain.mockResolvedValue({ blockId: "b2", tool: "imitate", body: "**试着照这段写一句**", cached: false });
});
afterEach(cleanup);

describe("BlockToolsPanel auto-run", () => {
  it("runs the tool 印记 chose, without her pressing anything", async () => {
    const consumed = mount("imitate");
    await waitFor(() => expect(explain).toHaveBeenCalledWith("r1", "b2", "imitate"));
    await waitFor(() => expect(consumed).toHaveBeenCalled());
    expect(explain).toHaveBeenCalledTimes(1);
  });

  it("stays quiet when 印记 named no tool", async () => {
    mount(null);
    expect(await screen.findByText("翻译")).toBeTruthy();
    expect(explain).not.toHaveBeenCalled();
  });

  it("releases the latch for a tool it cannot open", async () => {
    const consumed = mount("no-such-tool");
    await waitFor(() => expect(consumed).toHaveBeenCalled());
    expect(explain).not.toHaveBeenCalled();
  });
});
