import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { MaterialSource, Reference } from "@mind-imprint/contracts";
import { ReadingRoom, type ReadingRoomApi } from "@/studio/reading/ReadingRoom";
import { LITE_READING_CAPABILITIES, PRO_CAPABILITIES } from "@/rooms/capabilities";

/**
 * The capability gates, asserted where they actually live — ON THE RENDERED
 * ROOM, not on the plain-data preset module.
 *
 * Task 10 introduced `RoomCapabilities` and Task 12 mounts the room under the
 * lite preset, but until this file nothing rendered `ReadingRoom` with
 * `capabilities={LITE_READING_CAPABILITIES}` to confirm the gates suppress
 * anything. A JSX refactor that moved 证据笔记 or 追来源 out from behind
 * `caps.*` would have shipped them into the lite product silently, and the
 * lite edition has neither an evidence map nor an exploration graph behind
 * them.
 *
 * Each surface is asserted BOTH ways: absent under lite AND present under pro
 * with the same props. A one-sided test would keep passing if the surface
 * simply stopped rendering for everyone.
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

const REFERENCE = {
  id: "ref-1",
  title: "中国的可持续转型",
  classification: "",
  author: "",
  credentials: "",
  year: "",
  url: "https://example.org/a",
  tags: [],
  collectionId: null,
  credibility: null,
  evaluation: "",
  decision: "undecided",
  pending: false,
  searchHints: [],
  materialId: "atom-1",
  notes: [],
  readingStatus: "reading",
} as unknown as Reference;

// A fake that satisfies every method the room may reach for. Nothing here is
// called by a bare render — its job is to make the render legal.
const API: ReadingRoomApi = {
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
  async putReadingBrief() {},
  async getTakeawayDraft() {
    throw new Error("not used");
  },
  async postFinalizeReading() {
    return null;
  },
};

/** Both gated surfaces need their pro-only callbacks present — the gate is
 *  what must hide them, not a missing prop. */
function renderRoom(capabilities: typeof PRO_CAPABILITIES) {
  return render(
    <ReadingRoom
      projectId="atom-1"
      referenceId="atom-1"
      source={SOURCE}
      reference={REFERENCE}
      onSetEvidence={async () => REFERENCE}
      onSetTriage={async () => REFERENCE}
      onArchive={async () => REFERENCE}
      onTraceCitation={async () => []}
      onTraceSearch={async () => []}
      onAdoptSource={async () => {}}
      api={API}
      onBack={() => {}}
      capabilities={capabilities}
    />,
  );
}

afterEach(cleanup);

describe("ReadingRoom capability gates", () => {
  it("hides 证据笔记 and 追来源 under the lite preset", () => {
    renderRoom(LITE_READING_CAPABILITIES);
    expect(screen.queryByText("证据笔记")).toBeNull();
    expect(screen.queryByRole("button", { name: "追来源" })).toBeNull();
  });

  it("still shows both under the pro preset, with the same props", () => {
    renderRoom(PRO_CAPABILITIES);
    expect(screen.getByText("证据笔记")).toBeTruthy();
    expect(screen.getByRole("button", { name: "追来源" })).toBeTruthy();
  });

  it("keeps the reading surfaces the lite edition DOES have", () => {
    renderRoom(LITE_READING_CAPABILITIES);
    // The lens library, the article, and the 阅读成果 tab are the room — a gate
    // that suppressed those would be a bug, not restraint.
    expect(screen.getByText(/透镜库/)).toBeTruthy();
    expect(screen.getByText("中国的太阳能装机量在过去十年增长了十倍。")).toBeTruthy();
    expect(screen.getByRole("tab", { name: /阅读成果/ })).toBeTruthy();
  });

  it("drops the proposal-shaped half of 完成这篇 under lite", () => {
    // 新的线索 / 对论点的影响 are questions about a 立题 the lite edition does
    // not have. The finalize panel is only mounted on demand, so this asserts
    // the capability the room passes down rather than the open modal.
    expect(LITE_READING_CAPABILITIES.proposalImpact).toBe(false);
    expect(PRO_CAPABILITIES.proposalImpact).toBe(true);
  });
});
