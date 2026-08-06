import { useEffect, useState } from "react";
import { ChevronDown, Lightbulb } from "lucide-react";
import type { GrowthHistoryEntry } from "@mind-imprint/contracts";
import { api } from "@/api";
import { Card, EmptyState, Icon, SkeletonRow } from "@/ui";
import { DualAxisReport } from "@/shell/report/DualAxisReport";
// AbilityModel (能力素养) hidden 2026-07-31 for MVP focus — import removed to
// avoid an unused-symbol error; the component file stays for later restore.
// ToolkitCards (工具卡) moved to the top-level 图鉴 gallery tab (Task 7) —
// GrowthReport no longer renders an internal 工具卡 tab, only 学习记录.

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

const SURFACE_LABEL: Record<GrowthHistoryEntry["surface"], string> = {
  project: "项目", course: "课程", chat: "聊天",
};

function HistoryRow({ entry, open, onToggle }: { entry: GrowthHistoryEntry; open: boolean; onToggle: () => void }) {
  const date = entry.createdAt.slice(0, 10);
  return (
    <Card className="mt-3 overflow-hidden">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        className="flex w-full items-center gap-3 px-5 py-4 text-left font-sans"
      >
        <span className="shrink-0 rounded-mk-sm bg-mk-paper px-2.5 py-0.5 text-mk-small font-extrabold text-mk-secondary">
          {SURFACE_LABEL[entry.surface]}
        </span>
        <span className="min-w-0 flex-1 text-mk-h3 text-mk-ink">
          {entry.label}
          {entry.sublabel ? <span className="font-semibold text-mk-muted"> · {entry.sublabel}</span> : null}
        </span>
        <span className="shrink-0 text-mk-small text-mk-faint">{date}</span>
        <Icon
          icon={ChevronDown}
          size={16}
          className={cx("shrink-0 text-mk-faint transition-transform duration-150 ease-mk", open && "rotate-180")}
        />
      </button>
      {open && (
        <div className="px-5 pb-5">
          <DualAxisReport report={entry.report} />
        </div>
      )}
    </Card>
  );
}

function LearningRecord({ initialScopeId }: { initialScopeId?: string | null }) {
  const [entries, setEntries] = useState<GrowthHistoryEntry[] | undefined>(undefined);
  const [openId, setOpenId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await api.getGrowthHistory();
        if (cancelled) return;
        setEntries(list);
        // Deep-link: if arriving from a project's "查看评估报告", open that
        // entry; otherwise expand the newest.
        const focused = initialScopeId && list.find((e) => e.scopeId === initialScopeId);
        if (focused) setOpenId(`${focused.surface}:${focused.scopeId}`);
        else if (list.length > 0) setOpenId(`${list[0]!.surface}:${list[0]!.scopeId}`);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => { cancelled = true; };
  }, [initialScopeId]);

  return (
    <>
      <header className="flex items-center gap-4">
        <span className="flex h-14 w-14 shrink-0 items-center justify-center rounded-mk-full bg-mk-accent-50 text-mk-accent-600">
          <Icon icon={Lightbulb} size={26} />
        </span>
        <div className="min-w-0">
          <div className="text-mk-label text-mk-muted">成长报告</div>
          <h1 className="mt-0.5 text-mk-h1 text-mk-ink">你的思维印记</h1>
          <p className="mt-1 text-mk-body text-mk-muted">
            每完成一个任务、一节课，或在聊天里留下一次思维印记，都会汇集到这里——按真实过程给出的诊断，不是分数。
          </p>
        </div>
      </header>

      {error && (
        <div className="mt-4 rounded-mk-sm bg-mk-danger-bg px-3.5 py-2.5 text-mk-body text-mk-danger">{error}</div>
      )}

      {entries === undefined ? (
        <div className="mt-4 flex flex-col gap-3">
          {Array.from({ length: 3 }, (_, i) => (
            <Card key={i} className="p-5">
              <SkeletonRow />
            </Card>
          ))}
        </div>
      ) : entries.length === 0 ? (
        <Card className="mt-4 p-6">
          <EmptyState
            illustration="completed"
            title="还没有报告"
            body="完成一个任务、一节课，或在聊天里留下一次思维印记，报告会在这里汇集。"
          />
        </Card>
      ) : (
        entries.map((e) => {
          const id = `${e.surface}:${e.scopeId}`;
          return <HistoryRow key={id} entry={e} open={openId === id} onToggle={() => setOpenId(openId === id ? null : id)} />;
        })
      )}
    </>
  );
}

export function GrowthReport({ initialScopeId, onOpenCourse }: { initialScopeId?: string | null; onOpenCourse?: (courseId: string) => void } = {}) {
  // onOpenCourse is kept in the prop signature for call-site compatibility
  // with StudentApp (which still threads it through) but has no consumer
  // here now that 工具卡 (ToolkitCards) lives at the top-level 图鉴 tab
  // (Task 7) rather than as an internal tab of this component.
  void onOpenCourse;
  return (
    <div className="h-full min-h-0 flex-1 overflow-y-auto bg-mk-paper">
      <div className="mx-auto max-w-[760px] px-10 py-9">
        <LearningRecord initialScopeId={initialScopeId} />
      </div>
    </div>
  );
}
