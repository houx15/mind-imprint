import { useCallback, useEffect, useState } from "react";
import { Plus } from "lucide-react";
import { Icon } from "@/ui";
import { archiveNote, createNotes, listNotes, type Note } from "../../../api/notes";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Ideas —— 想办法。
 *
 * 产品负责人 2026-09-01：「brainstorm many solutions with the AI, combine and
 * compare ideas, choose a promising next experiment」。
 *
 * 点子就是第五种便签（kind='idea'），所以这块和便签板共用一张表——她在这里想
 * 出来的办法，回到板上还看得见。
 *
 * 界面上唯一的"规矩"是：想满四个才让挑。急着挑第一个想到的，是这一步最常见
 * 的失手。规矩只用一行字说出来，不拦她做别的。
 */

const ENOUGH = 4;

export function Ideas({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [ideas, setIdeas] = useState<Note[]>([]);
  const [draft, setDraft] = useState("");
  const [picked, setPicked] = useState<string | null>(null);
  const [why, setWhy] = useState("");
  const [merging, setMerging] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const all = await listNotes(projectId);
    setIdeas(all.filter((n) => n.kind === "idea"));
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  async function add() {
    const body = draft.trim();
    if (!body) return;
    setDraft("");
    try {
      const made = await createNotes(projectId, [{ kind: "idea", body }]);
      setIdeas((prev) => [...prev, ...made]);
    } catch {
      setError("没记下来，再试一次。");
    }
  }

  /** 合成一个：两个办法里各有一半对的，拼起来往往比两个都强。 */
  async function merge() {
    if (merging.length !== 2) return;
    const parts = ideas.filter((i) => merging.includes(i.id));
    if (parts.length !== 2) return;
    try {
      const made = await createNotes(projectId, [
        { kind: "idea", body: `${parts[0]!.body}，同时${parts[1]!.body}` },
      ]);
      await Promise.all(parts.map((p) => archiveNote(projectId, p.id)));
      setIdeas((prev) => [...prev.filter((i) => !merging.includes(i.id)), ...made]);
      setMerging([]);
      if (picked && merging.includes(picked)) setPicked(null);
    } catch {
      setError("没合上，再试一次。");
    }
  }

  function toggleMerge(id: string) {
    setMerging((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : prev.length < 2 ? [...prev, id] : [prev[1]!, id],
    );
  }

  const enough = ideas.length >= ENOUGH;
  const todo = !enough
    ? `再想 ${ENOUGH - ideas.length} 个`
    : !picked
      ? "挑一个先试"
      : why.trim()
        ? ""
        : "一句为什么先试它";

  function finish() {
    const one = ideas.find((i) => i.id === picked);
    if (!one) return;
    onFinish(
      { count: ideas.length, picked: one.id },
      `我想了 ${ideas.length} 个办法，先试这个：${one.body}。因为${why.trim()}`,
    );
  }

  return (
    <ToolFrame
      title={tool.label}
      task="先多想几个办法，再挑一个先试"
      why={tool.reason}
      todo={todo}
      finishLabel="就先试这个"
      onFinish={finish}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      <div className="flex items-end gap-2">
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") void add();
          }}
          placeholder="一个办法，回车记下"
          className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />
        <button
          type="button"
          onClick={() => void add()}
          disabled={!draft.trim()}
          aria-label="记下来"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Plus} size={15} />
        </button>
      </div>

      <p className="mt-2 text-mk-small text-mk-muted">
        {enough ? `${ideas.length} 个办法。挑一个先试。` : "先多想几个，别急着挑第一个。"}
      </p>

      <div className="mt-2 space-y-1.5">
        {ideas.map((i) => (
          <div
            key={i.id}
            className="rounded-mk-md border px-3 py-2"
            style={{
              borderColor:
                picked === i.id ? "var(--mk-accent-500)" : "var(--mk-border)",
              background:
                merging.includes(i.id)
                  ? "color-mix(in srgb, #10B981 12%, transparent)"
                  : undefined,
            }}
          >
            <p className="text-mk-small text-mk-ink">{i.body}</p>
            <div className="mt-1.5 flex gap-2">
              <button
                type="button"
                onClick={() => setPicked(picked === i.id ? null : i.id)}
                disabled={!enough}
                className="text-mk-small disabled:opacity-40"
                style={{ color: picked === i.id ? "var(--mk-accent-500)" : "var(--mk-secondary)" }}
              >
                {picked === i.id ? "✓ 先试这个" : "先试这个"}
              </button>
              <button
                type="button"
                onClick={() => toggleMerge(i.id)}
                className="text-mk-small text-mk-secondary"
              >
                {merging.includes(i.id) ? "✓ 要合" : "和别的合一下"}
              </button>
            </div>
          </div>
        ))}
      </div>

      {merging.length === 2 && (
        <button
          type="button"
          onClick={() => void merge()}
          className="mt-2 w-full rounded-mk-md border border-dashed border-mk-border py-2 text-mk-small text-mk-secondary"
        >
          把这两个合成一个
        </button>
      )}

      {picked && (
        <div className="mt-4 border-t border-mk-border pt-3">
          <label className="text-mk-small text-mk-secondary">为什么先试它？</label>
          <textarea
            value={why}
            onChange={(e) => setWhy(e.target.value)}
            rows={2}
            placeholder="比如：这个一个下午就能试出来"
            className="mt-1.5 w-full resize-none rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-2 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
          />
        </div>
      )}
    </ToolFrame>
  );
}
