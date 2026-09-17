import { describe, expect, it } from "vitest";
import type { AssignmentInboxItem } from "../api/assignments";
import { ApiError } from "../api/client";
import {
  errorMessage,
  inboxPanelLeft,
  openItemsForKind,
  sortUnreadFirst,
  startButtonLabel,
  startErrorText,
  stripDueAt,
} from "./inboxLogic";

function item(over: Partial<AssignmentInboxItem>): AssignmentInboxItem {
  return {
    type: "assignment",
    id: "a",
    kind: "reading",
    title: "t",
    instructions: "",
    className: "c",
    dueAt: "2026-09-20T14:00:00Z",
    status: "not_started",
    statusLabel: "未开始",
    atomId: null,
    unread: false,
    returnDueAt: null,
    returnNote: null,
    ...over,
  };
}

describe("startErrorText", () => {
  it("keeps a message the server already prefixed", () =>
    expect(startErrorText("开始失败：链接无法读取正文，请告知老师更换阅读材料")).toBe(
      "开始失败：链接无法读取正文，请告知老师更换阅读材料",
    ));
  it("prefixes a bare message once", () => expect(startErrorText("作业不存在")).toBe("开始失败：作业不存在"));
  it("never leaves an empty tail", () => expect(startErrorText("  ")).toBe("开始失败：没有更多信息"));
});

describe("errorMessage", () => {
  it("reads an ApiError's message without adding a prefix", () =>
    expect(errorMessage(new ApiError("not_found", "资源不存在", 404))).toBe("资源不存在"));
  it("non-errors become empty", () => expect(errorMessage({ x: 1 })).toBe(""));
});

describe("openItemsForKind", () => {
  const items: AssignmentInboxItem[] = [
    item({ id: "1", status: "not_started" }),
    item({ id: "2", status: "in_progress", atomId: "x" }),
    item({ id: "3", status: "overdue" }),
    item({ id: "4", status: "done", atomId: "y" }),
    item({ id: "5", status: "done_late", atomId: "z" }),
    item({ id: "6", kind: "writing", status: "not_started" }),
  ];
  it("keeps open items of that kind", () =>
    expect(openItemsForKind(items, "reading").map((it) => it.id)).toEqual(["1", "2", "3"]));
  it("is empty when nothing of that kind is open", () => expect(openItemsForKind(items, "project")).toEqual([]));
  // The home page lists every kind, soonest deadline first: a student who
  // signs in sees what is due before anything else (real-user walk,
  // 2026-09-17: the home page showed none of three new homeworks).
  it("lists every kind, soonest deadline first, when kind is null", () =>
    expect(
      openItemsForKind(
        [
          item({ id: "late", dueAt: "2026-09-23T10:00:00Z" }),
          item({ id: "soon", kind: "writing", dueAt: "2026-09-18T13:00:00Z" }),
          item({ id: "done", status: "done", dueAt: "2026-09-17T13:00:00Z" }),
        ],
        null,
      ).map((it) => it.id),
    ).toEqual(["soon", "late"]));
});

describe("sortUnreadFirst", () => {
  it("moves unread ahead and keeps order within each group", () =>
    expect(
      sortUnreadFirst([
        { id: "a", unread: false },
        { id: "b", unread: true },
        { id: "c", unread: false },
        { id: "d", unread: true },
      ]).map((it) => it.id),
    ).toEqual(["b", "d", "a", "c"]));
});

// Why this is tested: the rail rests at 220px on a wide landing and 56–68px
// elsewhere. A fixed offset covered the rail foot on the landing, and a
// screenshot is the only other place that shows it.
describe("inboxPanelLeft", () => {
  it("opens beside a wide landing rail", () => expect(inboxPanelLeft(220, 1440)).toBe(228));
  it("opens beside the folded rail", () => expect(inboxPanelLeft(68, 1440)).toBe(76));
  it("slides left to keep a 16px right margin", () => expect(inboxPanelLeft(220, 500)).toBe(124));
  it("never goes past the 16px left margin on a phone", () => expect(inboxPanelLeft(56, 400)).toBe(24));
  it("clamps to 16px when the viewport is narrower than the panel", () => expect(inboxPanelLeft(56, 320)).toBe(16));
});

describe("returned homework on the strip", () => {
  it("lists a returned writing as open", () =>
    expect(openItemsForKind([item({ id: "r", kind: "writing", status: "returned", atomId: "x" })], "writing").map((i) => i.id)).toEqual(
      ["r"],
    ));
  it("does not list a resubmitted writing", () =>
    expect(openItemsForKind([item({ kind: "writing", status: "resubmitted", atomId: "x" })], "writing")).toEqual([]));
  it("shows the return deadline for a returned item", () =>
    expect(stripDueAt({ dueAt: "2026-09-10T14:00:00Z", returnDueAt: "2026-09-18T14:00:00Z" })).toBe("2026-09-18T14:00:00Z"));
  // Past return_due_at with no new version the status is `overdue`, and the
  // deadline she missed is the return deadline, not the original one.
  it("keeps the return deadline once a returned item is overdue", () =>
    expect(stripDueAt({ dueAt: "2026-09-10T14:00:00Z", returnDueAt: "2026-09-18T14:00:00Z" })).toBe("2026-09-18T14:00:00Z"));
  it("shows the assignment deadline when never returned", () =>
    expect(stripDueAt({ dueAt: "2026-09-10T14:00:00Z", returnDueAt: null })).toBe("2026-09-10T14:00:00Z"));
});

describe("startButtonLabel", () => {
  it("not started", () => expect(startButtonLabel({ status: "not_started", atomId: null })).toBe("开始"));
  it("in progress", () => expect(startButtonLabel({ status: "in_progress", atomId: "x" })).toBe("继续"));
  it("overdue with a room", () => expect(startButtonLabel({ status: "overdue", atomId: "x" })).toBe("继续"));
  it("overdue and never opened", () => expect(startButtonLabel({ status: "overdue", atomId: null })).toBe("开始"));
});
