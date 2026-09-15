import type { ReactNode } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { IconButton } from "@/ui";
import { getStudentWeekly, postStudentWeeklyProse, suggestionLabel, type WeekCard } from "../api/weekly";
import { tintedChipStyle } from "./assignmentLogic";
import { formatMinutes } from "./format";
import { canGoNext, shiftWeek, splitWeekTitle } from "./weekNav";
import { useWeekly, type ProseState } from "./useWeekly";

/**
 * WeekSummaryCard — one student's week on the teacher's student page. The
 * title comes from the API (上周表现总结 · … for the latest completed week,
 * 表现总结 · … for an earlier one). Facts and rule cards arrive with the GET;
 * the prose is asked for once per week per mount (see `useWeekly`).
 *
 * The prose is plain text from the model: rendered as text nodes, never as
 * HTML, with 「」 and 《》 left as they are.
 *
 * Callers key this component by `${classId}:${userId}`.
 */
export function WeekSummaryCard({ classId, userId }: { classId: string; userId: string }) {
  const w = useWeekly({
    scope: userId,
    load: (ws) => getStudentWeekly(classId, userId, ws),
    post: (ws) => postStudentWeeklyProse(classId, userId, ws),
    wantsProse: (week) => !week.empty,
  });
  const { data } = w;

  return (
    <section className="mt-8 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:p-5">
      <WeekHeader
        title={data?.title ?? null}
        weekLabel={data?.weekLabel ?? ""}
        level="h2"
        onPrev={data?.hasPrev ? () => w.goToWeek(shiftWeek(data.weekStart, -7)) : undefined}
        onNext={data && canGoNext(data.weekStart, data.isLatest) ? () => w.goToWeek(shiftWeek(data.weekStart, 7)) : undefined}
      />

      {w.notStarted ? (
        <p className="mt-3 text-mk-body text-mk-muted">{w.notStarted}</p>
      ) : w.loadError ? (
        <LoadFailed message={w.loadError} onRetry={w.reload} />
      ) : data === null ? (
        <p className="mt-3 text-mk-body text-mk-muted">加载中…</p>
      ) : (
        <>
          <div className="mt-4 flex flex-wrap gap-3">
            <FactTile label="活跃天数" value={`${data.facts.activeDays} 天`} />
            <FactTile label="学习时长" value={formatMinutes(data.facts.minutes)} />
            <FactTile label="对话轮次" value={`${data.facts.turns} 轮`} />
            <FactTile label="完成" value={`${data.facts.finished.length} 项`} />
            <FactTile label="作业按时完成" value={`${data.facts.assignmentsDone} 份`} />
            <FactTile label="作业逾期完成" value={`${data.facts.assignmentsLate} 份`} />
            <FactTile label="作业逾期" value={`${data.facts.assignmentsOverdue} 份`} />
          </div>

          <div className="mt-5 grid gap-4 sm:grid-cols-2">
            <CardGroup title="值得表扬" cards={data.cards.filter((c) => c.kind === "praise")} />
            <CardGroup title="需要建议" cards={data.cards.filter((c) => c.kind === "watch")} />
          </div>

          <div className="mt-5">
            <h3 className="text-mk-label font-bold text-mk-muted">总结</h3>
            {data.prose ? (
              <>
                <p className="mt-1.5 whitespace-pre-wrap text-mk-body text-mk-ink">{data.prose.summary}</p>
                {data.prose.suggestions.length > 0 && (
                  <>
                    <h3 className="mt-4 text-mk-label font-bold text-mk-muted">建议</h3>
                    <ol className="mt-1.5 flex flex-col gap-1.5">
                      {data.prose.suggestions.map((sg, i) => {
                        const label = suggestionLabel(data.cards, sg.evidenceCode);
                        return (
                          <li key={i} className="text-mk-body text-mk-ink">
                            <span className="tabular-nums">{i + 1}. </span>
                            {sg.text}
                            {label && <span className="ml-2 text-mk-small text-mk-muted">{label}</span>}
                          </li>
                        );
                      })}
                    </ol>
                  </>
                )}
              </>
            ) : data.empty ? (
              <p className="mt-1.5 text-mk-body text-mk-muted">该周没有学习记录</p>
            ) : (
              <ProseStatus state={w.prose} onRetry={w.retryProse} />
            )}
          </div>
        </>
      )}
    </section>
  );
}

