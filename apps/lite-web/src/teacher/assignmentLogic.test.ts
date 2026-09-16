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
  disciplineChips,
  disciplineOptions,
  draftOnClassChange,
  draftOnKindChange,
  emptySettings,
  extractedCountText,
  failText,
  fillTitleIfEmpty,
  filterArticles,
  filterByDisciplines,
  pageArticles,
  pickArticle,
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
  // Only the personalized branch reads recipientIds — these non-personalized
  // calls pass `[]` since the value is irrelevant to them.
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

// A title/due edit alone must never overwrite a homework's stored rubric
// with whatever the draft happened to be initialized as — `rubric` is sent
// only when it actually differs from what the server has. `originalRubric`
// is required at the type level: a caller cannot construct an `EditDraft`
// without it.
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
  // A key-order difference alone must not read as a rubric change.
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

describe("draftOnClassChange", () => {
  it("clears picks (stale rows belong to the old class) and keeps the class-wide filter", () => {
    const d = draft({
      ...emptySettings("reading"),
      classId: "c1",
      readingSource: "personalized",
      disciplines: ["science"],
      personalTier: 3,
      picks: [{ userId: "u1", name: "甲", slug: "coral", title: "Coral reefs", tier: null, suggestedTier: 2, reason: "兴趣相关：科学", swapped: false }],
    });
    const next = draftOnClassChange(d, "c2");
    expect(next.classId).toBe("c2");
    expect(next.picks).toBeNull();
    expect(next.disciplines).toEqual(["science"]);
    expect(next.personalTier).toBe(3);
  });
});

// The card the AI mode holds is what the next turn shows the model, so a draft
// that says 「种类：writing」 beside a chosen article shows it a card that cannot
// render. These three fields are the same ones the server's set_fields clears
// on the same transition.
describe("draftOnKindChange", () => {
  const reading = draft({ ...emptySettings("reading"), readingSource: "library", slug: "coral", tier: 3 });

  it("takes the material with it when leaving 阅读", () => {
    const next = draftOnKindChange(reading, "writing");
    expect(next.kind).toBe("writing");
    expect(next.slug).toBe("");
    expect(next.readingSource).toBe("library");
    expect(next.tier).toBeNull();
  });

  it("clears the material for 项目 too", () => {
    expect(draftOnKindChange(reading, "project").slug).toBe("");
  });

  it("keeps the material when the kind is still 阅读", () => {
    const next = draftOnKindChange(reading, "reading");
    expect(next.slug).toBe("coral");
    expect(next.tier).toBe(3);
  });

  it("leaves the other cells alone", () => {
    const next = draftOnKindChange(draft({ ...emptySettings("reading"), title: "气候作业" }), "writing");
    expect(next.title).toBe("气候作业");
  });
});

describe("tierLabel / settingsSummary", () => {
  it("names tiers and summarises settings", () => {
    expect(tierLabel(null)).toBe("按学生水平");
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
    // A file was already uploaded: clearing the text asks her to fill it
    // back in, not to upload a file she already has.
    expect(validateSettings({ ...d, text: "" })).toBe("请填写文章正文");
    expect(buildPayload({ ...d, readingSource: "text" }, [])).toEqual({ source: "text", text: "正文" });
  });
  // A stray body left over from another tab must not let the upload tab
  // publish without a file — she'd see 上传文件 on the tab but the saved
  // homework would actually be whatever text was typed into a different tab
  // first.
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
  // buildCreateInput has no stored payload to fall back to, so a brand new
  // personalized homework still waits for the preview.
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
  // recipientIds is a required, typed parameter: these tests exercise it
  // explicitly.
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
    expect(settingsSummary("reading", { source: "personalized" })).toBe("个性化阅读 · 按学生水平");
  });
  // A hand-edited or stale stored payload could carry any number as a pick's
  // tier — only 1..5 whole numbers survive the read, everything else reads
  // as her own level.
  it("range-checks a saved pick's tier", () => {
    const d = settingsFromAssignment("reading", {
      source: "personalized",
      picks: {
        u1: { slug: "coral", tier: 9 },
        u2: { slug: "coral", tier: 0 },
        u3: { slug: "coral", tier: 2.5 },
        u4: { slug: "coral", tier: 3 },
      },
    });
    expect(d.savedPicks).toEqual({
      u1: { slug: "coral", tier: null },
      u2: { slug: "coral", tier: null },
      u3: { slug: "coral", tier: null },
      u4: { slug: "coral", tier: 3 },
    });
  });
});

