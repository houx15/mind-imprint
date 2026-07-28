import { useEffect, useState } from "react";
import type { MaterialSource, WorkspaceProjection } from "@mind-imprint/contracts";
import { api } from "../api";
import { ReadingRoom } from "../studio/reading/ReadingRoom";
import { Icon, BLOCK_META } from "./Icon";
import { Directory } from "./Directory";
import { getWorkspace } from "./api/workspace";
import { PlanBlock } from "./blocks/PlanBlock";
import { ReadingBlock } from "./blocks/ReadingBlock";
import { WritingBlock } from "./blocks/WritingBlock";
import { ReviewBlock } from "./blocks/ReviewBlock";
import type { BlockKey, PlanItem } from "./blocks/mockData";

// The top-level workspace: a project is a set of four rooms
// (项目管理 · 阅读 · 写作 · 回顾) the student moves between freely. No open
// project ⇒ the <Directory>; an open project ⇒ the left rail + the active
// room, landing in Project Management.
//
// The blocks run on local mock state this slice (real per-room API wiring is
// slices 2–5); only the shell (identity, qualification, the room swap) is live.
export function WorkspaceContainer({ onFinished }: { onFinished?: () => void }) {
  const [projectId, setProjectId] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<WorkspaceProjection | null>(null);
  const [room, setRoom] = useState<BlockKey>("plan");
  const [error, setError] = useState<string | null>(null);
  // The reading-room swap slot. When set, the focused ReadingRoom surface
  // replaces the rooms entirely (mirrors the shipped studio's own swap).
  // Nothing sets it yet — slice 3 wires a source-open to it; the mechanism
  // (and its onBack teardown) already exists.
  const [readingSource, setReadingSource] = useState<MaterialSource | null>(null);

  // Load the opened project's lean projection (title/qualification/proposal)
  // whenever the opened id changes. Landing room is always 项目管理.
  useEffect(() => {
    if (!projectId) return;
    let cancelled = false;
    setWorkspace(null);
    setError(null);
    (async () => {
      try {
        const w = await getWorkspace(projectId);
        if (!cancelled) setWorkspace(w);
      } catch {
        if (!cancelled) setError("加载失败，请重试");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [projectId]);

  function openProject(id: string) {
    setProjectId(id);
    setRoom("plan");
    setReadingSource(null);
  }

  function backToAll() {
    setProjectId(null);
    setWorkspace(null);
    setReadingSource(null);
    setError(null);
  }

  // Clicking a plan card jumps to the matching room (the doorway model).
  function openItem(item: PlanItem) {
    if (item.tag === "read") setRoom("reading");
    else if (item.tag === "write") setRoom("writing");
    else setRoom("reflection");
  }

  // No project open — the all-projects directory (its own create form carries
  // the empty affordance).
  if (projectId == null) {
    return <Directory onOpen={openProject} />;
  }

  // A source open for reading replaces the whole workspace with the focused
  // ReadingRoom surface — one level in from the directory↔workspace swap.
  if (readingSource != null) {
    return (
      <ReadingRoom
        projectId={projectId}
        source={readingSource}
        api={api}
        onBack={() => setReadingSource(null)}
      />
    );
  }

  return (
    <div className="flex h-full w-full bg-mk-bg font-sans text-mk-ink">
      <Rail room={room} onRoom={setRoom} onBack={backToAll} workspace={workspace} />
      <main className="min-w-0 flex-1 overflow-hidden">
        {error ? (
          <div className="flex h-full items-center justify-center text-[14px] font-semibold text-mk-accent">{error}</div>
        ) : (
          <>
            {room === "plan" && <PlanBlock fresh={false} onOpenItem={openItem} />}
            {room === "reading" && <ReadingBlock fresh={false} />}
            {room === "writing" && <WritingBlock />}
            {room === "reflection" && <ReviewBlock />}
          </>
        )}
      </main>
    </div>
  );
}

function Rail({
  room,
  onRoom,
  onBack,
  workspace,
}: {
  room: BlockKey;
  onRoom: (b: BlockKey) => void;
  onBack: () => void;
  workspace: WorkspaceProjection | null;
}) {
  return (
    <nav className="flex w-[236px] flex-none flex-col border-r border-mk-border bg-mk-surface">
      <div className="border-b border-mk-border px-5 py-5">
        <button type="button" onClick={onBack} className="mb-3 flex items-center gap-1 text-[13px] font-semibold text-mk-muted-2 hover:text-mk-primary">
          <Icon name="back" size={16} /> 全部项目
        </button>
        <h1 className="font-sans text-[15.5px] font-bold leading-snug text-mk-ink">{workspace?.title || "未命名项目"}</h1>
        <span className="mt-2 inline-block rounded-full bg-mk-primary-tint px-2.5 py-1 text-[11px] font-bold text-mk-primary">{workspace?.qualification || "项目"}</span>
      </div>

      <div className="flex flex-1 flex-col gap-1 px-3 py-4">
        {BLOCK_META.map((b, i) => {
          const active = room === b.key;
          return (
            <button
              key={b.key}
              type="button"
              onClick={() => onRoom(b.key)}
              className={`group relative flex items-center gap-3 rounded-mk px-3 py-2.5 text-left transition ${
                active ? "bg-mk-primary-tint" : "hover:bg-mk-bg"
              }`}
            >
              {active && <span className="absolute left-0 top-1/2 h-6 w-1 -translate-y-1/2 rounded-r bg-mk-primary" />}
              <span className={active ? "text-mk-primary" : "text-mk-muted-2 group-hover:text-mk-muted"}>
                <Icon name={b.key} size={19} />
              </span>
              <span className="flex flex-col">
                <span className={`text-[14px] font-bold ${active ? "text-mk-primary" : "text-mk-ink"}`}>{b.label}</span>
                <span className="text-[11px] font-medium tracking-wide text-mk-muted-2">
                  {String(i + 1).padStart(2, "0")} · {b.sub}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </nav>
  );
}
