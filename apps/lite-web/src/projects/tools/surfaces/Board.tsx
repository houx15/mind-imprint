import { Says, errorMarkdown } from "../../Says";
import { SelectionTray } from "../board/SelectionTray";
import { studentArtwork } from "../../../learning/StudentArtwork";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  listNoteLinks,
  linkNotes,
  unlinkNotes,
  NOTE_RELATIONS,
  noteKindMeta,
  updateNote,
  type Note,
  type NoteKind,
} from "../../../api/notes";
import { ToolFrame } from "../ToolFrame";
import { resolveUrl } from "../../../api/oss";
import type { NoteLink, NoteRelation } from "../../../api/notes";
import { NOTE_H, NOTE_W, boardSpot } from "../boardLayout";
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

// 尺寸与落位从 boardLayout 来——「观察日记」带回来的便签走的是同一份网格。
// 见 boardLayout.ts。
// 板子的最小高度。真实高度按便签量长出来——见 boardH。
//
// 🚨 2026-09-02：原来这是**固定**高度，配 overflow-hidden。每行两张、行距
// 102px，第 9 张落在 y=420，底边 508 > 460——被裁掉，拖不着，也找不回来。
// 一块"贴到第九张就开始吃便签"的头脑风暴板。
const BOARD_MIN_H = 460;

/**
 * 坐标视图下，上下各留出来的一条。
 *
 * 🚨 四个轴标签压在便签下面（它们是底图）。不留这一条，落在 y=0 的便签正好
 * 盖住「很要紧」——而那一行恰恰是这块板最该被看见的一头。
 */
const AXIS_GUTTER = 22;


/**
 * 坐标视图的两根轴。
 *
 * 🚨 横轴是「有多确定」，纵轴是「有多要紧」——这两件事最容易被当成一件。
 * 拆开之后，右上角（很要紧、但我在猜）就自己浮出来了，那几条正是她接下来该去
 * 弄清楚的东西。这块板真正的产出是那一角，不是一堆摆整齐的纸。
 *
 * 轴写死在这里，不进库：换一对轴是产品决定，不是每个项目各自的数据。
 */
const AXIS = {
  xLeft: "我确定",
  xRight: "我在猜",
  yTop: "很要紧",
  yBottom: "关系不大",
} as const;

