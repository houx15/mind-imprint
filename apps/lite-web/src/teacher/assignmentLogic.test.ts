import { describe, expect, it } from "vitest";
import { ApiError } from "../api/client";
import type { RecipientDTO } from "../api/assignments";
import type { LibraryArticle } from "../api/library";
import type { RosterRow } from "../api/teacher";
import {
  buildCreateInput,
  buildPatchInput,
  buildPayload,
  buildReturnInput,
  canEditSettings,
  emptySettings,
  failText,
  filterArticles,
  isArchiveSuccess,
  parseTargetWords,
  pickClassId,
  settingsFromAssignment,
  settingsSummary,
  statusChipStyle,
  tierLabel,
  unassignedStudents,
  validateSettings,
  type AssignmentDraft,
  type EditDraft,
} from "./assignmentLogic";
import { buildRubric, rubricDraftFromPayload, type RubricDraft } from "./rubricLogic";

function draft(over: Partial<AssignmentDraft> = {}): AssignmentDraft {
  return {
    ...emptySettings("writing"),
    prompt: "中国是否让地球变得更可持续？",
    targetWords: "800",
    classId: "c1",
    title: "议论文",
    instructions: "",
    dueInput: "2026-09-20T22:00",
    userIds: ["u1"],
    ...over,
  };
}

function recipient(over: Partial<RecipientDTO> = {}): RecipientDTO {
  return {
    userId: "u1",
    displayName: "Phoebe",
    avatarColor: "",
    status: "not_started",
    statusLabel: "未开始",
    atomId: null,
    startedAt: null,
    finishedAt: null,
    seenAt: null,
    returnedAt: null,
    returnDueAt: null,
    returnNote: null,
    versionCount: 0,
    ...over,
  };
}

// The server's messages are copied into the client checks; a drift here
// means the teacher sees two different sentences for the same mistake.
describe("validateSettings", () => {
  it("requires an article for a library reading, and accepts no level", () => {
    expect(validateSettings({ ...emptySettings("reading") })).toBe("请选择一篇文章");
    expect(validateSettings({ ...emptySettings("reading"), slug: "coral" })).toBeNull();
  });
  it("accepts only http/https links with a host", () => {
    const url = { ...emptySettings("reading"), readingSource: "url" as const };
    expect(validateSettings({ ...url, url: "javascript:alert(1)" })).toBe("请输入以 http 或 https 开头的链接");
    expect(validateSettings({ ...url, url: "example.org" })).toBe("请输入以 http 或 https 开头的链接");
    expect(validateSettings({ ...url, url: " https://example.org/a " })).toBeNull();
  });
  it("counts text length in characters, not UTF-16 units", () => {
    const text = { ...emptySettings("reading"), readingSource: "text" as const };
    expect(validateSettings({ ...text, text: "  " })).toBe("请粘贴文章正文");
    expect(validateSettings({ ...text, text: "😀".repeat(50000) })).toBeNull();
    expect(validateSettings({ ...text, text: "😀".repeat(50001) })).toBe("文章正文不能超过 50000 字");
  });
  it("checks writing prompt and word count", () => {
    const w = emptySettings("writing");
    expect(validateSettings(w)).toBe("请填写写作题目");
    expect(validateSettings({ ...w, prompt: "题" })).toBe("目标字数需在 1 到 100000 之间");
    expect(validateSettings({ ...w, prompt: "题", targetWords: "0" })).toBe("目标字数需在 1 到 100000 之间");
    expect(validateSettings({ ...w, prompt: "题", targetWords: "800" })).toBeNull();
  });
  it("requires a driving question for a project", () => {
    expect(validateSettings(emptySettings("project"))).toBe("请填写驱动问题");
  });
});

describe("parseTargetWords", () => {
  it("accepts whole numbers in range only", () => {
    expect(parseTargetWords(" 800 ")).toBe(800);
    expect(parseTargetWords("")).toBeNull();
    expect(parseTargetWords("12.5")).toBeNull();
    expect(parseTargetWords("-3")).toBeNull();
    expect(parseTargetWords("100001")).toBeNull();
  });
});

