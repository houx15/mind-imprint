import { ReportVisualSummary } from "../reports/ReportVisualSummary";
import { ProjectProgressVisual } from "../projects/ProjectProgressVisual";
import { StudioHeading } from "./StudioArtwork";
import { useEffect, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "@/api";
import type { ReportStat, AtomKind } from "@lite/api/reports";
import { getItem, getItemVersion, type ItemDetail } from "../api/teacher";
import type { WritingVersion, WritingVersionSummary } from "../api/writings";
import { queueWritingGrading, type GradingSummary } from "../api/gradings";
import { failText, tintedChipStyle } from "./assignmentLogic";
import { GRADING_STATUS_LABEL, gradingRowStatus, gradingStatusHue } from "./gradingLogic";
import { versionLine } from "../writings/finishedWriting";
import { formatMinutes, itemStatusLabel, kindLabel, langLabel, safeHttpUrl } from "./format";
import { displayStat } from "../reports/statLabels";
import { OutputRecord } from "./OutputRecord";
import { lensFieldLabels } from "./outputSummary";
import { useAlive } from "../shared/useAlive";
import { TeacherPage } from "./TeacherPage";

/**
 * ItemPage — one item (reading/writing/project), from the teacher's side.
 * Outputs and moments only — spec DEC-1, restated on screen by the line
 * under the header: the chat with 印记 never renders here, on this page or
 * any other teacher-facing one.
 *
 * ## The report's two-phase protocol
 * `getItem`'s `report` carries `prosePending`: the deterministic half
 * (stats, 学生原话/moments, 收获/keep) is generated and stored synchronously,
 * but 金句 + 这次的收获 (when `keep.source === "coach"`) come from a flagship
 * model call the SERVER runs on a FOLLOW-UP request (`ensureAtomReport`,
 * mirrored by `ReportPanel.tsx` on the student side). So this page shows
 * what is already there, plus a muted line, then re-fetches the item ONCE
 * — not a poll loop, since each request that lands while still pending pays
 * for another flagship call server-side. `retriedProse` flips once that one
 * follow-up has landed, which is what turns 报告文字生成中 into
 * 报告文字暂未生成 rather than leaving a promise the page can't keep on
 * screen forever.
 *
 * The retry effect is guarded by a plain `useRef` latch, deliberately with
 * NO cleanup that undoes it (see `useAlive`'s own doc comment on the exact
 * StrictMode trap that combination causes): under StrictMode's synthetic
 * mount→cleanup→remount, the first pass's `setTimeout` keeps running in the
 * background rather than being cancelled, the second pass sees the latch
 * already set and does not schedule a second one, and by the time the timer
 * actually fires (~3s later, long after the synthetic remount has settled)
 * `alive.current` is `true` again — so the one real retry always lands.
 *
 * 🚨 `alive.current` alone is not enough to gate that timer's result: this
 * page has no `key`, so an item→item route change (or pressing 重试 inside
 * the 3s window) keeps `ItemPage` mounted with the SAME component instance
 * while `classId`/`userId`/`atomId` move on. A timer scheduled for the OLD
 * item is still "alive" by every measure `alive.current` checks — the
 * component never unmounted — so without a second guard its response would
 * land on top of the NEW item's detail. `loadGen` is that second guard: the
 * main load effect bumps it on every load (a prop change or a 重试 click),
 * the timer captures the generation it was scheduled under, and it drops
 * its result — never touching `detail` or `retriedProse` — the moment that
 * number has moved on.
 */
export function ItemPage({
  classId,
  userId,
  atomId,
  onBack,
  onOpenGrading,
}: {
  classId: string;
  userId: string;
  atomId: string;
  onBack: () => void;
  onOpenGrading: (gradingId: string) => void;
}) {
  const [detail, setDetail] = useState<ItemDetail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [nonce, setNonce] = useState(0);
  const [retriedProse, setRetriedProse] = useState(false);
  const [proseError, setProseError] = useState<string | null>(null);
  const proseOnce = useRef(false);
  // 每次真正的加载（换项目，或按了 重试）都往前走一格。定时器排上号的时候
  // 记一份快照；等它真的响的时候，这个数如果已经变了，说明她已经离开了那个
  // 项目（或者按过 重试），这份迟到的回答就只能被认成过期，绝不能落到
  // `detail`/`retriedProse` 上盖住她现在正看着的那个项目。
  const loadGen = useRef(0);
  const alive = useAlive();

  useEffect(() => {
    let cancelled = false;
    // 每次真正的加载都往前走一格——见上面 `loadGen` 的注释。
    loadGen.current += 1;
    proseOnce.current = false;
    setDetail(null);
    setError(null);
    setRetriedProse(false);
    setProseError(null);
    getItem(classId, userId, atomId)
      .then((d) => {
        if (!cancelled) setDetail(d);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, userId, atomId, nonce]);

  useEffect(() => {
    if (!detail?.report?.prosePending) return;
    if (proseOnce.current) return;
    proseOnce.current = true;
    const myGen = loadGen.current;
    // 🚨 不在 cleanup 里 clearTimeout——见文件头注释，那正是会把这次重试在
    // StrictMode 下吞掉的组合。
    setTimeout(() => {
      // 只有这一次重取允许服务端生成报告文字；首次加载不带，页面不等模型。
      getItem(classId, userId, atomId, { prose: true })
        .then((d) => {
          if (!alive.current) return;
          if (loadGen.current !== myGen) return; // 她已经换了项目/按过重试，这份回答过期了。
          setDetail(d);
          setRetriedProse(true);
        })
        .catch((e: unknown) => {
          if (!alive.current) return;
          if (loadGen.current !== myGen) return;
          // 一次真的请求失败，不是「还没生成好」——两句话不能混着说。
          setProseError(e instanceof ApiError ? e.message : String(e));
        });
    }, 3000);
  }, [detail, classId, userId, atomId, alive]);

  return (
    <TeacherPage>
      <button
        type="button"
        onClick={onBack}
        className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      >
        <Icon icon={ArrowLeft} size={15} />
        返回
      </button>

      {error ? (
        <div className="mt-4 text-mk-small font-semibold text-mk-danger">
          加载失败：{error}{" "}
          <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
            重试
          </button>
        </div>
      ) : detail === null ? (
        <div className="mt-4 text-mk-body text-mk-muted">加载中…</div>
      ) : (
        <ItemBody
          classId={classId}
          userId={userId}
          atomId={atomId}
          detail={detail}
          retriedProse={retriedProse}
          proseError={proseError}
          onOpenGrading={onOpenGrading}
        />
      )}
    </TeacherPage>
  );
}

/** 报告的「金句 · 这次的收获」那句提示要不要显示、显示哪句——一个纯函数，
 *  和渲染分开，方便直接测。`null` = 不显示这一行。 */
export function prosePendingLabel(pending: boolean, retried: boolean): string | null {
  if (!pending) return null;
  return retried ? "报告文字暂未生成" : "报告文字生成中";
}

export interface CommentPointLine {
  kind: "good" | "issue" | "note";
  text: string;
  action: string | null;
  quote: string | null;
}

/** 印记's comment points in both stored shapes: an old string, or the
 *  {kind, text, action, quote} object writing_comment.go writes today
 *  (spec B5's bug — the page used to render only string points, so a real
 *  object point's quote/action/text never showed at all). */
export function commentPointLines(points: unknown[]): CommentPointLine[] {
  const out: CommentPointLine[] = [];
  for (const p of points) {
    if (typeof p === "string") {
      if (p.trim()) out.push({ kind: "note", text: p, action: null, quote: null });
      continue;
    }
    if (!p || typeof p !== "object") continue;
    const r = p as Record<string, unknown>;
    const text = typeof r.text === "string" ? r.text.trim() : "";
    if (!text) continue;
    out.push({
      kind: r.kind === "good" ? "good" : "issue",
      text,
      action: typeof r.action === "string" && r.action.trim() ? r.action : null,
      quote: typeof r.quote === "string" && r.quote.trim() ? r.quote : null,
    });
  }
  return out;
}

/** Whether the item page shows no 批改 control ("none" — nothing submitted
 *  yet), an "AI 批改" button ("grade" — submitted, no grading started), or a
 *  link that opens the existing one ("open" — never a second control that
 *  could start a competing/overwriting grading). */
export function itemGradingAction(versionCount: number, grading: GradingSummary | null): "none" | "grade" | "open" {
  if (grading) return "open";
  return versionCount > 0 ? "grade" : "none";
}

/** 阅读区的「收获」要不要显示。报告里的收获就是学生自己写的这一句时
 *  （`keep.source === "student"` 且文字相同），上面报告区已经显示过，这里不再重复。 */
export function showReadingTakeaway(
  takeaway: string | null | undefined,
  keep: { text: string; source: string } | null | undefined,
): boolean {
  if (!takeaway) return false;
  if (keep && keep.source === "student" && keep.text.trim() === takeaway.trim()) return false;
  return true;
}

/** What 写作原文 shows the teacher. Once she has submitted, the latest
 *  version: her draft may hold unsubmitted edits (she is revising, or a
 *  revision was cut off by the deadline), which are not the homework. The
 *  draft is shown only when nothing was ever submitted. */
export type TeacherManuscript =
  | { kind: "version"; version: WritingVersionSummary; revising: boolean }
  | { kind: "draft"; draft: string };

export function teacherManuscript(writing: {
  versions: readonly WritingVersionSummary[];
  revising: boolean;
  draft: string | null;
}): TeacherManuscript {
  const latest = writing.versions[0];
  if (latest) return { kind: "version", version: latest, revising: writing.revising };
  return { kind: "draft", draft: writing.draft ?? "" };
}

function shortDate(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日`;
}

function ItemBody({
  classId,
  userId,
  atomId,
  detail,
  retriedProse,
  proseError,
  onOpenGrading,
}: {
  classId: string;
  userId: string;
  atomId: string;
  detail: ItemDetail;
  retriedProse: boolean;
  /** I5: a real request error re-fetching the prose — never rendered as the
   *  「处理中」pending label, which would misreport a failure as a wait. */
  proseError: string | null;
  onOpenGrading: (gradingId: string) => void;
}) {
  const { item } = detail;
  const [documentView, setDocumentView] = useState<{ id: string; original: boolean } | null>(null);
  const showOriginal = documentView?.id === item.atomId && documentView.original;
  return (
    <>
      <StudioHeading label={`${kindLabel(item.kind)} · 学习成果`} title={item.title} kind={item.kind === "reading" ? "keepsake" : item.kind} />
      <p className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-mk-small text-mk-muted">
        <span>{itemStatusLabel(item.kind, item.status)}</span>
        <span>时长 {formatMinutes(item.minutes)}</span>
        <span>对话 {item.turns} 轮</span>
        {/* 没完成的项目没有完成日期：不留一个没有标签的「—」。 */}
        {item.finishedAt ? <span>完成于 {shortDate(item.finishedAt)}</span> : null}
      </p>
      <p className="mt-3 text-mk-small text-mk-muted">对话内容不向教师展示；以下为学生的产出。</p>

      {detail.writing && (
        <nav className="teacher-writing-views" aria-label="写作成果视图">
          <button type="button" aria-pressed={!showOriginal} onClick={() => setDocumentView({ id: item.atomId, original: false })}>报告与过程</button>
          <button type="button" aria-pressed={!!showOriginal} onClick={() => setDocumentView({ id: item.atomId, original: true })}>写作原文</button>
        </nav>
      )}
      {detail.writing && showOriginal && (
        <WritingManuscript classId={classId} userId={userId} atomId={atomId} writing={detail.writing} onOpenGrading={onOpenGrading} />
      )}

      {(!detail.writing || !showOriginal) && <>
      <ReportSection
        report={detail.report}
        reportError={detail.reportError}
        kind={item.kind === "writing" ? "writing" : "reading"}
        retriedProse={retriedProse}
        proseError={proseError}
      />
      <ReadingSection reading={detail.reading} keep={detail.report?.keep ?? null} />
      <WritingSection writing={detail.writing} />
      <ProjectSection project={detail.project} />
      </>}
    </>
  );
}

/** 写作原文: her latest submitted version (fetched on open), or her draft
 *  when nothing was ever submitted — see `teacherManuscript`. Also where the
 *  批改 control and version list live (controller ruling — not inside
 *  报告与过程, which is process data, not the manuscript itself). */
function WritingManuscript({
  classId,
  userId,
  atomId,
  writing,
  onOpenGrading,
}: {
  classId: string;
  userId: string;
  atomId: string;
  writing: NonNullable<ItemDetail["writing"]>;
  onOpenGrading: (gradingId: string) => void;
}) {
  const source = teacherManuscript(writing);
  const n = source.kind === "version" ? source.version.number : null;
  const [body, setBody] = useState<WritingVersion | null>(null);
  const [error, setError] = useState<string | null>(null);
  const alive = useAlive();
  const [busy, setBusy] = useState(false);
  const [gradeError, setGradeError] = useState<string | null>(null);

  useEffect(() => {
    if (n === null) return;
    let cancelled = false;
    setBody(null);
    setError(null);
    getItemVersion(classId, userId, atomId, n)
      .then((v) => {
        if (!cancelled) setBody(v);
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : String(e));
      });
    return () => {
      cancelled = true;
    };
  }, [classId, userId, atomId, n]);

  const action = itemGradingAction(writing.versions.length, writing.grading);
  const rowStatus = writing.grading ? gradingRowStatus({ version: writing.versions[0] ?? null, grading: writing.grading }) : null;

  async function grade() {
    if (busy) return;
    setBusy(true);
    setGradeError(null);
    try {
      const g = await queueWritingGrading(classId, userId, atomId);
      if (alive.current) onOpenGrading(g.id);
    } catch (e) {
      // A 409 means a draft/sent/in-flight row already exists for this
      // version — created after this page's own load (another tab, or a
      // regrade started elsewhere). Never overwrite it silently: refetch to
      // find its id and open THAT one, rather than just reporting an error.
      if (e instanceof ApiError && (e.code === "grading_exists" || e.code === "grading_in_progress")) {
        try {
          const fresh = await getItem(classId, userId, atomId);
          const gid = fresh.writing?.grading?.id ?? null;
          if (alive.current) {
            if (gid) onOpenGrading(gid);
            else setGradeError(failText("批改", e));
          }
        } catch (e2) {
          if (alive.current) setGradeError(failText("批改", e2));
        }
      } else if (alive.current) {
        setGradeError(failText("批改", e));
      }
    } finally {
      if (alive.current) setBusy(false);
    }
  }

  return (
    <section className="teacher-record-section teacher-writing-original" aria-labelledby="writing-original-heading">
      <h2 id="writing-original-heading" className="text-mk-h3 text-mk-ink">写作原文</h2>
      {source.kind === "version" ? (
        <>
          <p className="mt-1 flex flex-wrap items-center gap-2 text-mk-small text-mk-muted">
            <span>已提交的最新版本 · {versionLine(source.version, writing.lang)}</span>
            {source.revising && (
              <span className="rounded-mk-full px-2 py-0.5 text-mk-label font-semibold" style={tintedChipStyle("var(--mk-warning)")}>
                修改中
              </span>
            )}
          </p>

          {writing.versions.length > 1 && (
            <div className="mt-3">
              <h3 className="text-mk-label text-mk-muted">版本</h3>
              <ul className="mt-1.5 flex flex-col gap-1">
                {writing.versions.map((v) => (
                  <li key={v.number} className="text-mk-small text-mk-ink">
                    {versionLine(v, writing.lang)}
                  </li>
                ))}
              </ul>
            </div>
          )}

          <div className="mt-3 flex flex-wrap items-center gap-2">
            {action === "grade" && (
              <Button variant="secondary" size="sm" disabled={busy} onClick={() => void grade()}>
                AI 批改
              </Button>
            )}
            {action === "open" && writing.grading && rowStatus && (
              <>
                <span
                  className="inline-block whitespace-nowrap rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold"
                  style={tintedChipStyle(gradingStatusHue(rowStatus))}
                >
                  {GRADING_STATUS_LABEL[rowStatus]}
                </span>
                <Button variant="link" size="sm" onClick={() => onOpenGrading(writing.grading!.id)}>
                  查看批改
                </Button>
              </>
            )}
            {gradeError && (
              <span role="alert" className="text-mk-small font-semibold text-mk-danger">
                {gradeError}
              </span>
            )}
          </div>

          {error ? (
            <p className="mt-5 text-mk-small font-semibold text-mk-danger">加载失败：{error}</p>
          ) : body === null ? (
            <p className="mt-5 text-mk-body text-mk-muted">加载中…</p>
          ) : (
            <article className="teacher-writing-manuscript">{body.body}</article>
          )}
        </>
      ) : (
        <>
          <p className="mt-1 text-mk-small text-mk-muted">学生当前保存的正文</p>
          {source.draft.trim() ? (
            <article className="teacher-writing-manuscript">{source.draft}</article>
          ) : (
            <p className="mt-5 text-mk-body text-mk-muted">学生尚未保存写作正文</p>
          )}
        </>
      )}
    </section>
  );
}

function ReportSection({
  report,
  reportError,
  kind,
  retriedProse,
  proseError,
}: {
  report: ItemDetail["report"];
  reportError: string | null;
  kind: AtomKind;
  retriedProse: boolean;
  proseError: string | null;
}) {
  if (reportError) {
    // 服务端已经把「报告生成失败：」拼进这句原话里了，原样显示，不再叠一层
    // 前缀（AGENTS.md 界面文案 §8 的「后台原话」原则）。
    return (
      <section className="teacher-record-section">
        <h2 className="text-mk-h3 text-mk-ink">报告</h2>
        <p className="mt-2 text-mk-small font-semibold text-mk-danger">{reportError}</p>
      </section>
    );
  }
  if (!report) return null;

  const stats = report.stats
    .filter((s) => s.value !== 0)
    .map((s) => displayStat({ key: s.key, label: s.key, value: s.value, unit: s.unit } as ReportStat, kind));
  const pendingLabel = prosePendingLabel(report.prosePending, retriedProse);

  return (
    <section className="teacher-record-section">
      <h2 className="text-mk-h3 text-mk-ink">报告</h2>

      <ReportVisualSummary stats={stats} />

      {report.moments.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">学生原话</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {report.moments.map((m, i) => (
              <li key={i} className="teacher-evidence">
                <p className="text-mk-small italic text-mk-ink">「{m.quote}」</p>
                <p className="mt-1 text-mk-small text-mk-muted">来自：{m.where}</p>
              </li>
            ))}
          </ul>
        </div>
      )}

      {report.keep && (
        <div className="teacher-takeaway">
          <h3 className="text-mk-label text-mk-muted">收获</h3>
          <p className="mt-1.5 whitespace-pre-wrap text-mk-body text-mk-ink">{report.keep.text}</p>
          <p className="mt-2 text-mk-small text-mk-muted">
            {report.keep.source === "student" ? "学生自己写的收获" : "印记根据学习过程整理"}
          </p>
        </div>
      )}

      {proseError ? (
        <p className="mt-3 text-mk-small font-semibold text-mk-danger" role="status" aria-live="polite">
          报告加载失败：{proseError}
        </p>
      ) : pendingLabel ? (
        <p className="mt-3 text-mk-small text-mk-muted" role="status" aria-live="polite">
          {pendingLabel}
        </p>
      ) : null}
    </section>
  );
}

function ReadingSection({
  reading,
  keep,
}: {
  reading: ItemDetail["reading"];
  keep: { text: string; source: string } | null;
}) {
  if (!reading) return null;
  const { source, highlights, takeaway, lenses } = reading;
  const sourceHref = safeHttpUrl(source?.url);
  return (
    <section className="teacher-record-section">
      <h2 className="text-mk-h3 text-mk-ink">阅读</h2>

      {source && (source.librarySlug || source.url) ? (
        <p className="mt-2 text-mk-small text-mk-ink">
          来源：
          {source.librarySlug ? (
            <>
              分级阅读文章
              {source.level !== null ? ` · 阅读难度 ${source.level} 级` : ""}
            </>
          ) : sourceHref ? (
            <a href={sourceHref} target="_blank" rel="noreferrer" className="text-mk-accent-700 underline">
              {source.url}
            </a>
          ) : (
            // 不是 http/https 的地址不做成链接，只显示文字。
            <span>{source.url}</span>
          )}
        </p>
      ) : null}

      {highlights.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">划线与笔记</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {highlights.map((h, i) => (
              <li key={i} className="teacher-evidence">
                <p className="text-mk-small italic text-mk-muted">「{h.quote}」</p>
                {h.note && <p className="mt-1 text-mk-small text-mk-ink">{h.note}</p>}
              </li>
            ))}
          </ul>
        </div>
      )}

      {showReadingTakeaway(takeaway, keep) && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">收获</h3>
          <p className="mt-1.5 whitespace-pre-wrap text-mk-body text-mk-ink">{takeaway}</p>
        </div>
      )}

      {lenses.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">阅读透镜</h3>
          <div className="mt-2 flex flex-col gap-2">
            {lenses.map((l, i) => (
              <div key={i} className="teacher-evidence">
                {l.title && <p className="text-mk-small font-bold text-mk-ink">{l.title}</p>}
                <OutputRecord value={l.fields} labels={lensFieldLabels(l.title)} />
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

function WritingSection({ writing }: { writing: ItemDetail["writing"] }) {
  if (!writing) return null;
  const { targetWords, lang, structureKey, outline, snippets, comments } = writing;
  return (
    <section className="teacher-record-section">
      <h2 className="text-mk-h3 text-mk-ink">写作过程</h2>

      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-mk-small text-mk-ink">
        <span>目标字数 {targetWords && targetWords > 0 ? `${targetWords} ${lang === "en" ? "词" : "字"}` : "—"}</span>
        <span>语言 {langLabel(lang)}</span>
        {structureKey && <details><summary className="cursor-pointer">查看历史结构标识</summary>{structureKey}</details>}
      </div>

      {outline.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">提纲</h3>
          <ul className="mt-2 flex flex-col gap-1.5">
            {outline.map((o, i) => (
              <li key={i} className="text-mk-small text-mk-ink">
                <span className="text-mk-muted">{o.role}：</span>
                {o.text}
              </li>
            ))}
          </ul>
        </div>
      )}

      {snippets.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">片段</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {snippets.map((s, i) => (
              <li key={i} className="teacher-evidence text-mk-small text-mk-ink">
                {s.text}
              </li>
            ))}
          </ul>
        </div>
      )}

      {comments.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">AI 批注</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {comments.map((c, i) => (
              <li key={i} className="teacher-evidence">
                <p className="text-mk-small text-mk-ink">{c.summary}</p>
                {commentPointLines(c.points).length > 0 && (
                  <ul className="mt-1.5 flex flex-col gap-1.5">
                    {commentPointLines(c.points).map((p, j) => (
                      <li key={j} className="text-mk-small text-mk-ink">
                        {p.kind !== "note" && <span className="text-mk-muted">{p.kind === "good" ? "优点：" : "问题："}</span>}
                        {p.quote && <span className="text-mk-muted">「{p.quote}」 </span>}
                        {p.text}
                        {p.action && <span className="block text-mk-muted">修改建议：{p.action}</span>}
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}

function ProjectSection({ project }: { project: ItemDetail["project"] }) {
  if (!project) return null;
  // `steps` 服务端「还没有落地计划」时发 `null`——`normalizeItemDetail` 已经
  // 把它收口成空数组（C1），这里不用再兜一次。
  const { steps } = project;

  return (
    <section className="teacher-record-section">
      <h2 className="text-mk-h3 text-mk-ink">项目</h2>

      <p className="mt-2 text-mk-body text-mk-ink">{project.assigned && <span className="block text-mk-small text-mk-muted">驱动问题（作业）</span>}{project.idea}</p>
      <ProjectProgressVisual steps={steps} done={project.stepsDone} total={project.stepsTotal} />

      {project.tools.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">工具产出</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {project.tools.map((t, i) => (
              <li key={i} className="teacher-evidence">
                <p className="text-mk-small font-bold text-mk-ink">{t.label || t.key}</p>
                <OutputRecord value={t.result} />
              </li>
            ))}
          </ul>
        </div>
      )}

      {project.artifacts.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">成果</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {project.artifacts.map((a, i) => (
              <li key={i} className="teacher-evidence">
                <p className="text-mk-small font-bold text-mk-ink">{a.title}</p>
                <OutputRecord value={a.payload} />
              </li>
            ))}
          </ul>
        </div>
      )}

      {project.keeps.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">保留条目</h3>
          <ul className="mt-2 flex flex-col gap-1.5">
            {project.keeps.map((k, i) => (
              <li key={i} className="text-mk-small text-mk-ink">
                {k.text}
              </li>
            ))}
          </ul>
        </div>
      )}

      {project.courses.length > 0 && (
        <div className="mt-4">
          <h3 className="text-mk-label text-mk-muted">课程</h3>
          <ul className="mt-2 flex flex-col gap-2">
            {project.courses.map((c, i) => (
              <li key={i} className="teacher-evidence">
                <p className="text-mk-small font-bold text-mk-ink">课程学习记录</p>
                <details className="text-mk-small text-mk-muted"><summary className="cursor-pointer">查看课程标识</summary>{c.slug}</details>
                <p className="mt-1 text-mk-small text-mk-ink">{c.why}</p>
                <p className="mt-1 text-mk-small text-mk-muted">{c.takeaway}</p>
              </li>
            ))}
          </ul>
        </div>
      )}

      {project.siteToken && (
        <p className="mt-4 text-mk-small">
          <a
            href={`/p/${project.siteToken}`}
            target="_blank"
            rel="noreferrer"
            className="text-mk-accent-700 underline"
          >
            主页链接
          </a>
        </p>
      )}
    </section>
  );
}
