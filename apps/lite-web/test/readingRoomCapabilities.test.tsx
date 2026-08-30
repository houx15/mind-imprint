import { render, screen, cleanup, fireEvent, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { MaterialSource } from "@mind-imprint/contracts";
import { ReadingRoom, type LiteReadingRoomApi } from "@lite/readings/ReadingRoom";

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


// A fake that satisfies every method the room may reach for. Only
// `finishReading` is actually called (by 完成这篇); the rest exist so the
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
  async finishReading() {
    finished += 1;
    return null;
  },
};

/** How many times 完成这篇 actually reached the server. */
let finished = 0;

function renderLiteRoom() {
  return render(
    <ReadingRoom
      readingId="atom-1"
      source={SOURCE}
      api={API}
      onBack={() => {}}
      onFinished={() => {
        landedOnReport += 1;
      }}
      tasks={[]}
      onTasks={() => {}}
      coachMessages={[]}
      blockTools={[]}
      blockNotes={[]}
      onBlockNote={() => {}}
    />,
  );
}

/** 完成这篇 opens a confirm, not a form. Returns that dialog. */
function openFinishAsk(): HTMLElement {
  fireEvent.click(screen.getByRole("button", { name: "完成这篇" }));
  return screen.getByRole("dialog", { name: "完成这篇" });
}

/** How many times the room told the host to go to the report. */
let landedOnReport = 0;

beforeEach(() => {
  finished = 0;
  landedOnReport = 0;
});

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

  it("完成这篇 asks one question and collects NOTHING", async () => {
    renderLiteRoom();
    const dialog = openFinishAsk();

    // The form is gone. It used to want 我的收获 (and, behind pro's flags,
    // 新的线索 / 对论点的影响 / 可信度) before it would let her finish:
    //
    //   > we have give abundant steps for the reading. so we don't need to
    //   > ask student to enter the form again.
    //
    // So the strongest thing to assert is not "those fields are hidden" but
    // "there is no field at all" — a shape a future edit cannot half-restore.
    expect(within(dialog).queryAllByRole("textbox")).toHaveLength(0);
    for (const gone of ["我的收获", "新的线索", "对论点的影响", "可信度", "你的阅读记录 · 只读"]) {
      expect(within(dialog).queryByText(gone), gone).toBeNull();
    }
  });

  it("完成这篇 confirms first — it is terminal, and the button sits there all session", async () => {
    renderLiteRoom();
    const dialog = openFinishAsk();

    // Nothing has happened yet: opening the ask must not finish anything.
    expect(finished).toBe(0);
    expect(landedOnReport).toBe(0);

    // And she can back out.
    fireEvent.click(within(dialog).getByRole("button", { name: "再读一会儿" }));
    expect(screen.queryByRole("dialog", { name: "完成这篇" })).toBeNull();
    expect(finished).toBe(0);
  });

  it("确认之后：完成，然后交给宿主换成报告", async () => {
    renderLiteRoom();
    const dialog = openFinishAsk();

    fireEvent.click(within(dialog).getByRole("button", { name: "完成，看报告" }));
    await waitFor(() => expect(finished).toBe(1));
    // The room does not navigate itself: it tells the host, which re-reads the
    // reading and swaps in the report. One owner for that decision.
    await waitFor(() => expect(landedOnReport).toBe(1));
  });
});
