import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Plus } from "lucide-react";
import { Icon } from "@/ui";
import { apiErrorText } from "../../../api/errorText";
import {
  CHECK_QUESTIONS,
  answerCheck,
  autoLayout,
  createNode,
  deleteNode,
  depthTone,
  getTree,
  moveNode,
  outline,
  treeTodo,
  updateNode,
  type TreeState,
} from "../../../api/tree";
import { ToolFrame } from "../ToolFrame";
import { listNotes, noteKindMeta, placeNote, type Note } from "../../../api/notes";
import type { ToolSurfaceProps } from "../registry";

/**
 * Structure —— 结构审查。
 *
 * 产品负责人 2026-09-02：
 *   「it should be a mindmap, and can directly drag and move and press delete
 *    to delete. we don't need so many buttons.」
 *
 * 所以这里只有三个动作：拖着挪、点一下选中、按 Delete 删。改字是双击。上一版
 * 每行挂着「提出来 / 缩进去 / 去掉」三个按钮加一个展开箭头——那是一份大纲的
 * 编辑器，不是一张能让人看见形状的图。
 *
 * 🚨 结构是印记提的。她做的是审视：这个分法盖全了吗、顺得下来吗、有没有更好的。
 * 那三个问题是**思考框架**，不是必答题——做成门槛，一次审视就变成一份问卷。
 */

const NODE_W = 168;
const NODE_H = 56;
// 🚨 画布按内容量出来，不写死。
//
// 写死 760×520 塞进一栏 360px 的面板里，她看到的是一小扇窗，窗外一片空白——
// 三个节点的图看着像坏了。下限保证空图时不至于塌成一条缝。
const MIN_W = 320;
const MIN_H = 240;
const PAD = 16;

