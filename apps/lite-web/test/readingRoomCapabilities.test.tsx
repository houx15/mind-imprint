import { render, screen, cleanup, fireEvent, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { MaterialSource, TakeawayDraft } from "@mind-imprint/contracts";
import { ReadingRoom, type LiteReadingRoomApi } from "@lite/readings/ReadingRoom";
import { FinalizeReadingPanel } from "@/studio/reading/FinalizeReadingPanel";

/**
 * The pro-only reading surfaces, asserted ON LITE'S OWN RENDERED ROOM.
 *
 * ## Why this file was rewritten (2026-08-29)
 *
 * It used to render **pro's** `ReadingRoom` with `LITE_READING_CAPABILITIES`
 * to prove that 证据笔记 / 追来源 stay hidden. Once the reading room forked
 * (`apps/lite-web/src/readings/ReadingRoom.tsx`), lite stopped mounting that
 * component at all — so every assertion here was about a file the lite
 * product never renders. It stayed green and guarded nothing, which is worse
 * than no test: it looks like someone is watching.
 *
 * Worse, the two 完成这篇 cases asserted on the *preset constants*
 * (`LITE_READING_CAPABILITIES.proposalImpact`) rather than on anything the
 * room does. Lite's room now passes `proposalImpact={false}` /
 * `credibility={false}` to `FinalizeReadingPanel` as plain literals
 * (`ReadingRoom.tsx`, the `finalizeOpen` block), and flipping either literal
 * to `true` could not have failed a constant assertion — a lite student would
 * have started seeing 新的线索 / 对论点的影响, and 「可信度 · 尚未评估」, a
 * permanent lie dressed as a state (there is no CRAAP-style producer in lite
 * to ever fill it in). The only net that caught that was the live e2e walk.
 *
 * So: mount LITE's room, click 完成这篇 for real, and assert on the modal that
 * actually appears.
 *
 * ## Why an absence test here is not vacuous
 *
 * An absence assertion can pass because the guard works or because nothing
 * renders at all. Two things keep this honest:
 *
 *  - `renders the reading surfaces lite DOES have` proves the room mounted;
 *  - `the shared panel still HAS all three surfaces …` renders the same
 *    (still pro-shared) `FinalizeReadingPanel` with the flags on and finds
 *    every literal this file claims is absent — so the queries are known to
 *    match when the surface is present.
 */

const SOURCE: MaterialSource = {
  id: "atom-1",
  title: "中国的可持续转型",
  sourceUrl: "https://example.org/a",
  kind: "article",
  origin: "",
  blocks: [
    { id: "b1", text: "中国的太阳能装机量在过去十年增长了十倍。" },
    { id: "b2", text: "但同一时期，中国的碳排放总量仍居全球第一。" },
  ],
  locked: false,
  role: "",
  tier: "",
  takeaway: "",
  anchors: [],
  timeSpentS: 0,
  lateralRead: false,
  isLateralInstrument: false,
  siftSkipped: false,
  lateralRelation: "",
  lateralJudgment: "",
};

const DRAFT: TakeawayDraft = {
  record: {
    findings: ["中国的太阳能装机量在过去十年增长了十倍。"],
    // Deliberately POPULATED: if the room ever let the 可信度 row through, an
    // empty verdict would only render 尚未评估 and the 可信度 label would
    // still be the thing that fails. A filled verdict makes the failure loud
    // either way.
    credibility: { verdict: "可信", why: "来自 NASA 与 Nature Sustainability 的交叉印证。" },
    keyQuotes: [],
  },
  suggestedNewLeads: ["中国的碳排放总量为什么仍居第一？"],
  suggestedProposalImpact: "这篇支持了「转型正在发生」这一半。",
};

// A fake that satisfies every method the room may reach for. Only
// `getTakeawayDraft` is actually called (by 完成这篇); the rest exist so the
// render is legal.
const API: LiteReadingRoomApi = {
  // eslint-disable-next-line require-yield
  async *readTurn() {},
  // eslint-disable-next-line require-yield
  async *summonCard() {},
  async activateProjectCard() {},
  async evaluateCardSelection() {
    throw new Error("not used");
  },
  // eslint-disable-next-line require-yield
  async *submitProjectCard() {},
  async skipProjectCard() {},
  async getOpenCard() {
    return null;
  },
  async getTakeawayDraft() {
    return DRAFT;
  },
  async postFinalizeReading() {
    return null;
  },
};

function renderLiteRoom() {
  return render(
    <ReadingRoom
      readingId="atom-1"
      source={SOURCE}
      api={API}
      onBack={() => {}}
      tasks={[]}
      onTasks={() => {}}
      coachMessages={[]}
      blockTools={[]}
      blockNotes={[]}
      onBlockNote={() => {}}
    />,
  );
}

/**
 * Opens the real 完成这篇 modal, waits for the draft to land, and returns the
 * DIALOG — scoping every assertion to it, so the article behind the modal
 * cannot satisfy (or, with duplicate matches, break) a query about the panel.
 */
async function openFinalize(): Promise<HTMLElement> {
  fireEvent.click(screen.getByRole("button", { name: "完成这篇" }));
  // 正在整理你的阅读发现… is the loading half of the same modal; wait past it,
  // otherwise every absence below would pass on an empty panel.
  await screen.findByText("你的阅读记录 · 只读");
  return screen.getByRole("dialog", { name: "完成这篇" });
}

afterEach(cleanup);

describe("lite's reading room does not carry pro's reading surfaces", () => {
  it("has no 证据笔记 and no 追来源", () => {
    renderLiteRoom();
    // Both were `caps.evidenceMap` / `caps.explorationLeads` branches before
    // the fork; lite's file does not contain either. Re-importing pro's room
    // into lite, or copying those branches across, fails here.
    expect(screen.queryByText("证据笔记")).toBeNull();
    expect(screen.queryByRole("button", { name: "追来源" })).toBeNull();
  });

  it("renders the reading surfaces lite DOES have", () => {
    renderLiteRoom();
    // The room mounted — without this the absences above could pass on a
    // blank page. The lens library, the article, and 阅读成果 are the room.
    expect(screen.getByText(/透镜库/)).toBeTruthy();
    expect(screen.getByText("中国的太阳能装机量在过去十年增长了十倍。")).toBeTruthy();
    expect(screen.getByRole("tab", { name: /阅读成果/ })).toBeTruthy();
  });

  it("opens 完成这篇 without 新的线索 / 对论点的影响 — there is no 立题 behind them", async () => {
    renderLiteRoom();
    const dialog = await openFinalize();

    // The synthesis half is written against a research proposal lite has no
    // concept of. What survives is the one box that is hers.
    expect(within(dialog).queryByText("新的线索")).toBeNull();
    expect(within(dialog).queryByText("对论点的影响")).toBeNull();
    expect(within(dialog).getByText("我的收获")).toBeTruthy();
    // …and the draft's proposal-shaped suggestions never reach a field.
    expect(within(dialog).queryByDisplayValue("中国的碳排放总量为什么仍居第一？")).toBeNull();
  });

  it("opens 完成这篇 without 可信度 — a verdict with no producer is a lie, not a state", async () => {
    renderLiteRoom();
    const dialog = await openFinalize();

    expect(within(dialog).queryByText("可信度")).toBeNull();
    // The verdict itself, not just its label: 可信 — 来自 NASA… is in the
    // draft this room was handed, and must reach no row.
    expect(within(dialog).queryByText(/来自 NASA/)).toBeNull();
    // Her own confirmed findings ARE the record half, and they still show.
    expect(within(dialog).getByText("中国的太阳能装机量在过去十年增长了十倍。")).toBeTruthy();
  });

  it("the shared panel still HAS all three surfaces when the flags are on", () => {
    // Anti-vacuity control. `FinalizeReadingPanel` is still shared with pro,
    // so if a refactor deleted 新的线索 / 对论点的影响 / 可信度 outright, the
    // three absence tests above would keep passing while pro silently lost
    // them. This is the line that would go red instead.
    render(
      <FinalizeReadingPanel
        loading={false}
        draft={DRAFT}
        leadsText=""
        onLeadsChange={() => {}}
        impactText=""
        onImpactChange={() => {}}
        saving={false}
        done={false}
        onConfirm={() => {}}
        onClose={() => {}}
        proposalImpact
        credibility
      />,
    );
    expect(screen.getByText("新的线索")).toBeTruthy();
    expect(screen.getByText("对论点的影响")).toBeTruthy();
    expect(screen.getByText("可信度")).toBeTruthy();
  });
});
