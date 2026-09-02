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
  getTree,
  outline,
  treeTodo,
  updateNode,
  type TreeState,
} from "../../../api/tree";
import { ToolFrame } from "../ToolFrame";
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
const CANVAS_W = 760;
const CANVAS_H = 520;

export function Structure({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [state, setState] = useState<TreeState>({ tree: "main", nodes: [], checks: [] });
  const [picked, setPicked] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState<string | null>(null);
  const canvasRef = useRef<HTMLDivElement>(null);

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
  const layout = useMemo(() => autoLayout(state.nodes), [state.nodes]);
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
      const x = Math.max(0, Math.min(CANVAS_W - NODE_W, ev.clientX - rect.left - grabX));
      const y = Math.max(0, Math.min(CANVAS_H - NODE_H, ev.clientY - rect.top - grabY));
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
            width: CANVAS_W,
            height: CANVAS_H,
            background: "var(--mk-paper)",
            backgroundImage:
              "radial-gradient(color-mix(in srgb, var(--mk-border) 55%, transparent) 1px, transparent 1px)",
            backgroundSize: "16px 16px",
            touchAction: "none",
          }}
        >
          {state.nodes.length === 0 && (
            <p className="absolute inset-0 flex items-center justify-center px-6 text-center text-mk-small text-mk-faint">
              还没有结构。到对话里请印记先给一个，再在这里审。
            </p>
          )}

          {/* 连线。画在节点下面。 */}
          <svg className="pointer-events-none absolute inset-0" width={CANVAS_W} height={CANVAS_H}>
            {state.nodes.map((n) => {
              if (!n.parentId) return null;
              const a = at(n.parentId);
              const b = at(n.id);
              return (
                <line
                  key={n.id}
                  x1={a.x + NODE_W}
                  y1={a.y + NODE_H / 2}
                  x2={b.x}
                  y2={b.y + NODE_H / 2}
                  stroke="var(--mk-border)"
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
                onPointerDown={(e) => startDrag(n.id, e)}
                onDoubleClick={() => setEditing(n.id)}
                className="absolute select-none rounded-mk-md border bg-mk-surface px-2.5 py-2 shadow-mk-xs"
                style={{
                  left: pos.x,
                  top: pos.y,
                  width: NODE_W,
                  minHeight: NODE_H,
                  cursor: editing === n.id ? "text" : "grab",
                  borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)",
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
                  <p className="break-words text-mk-small text-mk-ink">{n.title}</p>
                )}
                {n.body && <p className="mt-0.5 text-mk-small text-mk-muted">{n.body}</p>}
              </div>
            );
          })}
        </div>
      </div>

      <p className="mt-1.5 text-mk-small text-mk-faint">
        拖着挪位置，点一下选中，Delete 删掉，双击改字。
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
