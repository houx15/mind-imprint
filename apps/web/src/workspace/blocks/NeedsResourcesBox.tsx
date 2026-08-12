import { useEffect, useRef, useState } from "react";
import type { ResourceNeed } from "@mind-imprint/contracts";
import { getResourceNeeds, putResourceNeeds } from "../../api/resourceNeeds";
import { Icon } from "../Icon";

// NeedsResourcesBox — slice 5 (§101/§115) · the "还需要探索的" box. While writing (or
// reading), the student jots things they realise they need to look up; a 去探索 jump
// carries them into the reading room. Persisted per project (whole-list PUT,
// debounced). Deliberately small + low-chrome (design 铁律: tool not toy) — it never
// nags, it just holds the note until the student chooses to explore (铁律②).

// A local id for freshly-added rows before the server mints one (kept stable so
// the row's input doesn't remount mid-edit). Derived from a monotonic counter.
let localSeq = 0;

export function NeedsResourcesBox({
  projectId,
  onExplore,
  reloadKey,
}: {
  projectId: string;
  // Jump to the reading room to explore. Optional `note` carries the item text.
  onExplore?: (note?: string) => void;
  // Bug 7 · bumped by the container when the coach added a keyword via
  // note_resource_need, so the box re-fetches to show it.
  reloadKey?: number;
}) {
  const [needs, setNeeds] = useState<ResourceNeed[]>([]);
  const [draft, setDraft] = useState("");
  const ref = useRef<ResourceNeed[]>([]);
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const loaded = await getResourceNeeds(projectId);
        if (cancelled) return;
        ref.current = loaded;
        setNeeds(loaded);
      } catch {
        /* leave empty */
      }
    })();
    return () => {
      cancelled = true;
      if (saveTimer.current) clearTimeout(saveTimer.current);
    };
  }, [projectId, reloadKey]);

  function commit(next: ResourceNeed[]) {
    ref.current = next;
    setNeeds(next);
    if (saveTimer.current) clearTimeout(saveTimer.current);
    saveTimer.current = setTimeout(() => {
      void putResourceNeeds(projectId, next).catch(() => {});
    }, 600);
  }

  function add() {
    const text = draft.trim();
    if (text === "") return;
    commit([...ref.current, { id: `local-${++localSeq}`, text, done: false }]);
    setDraft("");
  }
  function toggle(id: string) {
    commit(ref.current.map((n) => (n.id === id ? { ...n, done: !n.done } : n)));
  }
  function remove(id: string) {
    commit(ref.current.filter((n) => n.id !== id));
  }

  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-surface p-3">
      <div className="flex items-center gap-2">
        <Icon name="explore" />
        <h4 className="text-[13px] font-bold text-mk-ink">还需要探索的</h4>
        {onExplore && (
          <button
            type="button"
            onClick={() => onExplore()}
            className="ml-auto rounded-mk border border-mk-border px-2 py-0.5 text-[12px] font-semibold text-mk-accent hover:bg-mk-accent-50"
          >
            去阅读室探索
          </button>
        )}
      </div>

      {needs.length > 0 && (
        <ul className="mt-2 flex flex-col gap-1">
          {needs.map((n) => (
            <li key={n.id} className="group flex items-center gap-2">
              <input
                type="checkbox"
                checked={n.done}
                onChange={() => toggle(n.id)}
                className="h-3.5 w-3.5 flex-none accent-mk-accent"
                aria-label="标记已探索"
              />
              <span className={`min-w-0 flex-1 truncate text-[13px] ${n.done ? "text-mk-faint line-through" : "text-mk-ink"}`}>{n.text}</span>
              {onExplore && !n.done && (
                <button type="button" onClick={() => onExplore(n.text)} className="flex-none text-[12px] font-semibold text-mk-accent opacity-0 hover:underline group-hover:opacity-100">去探索</button>
              )}
              <button type="button" onClick={() => remove(n.id)} className="flex-none text-[14px] leading-none text-mk-faint opacity-0 hover:text-mk-danger group-hover:opacity-100" aria-label="删除">×</button>
            </li>
          ))}
        </ul>
      )}

      <div className="mt-2 flex items-center gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); add(); } }}
          placeholder="写下想查的资料/方向……"
          className="min-w-0 flex-1 rounded-mk border border-mk-border bg-mk-paper px-2 py-1 text-[13px] text-mk-ink placeholder:text-mk-faint focus:border-mk-accent focus:outline-none"
        />
        <button type="button" onClick={add} disabled={draft.trim() === ""} className="flex-none rounded-mk border border-mk-border px-2 py-1 text-[12px] font-bold text-mk-accent hover:bg-mk-accent-50 disabled:opacity-40">记下</button>
      </div>
    </div>
  );
}
