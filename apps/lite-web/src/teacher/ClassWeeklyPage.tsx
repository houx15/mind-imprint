import { ArrowLeft } from "lucide-react";
import { Icon } from "@/ui";
import { classCardProse, getClassWeekly, postClassWeeklyProse, type ClassWeekCard, type ClassWeeklyProse } from "../api/weekly";
import { formatMinutes } from "./format";
import { canGoNext, shiftWeek } from "./weekNav";
import { useWeekly } from "./useWeekly";
import { cardShowsProse } from "./classWeeklyLogic";
import { TeacherPage } from "./TeacherPage";
import { CardTag, FactTile, GroupShell, LoadFailed, ProseStatus, WeekHeader } from "./WeekSummaryCard";

/**
 * ClassWeeklyPage — `/classes/:classId/weekly`. The class's week: stat tiles,
 * 班级点评 (model prose, asked for once per week per mount, see `useWeekly`),
 * and the students the rules flagged, as 值得表扬 / 需要建议 cards. A card
 * opens that student's page. Each card's lead and action come from the same
 * prose request; before it lands the card shows the rule's evidence only.
 *
 * The title comes from the API (上周班级周报 · … for the latest completed week,
 * 班级周报 · … for an earlier one). Prose renders as plain text.
 *
 * The shell keys this page by class id.
 */
export function ClassWeeklyPage({
  classId,
  onBack,
  onOpenStudent,
}: {
  classId: string;
  onBack: () => void;
  onOpenStudent: (userId: string) => void;
}) {
  const w = useWeekly({
    scope: classId,
    load: (ws) => getClassWeekly(classId, ws),
    post: (ws) => postClassWeeklyProse(classId, ws),
    wantsProse: (week) => !week.empty,
  });
  const { data } = w;

  return (
    <TeacherPage width="wide">
      <button
        type="button"
        onClick={onBack}
        className="flex items-center gap-1.5 rounded-mk-sm text-mk-small text-mk-muted transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      >
        <Icon icon={ArrowLeft} size={15} />
        返回
      </button>

      <div className="mt-4">
        <WeekHeader
          title={data?.title ?? null}
          weekLabel={data?.weekLabel ?? ""}
          level="h1"
          onPrev={data?.hasPrev ? () => w.goToWeek(shiftWeek(data.weekStart, -7)) : undefined}
          onNext={data && canGoNext(data.weekStart, data.isLatest) ? () => w.goToWeek(shiftWeek(data.weekStart, 7)) : undefined}
        />
      </div>

      {w.notStarted ? (
        <p className="mt-4 text-mk-body text-mk-muted">{w.notStarted}</p>
      ) : w.loadError ? (
        <LoadFailed message={w.loadError} onRetry={w.reload} />
      ) : data === null ? (
        <p className="mt-4 text-mk-body text-mk-muted">加载中…</p>
      ) : (
        <>
          <div className="mt-4 flex flex-wrap gap-3">
            <FactTile label="活跃学生" value={`${data.stats.activeStudents}/${data.stats.classSize} 人`} />
            <FactTile label="学习时长" value={formatMinutes(data.stats.minutes)} />
            <FactTile label="对话轮次" value={`${data.stats.turns} 轮`} />
            <FactTile label="完成项目数" value={`${data.stats.finished} 项`} />
            <FactTile
              label="作业完成率"
              value={data.stats.assignmentRate < 0 ? "—" : `${data.stats.assignmentRate}%`}
            />
          </div>

          <section className="mt-8 rounded-mk-lg border border-mk-border bg-mk-surface p-4 sm:p-5">
            <h2 className="text-mk-h3 text-mk-ink">班级点评</h2>
            {data.prose ? (
              <p className="mt-2 whitespace-pre-wrap text-mk-body text-mk-ink">{data.prose.comment}</p>
            ) : data.empty ? (
              <p className="mt-2 text-mk-body text-mk-muted">该周没有学习记录</p>
            ) : (
              <ProseStatus state={w.prose} onRetry={w.retryProse} />
            )}
          </section>

          <div className="mt-8 grid gap-6 lg:grid-cols-2">
            <StudentGroup
              title="值得表扬"
              cards={data.praise}
              prose={data.prose}
              watchUserIds={new Set(data.watch.map((c) => c.userId))}
              onOpenStudent={onOpenStudent}
            />
            <StudentGroup
              title="需要建议"
              cards={data.watch}
              prose={data.prose}
              watchUserIds={new Set(data.watch.map((c) => c.userId))}
              onOpenStudent={onOpenStudent}
            />
          </div>
        </>
      )}
    </TeacherPage>
  );
}

function StudentGroup({
  title,
  cards,
  prose,
  watchUserIds,
  onOpenStudent,
}: {
  title: string;
  cards: ClassWeekCard[];
  prose: ClassWeeklyProse | null;
  /** Students with a watch card: their lead and action go there only. */
  watchUserIds: ReadonlySet<string>;
  onOpenStudent: (userId: string) => void;
}) {
  return (
    <GroupShell title={title} empty={cards.length === 0}>
      {cards.map((c) => {
        const written = cardShowsProse(c.kind, c.userId, watchUserIds) ? classCardProse(prose, c.userId) : null;
        return (
          <button
            key={`${c.userId}:${c.code}`}
            type="button"
            onClick={() => onOpenStudent(c.userId)}
            className="w-full rounded-mk-md border border-mk-border bg-mk-surface p-3.5 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          >
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-mk-body font-bold text-mk-ink">{c.name}</span>
              <CardTag kind={c.kind} label={c.label} />
            </div>
            <p className="mt-1.5 text-mk-small text-mk-muted">{c.evidence}</p>
            {written && (
              <>
                <p className="mt-2 whitespace-pre-wrap text-mk-small text-mk-ink">{written.lead}</p>
                <p className="mt-1.5 whitespace-pre-wrap text-mk-small text-mk-ink">
                  <span className="font-bold">{c.kind === "praise" ? "鼓励：" : "沟通："}</span>
                  {written.action}
                </p>
              </>
            )}
          </button>
        );
      })}
    </GroupShell>
  );
}
