import { useEffect, useState } from "react";
import { Plus, Trash2, ChevronUp, ChevronDown, Sparkles } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import { generateWritingOutline, putWritingOutline, type WritingOutlineItem } from "../api/writingRoom";

/**
 * OutlineStage — 大纲: "generate a candidate + the student edits + confirm."
 *
 * `draft` is the ONE editable list this component owns: it starts as
 * whatever is already persisted, `生成候选` overwrites it (client-side only,
 * nothing is saved yet) with a model-derived candidate she can still edit,
 * and `确认这份提纲` is the only thing that ever calls `putWritingOutline`
 * (a full replace). There is no separate "confirmed vs candidate" flag to
 * track — generation and hand-editing both just mutate the same draft.
 */

type DraftItem = { text: string; depth: number };

function toDraft(items: { text: string; depth: number }[]): DraftItem[] {
  return items.map((i) => ({ text: i.text, depth: i.depth }));
}

export function OutlineStage({
  writingId,
  outline,
  onSaved,
}: {
  writingId: string;
  outline: WritingOutlineItem[];
  onSaved: (outline: WritingOutlineItem[]) => void;
}) {
  const [draft, setDraft] = useState<DraftItem[]>(() => toDraft(outline));
  const [generating, setGenerating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);

  // The persisted outline can change out from under this component (e.g. a
  // reload finishing after mount) — but only before she has touched the
  // draft; once she starts editing, an incoming prop update must not stomp
  // on her in-progress edits.
  useEffect(() => {
    if (!dirty) setDraft(toDraft(outline));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [outline]);

  function updateItem(i: number, patch: Partial<DraftItem>) {
    setDirty(true);
    setDraft((d) => d.map((it, idx) => (idx === i ? { ...it, ...patch } : it)));
  }
  function removeItem(i: number) {
    setDirty(true);
    setDraft((d) => d.filter((_, idx) => idx !== i));
  }
  function addItem() {
    setDirty(true);
    setDraft((d) => [...d, { text: "", depth: 0 }]);
  }
  function move(i: number, delta: number) {
    setDirty(true);
    setDraft((d) => {
      const j = i + delta;
      if (j < 0 || j >= d.length) return d;
      const next = [...d];
      [next[i], next[j]] = [next[j]!, next[i]!];
      return next;
    });
  }

  async function generate() {
    setGenerating(true);
    setError(null);
    try {
      const candidates = await generateWritingOutline(writingId);
      setDirty(true);
      setDraft(toDraft(candidates));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "拟提纲失败，请重试。");
    } finally {
      setGenerating(false);
    }
  }

  async function confirm() {
    setSaving(true);
    setError(null);
    try {
      const cleaned = draft.map((it) => ({ text: it.text.trim(), depth: it.depth })).filter((it) => it.text);
      const saved = await putWritingOutline(writingId, cleaned);
      setDirty(false);
      onSaved(saved);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存提纲失败，请重试。");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-mk-h2 text-mk-ink">大纲</h2>
        <Button variant="secondary" size="sm" onClick={() => void generate()} loading={generating} iconStart={<Icon icon={Sparkles} size={14} />}>
          帮我拟一份候选
        </Button>
      </div>

      {draft.length === 0 && (
        <p className="rounded-mk-md border border-dashed border-mk-border p-4 text-mk-small text-mk-muted">
          还没有提纲条目——点「帮我拟一份候选」让印记根据你构思阶段说过的话拟一份，或者自己动手加一条。
        </p>
      )}

      <div className="flex flex-col gap-2">
        {draft.map((item, i) => (
          <div key={i} className="flex items-center gap-2">
            <select
              aria-label="层级"
              value={item.depth}
              onChange={(e) => updateItem(i, { depth: Number(e.target.value) })}
              className="w-14 shrink-0 rounded-mk-xs border border-mk-input-border bg-mk-surface px-1 py-2 text-mk-small text-mk-ink outline-none"
            >
              <option value={0}>一级</option>
              <option value={1}>二级</option>
              <option value={2}>三级</option>
            </select>
            <input
              value={item.text}
              onChange={(e) => updateItem(i, { text: e.target.value })}
              placeholder="这一部分要讲什么？"
              className="min-w-0 flex-1 rounded-mk-xs border border-mk-input-border bg-mk-surface px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              style={{ marginLeft: item.depth * 16 }}
            />
            <button type="button" aria-label="上移" onClick={() => move(i, -1)} className="text-mk-faint hover:text-mk-muted">
              <Icon icon={ChevronUp} size={16} />
            </button>
            <button type="button" aria-label="下移" onClick={() => move(i, 1)} className="text-mk-faint hover:text-mk-muted">
              <Icon icon={ChevronDown} size={16} />
            </button>
            <button type="button" aria-label="删除这条" onClick={() => removeItem(i)} className="text-mk-faint hover:text-mk-danger">
              <Icon icon={Trash2} size={16} />
            </button>
          </div>
        ))}
      </div>

      <button
        type="button"
        onClick={addItem}
        className="flex w-fit items-center gap-1.5 rounded-mk-sm px-2 py-1.5 text-mk-small text-mk-accent-700 hover:bg-mk-accent-50"
      >
        <Icon icon={Plus} size={14} /> 加一条
      </button>

      {error && <p className="text-mk-small text-mk-danger">{error}</p>}

      <div>
        <Button onClick={() => void confirm()} loading={saving} disabled={draft.every((it) => !it.text.trim())}>
          确认这份提纲
        </Button>
      </div>
    </div>
  );
}