// Editing an unstarted personalized homework must not be blocked by the
// "wait for the preview" message when the preview never loaded (or failed)
// — there is already a stored payload to fall back to.
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

  // Switching a stored library/url/text homework to personalized and saving
  // before the preview loads (or after it fails) must still wait — it has
  // no saved picks to fall back to, unlike a homework that was ALREADY
  // personalized.
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

  it("a fresh preview row always starts with a null tier, following the chip", () => {
    const rows = mergePickRows([preview()], 3, {});
    expect(rows[0]).toMatchObject({ slug: "coral", tier: null, swapped: false, reason: "暂无兴趣数据，按难度推荐" });
    expect(pickTierText(rows[0] as PickRow, 3)).toBe("进阶");
    expect(pickTierText({ ...(rows[0] as PickRow), tier: null, suggestedTier: 2 }, null)).toBe("按学生水平（基础）");
  });
  // A null pick tier resolves through the class-wide 难度 chip first, the
  // same order the server's `pickedTier` uses; with neither set, the
  // estimate names her suggested tier rather than stating it as fact.
  it("a null pick tier shows the class-wide chip before naming her suggested tier as an estimate", () => {
    const row: PickRow = { userId: "u1", name: "Phoebe", slug: "coral", title: "珊瑚", tier: null, suggestedTier: 2, reason: "", swapped: false };
    expect(pickTierText(row, 4)).toBe("高阶");
    expect(pickTierText(row, null)).toBe("按学生水平（基础）");
  });

  // I1: the class-wide 难度 chip must drive where an unswapped row starts,
  // on every chip change — not just the one the preview happened to run at.
  it("(a) a create with chip 高阶 saves rows with a null tier and the top-level chip", () => {
    const rows = mergePickRows([preview({ userId: "u1" }), preview({ userId: "u2", slug: "nasa" })], 4, {});
    expect(rows.every((r) => r.tier === null)).toBe(true);
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const, picks: rows, personalTier: 4 };
    expect(buildPayload(d, ["u1", "u2"])).toEqual({
      source: "personalized",
      tier: 4,
      picks: { u1: { slug: "coral", tier: null }, u2: { slug: "nasa", tier: null } },
    });
  });
  it("(b) changing the chip from 高阶 to 按学生水平 keeps rows null and drops the top-level tier", () => {
    let rows = mergePickRows([preview({ userId: "u1" })], 4, {});
    rows = mergePickRows([preview({ userId: "u1" })], null, keptFromRows(rows));
    expect(rows[0]).toMatchObject({ tier: null });
    const d = { ...emptySettings("reading"), readingSource: "personalized" as const, picks: rows, personalTier: null };
    expect(buildPayload(d, ["u1"])).toEqual({ source: "personalized", picks: { u1: { slug: "coral", tier: null } } });
  });
  it("(c) a swapped row keeps its chosen tier through a chip change", () => {
    let rows = mergePickRows([preview({ userId: "u1" })], 4, {});
    rows = swapPick(rows, "u1", { slug: "nasa", tier: 5 }, articles);
    const again = mergePickRows([preview({ userId: "u1" })], null, keptFromRows(rows));
    expect(again[0]).toMatchObject({ slug: "nasa", tier: 5, swapped: true });
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
  // Picks saved with tier: null keep following the class-wide chip even
  // after it changes, since the article recommendation is unchanged — this
  // must not read as 已更换.
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
  // A recipient's tier of null means her own level, shown as 「按学生水平」.
  it("describes a recipient's article", () => {
    expect(recipientReadingText({ slug: "coral", title: "珊瑚", tier: 3, state: "started" })).toBe("珊瑚 · 进阶");
    expect(recipientReadingText({ slug: "coral", title: "", tier: null, state: "picked" })).toBe("coral · 按学生水平");
    expect(recipientReadingText({ slug: "", title: "", tier: null, state: "pending" })).toBe("待推荐");
    expect(recipientReadingText(null)).toBe("—");
  });
});

