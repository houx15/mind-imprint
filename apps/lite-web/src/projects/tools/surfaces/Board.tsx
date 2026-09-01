import { useCallback, useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Icon } from "@/ui";
import {
  NOTE_KINDS,
  archiveNote,
  createNotes,
  groupByCluster,
  listNotes,
  noteKindMeta,
  updateNote,
  type Note,
  type NoteKind,
} from "../../../api/notes";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Board —— 便签板。
 *
 * 板上的操作只有一个真正要动脑：**把哪些放一起**。写便签是记录，归堆是思考——
 * 一堆零散的观察变成一条线索，就发生在她把两张便签拖到一起的那一下。所以未归
 * 类的那一列永远在最前面，堆的名字由她起。
 *
 * 收工要她写一句"看出了什么"。没有这一句，这块板就只是一次整理。
 */
export function Board({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [notes, setNotes] = useState<Note[]>([]);
  const [kind, setKind] = useState<NoteKind>("observation");
  const [draft, setDraft] = useState("");
  const [seen, setSeen] = useState("");
  const [dragging, setDragging] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setNotes(await listNotes(projectId));
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function add() {
    const body = draft.trim();
    if (!body) return;
    setDraft("");
    try {
      const made = await createNotes(projectId, [{ kind, body }]);
      setNotes((prev) => [...prev, ...made]);
    } catch {
      setError("这张没贴上去，再试一次。");
    }
  }

  async function moveTo(noteId: string, cluster: string) {
    const before = notes;
    setNotes((prev) => prev.map((n) => (n.id === noteId ? { ...n, cluster } : n)));
    try {
      await updateNote(projectId, noteId, { cluster });
    } catch {
      setNotes(before);
      setError("没挪过去，再试一次。");
    }
  }

  async function edit(noteId: string, body: string) {
    try {
      const got = await updateNote(projectId, noteId, { body });
      setNotes((prev) => prev.map((n) => (n.id === got.id ? got : n)));
    } catch {
      setError("没改成，再试一次。");
    }
  }

  async function remove(noteId: string) {
    setNotes((prev) => prev.filter((n) => n.id !== noteId));
    try {
      await archiveNote(projectId, noteId);
    } catch {
      await reload();
    }
  }

  async function newCluster() {
    const name = window.prompt("这一堆叫什么？");
    if (!name?.trim() || !dragging) return;
    await moveTo(dragging, name.trim());
  }

  const groups = groupByCluster(notes);
  const clustered = notes.filter((n) => n.cluster.trim() !== "").length;

  const todo =
    notes.length < 3
      ? "板上至少三张便签"
      : clustered === 0
        ? "把放得到一起的先归成一堆"
        : seen.trim()
          ? ""
          : "一句你看出的东西";

  return (
    <ToolFrame
      title={tool.label}
      task="把看到的、听到的、猜的、想问的都摊开，再把放得到一起的归成一堆"
      why={tool.reason}
      todo={todo}
      onFinish={() => onFinish({ noteCount: notes.length, seen: seen.trim() }, seen.trim())}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {/* 贴一张 */}
      <div className="mb-3">
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
            onKeyDown={(e) => {
              if (e.key === "Enter") void add();
            }}
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
      </div>

      {/* 板 */}
      <div className="space-y-3">
        {groups.map((g) => (
          <section
            key={g.cluster || "__none"}
            onDragOver={(e) => e.preventDefault()}
            onDrop={() => dragging && void moveTo(dragging, g.cluster)}
            className="rounded-mk-md border border-dashed border-mk-border p-2"
          >
            <p className="mb-1.5 text-mk-small text-mk-muted">
              {g.cluster || "还没归堆"}
              <span className="ml-1 text-mk-faint">{g.notes.length}</span>
            </p>
            {g.notes.length === 0 && (
              <p className="px-1 py-2 text-mk-small text-mk-faint">把便签拖到这里</p>
            )}
            <div className="space-y-1.5">
              {g.notes.map((n) => (
                <NoteCard
                  key={n.id}
                  note={n}
                  onDragStart={() => setDragging(n.id)}
                  onEdit={(body) => void edit(n.id, body)}
                  onRemove={() => void remove(n.id)}
                />
              ))}
            </div>
          </section>
        ))}

        {dragging && (
          <button
            type="button"
            onClick={() => void newCluster()}
            className="w-full rounded-mk-md border border-dashed border-mk-border py-2 text-mk-small text-mk-secondary"
          >
            放进一个新的堆…
          </button>
        )}
      </div>

      {/* 看出什么 */}
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

function NoteCard({
  note,
  onDragStart,
  onEdit,
  onRemove,
}: {
  note: Note;
  onDragStart: () => void;
  onEdit: (body: string) => void;
  onRemove: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState(note.body);
  const meta = noteKindMeta(note.kind);

  return (
    <div
      draggable={!editing}
      onDragStart={onDragStart}
      className="group rounded-mk-md px-2.5 py-2"
      style={{
        background: `color-mix(in srgb, ${meta.hue} 12%, transparent)`,
        borderLeft: `3px solid ${meta.hue}`,
      }}
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-mk-small" style={{ color: meta.hue }}>
          {meta.label}
        </span>
        <button
          type="button"
          onClick={onRemove}
          aria-label="拿下来"
          className="shrink-0 text-mk-faint opacity-0 transition-opacity group-hover:opacity-100"
        >
          <Icon icon={Trash2} size={13} />
        </button>
      </div>
      {editing ? (
        <textarea
          autoFocus
          value={text}
          onChange={(e) => setText(e.target.value)}
          onBlur={() => {
            setEditing(false);
            if (text.trim() && text.trim() !== note.body) onEdit(text.trim());
            else setText(note.body);
          }}
          rows={2}
          className="mt-1 w-full resize-none rounded-mk-sm border border-mk-input-border bg-mk-surface px-2 py-1 text-mk-small text-mk-ink outline-none"
        />
      ) : (
        <p
          onClick={() => setEditing(true)}
          className="mt-0.5 cursor-text text-mk-small text-mk-ink"
        >
          {note.body}
        </p>
      )}
      {note.author === "yinji" && (
        // 印记写的和她写的要分得开。她改过的也标出来——改动本身是她的思考。
        <p className="mt-1 text-mk-small text-mk-faint">
          {note.edited ? "印记写的，你改过" : "印记写的"}
        </p>
      )}
    </div>
  );
}
