import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Button } from "@/ui";
import { getAssignmentForAtom, type AssignmentForAtom } from "../api/assignments";
import { apiErrorText } from "../api/errorText";
import { listWritingGradings, type StudentGrading } from "../api/gradings";
import { getWritingVersion, listWritingVersions, type Writing, type WritingVersion, type WritingVersionList } from "../api/writings";
import { ReportPanel } from "../reports/ReportPanel";
import { splitParagraphs } from "../reports/paragraphs";
import { formatDeadline } from "../shared/deadline";
import { highlightSegments, MARK_STYLE, PIECE_CLS, quoteRanges } from "../shared/gradingText";
import { useAlive } from "../shared/useAlive";
import { tintedChipStyle } from "../teacher/assignmentLogic";
import {
  chipHue,
  chipToShow,
  effectiveDueAt,
  finishedBodyState,
  finishedHeadline,
  isReturnOpen,
  LOCKED_TEXT,
  NO_VERSION_TEXT,
  returnedLine,
  versionLine,
  type AssignmentLoadState,
} from "./finishedWriting";
import { TeacherGradingPanel } from "./TeacherGradingPanel";
import { diffVersions, type ParagraphDiff } from "./versionDiff";

/**
 * FinishedWritingPage — `/writings/:id` once a writing is finished and
 * either not being revised, or revising but locked (past its deadline, so
 * the room could not save anything anyway — `showFinishedPage` in
 * `WritingRoomHost`'s load effect decides which of the two this is).
 *
 * Header: title, chip, homework line, locked/returned line, 修改 and 报告.
 * Left column (44rem): the version being viewed, the latest by default.
 * Right rail: 版本, 老师批改 (absent when the teacher has sent none — see
 * `TeacherGradingPanel`), and 与当前版本对比 when an older version is
 * selected. Below 1024px the rail follows the text in one column. 报告 swaps
 * the columns for ReportPanel.
 *
 * `revise()` opens the compose/write view via `onRevise` (WritingRoomHost's
 * `reload()` re-runs the load effect; `isRevising(writing)` and `showFinishedPage`
 * then route it to the "ready" phase's `RevisingStrip` unless it is locked).
 * 修改 is disabled while `locked` — clicking it would always get 403
 * `writing_locked` — and the locked line explains why in the same words the
 * room shows if she somehow still tries.
 */