export function Structure({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [state, setState] = useState<TreeState>({ tree: "main", nodes: [], checks: [] });
  const [picked, setPicked] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const canvasRef = useRef<HTMLDivElement>(null);
  // 她自己攒下来的材料。放进节点的、和放不进去的。
  const [notes, setNotes] = useState<Note[]>([]);
  // 正拿在手上准备放的那一条。
  const [holding, setHolding] = useState<string | null>(null);

  const loadNotes = useCallback(async () => {
    try {
      setNotes(await listNotes(projectId));
    } catch {
      // 材料拉不到，这张图照样能审——只是少了那把尺子。
    }
  }, [projectId]);
  useEffect(() => {
    void loadNotes();
  }, [loadNotes]);

  /**
   * 把手上那条材料放进一块，或者从结构里拿回来。
   *
   * 🚨 「这个分法盖全了吗」以前只能靠她盯着提纲想「大概全了吧」。材料一条条放
   * 进去之后，答案就在剩下的那几条里——放不进去的，就是这个结构没盖到的地方。
   */
  async function place(noteId: string, nodeId: string | null) {
    try {
      const got = await placeNote(projectId, noteId, nodeId);
      setNotes((prev) => prev.map((n) => (n.id === got.id ? got : n)));
      setHolding(null);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  const reload = useCallback(async () => {
    try {
      setState(await getTree(projectId));
    } catch (err) {
      setError(apiErrorText(err));
    }
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const ordered = useMemo(() => outline(state.nodes), [state.nodes]);

  /**
   * 每一块量出来的真实高度。
   *
   * 🚨 块高是 minHeight，标题加说明一换行就撑起来；而自动排版原来按固定行距往
   * 下排，于是长的那几块被后一块盖住半句。量一遍再排，形状才是真的。
   *
   * ResizeObserver 而不是渲染后读一次：字号、面板宽度、她双击改字都会改变高度。
   */
  const [heights, setHeights] = useState<Map<string, number>>(new Map());
  const obs = useRef<ResizeObserver | null>(null);
  if (!obs.current && typeof ResizeObserver !== "undefined") {
    obs.current = new ResizeObserver((entries) => {
      setHeights((prev) => {
        let changed = false;
        const next = new Map(prev);
        for (const e of entries) {
          const id = (e.target as HTMLElement).dataset.nodeId;
          if (!id) continue;
          const h = Math.round(e.contentRect.height);
          // 只在真的变了的时候 setState，否则观察→重排→再观察会自己转起来。
          if (next.get(id) !== h) {
            next.set(id, h);
            changed = true;
          }
        }
        return changed ? next : prev;
      });
    });
  }
  const measure = useCallback((el: HTMLDivElement | null) => {
    if (el) obs.current?.observe(el);
  }, []);
  useEffect(() => () => obs.current?.disconnect(), []);

  const layout = useMemo(() => autoLayout(state.nodes, heights), [state.nodes, heights]);
  /** 这一块占多高：量到了用量到的，没量到按最矮的算。 */
  const hOf = useCallback(
    (id: string) => Math.max(NODE_H, heights.get(id) ?? NODE_H),
    [heights],
  );
  const size = useMemo(() => {
    let w = MIN_W;
    let h = MIN_H;
    for (const [id, { x, y }] of layout) {
      w = Math.max(w, x + NODE_W + PAD);
      h = Math.max(h, y + hOf(id) + PAD);
    }
    return { w, h };
  }, [layout, hOf]);
  const at = useCallback(
    (id: string) => layout.get(id) ?? { x: 16, y: 16 },
    [layout],
  );

  // Delete 删掉选中的那一块。焦点在输入框里时不拦——那时候 Delete 是退格。
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (!picked || editing) return;
      const tag = (e.target as HTMLElement | null)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA") return;
      if (e.key !== "Delete" && e.key !== "Backspace") return;
      e.preventDefault();
      const id = picked;
      setPicked(null);
      setState((prev) => ({ ...prev, nodes: prev.nodes.filter((n) => n.id !== id) }));
      void deleteNode(projectId, id).catch(async (err) => {
        setError(apiErrorText(err));
        await reload();
      });
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [picked, editing, projectId, reload]);

  /** id 的所有后代。挂到自己的后代下面会做出一个环，那棵树就再也画不出来了。 */
  const descendantsOf = useCallback(
    (id: string) => {
      const out = new Set<string>([id]);
      let grew = true;
      while (grew) {
        grew = false;
        for (const n of state.nodes) {
          if (n.parentId && out.has(n.parentId) && !out.has(n.id)) {
            out.add(n.id);
            grew = true;
          }
        }
      }
      return out;
    },
    [state.nodes],
  );

  /**
   * 松手的地方压在谁身上。
   *
   * 返回 undefined = 没压在任何人身上，这一下只是挪位置。
   * 返回 null = 压在空白的最外层，挂回顶层。
   * 返回 id = 挂到那一块下面。
   */
  function dropTarget(id: string, at: { x: number; y: number }): string | null | undefined {
    const cx = at.x + NODE_W / 2;
    const cy = at.y + NODE_H / 2;
    const banned = descendantsOf(id);
    for (const n of state.nodes) {
      if (banned.has(n.id)) continue;
      const p = layout.get(n.id);
      if (!p) continue;
      if (cx >= p.x && cx <= p.x + NODE_W && cy >= p.y && cy <= p.y + NODE_H) {
        // 三层封顶，和「加一块」那边同一条线。
        return n.depth < 2 ? n.id : undefined;
      }
    }
    return undefined;
  }

  function startDrag(id: string, e: React.PointerEvent) {
    if (editing) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    const start = at(id);
    const grabX = e.clientX - rect.left - start.x;
    const grabY = e.clientY - rect.top - start.y;
    let moved = false;
    let last = start;

    const onMove = (ev: PointerEvent) => {
      moved = true;
      const x = Math.max(0, Math.min(rect.width - NODE_W, ev.clientX - rect.left - grabX));
      const y = Math.max(0, Math.min(rect.height - NODE_H, ev.clientY - rect.top - grabY));
      last = { x, y };
      setState((prev) => ({
        ...prev,
        nodes: prev.nodes.map((n) => (n.id === id ? { ...n, x, y } : n)),
      }));
    };
    const onUp = () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      if (!moved) {
        setPicked((p) => (p === id ? null : id));
        return;
      }
      // 🚨 拖到另一块上面 = 挂到它下面。
      //
      // 以前拖动只改 x/y，parentId 一个字没动——于是连线还指着原来的父节点，
      // 她越整理，图越乱：位置说的是一回事，线说的是另一回事。而「改结构」正是
      // 这一屏存在的理由（设计文档：让学生看见、**修改**）。moveNode 这个函数
      // 写好了很久，一次也没被调用过。
      const onto = dropTarget(id, last);
      if (onto !== undefined) {
        void moveNode(projectId, id, { parentId: onto ?? "" })
          .then(reload)
          .catch(async (err) => {
            setError(apiErrorText(err));
            await reload();
          });
        return;
      }
      void updateNode(projectId, id, { x: last.x, y: last.y }).catch((err) =>
        setError(apiErrorText(err)),
      );
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  }

  async function add() {
    const title = draft.trim();
    if (!title) return;
    setDraft("");
    try {
      // 加在选中那一块下面；没选就加在最外层。
      const parent = picked ? state.nodes.find((n) => n.id === picked) : null;
      await createNode(projectId, {
        title,
        parentId: parent && parent.depth < 3 ? parent.id : undefined,
        ordinal: state.nodes.length,
      });
      await reload();
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function rename(id: string, title: string) {
    try {
      await updateNode(projectId, id, { title });
      await reload();
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function saveCheck(q: (typeof CHECK_QUESTIONS)[number]["question"], answer: string) {
    try {
      await answerCheck(projectId, q, answer);
      await reload();
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  /** 收工时带走整张结构的内容，不是"我分成了 N 块"这种概述。 */
  function fullText(): string {
    return ordered
      .map((n) => "　".repeat(n.depth) + n.title + (n.body ? `：${n.body}` : ""))
      .join("\n");
  }

  return (
    <ToolFrame
      title={tool.label}
      task="从完整性、连贯性等角度审查整体结构是否合理"
      why={tool.reason}
      todo={treeTodo(state)}
      finishLabel="没有问题"
      onFinish={() => onFinish({ nodes: state.nodes.length, outline: fullText() }, "")}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {/* 图。横向可滚——一张摊得开的图比一列挤住的字有用。 */}
      <div className="-mx-1 overflow-auto rounded-mk-md border border-mk-border">
        <div
          ref={canvasRef}
          className="relative"
          style={{
            width: size.w,
            height: size.h,
            background: "var(--mk-paper)",
            backgroundImage:
              "radial-gradient(color-mix(in srgb, var(--mk-border) 55%, transparent) 1px, transparent 1px)",
            backgroundSize: "16px 16px",
            touchAction: "none",
          }}
        >
          {state.nodes.length === 0 && (
            <p className="absolute inset-0 flex items-center justify-center px-6 text-center text-mk-small text-mk-faint">
              还没有结构。印记给出提纲后，会在这里让你审核。
            </p>
          )}

          {/* 连线。画在节点下面。 */}
          <svg className="pointer-events-none absolute inset-0" width={size.w} height={size.h}>
            {state.nodes.map((n) => {
              if (!n.parentId) return null;
              const a = at(n.parentId);
              const b = at(n.id);
              // 连线跟着子节点的颜色走：同一支的线是同一个色，图才看得出分叉。
              // 曲线而不是直线——直角折线在这么小的画布上会糊成一团。
              const mx = (a.x + NODE_W + b.x) / 2;
              // 线接在两块各自的**腰**上。用固定 NODE_H 算，高的那几块线会从
              // 肩膀上斜着穿出去——线上那几道斜杠就是这么来的。
              const ay = a.y + hOf(n.parentId) / 2;
              const by = b.y + hOf(n.id) / 2;
              return (
                <path
                  key={n.id}
                  d={`M ${a.x + NODE_W} ${ay} C ${mx} ${ay}, ${mx} ${by}, ${b.x} ${by}`}
                  fill="none"
                  stroke={depthTone(n.depth).solid}
                  strokeOpacity={0.5}
                  strokeWidth={1.5}
                />
              );
            })}
          </svg>

          {state.nodes.map((n) => {
            const pos = at(n.id);
            const on = picked === n.id;
            return (
              <div
                key={n.id}
                ref={measure}
                data-node-id={n.id}
                onPointerDown={(e) => {
                  // 手上拿着一条材料时，点一块就是放进去——这一下比拖更稳，
                  // 尤其在这么小的画布上。
                  if (holding) {
                    void place(holding, n.id);
                    return;
                  }
                  startDrag(n.id, e);
                }}
                onDoubleClick={() => setEditing(n.id)}
                className="absolute select-none rounded-mk-md border px-2.5 py-2 shadow-mk-xs"
                style={{
                  left: pos.x,
                  top: pos.y,
                  width: NODE_W,
                  minHeight: NODE_H,
                  cursor: editing === n.id ? "text" : "grab",
                  // 🚨 整块淡底表示"第几层"，不用左侧色条。
                  // 产品负责人 2026-09-03：「I hate left color bar designs,
                  // especially when we have a huge list of that」——一张图上
                  // 十几个节点各挂一条竖带，看着就是一排栅栏。
                  borderColor: on ? depthTone(n.depth).solid : "var(--mk-border)",
                  background: depthTone(n.depth).bg,
                  color: depthTone(n.depth).fg,
                  boxShadow: on
                    ? `0 0 0 2px color-mix(in srgb, ${depthTone(n.depth).solid} 35%, transparent)`
                    : undefined,
                }}
              >
                {editing === n.id ? (
                  <input
                    autoFocus
                    defaultValue={n.title}
                    onPointerDown={(e) => e.stopPropagation()}
                    onBlur={(e) => {
                      setEditing(null);
                      const v = e.target.value.trim();
                      if (v && v !== n.title) void rename(n.id, v);
                    }}
                    className="w-full rounded-mk-sm border border-mk-input-border bg-mk-surface px-1.5 py-0.5 text-mk-small text-mk-ink outline-none"
                  />
                ) : (
                  <p className="break-words text-mk-small">{n.title}</p>
                )}
                {n.body && <p className="mt-0.5 text-mk-small text-mk-muted">{n.body}</p>}
                {/* 这一块底下压着几条材料。 */}
                {notes.filter((x) => x.treeNodeId === n.id).length > 0 && (
                  <p className="mt-0.5 text-mk-small" style={{ color: depthTone(n.depth).solid }}>
                    材料 {notes.filter((x) => x.treeNodeId === n.id).length}
                  </p>
                )}
              </div>
            );
          })}
        </div>
      </div>

      <p className="mt-1.5 text-mk-small text-mk-faint">
        拖着挪位置，拖到另一块上面就挂到它下面，点一下选中，Delete 删掉，双击改字。
      </p>

      {/* 🚨 材料托盘。这是「盖全了吗」的那把尺子：
          她自己攒的观察、原话、推论、问题，一条一条放进结构里；
          **放不进去的那几条就是这个结构没盖到的地方**。
          那几条是她亲手收集的，比任何自评都硬。 */}
      {notes.length > 0 && (
        <div className="mt-3 border-t border-mk-border pt-3">
          <div className="flex items-baseline justify-between">
            <p className="text-mk-body font-semibold text-mk-ink">放不进去的材料</p>
            <p className="text-mk-small text-mk-muted">
              已放进 {notes.filter((n) => n.treeNodeId).length} / {notes.length}
            </p>
          </div>
          <p className="mt-0.5 text-mk-small text-mk-muted">
            {holding
              ? "请点结构里的一块，把它放进去。"
              : "请点一条材料，再点它该属于的那一块。剩下的就是这个结构没盖到的地方。"}
          </p>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {notes
              .filter((n) => !n.treeNodeId)
              .map((n) => {
                const meta = noteKindMeta(n.kind);
                const on = holding === n.id;
                return (
                  <button
                    key={n.id}
                    type="button"
                    onClick={() => setHolding(on ? null : n.id)}
                    className="max-w-[220px] truncate rounded-mk-md px-2 py-1 text-mk-small"
                    style={{
                      background: `color-mix(in srgb, ${meta.hue} ${on ? 30 : 14}%, var(--mk-surface))`,
                      outline: on ? `2px solid ${meta.hue}` : undefined,
                    }}
                  >
                    {n.body}
                  </button>
                );
              })}
            {notes.every((n) => n.treeNodeId) && (
              <p className="text-mk-small" style={{ color: "var(--mk-success)" }}>
                材料都放进去了。
              </p>
            )}
          </div>
        </div>
      )}
      <p className="hidden">
      </p>

      <div className="mt-2 flex items-end gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && void add()}
          placeholder={picked ? "在选中的那一块下面加一块" : "加一块"}
          className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
        <button
          type="button"
          onClick={() => void add()}
          disabled={!draft.trim()}
          aria-label="加一块"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Plus} size={15} />
        </button>
      </div>

      {/* 三个问题：思考框架，不是必答题。 */}
      {state.nodes.length > 0 && (
        <div className="mt-4 space-y-2 border-t border-mk-border pt-3">
          <p className="text-mk-small text-mk-muted">建议的三个问题</p>
          {CHECK_QUESTIONS.map((q) => (
            <CheckRow
              key={q.question}
              ask={q.ask}
              hint={q.hint}
              value={state.checks.find((c) => c.question === q.question)?.answer ?? ""}
              onSave={(a) => void saveCheck(q.question, a)}
            />
          ))}
        </div>
      )}
    </ToolFrame>
  );
}

function CheckRow({
  ask,
  hint,
  value,
  onSave,
}: {
  ask: string;
  hint: string;
  value: string;
  onSave: (v: string) => void;
}) {
  const [text, setText] = useState(value);
  useEffect(() => setText(value), [value]);
  return (
    <div className="rounded-mk-md border border-mk-border px-3 py-2">
      <p className="text-mk-small text-mk-ink">{ask}</p>
      {hint && <p className="mt-0.5 text-mk-small text-mk-muted">{hint}</p>}
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text.trim() !== value.trim() && onSave(text.trim())}
        rows={2}
        placeholder="想到什么都可以写在这里"
        className="mt-1.5 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </div>
  );
}