describe("buildPayload", () => {
  it("omits tier when the student's current level is used", () => {
    const p = buildPayload({ ...emptySettings("reading"), slug: "coral", tier: null });
    expect(p).toEqual({ source: "library", slug: "coral" });
    expect("tier" in p).toBe(false);
    expect(buildPayload({ ...emptySettings("reading"), slug: "coral", tier: 3 })).toEqual({ source: "library", slug: "coral", tier: 3 });
  });
  it("sends only the chosen source's field", () => {
    expect(buildPayload({ ...emptySettings("reading"), readingSource: "url", slug: "left", url: "https://a.org" })).toEqual({
      source: "url",
      url: "https://a.org",
    });
  });
  // Ruling: there is no rubric editor on a new homework's create form —
  // buildPayload never sends a rubric for a writing homework, customized or
  // not; the server always applies its own default for `lang` at creation,
  // and a rubric only ever exists once the homework does (see
  // `buildPatchInput rubric (writing only)` below).
  it("never sends rubric for a writing homework — there is no create-time editor", () => {
    const untouched = buildPayload({ ...emptySettings("writing"), prompt: "题", targetWords: "800" });
    expect("rubric" in untouched).toBe(false);
    const customized: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "论证", note: "看证据" }], focus: "" };
    const withRubricSet = buildPayload({ ...emptySettings("writing"), prompt: "题", targetWords: "800", rubric: customized });
    expect("rubric" in withRubricSet).toBe(false);
  });
});

describe("settingsFromAssignment", () => {
  it("round-trips a writing payload through the draft", () => {
    const payload = { prompt: "题目", targetWords: 800, lang: "en" };
    expect(buildPayload(settingsFromAssignment("writing", payload))).toEqual(payload);
  });
  it("leaves a missing word count empty, never 0", () => {
    expect(settingsFromAssignment("writing", { prompt: "p", lang: "zh" }).targetWords).toBe("");
    expect(settingsFromAssignment("writing", { prompt: "p", targetWords: 0, lang: "zh" }).targetWords).toBe("");
  });
});

describe("buildCreateInput", () => {
  it("converts the deadline to Beijing ISO and drops empty instructions", () => {
    const r = buildCreateInput(draft());
    expect(r.ok).toBe(true);
    if (!r.ok) return;
    expect(r.value.dueAt).toBe("2026-09-20T22:00:00+08:00");
    expect("instructions" in r.value).toBe(false);
    expect(r.value.payload).toEqual({ prompt: "中国是否让地球变得更可持续？", targetWords: 800, lang: "zh" });
  });
  it("reports the first problem in form order", () => {
    expect(buildCreateInput(draft({ classId: "" }))).toEqual({ ok: false, error: "请选择班级" });
    expect(buildCreateInput(draft({ title: " " }))).toEqual({ ok: false, error: "请填写作业标题，不超过 200 字" });
    expect(buildCreateInput(draft({ dueInput: "" }))).toEqual({ ok: false, error: "请填写截止时间" });
    expect(buildCreateInput(draft({ dueInput: "2026-09-20" }))).toEqual({ ok: false, error: "截止时间格式错误" });
    expect(buildCreateInput(draft({ userIds: [] }))).toEqual({ ok: false, error: "请至少选择一名学生" });
  });
});

describe("buildPatchInput", () => {
  const edit: EditDraft = { title: "新标题", instructions: " ", dueInput: "2026-09-21T08:00", settings: emptySettings("project"), originalRubric: null };
  it("leaves kind and payload out when settings are locked, even if the draft is invalid", () => {
    const r = buildPatchInput(edit, false);
    expect(r).toEqual({ ok: true, value: { title: "新标题", instructions: "", dueAt: "2026-09-21T08:00:00+08:00" } });
  });
  it("validates and sends settings when editable", () => {
    expect(buildPatchInput(edit, true)).toEqual({ ok: false, error: "请填写驱动问题" });
    const r = buildPatchInput({ ...edit, settings: { ...edit.settings, drivingQuestion: "问题" } }, true);
    expect(r.ok && r.value.kind).toBe("project");
  });
});