export function FinishedWritingPage({
  writing,
  onBack,
  onRevise,
}: {
  writing: Writing;
  onBack: () => void;
  /** Starts revising and reopens the room. Throws on failure. */
  onRevise: () => Promise<void>;
}) {
  const alive = useAlive();
  const [list, setList] = useState<WritingVersionList | null>(null);
  const [assignment, setAssignment] = useState<AssignmentForAtom | null>(null);
  const [assignmentState, setAssignmentState] = useState<AssignmentLoadState>("loading");
  const [assignmentError, setAssignmentError] = useState<string | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [bodies, setBodies] = useState<Record<number, WritingVersion>>({});
  const requested = useRef<Set<number>>(new Set());
  const [selected, setSelected] = useState<number | null>(null);
  const [compare, setCompare] = useState(false);
  const [showReport, setShowReport] = useState(false);
  const [revising, setRevising] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [gradings, setGradings] = useState<StudentGrading[]>([]);
  const [gradingsError, setGradingsError] = useState<string | null>(null);
  const [highlight, setHighlight] = useState<{ version: number; quote: string } | null>(null);

  useEffect(() => {
    setList(null);
    setLoadError(null);
    setBodies({});
    requested.current = new Set();
    setSelected(null);
    setCompare(false);
    setAssignment(null);
    setAssignmentState("loading");
    setAssignmentError(null);
    setGradings([]);
    setGradingsError(null);
    setHighlight(null);
    listWritingVersions(writing.id)
      .then((l) => {
        if (!alive.current) return;
        setList(l);
        setSelected(l.versions[0]?.number ?? null);
      })
      .catch((e: unknown) => {
        if (alive.current) setLoadError(apiErrorText(e));
      });
    // `getAssignmentForAtom` resolves `null` on a genuine 200 for a writing
    // that is not homework — that is NOT the same thing as a thrown error
    // (network, 500, auth), so a failure must never be folded into "not
    // homework" (`chipToShow` refuses to guess a chip until this settles).
    getAssignmentForAtom(writing.id)
      .then((a) => {
        if (!alive.current) return;
        setAssignment(a);
        setAssignmentState("loaded");
      })
      .catch((e: unknown) => {
        if (!alive.current) return;
        setAssignmentState("failed");
        setAssignmentError(apiErrorText(e));
      });
    // Fetched fresh on every load, no stale cache — a grading re-sent after
    // the teacher edits it shows the updated content the next time this
    // page opens (controller ruling 3).
    listWritingGradings(writing.id)
      .then((g) => {
        if (alive.current) setGradings(g);
      })
      .catch((e: unknown) => {
        if (alive.current) setGradingsError(apiErrorText(e));
      });
  }, [writing.id, alive]);

  const latest = list?.versions[0]?.number ?? null;

  useEffect(() => {
    // Also fetches every graded version's body, not only the one on screen:
    // the 老师批改 panel needs each grading's own text to tell whether a
    // point's quote actually highlights there, and preloading it here means
    // clicking a quote switches version instantly instead of triggering a
    // fresh fetch on top of the switch.
    const need = new Set<number>();
    if (selected !== null) need.add(selected);
    if (latest !== null) need.add(latest);
    for (const g of gradings) need.add(g.versionNumber);
    for (const n of need) {
      if (requested.current.has(n)) continue;
      requested.current.add(n);
      getWritingVersion(writing.id, n)
        .then((v) => {
          if (alive.current) setBodies((b) => ({ ...b, [n]: v }));
        })
        .catch((e: unknown) => {
          if (alive.current) setLoadError(apiErrorText(e));
        });
    }
  }, [selected, latest, gradings, writing.id, alive]);

  const shown = selected !== null ? bodies[selected] : undefined;
  const latestBody = latest !== null ? bodies[latest] : undefined;
  const diff = useMemo(
    () => (compare && shown && latestBody && shown.number !== latestBody.number ? diffVersions(shown.body, latestBody.body) : null),
    [compare, shown, latestBody],
  );

  const locked = list?.locked ?? false;
  const chip = chipToShow(assignmentState, { assignment, locked });

  async function revise() {
    if (revising) return;
    setRevising(true);
    setActionError(null);
    try {
      await onRevise();
    } catch (e) {
      if (alive.current) setActionError(`修改失败：${apiErrorText(e)}`);
    } finally {
      if (alive.current) setRevising(false);
    }
  }

  const selectedSummary = list?.versions.find((v) => v.number === selected) ?? null;
  const bodyState = finishedBodyState({
    listLoaded: list !== null,
    versionCount: list?.versions.length ?? 0,
    loadError,
    bodyLoaded: shown !== undefined,
  });

  return (
    <div className="flex w-full flex-col pb-14">
      <header className="mk-rp-measure flex flex-col gap-3 pt-8">
        <div className="flex flex-wrap items-center gap-3">
          <Button variant="secondary" onClick={onBack}>
            回到写作
          </Button>
          {chip && (
            <span className="rounded-mk-full px-2.5 py-1 text-mk-label font-semibold" style={tintedChipStyle(chipHue(chip))}>
              {chip}
            </span>
          )}
          <div className="flex flex-wrap gap-2 sm:ml-auto">
            <Button variant="secondary" onClick={() => void revise()} disabled={locked || revising || list === null}>
              修改
            </Button>
            <Button variant={showReport ? "primary" : "secondary"} onClick={() => setShowReport((v) => !v)}>
              {showReport ? "正文" : "报告"}
            </Button>
          </div>
        </div>
        {assignmentError && (
          <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
            加载失败：{assignmentError}
          </p>
        )}
        <h1 className="font-mk-piece text-mk-report-title text-mk-ink">{finishedHeadline(writing.title, list?.versions ?? null)}</h1>
        {assignment && (
          <p className="text-mk-small text-mk-muted">
            作业 · {assignment.title} · 截止 {formatDeadline(effectiveDueAt(assignment))}
          </p>
        )}
        {/* A state, not an error: muted, like the chip beside it. */}
        {locked && (
          <p role="status" className="text-mk-small font-semibold text-mk-muted">
            {LOCKED_TEXT}
          </p>
        )}
        {assignment && !locked && isReturnOpen(assignment) && (
          <div className="flex flex-col gap-1 rounded-mk-md border border-mk-border bg-mk-surface px-4 py-3 text-mk-small">
            <p className="font-semibold text-mk-ink">{returnedLine(assignment)}</p>
            {assignment.returnNote && <p className="whitespace-pre-wrap text-mk-secondary">退回说明：{assignment.returnNote}</p>}
          </div>
        )}
        {actionError && (
          <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
            {actionError}
          </p>
        )}
      </header>

      {showReport ? (
        <ReportPanel kind="writing" atomId={writing.id} />
      ) : (
        <div className="mk-rp-measure mt-8 grid grid-cols-1 gap-10 lg:grid-cols-[minmax(0,44rem)_minmax(16rem,1fr)]">
          <article className="min-w-0">
            {selectedSummary && selected !== latest && (
              <p className="mb-4 text-mk-small text-mk-muted">{versionLine(selectedSummary, writing.lang)}</p>
            )}
            {bodyState === "no_versions" ? (
              <p className="text-mk-body text-mk-muted">{NO_VERSION_TEXT}</p>
            ) : bodyState === "error" ? null : (
              <VersionText
                version={shown}
                diff={diff}
                highlight={highlight && highlight.version === selected ? highlight.quote : null}
              />
            )}
          </article>

          <aside className="flex min-w-0 flex-col gap-3">
            <h2 className="text-mk-small font-semibold text-mk-secondary">版本</h2>
            {loadError && (
              <p role="alert" className="break-words text-mk-small font-semibold text-mk-danger">
                加载失败：{loadError}
              </p>
            )}
            {list === null && !loadError && <p className="text-mk-small text-mk-muted">加载中…</p>}
            <ul className="flex flex-col gap-1">
              {list?.versions.map((v) => (
                <li key={v.number}>
                  <button
                    type="button"
                    aria-pressed={v.number === selected}
                    onClick={() => {
                      setSelected(v.number);
                      if (v.number === latest) setCompare(false);
                      setHighlight(null);
                    }}
                    className="w-full rounded-mk-sm px-3 py-2 text-left text-mk-small text-mk-ink transition-colors duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                    style={v.number === selected ? SELECTED_STYLE : undefined}
                  >
                    {versionLine(v, writing.lang)}
                  </button>
                </li>
              ))}
            </ul>
            {selected !== null && latest !== null && selected !== latest && (
              <div className="flex flex-col gap-2">
                <Button variant={compare ? "primary" : "secondary"} size="sm" onClick={() => setCompare((c) => !c)}>
                  与当前版本对比
                </Button>
                {compare && (
                  <p className="flex flex-wrap gap-3 text-mk-small text-mk-muted">
                    <ins style={ADD_STYLE}>新增</ins>
                    <del style={DEL_STYLE}>删除</del>
                  </p>
                )}
              </div>
            )}
            <TeacherGradingPanel
              gradings={gradings}
              error={gradingsError}
              shownVersion={selected}
              bodies={bodies}
              onQuote={(version, quote) => {
                setSelected(version);
                setCompare(false);
                setHighlight({ version, quote });
              }}
            />
          </aside>
        </div>
      )}
    </div>
  );
}

