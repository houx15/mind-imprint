import { useEffect, useState } from "react";
import { ArrowRight } from "lucide-react";
import { Button, Icon } from "@/ui";
import { api } from "@/api";
import { classCardProse, getClassWeekly, postClassWeeklyProse, type ClassWeekCard, type ClassWeeklyProse } from "../api/weekly";
import bookmark from "../home/assets/yinji-bookmark.webp";
import { formatMinutes } from "./format";
import { canGoNext, shiftWeek, splitWeekTitle } from "./weekNav";
import { useWeekly } from "./useWeekly";
import { cardShowsProse } from "./classWeeklyLogic";
import { TeacherPage } from "./TeacherPage";
import { BackLink, StudioEmpty, StudioError, StudioHeading, StudioLoading } from "./StudioArtwork";
import { CardTag, FactTile, GroupShell, ProseStatus, WeekNav } from "./WeekSummaryCard";

/**
 * ClassWeeklyPage — `/classes/:classId/weekly`. The class's week: stat tiles,
 * 班级点评 (model prose, asked for once per week per mount, see `useWeekly`),
 * and the students the rules flagged, as 值得表扬 / 需要建议 cards. A card
 * opens that student's page. Each card's lead and action come from the same
 * prose request; before it lands the card shows the rule's evidence only.
 *
 * The title comes from the API (上周班级周报 · … for the latest completed week,
 * 班级周报 · … for an earlier one): its head is the page title, the week
 * label sits beside the week arrows. Prose renders as plain text.
 *
 * The shell keys this page by class id.
 */
export function ClassWeeklyPage({
  classId,
  onBack,
  onOpenStudent,
  onOpenChat,
  onNewAssignment,
}: {
  classId: string;
  onBack: () => void;
  onOpenStudent: (userId: string) => void;
  onOpenChat?: () => void;
  onNewAssignment?: () => void;
}) {
  const w = useWeekly({
    scope: classId,
    load: (ws) => getClassWeekly(classId, ws),
    post: (ws) => postClassWeeklyProse(classId, ws),
    wantsProse: (week) => !week.empty,
  });
  const { data } = w;

  // The class name is the kicker only; on failure the kicker says 班级周报.
  const [className, setClassName] = useState<string | null>(null);
  useEffect(() => {
    let cancelled = false;
    api
      .getClass(classId)
      .then((d) => {
        if (!cancelled) setClassName(d.class.name);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [classId]);

  const { head, label } = splitWeekTitle(data?.title ?? "", data?.weekLabel ?? "");
  const title = head.replace(/\s*·\s*$/, "") || "班级周报";

  return (
    <TeacherPage width="wide">
      <BackLink label="返回班级" onClick={onBack} />

      <StudioHeading
        kicker={className ?? "班级周报"}
        title={title}
        description="按周汇总本班的学习记录，并列出值得表扬和需要沟通的学生。"
        kind="quest"
        actions={
          <>
            <WeekNav
              label={label || data?.weekLabel || ""}
              onPrev={data?.hasPrev ? () => w.goToWeek(shiftWeek(data.weekStart, -7)) : undefined}
              onNext={data && canGoNext(data.weekStart, data.isLatest) ? () => w.goToWeek(shiftWeek(data.weekStart, 7)) : undefined}
            />
            {onOpenChat && (
              <Button variant="secondary" size="sm" onClick={onOpenChat}>
                询问印记
              </Button>
            )}
          </>
        }
      />

      {w.notStarted ? (
        <StudioEmpty kind="quest" title="暂无周报">
          {w.notStarted}
        </StudioEmpty>
      ) : w.loadError ? (
        <StudioError message={w.loadError} onRetry={w.reload} />
      ) : data === null ? (
        <StudioLoading />
      ) : data.empty ? (
        <StudioEmpty
          kind="discovery"
          title="该周没有学习记录"
          action={onNewAssignment ? { label: "布置作业", onClick: onNewAssignment } : undefined}
        >
          {`本班 ${data.stats.classSize} 名学生在这一周没有使用平台。周报在有学习记录后生成。请布置作业，或查看其他周。`}
        </StudioEmpty>
      ) : (
        <>
          <div className="teacher-facts">
            <FactTile label="活跃学生" value={`${data.stats.activeStudents}/${data.stats.classSize} 人`} />
            <FactTile label="学习时长" value={formatMinutes(data.stats.minutes)} />
            <FactTile label="对话轮次" value={`${data.stats.turns} 轮`} />
            <FactTile label="完成项目数" value={`${data.stats.finished} 项`} />
            <FactTile
              label="作业完成率"
              value={data.stats.assignmentRate < 0 ? "—" : `${data.stats.assignmentRate}%`}
            />
          </div>

          <section className="teacher-panel mt-6">
            <div className="teacher-prose-head">
              <img src={bookmark} alt="" />
              <div>
                <h2 className="teacher-panel-title">班级点评</h2>
                <p>印记根据本周的学习记录整理</p>
              </div>
            </div>
            {data.prose ? (
              <p className="mt-3 whitespace-pre-wrap text-mk-body leading-relaxed text-mk-ink">{data.prose.comment}</p>
            ) : (
              <ProseStatus state={w.prose} onRetry={w.retryProse} />
            )}
          </section>

          <div className="mt-8 grid gap-6 lg:grid-cols-2">
            <StudentGroup
              title="值得表扬"
              emptyText="本周记录尚未触发表扬建议"
              cards={data.praise}
              prose={data.prose}
              watchUserIds={new Set(data.watch.map((c) => c.userId))}
              onOpenStudent={onOpenStudent}
            />
            <StudentGroup
              title="需要建议"
              emptyText="本周记录尚未触发沟通建议"
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
  emptyText,
  cards,
  prose,
  watchUserIds,
  onOpenStudent,
}: {
  title: string;
  emptyText: string;
  cards: ClassWeekCard[];
  prose: ClassWeeklyProse | null;
  /** Students with a watch card: their lead and action go there only. */
  watchUserIds: ReadonlySet<string>;
  onOpenStudent: (userId: string) => void;
}) {
  return (
    <GroupShell title={title} count={cards.length} emptyText={emptyText}>
      {cards.map((c) => {
        const written = cardShowsProse(c.kind, c.userId, watchUserIds) ? classCardProse(prose, c.userId) : null;
        return (
          <button
            key={`${c.userId}:${c.code}`}
            type="button"
            onClick={() => onOpenStudent(c.userId)}
            className="teacher-week-student"
          >
            <div className="flex flex-wrap items-center gap-2">
              <span className="teacher-avatar">{Array.from(c.name)[0]}</span>
              <span className="text-mk-body font-bold text-mk-ink">{c.name}</span>
              <CardTag kind={c.kind} label={c.label} />
            </div>
            <p className="mt-2 text-mk-small text-mk-muted">{c.evidence}</p>
            {written && (
              <>
                <p className="mt-2 whitespace-pre-wrap text-mk-small text-mk-ink">{written.lead}</p>
                <p className="mt-1.5 whitespace-pre-wrap text-mk-small text-mk-ink">
                  <span className="font-bold">{c.kind === "praise" ? "鼓励：" : "沟通："}</span>
                  {written.action}
                </p>
              </>
            )}
            <span className="teacher-week-student-open">
              查看学生
              <Icon icon={ArrowRight} size={14} />
            </span>
          </button>
        );
      })}
    </GroupShell>
  );
}
