import { describe, expect, it } from "vitest";
import type { AssignmentInboxItem, InboxItemDTO } from "../api/assignments";
import { ApiError } from "../api/client";
import {
  errorMessage,
  openItemsForKind,
  sortUnreadFirst,
  startButtonLabel,
  startErrorText,
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
  const report: InboxItemDTO = { type: "parent_report", id: "r", publishedAt: "", unread: true };
  const items: InboxItemDTO[] = [
    report,
    item({ id: "1", status: "not_started" }),
    item({ id: "2", status: "in_progress", atomId: "x" }),
    item({ id: "3", status: "overdue" }),
    item({ id: "4", status: "done", atomId: "y" }),
    item({ id: "5", status: "done_late", atomId: "z" }),
    item({ id: "6", kind: "writing", status: "not_started" }),
  ];
  it("keeps open items of that kind and skips parent reports", () =>
    expect(openItemsForKind(items, "reading").map((it) => it.id)).toEqual(["1", "2", "3"]));
  it("is empty when nothing of that kind is open", () => expect(openItemsForKind(items, "project")).toEqual([]));
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

describe("startButtonLabel", () => {
  it("not started", () => expect(startButtonLabel({ status: "not_started", atomId: null })).toBe("开始"));
  it("in progress", () => expect(startButtonLabel({ status: "in_progress", atomId: "x" })).toBe("继续"));
  it("overdue with a room", () => expect(startButtonLabel({ status: "overdue", atomId: "x" })).toBe("继续"));
  it("overdue and never opened", () => expect(startButtonLabel({ status: "overdue", atomId: null })).toBe("开始"));
});
