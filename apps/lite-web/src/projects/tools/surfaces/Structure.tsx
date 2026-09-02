import { useCallback, useEffect, useState } from "react";
import { ChevronRight, CornerDownRight, Plus, Trash2 } from "lucide-react";
import { Icon } from "@/ui";
import {
  CHECK_QUESTIONS,
  answerCheck,
  createNode,
  deleteNode,
  getTree,
  indentTarget,
  moveNode,
  outline,
  treeTodo,
  updateNode,
  type TreeNode,
  type TreeState,
} from "../../../api/tree";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";
import { apiErrorText } from "../../../api/errorText";

/**
 * Structure —— 先看结构。
 *
 * 产品负责人 2026-09-01：让她知道**这里有一个结构**，否则很多人会照着一个结构
 * 写完，从没想过它为什么长这样。所以这张图必须能改：能改，才会去想改不改。
 *
 * 画成缩进的大纲而不是画布上的连线图。右边这一栏只有 360px，连线图在这里既
 * 看不清也改不动；大纲的缩进就是层级，加一条、缩进去、提出来，三个按钮说完。
 *
 * 底下三个问题是她原话里的三条，答完才算看过这个结构。
 */
export function Structure({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [state, setState] = useState<TreeState>({ tree: "main", nodes: [], checks: [] });
  const [draft, setDraft] = useState("");
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setState(await getTree(projectId));
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const ordered = outline(state.nodes);

  function fail(msg: string) {
    return () => setError(msg);
  }

  async function add() {
    const title = draft.trim();
    if (!title) return;
    setDraft("");
    try {
      await createNode(projectId, { title, ordinal: state.nodes.length });
      await reload();
    } catch {
      fail("没加上，再试一次。")();
    }
  }

  async function indent(node: TreeNode) {
    const target = indentTarget(ordered, node.id);
    if (!target) return;
    try {
      await moveNode(projectId, node.id, { parentId: target.id, ordinal: node.ordinal });
      await reload();
    } catch (e) {
      setError(apiErrorText(e));
    }
  }

  async function outdent(node: TreeNode) {
    if (!node.parentId) return;
    const parent = state.nodes.find((n) => n.id === node.parentId);
    try {
      await moveNode(projectId, node.id, {
        parentId: parent?.parentId ?? "",
        ordinal: node.ordinal,
      });
      await reload();
    } catch {
      fail("挪不出来。")();
    }
  }

  async function rename(node: TreeNode, title: string) {
    try {
      await updateNode(projectId, node.id, { title });
      await reload();
    } catch {
      fail("没改成。")();
    }
  }

  async function setBody(node: TreeNode, body: string) {
    try {
      await updateNode(projectId, node.id, { body });
      await reload();
    } catch {
      fail("没记下。")();
    }
  }

  async function remove(node: TreeNode) {
    try {
      await deleteNode(projectId, node.id);
      await reload();
    } catch {
      fail("没删掉。")();
    }
  }

  async function saveCheck(question: (typeof CHECK_QUESTIONS)[number]["question"], answer: string) {
    try {
      await answerCheck(projectId, question, answer);
      await reload();
    } catch {
      fail("没记下。")();
    }
  }

  const todo = treeTodo(state);

  return (
    <ToolFrame
      title={tool.label}
      task="先把整体分成几块，再看这个分法对不对"
      why={tool.reason}
      todo={todo}
      finishLabel="结构就这样"
      onFinish={() =>
        onFinish({ nodes: state.nodes.length }, "")
      }
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      <div className="space-y-1">
        {ordered.map((n) => (
          <Row
            key={n.id}
            node={n}
            canIndent={!!indentTarget(ordered, n.id)}
            open={open === n.id}
            onToggle={() => setOpen(open === n.id ? null : n.id)}
            onRename={(t) => void rename(n, t)}
            onBody={(b) => void setBody(n, b)}
            onIndent={() => void indent(n)}
            onOutdent={() => void outdent(n)}
            onRemove={() => void remove(n)}
          />
        ))}
      </div>

      <div className="mt-2 flex items-end gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && void add()}
          placeholder="再分一块出来"
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

      {/* 三个问题 */}
      {state.nodes.length >= 3 && (
        <div className="mt-4 space-y-2 border-t border-mk-border pt-3">
          <p className="text-mk-small text-mk-muted">看着这张图，想三件事</p>
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

function Row({
  node,
  canIndent,
  open,
  onToggle,
  onRename,
  onBody,
  onIndent,
  onOutdent,
  onRemove,
}: {
  node: TreeNode;
  canIndent: boolean;
  open: boolean;
  onToggle: () => void;
  onRename: (t: string) => void;
  onBody: (b: string) => void;
  onIndent: () => void;
  onOutdent: () => void;
  onRemove: () => void;
}) {
  const [title, setTitle] = useState(node.title);
  const [body, setBody] = useState(node.body);
  useEffect(() => setTitle(node.title), [node.title]);
  useEffect(() => setBody(node.body), [node.body]);

  return (
    <div style={{ paddingLeft: node.depth * 16 }}>
      <div className="group flex items-center gap-1 rounded-mk-md border border-mk-border px-2 py-1.5">
        <button
          type="button"
          onClick={onToggle}
          aria-label="这一块要放什么"
          className="shrink-0 text-mk-faint"
          style={{ transform: open ? "rotate(90deg)" : undefined }}
        >
          <Icon icon={ChevronRight} size={13} />
        </button>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onBlur={() => title.trim() && title.trim() !== node.title && onRename(title.trim())}
          className="min-w-0 flex-1 bg-transparent text-mk-small text-mk-ink outline-none"
        />
        <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
          <button
            type="button"
            onClick={onOutdent}
            disabled={!node.parentId}
            aria-label="提出来"
            className="px-1 text-mk-small text-mk-faint disabled:opacity-30"
          >
            ←
          </button>
          <button
            type="button"
            onClick={onIndent}
            disabled={!canIndent}
            aria-label="缩进去"
            className="px-1 text-mk-small text-mk-faint disabled:opacity-30"
          >
            →
          </button>
          <button type="button" onClick={onRemove} aria-label="去掉" className="px-1 text-mk-faint">
            <Icon icon={Trash2} size={12} />
          </button>
        </div>
      </div>
      {open && (
        <div className="mt-1 flex items-start gap-1 pl-5">
          <Icon icon={CornerDownRight} size={12} className="mt-2 shrink-0 text-mk-faint" />
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            onBlur={() => body.trim() !== node.body.trim() && onBody(body.trim())}
            rows={2}
            placeholder="这一块要放什么"
            className="w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
          />
        </div>
      )}
    </div>
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
      <p className="mt-0.5 text-mk-small text-mk-muted">{hint}</p>
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onBlur={() => text.trim() !== value.trim() && onSave(text.trim())}
        rows={2}
        placeholder="一句"
        className="mt-1.5 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
      />
    </div>
  );
}
