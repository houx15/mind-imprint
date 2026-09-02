import { useCallback, useEffect, useRef, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Icon } from "@/ui";
import { apiErrorText } from "../../../api/errorText";
import {
  NOTE_KINDS,
  archiveNote,
  clusterNotes,
  createNotes,
  listNotes,
  moveNote,
  noteKindMeta,
  updateNote,
  type Note,
  type NoteKind,
} from "../../../api/notes";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Board —— 一块真的板，不是一张分了组的清单。
 *
 * 产品负责人 2026-09-02：「like sticky notes, we can let students do things like
 * drag, add, modify, and reframe, like a small game.」
 *
 * 上一版是按堆分的几列，只能在列之间拖。这一版便签摊在一块板上，位置由她自己
 * 摆（x/y 落库），可以随便挪、点着选、把选中的几张归成一堆。
 *
 * 🚨 摆位置本身是有意义的，不只是好玩：把两张纸挪到一起，是"我觉得这两件事有
 * 关系"这个判断的第一次出手，而且它发生在她能把这句话说出口之前。所以位置要
 * 存下来，不能每次打开重排。
 */

const NOTE_W = 148;
const NOTE_H = 88;
const BOARD_H = 460;

/** 新便签落在哪：沿网格铺开，之后她自己挪。 */
function nextSpot(n: number): { x: number; y: number } {
  const perRow = 2;
  return { x: 12 + (n % perRow) * (NOTE_W + 14), y: 12 + Math.floor(n / perRow) * (NOTE_H + 14) };
}

