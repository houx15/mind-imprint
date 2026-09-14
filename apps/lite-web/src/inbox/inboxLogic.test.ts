import { describe, expect, it } from "vitest";
import type { AssignmentInboxItem, InboxItemDTO, ParentReportInboxItem } from "../api/assignments";
import { ApiError } from "../api/client";
import {
  assignmentItems,
  errorMessage,
  inboxChipLabel,
  inboxTargetPath,
  publishedLabel,
  inboxPanelLeft,
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

describe("mixed inbox items", () => {
  const report: ParentReportInboxItem = {
    type: "parent_report",
    id: "r1",
    title: "家长报告（8月17日–9月13日）",
    className: "c",
    publishedAt: "2026-09-13T16:30:00Z",
    unread: true,
  };
  const items: InboxItemDTO[] = [item({ id: "a1", kind: "writing" }), report, item({ id: "a2", unread: true })];

  it("assignmentItems keeps only assignments", () =>
    expect(assignmentItems(items).map((it) => it.id)).toEqual(["a1", "a2"]));
  it("sortUnreadFirst keeps both kinds", () =>
    expect(sortUnreadFirst(items).map((it) => it.id)).toEqual(["r1", "a2", "a1"]));
  it("chip for an assignment is its kind", () => expect(inboxChipLabel(item({ kind: "writing" }))).toBe("写作"));
  it("chip for a report is 报告", () => expect(inboxChipLabel(report)).toBe("报告"));
  // Beijing date: 16:30Z on the 13th is already the 14th for her.
  it("report meta line names the Beijing publish date", () => expect(publishedLabel(report.publishedAt)).toBe("发布于 9月14日"));
  it("no meta line for a missing date", () => expect(publishedLabel("")).toBe(""));
  it("a report opens its own page", () => expect(inboxTargetPath(report)).toBe("/parent-reports/r1"));
  it("an assignment has no page to open directly", () => expect(inboxTargetPath(item({}))).toBeNull());
});

describe("openItemsForKind", () => {
  const report: InboxItemDTO = { type: "parent_report", id: "r", title: "", className: "", publishedAt: "", unread: true };
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

describe("startButtonLabel", () => {
  it("not started", () => expect(startButtonLabel({ status: "not_started", atomId: null })).toBe("开始"));
  it("in progress", () => expect(startButtonLabel({ status: "in_progress", atomId: "x" })).toBe("继续"));
  it("overdue with a room", () => expect(startButtonLabel({ status: "overdue", atomId: "x" })).toBe("继续"));
  it("overdue and never opened", () => expect(startButtonLabel({ status: "overdue", atomId: null })).toBe("开始"));
});