export function Board({
  projectId,
  boardAxes,
  onSetBoardAxes,
  tool,
  onFinish,
  onClose,
}: ToolSurfaceProps) {
  const [notes, setNotes] = useState<Note[]>([]);
  const [kind, setKind] = useState<NoteKind | null>(null);
  const [draft, setDraft] = useState("");
  const [savingNote, setSavingNote] = useState(false);
  const [seen, setSeen] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
  const [dragging, setDragging] = useState<{ id: string; x: number; y: number } | null>(null);
  const dragCleanup = useRef<(() => void) | null>(null);
  useEffect(() => () => dragCleanup.current?.(), []);
  // 她连出来的关系。矛盾那几条是这块板最要紧的产出。
  const [links, setLinks] = useState<NoteLink[]>([]);

  const loadLinks = useCallback(async () => {
    try {
      setLinks(await listNoteLinks(projectId));
    } catch {
      // 连线拉不到，板子照样能用——只是少了那几根线。
    }
  }, [projectId]);
  useEffect(() => {
    void loadLinks();
  }, [loadLinks]);

  /** 把选中的两张连起来，并说清楚是哪一种关系。 */
  async function link(relation: NoteRelation) {
    if (picked.length !== 2) return;
    try {
      const got = await linkNotes(projectId, picked[0]!, picked[1]!, relation);
      setLinks((prev) => [...prev.filter((l) => l.id !== got.id), got]);
      setPicked([]);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function cutLink(id: string) {
    try {
      await unlinkNotes(projectId, id);
      setLinks((prev) => prev.filter((l) => l.id !== id));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }
  const [editing, setEditing] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const boardRef = useRef<HTMLDivElement>(null);
  // 🚨 板子按最低那张便签长。固定高度 + overflow-hidden 会把第 9 张之后的
  // 便签裁掉，而且拖不着、找不回——一块吃便签的头脑风暴板。外层容器负责滚动。
  const boardH = useMemo(
    () => notes.reduce((h, n) => Math.max(h, n.y + NOTE_H + 12), BOARD_MIN_H),
    [notes],
  );
  // 🚨 下一张便签落在第几个位子，用 ref 同步地取，不看 notes.length。
  //
  // notes.length 来自这一帧的闭包：她连着敲三次回车，三次 add 拿到的都是同一个
  // 长度，三张便签会摞在同一个位置上——e2e 里就是这么撞出来的（后一张挡住了
  // 前一张，点不中）。座位号必须在 await 之前就定下来。
  const seatRef = useRef(0);

  const reload = useCallback(async () => {
    const loaded = await listNotes(projectId);
    setNotes(loaded);
    // 🚨 只涨不跌。reload 是异步的：StrictMode 的二次挂载（或者任何一次晚到的
    // 刷新）会在她已经贴了几张之后才 resolve，把座位号打回去——于是后面几张又
    // 从 0 开始，直接压在前几张身上。
    seatRef.current = Math.max(seatRef.current, loaded.length);
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function add() {
    const body = draft.trim();
    if (!body || !kind || savingNote) return;
    setSavingNote(true);
    setDraft("");
    try {
      const seat = seatRef.current++;
      const [made] = await createNotes(projectId, [{ kind, body }]);
      if (!made) return;
      const spot = boardSpot(seat);
      const placed = await moveNote(projectId, made.id, spot.x, spot.y);
      setNotes((prev) => [...prev, placed]);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setSavingNote(false);
    }
  }

  /**
   * 拖动。落下才写库——拖的过程里每一帧发一次请求没有必要。
   *
   * 没挪动就是一次点击，用来选中。两个动作合在一个手势里，是因为对她来说
   * 「碰一下这张纸」本来就是一件事。
   */
  /**
   * 板子当前能放纸的范围。坐标视图下存的是 0–1，渲染时乘回来。
   *
   * 🚨 axes 要显式传进来，不能读 boardAxes：toggleAxes 换算时用的是**新**的模式，
   * 而那一刻 state 还是旧的。
   */
  function span(axes: boolean) {
    const rect = boardRef.current?.getBoundingClientRect();
    const gutter = axes ? AXIS_GUTTER * 2 : 0;
    return {
      w: Math.max(1, (rect?.width ?? NOTE_W * 2) - NOTE_W),
      h: Math.max(1, boardH - NOTE_H - gutter),
    };
  }

  /** 存进库的值 → 屏幕上的像素。 */
  function toPx(n: Note) {
    if (!boardAxes) return { x: n.x, y: n.y };
    const s = span(true);
    return { x: n.x * s.w, y: AXIS_GUTTER + n.y * s.h };
  }

  /**
   * 开/关坐标视图。
   *
   * 🚨 两种模式下 x/y 的单位不一样（像素 vs 0–1），所以切换时要把现有的便签
   * 换算一遍——不换算，纸会全部堆到左上角或者飞出板外。
   */
  async function toggleAxes() {
    const on = !boardAxes;
    const s = span(on);
    try {
      for (const n of notes) {
        const next = on
          ? {
              x: Math.min(1, n.x / s.w),
              y: Math.min(1, Math.max(0, (n.y - AXIS_GUTTER) / s.h)),
            }
          : { x: n.x * s.w, y: AXIS_GUTTER + n.y * s.h };
        setNotes((prev) => prev.map((m) => (m.id === n.id ? { ...m, ...next } : m)));
        await moveNote(projectId, n.id, next.x, next.y);
      }
      await onSetBoardAxes(on);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  function startDrag(note: Note, e: React.PointerEvent) {
    if (editing || e.button !== 0 || dragCleanup.current) return;
    const board = boardRef.current;
    if (!board) return;
    const rect = board.getBoundingClientRect();
    const at = toPx(note);
    const grabX = e.clientX - rect.left - at.x;
    const grabY = e.clientY - rect.top - at.y;
    let moved = false;
    let last = { x: note.x, y: note.y };

    const onMove = (ev: PointerEvent) => {
      if (!moved && Math.abs(ev.clientX-e.clientX)+Math.abs(ev.clientY-e.clientY)<5) return;
      if (!moved) setDragging({ id: note.id, ...at });
      moved = true;
      const px = Math.max(0, Math.min(rect.width - NOTE_W, ev.clientX - rect.left - grabX));
      // 坐标视图下上下各让开一条，便签不许压住「很要紧 / 关系不大」两个标签。
      const lo = boardAxes ? AXIS_GUTTER : 0;
      const hi = boardH - NOTE_H - lo;
      const py = Math.max(lo, Math.min(hi, ev.clientY - rect.top - grabY));
      // 坐标视图下存相对值：位置是一句判断，不该跟着面板宽度变。
      const s2 = span(boardAxes);
      const x = boardAxes ? px / s2.w : px;
      const y = boardAxes ? (py - AXIS_GUTTER) / s2.h : py;
      last = { x, y };
      setNotes((prev) => prev.map((n) => (n.id === note.id ? { ...n, x, y } : n)));
    };
    const detach = () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", cancel);
      window.removeEventListener("keydown", onKey);
      dragCleanup.current = null;
    };
    const cancel = () => {
      detach(); setDragging(null);
      if (moved) setNotes(prev => prev.map(n => n.id === note.id ? { ...n, x: note.x, y: note.y } : n));
    };
    const onKey = (ev: KeyboardEvent) => {
      if (ev.key === "Escape") { ev.preventDefault(); cancel(); }
    };
    const onUp = () => {
      detach(); setDragging(null);
      if (!moved) {
        setPicked((prev) =>
          prev.includes(note.id) ? prev.filter((x) => x !== note.id) : [...prev, note.id],
        );
        return;
      }
      // 🚨 这一处、也只有这一处传 dragged=true：她真的用手把这张纸挪到了那儿。
      // 排座位和坐标换算走的是同一个函数，但那两次不是她的判断。
      void moveNote(projectId, note.id, last.x, last.y, true).catch((err) =>
        setError(apiErrorText(err)),
      );
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    window.addEventListener("pointercancel", cancel);
    window.addEventListener("keydown", onKey);
    dragCleanup.current = detach;
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
  // The board supports a single prediction as well as grouping many notes.
  // Quantity and clustering describe student work; they are not completion gates.
  const todo = draft.trim()
    ? "请添加正在输入的便签，或清空输入"
    : notes.length === 0
      ? "请添加一条记录"
      : "";

  return (
    <ToolFrame
      title={tool.label}
      task="请记录当前想法；需要比较时，可移动、分类或连接便签"
      why={tool.reason}
      todo={todo}
      busy={savingNote}
      finishLabel="完成并讨论"
      onFinish={() => onFinish({ noteCount: notes.length, clusters }, seen.trim())}
      onClose={onClose}
    >
      {error && (
        <div className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          <Says content={errorMarkdown(error)} />
        </div>
      )}

      <p className="mb-2 text-mk-small text-mk-secondary">请标明记录类型。预测与猜测请选择「我的推论」。</p>
      <div className="flex flex-wrap gap-1" role="group" aria-label="记录类型">
        {NOTE_KINDS.map((k) => (
          <button
            key={k.kind}
            type="button"
            onClick={() => setKind(k.kind)}
            aria-pressed={kind === k.kind}
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
          disabled={!draft.trim() || !kind || savingNote}
          aria-label="贴上去"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Plus} size={15} />
        </button>
      </div>

      <div className="mt-3 max-h-[460px] overflow-y-auto rounded-mk-md border border-mk-border">
      <div
        ref={boardRef}
        className="relative"
        style={{
          height: boardH,
          background: "var(--mk-paper)",
          backgroundImage:
            "radial-gradient(color-mix(in srgb, var(--mk-border) 60%, transparent) 1px, transparent 1px)",
          backgroundSize: "14px 14px",
          touchAction: "none",
        }}
      >
        {dragging && <div className="student-drag-origin" aria-hidden="true" style={{left:dragging.x,top:dragging.y,width:NOTE_W,height:NOTE_H}} />}
        {/* 坐标底。两条线 + 四个角的标签，压在便签下面。 */}
        {boardAxes && (
          <div className="pointer-events-none absolute inset-0">
            <div
              className="absolute left-0 right-0 top-1/2"
              style={{ borderTop: "1px dashed var(--mk-border)" }}
            />
            <div
              className="absolute bottom-0 top-0 left-1/2"
              style={{ borderLeft: "1px dashed var(--mk-border)" }}
            />
            <span className="absolute left-1.5 top-1/2 -translate-y-1/2 text-mk-small text-mk-faint">
              {AXIS.xLeft}
            </span>
            <span className="absolute right-1.5 top-1/2 -translate-y-1/2 text-mk-small text-mk-faint">
              {AXIS.xRight}
            </span>
            <span className="absolute left-1/2 top-1 -translate-x-1/2 text-mk-small text-mk-faint">
              {AXIS.yTop}
            </span>
            <span className="absolute bottom-1 left-1/2 -translate-x-1/2 text-mk-small text-mk-faint">
              {AXIS.yBottom}
            </span>
            {/* 右上角是这块板真正的产出：又要紧、又没把握的那几条。 */}
            <span
              className="absolute right-2 top-5 rounded-mk-md px-2 py-0.5 text-mk-small"
              style={{ background: "var(--mk-warning-bg)", color: "var(--mk-ink)" }}
            >
              先去弄清楚这一角
            </span>
          </div>
        )}

        {/* 🚨 她连出来的线，画在便签下面。矛盾那几根用 berry——两条都是她亲眼
            看到的却互相打架，那正是真正的问题冒出来的地方，不该和别的线一个样。 */}
        <svg className="pointer-events-none absolute inset-0 h-full w-full">
          {links.map((l) => {
            const a = notes.find((n) => n.id === l.fromId);
            const b = notes.find((n) => n.id === l.toId);
            if (!a || !b) return null;
            const pa = toPx(a);
            const pb = toPx(b);
            const meta = NOTE_RELATIONS.find((r) => r.relation === l.relation);
            return (
              <line
                key={l.id}
                x1={pa.x + NOTE_W / 2}
                y1={pa.y + NOTE_H / 2}
                x2={pb.x + NOTE_W / 2}
                y2={pb.y + NOTE_H / 2}
                stroke={meta?.hue ?? "var(--mk-border)"}
                strokeWidth={l.relation === "contradicts" ? 2.5 : 1.5}
                strokeDasharray={l.relation === "contradicts" ? "5 3" : undefined}
              />
            );
          })}
        </svg>

        {notes.length === 0 && (
          <div className="student-tool-empty absolute inset-0"><img src={studentArtwork.ideas} alt="" /><p>暂无便签。请在上方记录第一条材料。</p></div>
        )}

        {notes.map((n) => {
          const meta = noteKindMeta(n.kind);
          const on = picked.includes(n.id);
          return (
            <div
              key={n.id}
              onPointerDown={(e) => startDrag(n, e)}
              onDoubleClick={() => setEditing(n.id)}
              data-selected={on || undefined}
              className="student-sticky group absolute select-none rounded-mk-md px-2.5 py-2 shadow-mk-xs"
              style={{
                left: toPx(n).x,
                top: toPx(n).y,
                width: NOTE_W,
                minHeight: NOTE_H,
                zIndex: dragging?.id === n.id ? 20 : undefined,
                transform: dragging?.id === n.id ? "rotate(-3deg) scale(1.035)" : undefined,
                boxShadow: dragging?.id === n.id ? "0 18px 30px -12px color-mix(in srgb,var(--mk-ink) 35%,transparent)" : undefined,
                cursor: editing === n.id ? "text" : "grab",
                background: `color-mix(in srgb, ${meta.hue} 14%, var(--mk-surface))`,
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
              {/* 她带回来的那张照片。key 存在库里，URL 现换——签出来的会过期。 */}
              {n.imageKey && <NotePhoto objectKey={n.imageKey} />}

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


      </div>
      </div>

      <SelectionTray title="材料关系" items={notes.filter(n => picked.includes(n.id))} onRemove={id => setPicked(prev => prev.filter(x => x !== id))}>
                {picked.length > 0 && naming && (
          <div className="flex w-full items-center gap-2 rounded-mk-full border border-mk-border bg-mk-surface px-3 py-1.5 shadow-mk-xs">
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
              确认
            </button>
          </div>
        )}

        {/* 🚨 原来是 `left-1/2 -translate-x-1/2`：绝对定位从中线起算，可用宽度
            只剩画布的一半。选中两张时这一行有 8 个孩子，挤不下就逐个缩到
            min-content——「互相矛盾」被排成了一列四个字，一个字一行。
            改成 inset-x + mx-auto 拿到整幅宽度，并允许换行；每个孩子
            nowrap + 不许收缩，宁可多占一行也不许再拆字。 */}
        {picked.length > 0 && !naming && (
          <div className="mx-auto flex w-fit max-w-full flex-wrap items-center justify-center gap-x-2 gap-y-1 rounded-mk-lg border border-mk-border bg-mk-surface px-3 py-1.5 shadow-mk-xs">
            <span className="shrink-0 whitespace-nowrap text-mk-small text-mk-secondary">
              选了 {picked.length} 张
            </span>
            <button
              type="button"
              onClick={() => setNaming(true)}
              disabled={picked.length < 2}
              className="shrink-0 whitespace-nowrap rounded-mk-full px-2.5 py-0.5 text-mk-small text-white disabled:opacity-40"
              style={{ background: "var(--mk-accent-500)" }}
            >
              归成一堆
            </button>
            {/* 🚨 正好选中两张时，才谈得上「它们之间是什么关系」。 */}
            {picked.length === 2 &&
              NOTE_RELATIONS.map((r) => (
                <button
                  key={r.relation}
                  type="button"
                  onClick={() => void link(r.relation)}
                  className="shrink-0 whitespace-nowrap rounded-mk-full px-2 py-0.5 text-mk-small"
                  style={{
                    background: `color-mix(in srgb, ${r.hue} 18%, transparent)`,
                    color: "var(--mk-ink)",
                  }}
                >
                  {r.label}
                </button>
              ))}
            <button
              type="button"
              onClick={() => void regroup("")}
              className="shrink-0 whitespace-nowrap text-mk-small text-mk-secondary"
            >
              拆开
            </button>
            <button
              type="button"
              onClick={() => setPicked([])}
              className="shrink-0 whitespace-nowrap text-mk-small text-mk-faint"
            >
              取消
            </button>
          </div>
        )}
      </SelectionTray>

      {/* 🚨 矛盾单独列出来，不只画成一根线。
          两条都是她亲眼看到的、却互相打架——真正的问题几乎都从那儿长出来，
          而一根画在图上的虚线太容易被略过。 */}
      {links.some((l) => l.relation === "contradicts") && (
        <div
          className="mt-3 rounded-mk-md px-3 py-2.5"
          style={{ background: "var(--mk-berry-bg)" }}
        >
          <p className="text-mk-small font-semibold" style={{ color: "var(--mk-berry-fg)" }}>
            对不上的两条
          </p>
          <p className="mt-0.5 text-mk-small text-mk-muted">
            两边都是你看到的，却打架。请先弄清楚这里发生了什么。
          </p>
          {links
            .filter((l) => l.relation === "contradicts")
            .map((l) => {
              const a = notes.find((n) => n.id === l.fromId);
              const b = notes.find((n) => n.id === l.toId);
              if (!a || !b) return null;
              return (
                <div key={l.id} className="mt-1.5 flex items-start gap-2">
                  <p className="min-w-0 flex-1 text-mk-small text-mk-ink">
                    {a.body} ↔ {b.body}
                  </p>
                  <button
                    type="button"
                    onClick={() => void cutLink(l.id)}
                    className="shrink-0 text-mk-small text-mk-faint"
                  >
                    拆开
                  </button>
                </div>
              );
            })}
        </div>
      )}

      {/* 坐标视图的开关。 */}
      <div className="mt-1.5 flex items-center justify-between gap-2">
        <p className="text-mk-small text-mk-faint">拖着挪位置，点一下选中，双击改字。</p>
        <button
          type="button"
          onClick={() => void toggleAxes()}
          className="shrink-0 rounded-mk-full border border-mk-border px-2.5 py-0.5 text-mk-small"
          style={
            boardAxes
              ? { background: "var(--mk-accent-500)", color: "var(--mk-surface)", borderColor: "transparent" }
              : { color: "var(--mk-secondary)" }
          }
        >
          坐标视图
        </button>
      </div>

      {clusters.length > 0 && (
        <p className="mt-1 text-mk-small text-mk-muted">
          已经归了 {clusters.length} 堆：{clusters.join("、")}
        </p>
      )}

      <div className="mt-4 border-t border-mk-border pt-3">
        <label className="text-mk-small text-mk-secondary">补充思考（选填）</label>
        <textarea
          value={seen}
          onChange={(e) => setSeen(e.target.value)}
          rows={3}
          placeholder="请记录发现的联系、差异或待确认的问题"
          className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
      </div>
    </ToolFrame>
  );
}

/**
 * NotePhoto —— 便签上那张照片。
 *
 * 🚨 库里存的是 OSS 的 key，不是 URL：签出来的 URL 几分钟就过期。所以这里挂载
 * 时现换一个签好的 GET，换不到就什么都不显示——一张碎图比没有图更让人以为是
 * 自己弄丢了东西。
 */
function NotePhoto({ objectKey }: { objectKey: string }) {
  const [url, setUrl] = useState("");
  useEffect(() => {
    let alive = true;
    void resolveUrl(objectKey)
      .then((u) => alive && setUrl(u))
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [objectKey]);
  if (!url) return null;
  return (
    <img
      src={url}
      alt="她拍的"
      draggable={false}
      className="mt-1 max-h-24 w-full rounded-mk-sm object-cover"
    />
  );
}
