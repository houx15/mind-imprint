import { describe, expect, it } from "vitest";
import { displayStat } from "@lite/reports/statLabels";
import type { ReportStat } from "@lite/api/reports";

/**
 * statLabels — a pure function, and exactly the kind of thing worth a test:
 * you cannot tell by reading a report that a number is standing under the
 * wrong noun.
 *
 * The bug this pins: `focusMinutes` is the ONE key both rooms emit, and both
 * the server literal and this table used to call it 阅读时长 — so a writing
 * report announced 「阅读时长 12 分钟」 over minutes she spent WRITING, on the
 * page, on the exported picture, and on the link she shares.
 *
 * The stored-blob case matters as much as the wording: labels resolve on the
 * client precisely so that every writing report ALREADY in the database
 * corrects itself on re-serve, with no migration and no regeneration. So the
 * inputs below carry the stale server label on purpose.
 */

const stat = (over: Partial<ReportStat> = {}): ReportStat => ({
  key: "focusMinutes",
  label: "阅读时长",
  value: 12,
  unit: "分钟",
  ...over,
});

describe("displayStat", () => {
  it("names focusMinutes for the room it was measured in", () => {
    expect(displayStat(stat(), "reading").label).toBe("阅读时长");
    expect(displayStat(stat(), "writing").label).toBe("写作时长");
  });

  it("corrects a report stored with the old server wording", () => {
    // A writing report generated before the fix carries Label:"阅读时长" in
    // its blob forever. Re-serving it must not show that.
    const stored = stat({ label: "阅读时长" });
    expect(displayStat(stored, "writing").label).toBe("写作时长");
    // …and never by mutating the caller's object.
    expect(stored.label).toBe("阅读时长");
  });

  it("leaves the shared keys alone in both rooms", () => {
    // chatTurns is emitted by both and means the same thing in both; only
    // focusMinutes needed splitting.
    const turns = stat({ key: "chatTurns", label: "和印记聊了", unit: "轮" });
    for (const kind of ["reading", "writing"] as const) {
      const out = displayStat(turns, kind);
      expect(out.label).toBe("AI 对话轮数");
      // `unit: ""` deliberately CLEARS the stored 轮 — the label already
      // names the quantity.
      expect(out.unit).toBe("");
    }
  });

  it("keeps each room's own keys reading correctly", () => {
    expect(displayStat(stat({ key: "words", label: "x", unit: "" }), "writing")).toMatchObject({
      label: "写了",
      unit: "字",
    });
    expect(displayStat(stat({ key: "wordsRead", label: "x", unit: "" }), "reading")).toMatchObject({
      label: "读了",
      unit: "字",
    });
  });

  it("keeps the server's 词 on an English piece", () => {
    expect(displayStat(stat({ key: "words", label: "写了", unit: "词" }), "writing").unit).toBe("词");
    expect(displayStat(stat({ key: "wordsRead", label: "读了", unit: "词" }), "reading").unit).toBe("词");
  });

  it("falls through to the server's wording for a key it does not know", () => {
    // A stat added server-side still renders — with server wording — before
    // this map learns about it.
    const unknown = stat({ key: "somethingNew", label: "新指标", unit: "次" });
    expect(displayStat(unknown, "writing")).toEqual(unknown);
    expect(displayStat(unknown, "reading")).toEqual(unknown);
  });
});