const SELECTED_STYLE: CSSProperties = { background: "color-mix(in srgb, var(--mk-accent-500) 12%, var(--mk-surface))" };
const ADD_STYLE: CSSProperties = {
  background: "color-mix(in srgb, var(--mk-success) 20%, transparent)",
  textDecoration: "none",
};
const DEL_STYLE: CSSProperties = {
  background: "color-mix(in srgb, var(--mk-danger) 14%, transparent)",
  textDecoration: "line-through",
};

/**
 * `highlight` is a quote text from a 老师批改 point (`TeacherGradingPanel`'s
 * `onQuote`) — set whenever the version being shown is the one that quote
 * was picked against. Takes priority over `diff`: a quote click always
 * clears `compare` first (see the aside), so the two never coexist for the
 * same render.
 */
function VersionText({
  version,
  diff,
  highlight,
}: {
  version: WritingVersion | undefined;
  diff: ParagraphDiff[] | null;
  highlight: string | null;
}) {
  const markRef = useRef<HTMLElement | null>(null);
  useEffect(() => {
    markRef.current?.scrollIntoView({ block: "center", behavior: "smooth" });
  }, [highlight, version?.number]);
  if (!version) return <p className="text-mk-body text-mk-muted">加载中…</p>;
  if (!diff && highlight) {
    const segments = highlightSegments(version.body, quoteRanges(version.body, [highlight]));
    return (
      <div className={PIECE_CLS}>
        {segments.map((seg, i) =>
          seg.index === null ? (
            <span key={i}>{seg.text}</span>
          ) : (
            <mark key={i} ref={markRef} style={MARK_STYLE}>
              {seg.text}
            </mark>
          ),
        )}
      </div>
    );
  }
  if (diff) {
    return (
      <div className="flex flex-col gap-6">
        {diff.map((d, i) => (
          <DiffParagraph key={i} d={d} />
        ))}
      </div>
    );
  }
  const paragraphs = splitParagraphs(version.body);
  if (paragraphs.length === 0) return <p className="text-mk-body text-mk-muted">这一版没有正文。</p>;
  return (
    <div className="flex flex-col gap-6">
      {paragraphs.map((p, i) => (
        <p key={i} className={PIECE_CLS}>
          {p}
        </p>
      ))}
    </div>
  );
}

function DiffParagraph({ d }: { d: ParagraphDiff }) {
  // Checked first: "change" is the only member whose `kind` is a single
  // literal, so this is the one branch TS can narrow both ways on — the
  // other three share a `"same" | "add" | "del"` kind and TS will not split
  // that union member by member across sequential `===` checks.
  if (d.kind === "change") {
    return (
      <p className={PIECE_CLS}>
        {d.parts.map((part, i) =>
          part.kind === "same" ? (
            <span key={i}>{part.text}</span>
          ) : part.kind === "add" ? (
            <ins key={i} style={ADD_STYLE}>
              {part.text}
            </ins>
          ) : (
            <del key={i} style={DEL_STYLE}>
              {part.text}
            </del>
          ),
        )}
      </p>
    );
  }
  if (d.kind === "add")
    return (
      <p className={PIECE_CLS}>
        <ins style={ADD_STYLE}>{d.text}</ins>
      </p>
    );
  if (d.kind === "del")
    return (
      <p className={PIECE_CLS}>
        <del style={DEL_STYLE}>{d.text}</del>
      </p>
    );
  return <p className={PIECE_CLS}>{d.text}</p>;
}
