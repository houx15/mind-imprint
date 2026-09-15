import { describe, expect, it } from "vitest";
import {
  normalizeAssignmentForAtom,
  normalizeExtractResult,
  normalizeInboxResponse,
  normalizePreviewRows,
  normalizeRecipientDTO,
} from "./assignments";

function inboxItem(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    type: "assignment",
    id: "a1",
    kind: "reading",
    title: "读一读《中国是否让地球变得更可持续？》",
    instructions: "",
    className: "初三 1 班",
    dueAt: "2026-09-20T14:00:00Z",
    status: "not_started",
    statusLabel: "未开始",
    atomId: null,
    unread: true,
    ...over,
  };
}

describe("normalizeInboxResponse", () => {
  it("defaults unread to the count of unread:true items when the field is missing", () => {
    const raw = {
      items: [inboxItem({ id: "a1", unread: true }), inboxItem({ id: "a2", unread: false })],
      // `unread` deliberately absent
    };
    const result = normalizeInboxResponse(raw);
    expect(result.unread).toBe(1);
  });

  it("still uses the server's own unread count when it is present", () => {
    const raw = { items: [inboxItem({ unread: true })], unread: 5 };
    expect(normalizeInboxResponse(raw).unread).toBe(5);
  });

  it("maps an unknown status value to not_started", () => {
    const raw = { items: [inboxItem({ status: "some_future_status" })], unread: 1 };
    const item = normalizeInboxResponse(raw).items[0];
    expect(item?.type).toBe("assignment");
    if (item?.type !== "assignment") throw new Error("expected an assignment item");
    expect(item.status).toBe("not_started");
  });

  // A server from before the parent end was removed can still send report
  // rows during a deploy. Coerced, one would show as a 阅读 assignment with
  // no due date that starts nothing.
  it("drops rows that are not assignments, and counts unread from the rows kept", () => {
    const raw = {
      items: [
        inboxItem({ id: "a1", unread: false }),
        { type: "parent_report", id: "r1", title: "家长报告（8月17日–9月13日）", className: "初三 1 班", unread: true },
        { id: "x1", unread: true },
      ],
      unread: 2,
    };
    const result = normalizeInboxResponse(raw);
    expect(result.items.map((it) => it.id)).toEqual(["a1"]);
    expect(result.unread).toBe(0);
  });

  it("keeps sent grading rows next to assignments", () => {
    const raw = {
      items: [
        inboxItem({ id: "a1", unread: false }),
        { type: "grading", id: "g1", atomId: "w1", writingTitle: "雨水去哪儿了", sentAt: "2026-09-15T06:20:00Z", unread: true },
      ],
      unread: 1,
    };
    const result = normalizeInboxResponse(raw);
    expect(result.items.map((it) => `${it.type}:${it.id}`)).toEqual(["assignment:a1", "grading:g1"]);
    expect(result.unread).toBe(1);
  });
});

describe("normalizeExtractResult", () => {
  it("keeps targetWords null instead of coercing it to 0", () => {
    const result = normalizeExtractResult({ prompt: "写一篇关于校园可持续发展的短文", targetWords: null, lang: "zh" });
    expect(result.targetWords).toBeNull();
  });

  it("keeps a real targetWords number as-is", () => {
    const result = normalizeExtractResult({ prompt: "Write a short essay", targetWords: 800, lang: "en" });
    expect(result.targetWords).toBe(800);
  });
});

describe("return fields", () => {
  it("keeps returned and resubmitted statuses and the return columns on a recipient", () => {
    const r = normalizeRecipientDTO({
      userId: "u",
      status: "returned",
      statusLabel: "已退回",
      returnedAt: "2026-09-15T10:00:00+08:00",
      returnDueAt: "2026-09-18T22:00:00+08:00",
      returnNote: "请补充第二段的论据",
      versionCount: 2,
    });
    expect([r.status, r.returnedAt, r.returnDueAt, r.returnNote, r.versionCount]).toEqual([
      "returned",
      "2026-09-15T10:00:00+08:00",
      "2026-09-18T22:00:00+08:00",
      "请补充第二段的论据",
      2,
    ]);
    expect(normalizeRecipientDTO({ status: "resubmitted" }).status).toBe("resubmitted");
    expect(normalizeRecipientDTO({}).versionCount).toBe(0);
  });

  it("reads return fields on for-atom, with safe defaults from an older server", () => {
    expect(normalizeAssignmentForAtom({ id: "a", title: "t", dueAt: "d" })).toEqual({
      id: "a",
      kind: "writing",
      title: "t",
      dueAt: "d",
      returnedAt: null,
      returnDueAt: null,
      returnNote: null,
      resubmitted: false,
    });
    expect(normalizeAssignmentForAtom({ id: "a", kind: "reading", returnedAt: "x", resubmitted: true })?.resubmitted).toBe(true);
  });

  it("carries the return deadline and note into inbox items", () => {
    const { items } = normalizeInboxResponse({
      items: [{ type: "assignment", id: "a", status: "returned", returnDueAt: "2026-09-18T22:00:00+08:00", returnNote: "n" }],
    });
    const item = items[0];
    if (item?.type !== "assignment") throw new Error("expected an assignment item");
    expect([item.status, item.returnDueAt, item.returnNote]).toEqual(["returned", "2026-09-18T22:00:00+08:00", "n"]);
  });
});

describe("normalizeRecipientDTO reading", () => {
  const base = { userId: "u1", displayName: "Phoebe", status: "not_started" };
  it("is null when the server sends none or an unknown state", () => {
    expect(normalizeRecipientDTO(base).reading).toBeNull();
    expect(normalizeRecipientDTO({ ...base, reading: { slug: "a", state: "later" } }).reading).toBeNull();
  });
  it("keeps a picked article with an open tier", () => {
    expect(normalizeRecipientDTO({ ...base, reading: { slug: "coral", title: "珊瑚", tier: null, state: "picked" } }).reading).toEqual({
      slug: "coral",
      title: "珊瑚",
      tier: null,
      state: "picked",
    });
  });
  it("drops a tier outside 1..5", () => {
    expect(normalizeRecipientDTO({ ...base, reading: { slug: "coral", title: "珊瑚", tier: 9, state: "started" } }).reading?.tier).toBeNull();
  });
});

describe("normalizePreviewRows", () => {
  it("drops rows without a student or an article and repairs the tier", () => {
    const rows = normalizePreviewRows({
      rows: [
        { userId: "u1", name: "Phoebe", slug: "coral", title: "珊瑚", tier: 3, suggestedTier: 2, reason: "暂无兴趣数据，按难度推荐" },
        { userId: "u2", name: "林知遥", slug: "", title: "", tier: 3, suggestedTier: 2, reason: "" },
        { name: "no id", slug: "x" },
        { userId: "u3", name: "王", slug: "nasa", title: "NASA", tier: 0, suggestedTier: 4, reason: "" },
      ],
    });
    expect(rows.map((r) => r.userId)).toEqual(["u1", "u3"]);
    expect(rows[1]).toMatchObject({ tier: 4, suggestedTier: 4 });
  });
  it("is empty for a malformed body", () => {
    expect(normalizePreviewRows(null)).toEqual([]);
  });
});
