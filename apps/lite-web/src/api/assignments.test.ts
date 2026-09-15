import { describe, expect, it } from "vitest";
import { normalizeExtractResult, normalizeInboxResponse } from "./assignments";

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
    expect(item?.status).toBe("not_started");
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