export function Board({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [notes, setNotes] = useState<Note[]>([]);
  const [kind, setKind] = useState<NoteKind>("observation");
  const [draft, setDraft] = useState("");
  const [seen, setSeen] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
  const [editing, setEditing] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const boardRef = useRef<HTMLDivElement>(null);
  // 🚨 下一张便签落在第几个位子，用 ref 同步地取，不看 notes.length。
  //
  // notes.length 来自这一帧的闭包：她连着敲三次回车，三次 add 拿到的都是同一个
  // 长度，三张便签会摞在同一个位置上——e2e 里就是这么撞出来的（后一张挡住了
  // 前一张，点不中）。座位号必须在 await 之前就定下来。
  const seatRef = useRef(0);

  const reload = useCallback(async () => {
    const loaded = await listNotes(projectId);
    setNotes(loaded);
    seatRef.current = loaded.length;
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function add() {
    const body = draft.trim();
    if (!body) return;
    setDraft("");
    try {
      const seat = seatRef.current++;
      const [made] = await createNotes(projectId, [{ kind, body }]);
      if (!made) return;
      const spot = nextSpot(seat);
      const placed = await moveNote(projectId, made.id, spot.x, spot.y);
      setNotes((prev) => [...prev, placed]);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  /**
   * 拖动。落下才写库——拖的过程里每一帧发一次请求没有必要。
   *
   * 没挪动就是一次点击，用来选中。两个动作合在一个手势里，是因为对她来说
   * 「碰一下这张纸」本来就是一件事。
   */
  function startDrag(note: Note, e: React.PointerEvent) {
    if (editing) return;
    const board = boardRef.current;
    if (!board) return;
    const rect = board.getBoundingClientRect();
    const grabX = e.clientX - rect.left - note.x;
    const grabY = e.clientY - rect.top - note.y;
    let moved = false;
    let last = { x: note.x, y: note.y };

    const onMove = (ev: PointerEvent) => {
      moved = true;
      const x = Math.max(0, Math.min(rect.width - NOTE_W, ev.clientX - rect.left - grabX));
      const y = Math.max(0, Math.min(BOARD_H - NOTE_H, ev.clientY - rect.top - grabY));
      last = { x, y };
      setNotes((prev) => prev.map((n) => (n.id === note.id ? { ...n, x, y } : n)));
    };
    const onUp = () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      if (!moved) {
        setPicked((prev) =>
          prev.includes(note.id) ? prev.filter((x) => x !== note.id) : [...prev, note.id],
        );
        return;
      }
      void moveNote(projectId, note.id, last.x, last.y).catch((err) => setError(apiErrorText(err)));
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  }

  async function regroup(cluster: string) {
    if (!picked.length) return;
    try {
      const got = await clusterNotes(projectId, picked, cluster);
      const byId = new Map(got.map((n) => [n.id, n]));
      setNotes((prev) => prev.map((n) => byId.get(n.id) ?? n));
      setPicked([]);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  // 起名用板上的一个输入框，不用 window.prompt——系统弹窗会把她从板上拽走，
  // 而她正要一边看着这几张纸一边想它们的共同点叫什么。
  const [naming, setNaming] = useState(false);
  const [pileName, setPileName] = useState("");

  async function saveBody(id: string, body: string) {
    try {
      const got = await updateNote(projectId, id, { body });
      setNotes((prev) => prev.map((n) => (n.id === got.id ? got : n)));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function remove(id: string) {
    setNotes((prev) => prev.filter((n) => n.id !== id));
    setPicked((prev) => prev.filter((x) => x !== id));
    try {
      await archiveNote(projectId, id);
    } catch {
      await reload();
    }
  }

  const clusters = [...new Set(notes.map((n) => n.cluster.trim()).filter(Boolean))];
  const grouped = notes.filter((n) => n.cluster.trim()).length;
  const todo =
    notes.length < 3
      ? `再贴 ${3 - notes.length} 张`
      : grouped === 0
        ? "把有关系的几张挪到一起，归成一堆"
        : seen.trim()
          ? ""
          : "一句你看出来的东西";

  return (
    <ToolFrame
      title={tool.label}
      task="把看到的、听到的、猜的、想问的贴上来，再把有关系的挪到一起"
      why={tool.reason}
      todo={todo}
      onFinish={() => onFinish({ noteCount: notes.length, clusters }, seen.trim())}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      <div className="flex flex-wrap gap-1">
        {NOTE_KINDS.map((k) => (
          <button
            key={k.kind}
            type="button"
            onClick={() => setKind(k.kind)}
            className="rounded-mk-full px-2.5 py-1 text-mk-small"
            style={
              kind === k.kind
                ? { background: k.hue, color: "#fff" }
                : {
                    background: `color-mix(in srgb, ${k.hue} 14%, transparent)`,
                    color: "var(--mk-secondary)",
                  }
            }
          >
            {k.label}
          </button>
        ))}
      </div>
      <div className="mt-2 flex items-end gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && void add()}
          placeholder="写一条，回车贴上去"
          className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
        <button
          type="button"
          onClick={() => void add()}
          disabled={!draft.trim()}
          aria-label="贴上去"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Plus} size={15} />
        </button>
      </div>

      <div
        ref={boardRef}
        className="relative mt-3 overflow-hidden rounded-mk-md border border-mk-border"
        style={{
          height: BOARD_H,
          background: "var(--mk-paper)",
          backgroundImage:
            "radial-gradient(color-mix(in srgb, var(--mk-border) 60%, transparent) 1px, transparent 1px)",
          backgroundSize: "14px 14px",
          touchAction: "none",
        }}
      >
        {notes.length === 0 && (
          <p className="absolute inset-0 flex items-center justify-center px-6 text-center text-mk-small text-mk-faint">
            板上还什么都没有。先把你想到的一条一条贴上来。
          </p>
        )}

        {notes.map((n) => {
          const meta = noteKindMeta(n.kind);
          const on = picked.includes(n.id);
          return (
            <div
              key={n.id}
              onPointerDown={(e) => startDrag(n, e)}
              onDoubleClick={() => setEditing(n.id)}
              className="group absolute select-none rounded-mk-md px-2.5 py-2 shadow-mk-xs"
              style={{
                left: n.x,
                top: n.y,
                width: NOTE_W,
                minHeight: NOTE_H,
                cursor: editing === n.id ? "text" : "grab",
                background: `color-mix(in srgb, ${meta.hue} 14%, var(--mk-surface))`,
                borderLeft: `3px solid ${meta.hue}`,
                outline: on ? "2px solid var(--mk-accent-500)" : undefined,
                outlineOffset: 1,
              }}
            >
              <div className="flex items-start justify-between gap-1">
                <span className="text-mk-small" style={{ color: meta.hue }}>
                  {meta.label}
                </span>
                <button
                  type="button"
                  aria-label="拿下来"
                  onPointerDown={(e) => e.stopPropagation()}
                  onClick={() => void remove(n.id)}
                  className="text-mk-faint opacity-0 transition-opacity group-hover:opacity-100"
                >
                  <Icon icon={Trash2} size={12} />
                </button>
              </div>

              {editing === n.id ? (
                <textarea
                  autoFocus
                  defaultValue={n.body}
                  onPointerDown={(e) => e.stopPropagation()}
                  onBlur={(e) => {
                    setEditing(null);
                    const v = e.target.value.trim();
                    if (v && v !== n.body) void saveBody(n.id, v);
                  }}
                  rows={3}
                  className="mt-1 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-1.5 py-1 text-mk-small text-mk-ink outline-none"
                />
              ) : (
                <p className="mt-0.5 break-words text-mk-small text-mk-ink">{n.body}</p>
              )}

              {n.cluster.trim() && (
                <span className="mt-1 inline-block rounded-mk-full bg-mk-surface px-1.5 text-mk-small text-mk-secondary">
                  {n.cluster}
                </span>
              )}
              {n.author === "yinji" && (
                <p className="mt-0.5 text-mk-small text-mk-faint">
                  {n.edited ? "印记写的，你改过" : "印记写的"}
                </p>
              )}
            </div>
          );
        })}

        {picked.length > 0 && naming && (
          <div className="absolute bottom-2 left-1/2 flex w-[86%] -translate-x-1/2 items-center gap-2 rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1.5 shadow-mk-xs">
            <input
              autoFocus
              value={pileName}
              onChange={(e) => setPileName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && pileName.trim()) {
                  void regroup(pileName.trim());
                  setPileName("");
                  setNaming(false);
                }
                if (e.key === "Escape") setNaming(false);
              }}
              placeholder="这几张是一回事，因为……"
              className="min-w-0 flex-1 bg-transparent text-mk-small text-mk-ink outline-none placeholder:text-mk-faint"
            />
            <button
              type="button"
              disabled={!pileName.trim()}
              onClick={() => {
                void regroup(pileName.trim());
                setPileName("");
                setNaming(false);
              }}
              className="shrink-0 rounded-mk-full px-2.5 py-0.5 text-mk-small text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              就叫这个
            </button>
          </div>
        )}

        {picked.length > 0 && !naming && (
          <div className="absolute bottom-2 left-1/2 flex -translate-x-1/2 items-center gap-2 rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1.5 shadow-mk-xs">
            <span className="text-mk-small text-mk-secondary">选了 {picked.length} 张</span>
            <button
              type="button"
              onClick={() => setNaming(true)}
              disabled={picked.length < 2}
              className="rounded-mk-full px-2.5 py-0.5 text-mk-small text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              归成一堆
            </button>
            <button
              type="button"
              onClick={() => void regroup("")}
              className="text-mk-small text-mk-secondary"
            >
              拆开
            </button>
            <button
              type="button"
              onClick={() => setPicked([])}
              className="text-mk-small text-mk-faint"
            >
              取消
            </button>
          </div>
        )}
      </div>

      <p className="mt-1.5 text-mk-small text-mk-faint">拖着挪位置，点一下选中，双击改字。</p>

      {clusters.length > 0 && (
        <p className="mt-1 text-mk-small text-mk-muted">
          已经归了 {clusters.length} 堆：{clusters.join("、")}
        </p>
      )}

      <div className="mt-4 border-t border-mk-border pt-3">
        <label className="text-mk-small text-mk-secondary">这些摆在一起，你看出什么了？</label>
        <textarea
          value={seen}
          onChange={(e) => setSeen(e.target.value)}
          rows={3}
          placeholder="一句话就够"
          className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
      </div>
    </ToolFrame>
  );
}