// Ruling: a title/due edit alone must never overwrite a homework's stored
// rubric with whatever the draft happened to be initialized as — `rubric`
// is sent only when it actually differs from what the server has.
// `originalRubric` is required at the type level (fix round 1): a caller
// cannot construct an `EditDraft` without it, unlike the earlier optional
// field, which the real caller (AssignmentDetailPage.tsx) omitted, so every
// writing PATCH silently sent `rubric`.
describe("buildPatchInput rubric (writing only)", () => {
  const loaded = rubricDraftFromPayload({ rubric: { scale: "letter", dimensions: [{ name: "内容", note: "" }], focus: "" } });
  const writingEdit = (rubric: RubricDraft, originalRubric: RubricDraft | null = loaded): EditDraft => ({
    title: "标题",
    instructions: "",
    dueInput: "2026-09-21T08:00",
    settings: { ...emptySettings("writing"), prompt: "p", targetWords: "800", rubric },
    originalRubric,
  });
  it("omits rubric from the patch when the draft is unchanged", () => {
    const r = buildPatchInput(writingEdit(loaded!), false);
    expect(r.ok).toBe(true);
    if (r.ok) expect("rubric" in r.value).toBe(false);
  });
  it("sends rubric when a dimension changed", () => {
    const changed: RubricDraft = { ...loaded!, dimensions: [{ name: "论证", note: "" }] };
    const r = buildPatchInput(writingEdit(changed), false);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.value.rubric).toEqual(buildRubric(changed));
  });
  // Fix round 1: the previous JSON.stringify-based comparison would have
  // read this as a change and sent a needless rubric PATCH.
  it("treats the same dimension content built in a different key order as unchanged", () => {
    const reordered: RubricDraft = { ...loaded!, dimensions: [{ note: loaded!.dimensions[0]!.note, name: loaded!.dimensions[0]!.name }] };
    const r = buildPatchInput(writingEdit(reordered), false);
    expect(r.ok).toBe(true);
    if (r.ok) expect("rubric" in r.value).toBe(false);
  });
  it("also validates the rubric when settings are locked, since it stays editable", () => {
    const invalid: RubricDraft = { ...loaded!, dimensions: [] };
    expect(buildPatchInput(writingEdit(invalid), false)).toEqual({ ok: false, error: "评分维度需有 1 到 6 项" });
  });
  // Without an original to compare against (a writing homework whose
  // payload carried no rubric — should not happen for a real one), the
  // safer default is to send it rather than guess "unchanged".
  it("sends rubric when no original is known to compare against", () => {
    const r = buildPatchInput(writingEdit(loaded!, null), false);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.value.rubric).toEqual(buildRubric(loaded!));
  });
});

describe("canEditSettings", () => {
  it("locks once any recipient has started, and not before — even when overdue", () => {
    expect(canEditSettings([])).toBe(true);
    expect(canEditSettings([recipient(), recipient({ status: "overdue", statusLabel: "已逾期" })])).toBe(true);
    expect(canEditSettings([recipient(), recipient({ atomId: "a1", startedAt: "2026-09-15T00:00:00Z", status: "in_progress" })])).toBe(false);
  });
});

describe("unassignedStudents", () => {
  it("keeps roster order and drops recipients", () => {
    const roster = [{ id: "u1" }, { id: "u2" }, { id: "u3" }] as RosterRow[];
    expect(unassignedStudents(roster, [recipient({ userId: "u2" })]).map((s) => s.id)).toEqual(["u1", "u3"]);
  });
});

describe("failText", () => {
  it("prefixes the verb unless the server already did", () => {
    expect(failText("发布", new ApiError("x", "请至少选择一名学生", 400))).toBe("发布失败：请至少选择一名学生");
    expect(failText("提取", new Error("提取失败：模型无响应"))).toBe("提取失败：模型无响应");
    expect(failText("归档", "boom")).toBe("归档失败：boom");
  });
});

describe("isArchiveSuccess", () => {
  it("treats only a 404 ApiError as already archived", () => {
    expect(isArchiveSuccess(new ApiError("not_found", "资源不存在", 404))).toBe(true);
    expect(isArchiveSuccess(new ApiError("forbidden", "无权限", 403))).toBe(false);
    expect(isArchiveSuccess(new Error("network"))).toBe(false);
  });
});

describe("pickClassId", () => {
  it("prefers an explicit choice, then the remembered class, each only if still hers", () => {
    expect(pickClassId(["a", "b"], "b", "a")).toBe("b");
    expect(pickClassId(["a", "b"], "gone", "b")).toBe("b");
    expect(pickClassId(["a", "b"], null, "gone")).toBe("a");
    expect(pickClassId([], "a", "a")).toBe("");
  });
});