/** Title row with ‹ 上一周 / 下一周 ›. A missing handler disables the button. */
export function WeekHeader({
  title,
  weekLabel,
  level,
  onPrev,
  onNext,
}: {
  title: string | null;
  weekLabel: string;
  level: "h1" | "h2";
  onPrev?: () => void;
  onNext?: () => void;
}) {
  const Heading = level;
  // The week label stays on one line; a narrow screen wraps before it.
  const { head, label } = splitWeekTitle(title ?? "", weekLabel);
  return (
    <div className="flex flex-wrap items-center justify-between gap-2">
      <Heading className={level === "h1" ? "teacher-page-title" : "text-mk-h3 text-mk-ink"}>
        {head}
        {label && <span className="whitespace-nowrap">{label}</span>}
      </Heading>
      <div className="flex shrink-0 items-center gap-1.5">
        <IconButton icon={ChevronLeft} label="上一周" variant="secondary" size="sm" disabled={!onPrev} onClick={onPrev} />
        <IconButton icon={ChevronRight} label="下一周" variant="secondary" size="sm" disabled={!onNext} onClick={onNext} />
      </div>
    </div>
  );
}

export function LoadFailed({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="mt-3 text-mk-small font-semibold text-mk-danger">
      加载失败：{message}{" "}
      <button type="button" onClick={onRetry} className="cursor-pointer underline">
        重试
      </button>
    </div>
  );
}

/** The prose block while there is no prose: 总结生成中, the failure with 重试,
 * or (asked before and nothing came back) a button to ask. */
export function ProseStatus({ state, onRetry }: { state: ProseState; onRetry: () => void }) {
  if (state.status === "pending") {
    return <p className="mt-1.5 text-mk-body text-mk-muted">总结生成中</p>;
  }
  if (state.status === "error") {
    return (
      <p className="mt-1.5 break-words text-mk-small font-semibold text-mk-danger">
        总结生成失败：{state.message}{" "}
        <button type="button" onClick={onRetry} className="cursor-pointer underline">
          重试
        </button>
      </p>
    );
  }
  return (
    <p className="mt-1.5 text-mk-body text-mk-muted">
      总结待生成{" "}
      <button type="button" onClick={onRetry} className="cursor-pointer text-mk-small font-semibold text-mk-accent-700 underline">
        生成总结
      </button>
    </p>
  );
}

export function FactTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-[104px] rounded-mk-md border border-mk-border bg-mk-surface px-3.5 py-2.5">
      <div className="text-mk-h3 tabular-nums text-mk-ink">{value}</div>
      <div className="mt-0.5 text-mk-label text-mk-muted">{label}</div>
    </div>
  );
}

/** Tint for a rule card's label chip: praise green, watch amber, with the same
 * mix as the status chips. `color-mix` because Tailwind alpha modifiers on
 * `mk-*` tokens emit no CSS. At 78% hue the text read 3.34:1 (praise) and
 * 2.76:1 (watch) in lite light mode; at 40% it reads 5.97:1 and 5.46:1. */
export function cardTagStyle(kind: string): { background: string; color: string } {
  return tintedChipStyle(kind === "praise" ? "var(--mk-success)" : "var(--mk-warning)");
}

export function CardTag({ kind, label }: { kind: string; label: string }) {
  return (
    <span className="inline-block whitespace-nowrap rounded-mk-full px-2.5 py-0.5 text-mk-small font-bold" style={cardTagStyle(kind)}>
      {label}
    </span>
  );
}

function CardGroup({ title, cards }: { title: string; cards: WeekCard[] }) {
  return (
    <GroupShell title={title} empty={cards.length === 0}>
      {cards.map((c) => (
        <div key={c.code} className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
          <CardTag kind={c.kind} label={c.label} />
          <p className="mt-1.5 text-mk-small text-mk-ink">{c.evidence}</p>
        </div>
      ))}
    </GroupShell>
  );
}

export function GroupShell({ title, empty, children }: { title: string; empty: boolean; children: ReactNode }) {
  return (
    <div>
      <h3 className="text-mk-label font-bold text-mk-muted">{title}</h3>
      {empty ? (
        <p className="mt-1.5 text-mk-small text-mk-muted">暂无</p>
      ) : (
        <div className="mt-1.5 flex flex-col gap-2">{children}</div>
      )}
    </div>
  );
}
