import { describe, expect, it } from "vitest";
import { ApiError } from "../api/client";
import type { PreviewRow, RecipientDTO } from "../api/assignments";
import type { LibraryArticle } from "../api/library";
import type { RosterRow } from "../api/teacher";
import {
  assignmentFileName,
  buildCreateInput,
  buildPatchInput,
  buildPayload,
  buildReturnInput,
  canEditSettings,
  disciplineOptions,
  emptySettings,
  extractedCountText,
  failText,
  fillTitleIfEmpty,
  filterArticles,
  isArchiveSuccess,
  keptFromRows,
  keptFromSaved,
  mergePickRows,
  parseTargetWords,
  pickClassId,
  pickTierText,
  readExtractResult,
  recipientReadingText,
  settingsFromAssignment,
  settingsSummary,
  statusChipStyle,
  swapPick,
  tabAfterAssignmentChange,
  tierLabel,
  unassignedStudents,
  validateSettings,
  visiblePickRows,
  type AssignmentDraft,
  type EditDraft,
  type PickRow,
  type SettingsDraft,
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
    reading: null,
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
  // recipientIds is a required parameter (fix round 2) but only the
  // personalized branch reads it — these non-personalized calls pass `[]`
  // since the value is irrelevant to them.
  it("omits tier when the student's current level is used", () => {
    const p = buildPayload({ ...emptySettings("reading"), slug: "coral", tier: null }, []);
    expect(p).toEqual({ source: "library", slug: "coral" });
    expect("tier" in p).toBe(false);
    expect(buildPayload({ ...emptySettings("reading"), slug: "coral", tier: 3 }, [])).toEqual({ source: "library", slug: "coral", tier: 3 });
  });
  it("sends only the chosen source's field", () => {
    expect(buildPayload({ ...emptySettings("reading"), readingSource: "url", slug: "left", url: "https://a.org" }, [])).toEqual({
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
    const untouched = buildPayload({ ...emptySettings("writing"), prompt: "题", targetWords: "800" }, []);
    expect("rubric" in untouched).toBe(false);
    const customized: RubricDraft = { scale: "letter", max: "", dimensions: [{ name: "论证", note: "看证据" }], focus: "" };
    const withRubricSet = buildPayload({ ...emptySettings("writing"), prompt: "题", targetWords: "800", rubric: customized }, []);
    expect("rubric" in withRubricSet).toBe(false);
  });
});

describe("settingsFromAssignment", () => {
  it("round-trips a writing payload through the draft", () => {
    const payload = { prompt: "题目", targetWords: 800, lang: "en" };
    expect(buildPayload(settingsFromAssignment("writing", payload), [])).toEqual(payload);
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
    const r = buildPatchInput(edit, false, []);
    expect(r).toEqual({ ok: true, value: { title: "新标题", instructions: "", dueAt: "2026-09-21T08:00:00+08:00" } });
  });
  it("validates and sends settings when editable", () => {
    expect(buildPatchInput(edit, true, [])).toEqual({ ok: false, error: "请填写驱动问题" });
    const r = buildPatchInput({ ...edit, settings: { ...edit.settings, drivingQuestion: "问题" } }, true, []);
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
    const r = buildPatchInput(writingEdit(loaded!), false, []);
    expect(r.ok).toBe(true);
    if (r.ok) expect("rubric" in r.value).toBe(false);
  });
  it("sends rubric when a dimension changed", () => {
    const changed: RubricDraft = { ...loaded!, dimensions: [{ name: "论证", note: "" }] };
    const r = buildPatchInput(writingEdit(changed), false, []);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.value.rubric).toEqual(buildRubric(changed));
  });
  // Fix round 1: the previous JSON.stringify-based comparison would have
  // read this as a change and sent a needless rubric PATCH.
  it("treats the same dimension content built in a different key order as unchanged", () => {
    const reordered: RubricDraft = { ...loaded!, dimensions: [{ note: loaded!.dimensions[0]!.note, name: loaded!.dimensions[0]!.name }] };
    const r = buildPatchInput(writingEdit(reordered), false, []);
    expect(r.ok).toBe(true);
    if (r.ok) expect("rubric" in r.value).toBe(false);
  });
  it("also validates the rubric when settings are locked, since it stays editable", () => {
    const invalid: RubricDraft = { ...loaded!, dimensions: [] };
    expect(buildPatchInput(writingEdit(invalid), false, [])).toEqual({ ok: false, error: "评分维度需有 1 到 6 项" });
  });
  // Without an original to compare against (a writing homework whose
  // payload carried no rubric — should not happen for a real one), the
  // safer default is to send it rather than guess "unchanged".
  it("sends rubric when no original is known to compare against", () => {
    const r = buildPatchInput(writingEdit(loaded!, null), false, []);
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

describe("tabAfterAssignmentChange", () => {
  // The reset effect this backs runs on every mount too, not only a real
  // switch — the first run (prevId === nextId, since the ref starts equal
  // to the current id) must leave whatever tab `?tab=grading` seeded alone,
  // or a fresh page load lands her on 学生 no matter what the URL asked for.
  it("keeps the current tab when the id has not actually changed", () => {
    expect(tabAfterAssignmentChange("a1", "a1", "grading")).toBe("grading");
    expect(tabAfterAssignmentChange("a1", "a1", "students")).toBe("students");
  });
  it("resets to 学生 on a genuine switch to a different assignment", () => {
    expect(tabAfterAssignmentChange("a1", "a2", "grading")).toBe("students");
    expect(tabAfterAssignmentChange("a1", "a2", "students")).toBe("students");
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

function article(slug: string, zhTitle: string, tags: string[] = []): LibraryArticle {
  return {
    slug,
    title: slug,
    zhTitle,
    reason: "",
    field: "science",
    tags: tags.map((id) => ({ id, zh: `学科${id}`, field: "science" })),
    coverUrl: "",
    levels: [],
    finished: false,
  };
}

function preview(over: Partial<PreviewRow> = {}): PreviewRow {
  return { userId: "u1", name: "Phoebe", slug: "coral", title: "珊瑚", tier: 2, suggestedTier: 2, reason: "暂无兴趣数据，按难度推荐", ...over };
}

describe("readExtractResult", () => {
  it("keeps the trimmed text, the file name and a title", () => {
    expect(readExtractResult({ title: "雨水花园", text: "  正文  " }, "rain.pdf")).toEqual({ ok: true, text: "正文", fileName: "rain.pdf", title: "雨水花园" });
  });
  it("uses the file name without its extension when the document has no title", () => {
    const r = readExtractResult({ title: "", text: "x" }, "校园积水调查.docx");
    expect(r.ok && r.title).toBe("校园积水调查");
  });
  it("refuses more than 50000 characters, counting characters not bytes", () => {
    expect(readExtractResult({ title: "", text: "雨".repeat(50000) }, "a.txt").ok).toBe(true);
    expect(readExtractResult({ title: "", text: "雨".repeat(50001) }, "a.txt")).toEqual({ ok: false, error: "提取失败：正文超过 50000 字" });
  });
  it("caps the file name at 200 characters", () => {
    const r = readExtractResult({ title: "t", text: "x" }, `${"名".repeat(250)}.pdf`);
    expect(r.ok && [...r.fileName].length).toBe(200);
  });
  it("counts the extracted characters", () => {
    expect(extractedCountText(" 你好 ")).toBe("已提取 2 字");
  });
  it("fills the title only when it is empty", () => {
    expect(fillTitleIfEmpty("  ", "雨水花园")).toBe("雨水花园");
    expect(fillTitleIfEmpty("第三周阅读", "雨水花园")).toBe("第三周阅读");
  });
  it("refuses when nothing came back at all", () => {
    expect(readExtractResult({ title: "", text: "   " }, "scan.pdf")).toEqual({ ok: false, error: "提取失败：文件中没有读到文字" });
  });
});

describe("file and personalized settings", () => {
  it("builds a text payload with the file name from the upload tab", () => {
    const d = { ...emptySettings("reading"), readingSource: "file" as const, text: " 正文 ", fileName: "rain.pdf" };
    expect(validateSettings(d)).toBeNull();
    expect(buildPayload(d, [])).toEqual({ source: "text", text: "正文", fileName: "rain.pdf" });
    expect(validateSettings({ ...d, text: "" })).toBe("请上传文件");
    expect(buildPayload({ ...d, readingSource: "text" }, [])).toEqual({ source: "text", text: "正文" });
  });
  // Controller ruling 1: a stray body from another tab must not let the
  // upload tab publish without a file — she'd see "上传文件" in the tab but
  // the saved homework would actually be whatever text happened to be typed
  // into a different tab first.
  it("fails without a file even when a text body carried over from another tab", () => {
    const d = { ...emptySettings("reading"), readingSource: "file" as const, text: "正文", fileName: "" };
    expect(validateSettings(d)).toBe("请上传文件");
  });
  it("reopens a stored text with a file name on the upload tab", () => {
    const d = settingsFromAssignment("reading", { source: "text", text: "正文", fileName: "rain.pdf" });
    expect(d.readingSource).toBe("file");
    expect(settingsSummary("reading", { source: "text", text: "正文", fileName: "rain.pdf" })).toBe("上传文件 · rain.pdf · 2 字");
    expect(assignmentFileName({ kind: "reading", payload: { source: "text", text: "x", fileName: "rain.pdf" } })).toBe("rain.pdf");
    expect(assignmentFileName({ kind: "reading", payload: { source: "text", text: "x" } })).toBeNull();
  });
  it("waits for the preview before a personalized homework can be saved", () => {
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const };
    expect(validateSettings(d)).toBe("请等待推荐列表加载完成");
    expect(validateSettings({ ...d, picks: [] })).toBeNull();
  });
  // Controller ruling 2, "new homework" leg: buildCreateInput has no stored
  // payload to fall back to, so it still waits for the preview.
  it("still blocks creating a brand new personalized homework while the preview is loading", () => {
    const d: AssignmentDraft = {
      ...emptySettings("reading"),
      readingSource: "personalized",
      classId: "c1",
      title: "个性化阅读",
      instructions: "",
      dueInput: "2026-09-20T22:00",
      userIds: ["u1"],
    };
    expect(buildCreateInput(d)).toEqual({ ok: false, error: "请等待推荐列表加载完成" });
  });
  it("sends picks for recipients only, the filter and the tier when set", () => {
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2", slug: "nasa", title: "NASA" })], null, {});
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const, picks: rows, disciplines: ["astronomy"], personalTier: 4 };
    expect(buildPayload(d, ["u2"])).toEqual({
      source: "personalized",
      disciplines: ["astronomy"],
      tier: 4,
      picks: { u2: { slug: "nasa", tier: null } },
    });
    expect(buildPayload({ ...d, disciplines: [], personalTier: null }, ["u1", "u2"])).toEqual({
      source: "personalized",
      picks: { u1: { slug: "coral", tier: null }, u2: { slug: "nasa", tier: null } },
    });
  });
  // Fix round 2: `recipientIds` is now a required, typed parameter (not an
  // optional one with a runtime throw — the one production caller of
  // buildPatchInput did not know to satisfy an optional-with-throw contract
  // and would have thrown uncaught on every settings-editable personalized
  // save). These tests exercise it explicitly rather than asserting a throw.
  it("keeps only the named recipients' picks — an empty recipientIds sends none", () => {
    const rows = mergePickRows([preview({ userId: "u1" })], null, {});
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const, picks: rows };
    expect(buildPayload(d, [])).toEqual({ source: "personalized", picks: {} });
  });
  it("sends every pick when recipientIds names them all", () => {
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2", slug: "nasa" })], null, {});
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const, picks: rows };
    expect(buildPayload(d, ["u1", "u2"])).toEqual({
      source: "personalized",
      picks: { u1: { slug: "coral", tier: null }, u2: { slug: "nasa", tier: null } },
    });
  });
  it("reads a stored personalized payload back", () => {
    const d = settingsFromAssignment("reading", {
      source: "personalized",
      disciplines: ["astronomy", 3],
      tier: 4,
      picks: { u1: { slug: "coral", tier: null }, u2: { slug: 7 } },
    });
    expect(d).toMatchObject({ readingSource: "personalized", disciplines: ["astronomy"], personalTier: 4, picks: null });
    expect(d.savedPicks).toEqual({ u1: { slug: "coral", tier: null } });
    expect(settingsSummary("reading", { source: "personalized", tier: 4, disciplines: ["a", "b"] })).toBe("个性化阅读 · 高阶 · 学科筛选 2 项");
    // Promoted minor: personalTierLabel, not tierLabel — null here means
    // each student's own level (按学生水平), not the library source's
    // fixed class-wide default (按学生当前水平).
    expect(settingsSummary("reading", { source: "personalized" })).toBe("个性化阅读 · 按学生水平");
  });
});

// Controller ruling 2: editing an unstarted personalized homework must not
// be blocked by the "wait for the preview" message when the preview never
// loaded (or failed) — there is already a stored payload to fall back to.
describe("buildPatchInput on a personalized reading whose preview has not loaded", () => {
  const editOf = (settings: ReturnType<typeof settingsFromAssignment>): EditDraft => ({
    title: "标题",
    instructions: "",
    dueInput: "2026-09-21T08:00",
    settings,
    originalRubric: null,
  });

  it("saves using the saved picks when the preview never loaded", () => {
    const settings = settingsFromAssignment("reading", { source: "personalized", picks: { u1: { slug: "coral", tier: null } } });
    expect(settings.picks).toBeNull(); // the preview call never ran
    const r = buildPatchInput(editOf(settings), true, ["u1"]);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.value.payload).toEqual({ source: "personalized", picks: { u1: { slug: "coral", tier: null } } });
  });

  it("saves the merged picks once the preview loaded and she swapped one", () => {
    const settings = settingsFromAssignment("reading", { source: "personalized", picks: { u1: { slug: "coral", tier: null } } });
    const articles = [article("coral", "珊瑚"), article("nasa", "NASA")];
    const kept = keptFromSaved(settings.savedPicks, articles);
    const merged = mergePickRows([preview({ userId: "u1" })], null, kept);
    const swapped = swapPick(merged, "u1", { slug: "nasa", tier: 5 }, articles);
    const r = buildPatchInput(editOf({ ...settings, picks: swapped }), true, ["u1"]);
    expect(r.ok).toBe(true);
    if (r.ok) expect(r.value.payload).toEqual({ source: "personalized", picks: { u1: { slug: "nasa", tier: 5 } } });
  });

  // Fix round 1 (Important finding 1): the earlier skip only checked the
  // CURRENT readingSource, so switching a stored library/url/text homework
  // to personalized and saving before the preview loads (or after it fails)
  // sent `{source:"personalized", picks:{}}` — every student then got an
  // unseen automatic recommendation. Only a homework that was ALREADY
  // personalized has saved picks to fall back to.
  it("still waits for the preview when a stored library homework is switched to personalized", () => {
    const stored = settingsFromAssignment("reading", { source: "library", slug: "coral" });
    expect(stored.storedPersonalized).toBe(false);
    const switched: SettingsDraft = { ...stored, readingSource: "personalized", picks: null };
    const r = buildPatchInput(editOf(switched), true, ["u1"]);
    expect(r).toEqual({ ok: false, error: "请等待推荐列表加载完成" });
  });
});

describe("pick rows", () => {
  const articles = [article("coral", "珊瑚", ["biology"]), article("nasa", "NASA", ["astronomy", "biology"])];

  it("takes the preview and the chosen tier", () => {
    const rows = mergePickRows([preview()], 3, {});
    expect(rows[0]).toMatchObject({ slug: "coral", tier: 3, swapped: false, reason: "暂无兴趣数据，按难度推荐" });
    expect(pickTierText(rows[0] as PickRow, 3)).toBe("进阶");
    expect(pickTierText({ ...(rows[0] as PickRow), tier: null, suggestedTier: 2 }, null)).toBe("基础");
  });
  // Fix round 1 (Important finding 2): a null pick tier resolves through the
  // class-wide 难度 chip first, the same order the server's `pickedTier`
  // uses — showing the suggested tier instead (the old `row.tier ??
  // row.suggestedTier`) was wrong whenever a chip was set.
  it("a null pick tier shows the class-wide chip before falling back to her suggested tier", () => {
    const row: PickRow = { userId: "u1", name: "Phoebe", slug: "coral", title: "珊瑚", tier: null, suggestedTier: 2, reason: "", swapped: false };
    expect(pickTierText(row, 4)).toBe("高阶");
    expect(pickTierText(row, null)).toBe("基础");
  });
  it("a swap is kept when the preview is run again", () => {
    let rows = mergePickRows([preview()], null, {});
    rows = swapPick(rows, "u1", { slug: "nasa", tier: 5 }, articles);
    expect(rows[0]).toMatchObject({ slug: "nasa", title: "NASA", tier: 5, reason: "已更换", swapped: true });
    const again = mergePickRows([preview({ slug: "coral" }), preview({ userId: "u9" })], null, keptFromRows(rows));
    expect(again[0]).toMatchObject({ slug: "nasa", tier: 5, swapped: true });
    expect(again[1]).toMatchObject({ userId: "u9", swapped: false });
  });
  it("a saved pick that matches the new preview is not marked as swapped", () => {
    const kept = keptFromSaved({ u1: { slug: "coral", tier: null }, u2: { slug: "nasa", tier: 2 } }, articles);
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2" })], null, kept);
    expect(rows[0]).toMatchObject({ slug: "coral", swapped: false });
    expect(rows[1]).toMatchObject({ slug: "nasa", title: "NASA", tier: 2, swapped: true });
  });
  // Fix round 1 (Important finding 3, plan-mandated): picks saved with
  // tier: null, then the teacher raises the class-wide 难度 chip — the
  // article recommendation is unchanged, so this must not read as 已更换.
  it("a saved pick with the same article as the fresh preview is not a swap even if only its tier differs", () => {
    const kept = keptFromSaved({ u1: { slug: "coral", tier: null } }, articles);
    const rows = mergePickRows([preview({ userId: "u1", slug: "coral" })], 4, kept);
    expect(rows[0]).toMatchObject({ slug: "coral", tier: null, swapped: false, reason: "暂无兴趣数据，按难度推荐" });
  });
  it("a saved pick for a different article is still shown as a swap", () => {
    const kept = keptFromSaved({ u1: { slug: "nasa", tier: null } }, articles);
    const rows = mergePickRows([preview({ userId: "u1", slug: "coral" })], 4, kept);
    expect(rows[0]).toMatchObject({ slug: "nasa", title: "NASA", swapped: true, reason: "已更换" });
  });
  it("a session swap stays marked as a swap even after the class-wide tier changes", () => {
    let rows = mergePickRows([preview({ userId: "u1" })], null, {});
    rows = swapPick(rows, "u1", { slug: "nasa", tier: 5 }, articles);
    const again = mergePickRows([preview({ userId: "u1" })], 4, keptFromRows(rows));
    expect(again[0]).toMatchObject({ slug: "nasa", tier: 5, swapped: true, reason: "已更换" });
  });
  it("a student who left the class is dropped", () => {
    const rows = mergePickRows([preview({ userId: "u1" })], null, { gone: { slug: "nasa", title: "NASA", tier: null, origin: "saved" } });
    expect(rows.map((r) => r.userId)).toEqual(["u1"]);
  });
  it("shows only checked recipients", () => {
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2" })], null, {});
    expect(visiblePickRows(rows, ["u2"]).map((r) => r.userId)).toEqual(["u2"]);
  });
  it("lists each discipline tag once, in library order", () => {
    expect(disciplineOptions(articles).map((t) => t.id)).toEqual(["biology", "astronomy"]);
  });
  // Controller ruling 3: a detail/preview tier of null means her own level,
  // shown as 「按学生水平」 — distinct from tierLabel(null)'s
  // 「按学生当前水平」, which is a class-wide setting's own summary text.
  it("describes a recipient's article", () => {
    expect(recipientReadingText({ slug: "coral", title: "珊瑚", tier: 3, state: "started" })).toBe("珊瑚 · 进阶");
    expect(recipientReadingText({ slug: "coral", title: "", tier: null, state: "picked" })).toBe("coral · 按学生水平");
    expect(recipientReadingText({ slug: "", title: "", tier: null, state: "pending" })).toBe("待推荐");
    expect(recipientReadingText(null)).toBe("—");
  });
});