describe("tierLabel / settingsSummary", () => {
  it("names tiers and summarises settings", () => {
    expect(tierLabel(null)).toBe("按学生当前水平");
    expect(tierLabel(5)).toBe("原文");
    expect(settingsSummary("writing", { prompt: "p", targetWords: 800, lang: "zh" })).toBe("目标字数 800 · 中文");
    expect(settingsSummary("reading", { source: "library", slug: "coral", tier: 3 }, "Coral reefs")).toBe("分级阅读库 · Coral reefs · 进阶");
    expect(settingsSummary("reading", { source: "text", text: "你好" })).toBe("正文 · 2 字");
  });
});

describe("filterArticles", () => {
  it("matches either title case-insensitively", () => {
    const list = [
      { slug: "a", title: "Coral Reefs", zhTitle: "珊瑚礁" },
      { slug: "b", title: "Solar Power", zhTitle: "太阳能" },
    ] as LibraryArticle[];
    expect(filterArticles(list, "coral").map((a) => a.slug)).toEqual(["a"]);
    expect(filterArticles(list, "太阳").map((a) => a.slug)).toEqual(["b"]);
    expect(filterArticles(list, " ").length).toBe(2);
  });
});

describe("buildReturnInput", () => {
  const now = Date.parse("2026-09-15T10:00:00+08:00");

  it("rejects a missing or malformed time", () => {
    expect(buildReturnInput("", "", now)).toEqual({ ok: false, error: "请填写新的截止时间" });
    expect(buildReturnInput("2026-09-18", "", now)).toEqual({ ok: false, error: "请填写新的截止时间" });
  });
  it("rejects a time that is not after now", () =>
    expect(buildReturnInput("2026-09-15T10:00", "", now)).toEqual({ ok: false, error: "新的截止时间需要晚于现在" }));
  it("rejects a note over 500 characters", () =>
    expect(buildReturnInput("2026-09-18T22:00", "字".repeat(501), now)).toEqual({ ok: false, error: "退回说明不超过 500 字" }));
  it("accepts exactly 500 characters", () =>
    expect(buildReturnInput("2026-09-18T22:00", "字".repeat(500), now).ok).toBe(true));
  it("reads the input as Beijing time, trims the note and drops an empty one", () => {
    expect(buildReturnInput("2026-09-18T22:00", "  请补充第二段的论据 ", now)).toEqual({
      ok: true,
      value: { dueAt: "2026-09-18T22:00:00+08:00", note: "请补充第二段的论据" },
    });
    expect(buildReturnInput("2026-09-18T22:00", "   ", now)).toEqual({ ok: true, value: { dueAt: "2026-09-18T22:00:00+08:00" } });
  });
});

// The text colours are pinned because their contrast was measured, not seen:
// a chip that reads 3:1 in dark mode still passes every render assertion.
describe("statusChipStyle", () => {
  it("pins the text colour per status", () => {
    const colors = Object.fromEntries(
      (["not_started", "in_progress", "done", "done_late", "overdue"] as const).map((s) => [s, statusChipStyle(s).color]),
    );
    expect(colors).toEqual({
      not_started: "color-mix(in srgb, var(--mk-muted) 40%, var(--mk-ink))",
      in_progress: "var(--mk-accent-700)",
      done: "color-mix(in srgb, var(--mk-success) 40%, var(--mk-ink))",
      done_late: "color-mix(in srgb, var(--mk-warning) 40%, var(--mk-ink))",
      overdue: "color-mix(in srgb, var(--mk-danger) 40%, var(--mk-ink))",
    });
  });

  it("keeps the 14% tint of the status hue as the background", () => {
    expect(statusChipStyle("in_progress").background).toBe("color-mix(in srgb, var(--mk-accent-500) 14%, var(--mk-surface))");
    expect(statusChipStyle("overdue").background).toBe("color-mix(in srgb, var(--mk-danger) 14%, var(--mk-surface))");
  });

  it("falls back to the muted tint for an unknown status", () => {
    expect(statusChipStyle("weird")).toEqual(statusChipStyle("not_started"));
    expect(statusChipStyle("toString")).toEqual(statusChipStyle("not_started"));
  });
});