describe("library picker selection", () => {
  const art = (slug: string, tagIds: string[] = []): LibraryArticle => ({
    slug,
    title: slug,
    zhTitle: slug,
    reason: "",
    field: "science",
    tags: tagIds.map((id) => ({ id, zh: id, field: "science" })),
    coverUrl: "",
    levels: [],
    finished: false,
  });

  // The recommended row and the full grid are separate lists of separate
  // objects; selection must follow the slug so both show the same state.
  it("an article picked in the recommended row is the selected one in the grid", () => {
    const recommended = art("coral", ["biology"]);
    const inGrid = art("coral", ["biology"]);
    const next = pickArticle({ slug: "", tier: null }, recommended.slug);
    expect(next).toEqual({ slug: "coral", tier: null });
    expect(pickArticle(next, inGrid.slug)).toBe(next);
  });
  it("clicking the selected article keeps it and its tier", () => {
    const current = { slug: "coral", tier: 3 };
    expect(pickArticle(current, "coral")).toBe(current);
  });
  it("switching article resets the tier", () => {
    expect(pickArticle({ slug: "coral", tier: 3 }, "nasa")).toEqual({ slug: "nasa", tier: null });
  });
  it("filters by any of the chosen disciplines, and not at all when none are chosen", () => {
    const list = [art("a", ["biology"]), art("b", ["astronomy"]), art("c", ["biology", "physics"]), art("d")];
    expect(filterByDisciplines(list, []).map((a) => a.slug)).toEqual(["a", "b", "c", "d"]);
    expect(filterByDisciplines(list, ["physics", "astronomy"]).map((a) => a.slug)).toEqual(["b", "c"]);
  });
  it("puts a selected article past the cut first, keeping the page length", () => {
    const list = ["a", "b", "c", "d", "e"].map((s) => art(s));
    expect(pageArticles(list, 3, "").map((a) => a.slug)).toEqual(["a", "b", "c"]);
    expect(pageArticles(list, 3, "e").map((a) => a.slug)).toEqual(["e", "a", "b"]);
    expect(pageArticles(list, 3, "gone").map((a) => a.slug)).toEqual(["a", "b", "c"]);
    expect(pageArticles(list, 9, "e")).toBe(list);
  });
  it("leaves a selected article inside the page where it is", () => {
    const list = ["a", "b", "c", "d", "e"].map((s) => art(s));
    expect(pageArticles(list, 3, "b").map((a) => a.slug)).toEqual(["a", "b", "c"]);
  });
  it("orders discipline chips by article count, ties in library order", () => {
    const list = [art("1", ["hist"]), art("2", ["bio", "phys"]), art("3", ["phys"]), art("4", ["phys", "bio"]), art("5", ["art"])];
    const all = disciplineChips(list, { limit: 9, expanded: false, chosen: [] });
    expect(all.shown.map((t) => t.id)).toEqual(["phys", "bio", "hist", "art"]);
    expect(all.hidden).toBe(0);
  });
  it("collapses the chips to the most common, keeping a chosen one visible", () => {
    const list = [art("1", ["hist"]), art("2", ["bio", "phys"]), art("3", ["phys"]), art("4", ["phys", "bio"]), art("5", ["art"])];
    const collapsed = disciplineChips(list, { limit: 2, expanded: false, chosen: [] });
    expect(collapsed.shown.map((t) => t.id)).toEqual(["phys", "bio"]);
    expect(collapsed.hidden).toBe(2);
    const withChosen = disciplineChips(list, { limit: 2, expanded: false, chosen: ["art"] });
    expect(withChosen.shown.map((t) => t.id)).toEqual(["phys", "bio", "art"]);
    expect(withChosen.hidden).toBe(1);
    expect(disciplineChips(list, { limit: 2, expanded: true, chosen: [] }).shown).toHaveLength(4);
  });
});
