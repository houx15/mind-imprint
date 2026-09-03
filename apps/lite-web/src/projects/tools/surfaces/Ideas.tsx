import { apiErrorText } from "../../../api/errorText";
import { useCallback, useEffect, useState } from "react";
import { Plus } from "lucide-react";
import { Icon } from "@/ui";
import { archiveNote, createNotes, listNotes, pickIdea, type Note } from "../../../api/notes";
import { ToolFrame } from "../ToolFrame";
import { DONE, tone } from "../../../shared/tone";
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
    } catch (err) {
      setError(apiErrorText(err));
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
    } catch (err) {
      setError(apiErrorText(err));
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

  async function finish() {
    const one = ideas.find((i) => i.id === picked);
    if (!one) return;
    // 🚨 先落库，再收工。印记从来看不到 onFinish 的 payload——回灌是回头读表的，
    // 所以「她挑了哪一条」不写进表里就等于没发生。线上就是这么错的：她挑第三条，
    // 印记照着列表第一条说「你那条点子说……」。
    try {
      await pickIdea(projectId, one.id, why.trim());
    } catch (err) {
      setError(apiErrorText(err));
      return;
    }
    // 她写的是"为什么先试它"，回传的就是这一句。
    onFinish({ count: ideas.length, picked: one.id, idea: one.body }, why.trim());
  }

  return (
    <ToolFrame
      title={tool.label}
      task="先多想几个办法，再挑一个先试"
      why={tool.reason}
      todo={todo}
      finishLabel="确认选择"
      onFinish={() => void finish()}
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
          aria-label="添加"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-mk-full text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Plus} size={15} />
        </button>
      </div>

      {/* 🚨 「想满四个才让挑」原来只是一行字，她看不见自己还差几个——按钮是灰的，
          原因写在别处。四个格子亮起来，规矩就成了看得见的东西。 */}
      <div className="mt-3 flex items-center gap-2">
        <div className="flex gap-1">
          {Array.from({ length: Math.max(ENOUGH, ideas.length) }, (_, i) => (
            <span
              key={i}
              className="h-2 w-6 rounded-mk-full"
              style={{
                background: i < ideas.length ? DONE.solid : DONE.bg,
              }}
            />
          ))}
        </div>
        <p className="text-mk-small text-mk-muted">
          {enough
            ? `${ideas.length} 个办法。挑一个先试。`
            : `还差 ${ENOUGH - ideas.length} 个。别急着挑第一个。`}
        </p>
      </div>

      <div className="mt-2 space-y-1.5">
        {ideas.map((i) => (
          <div
            key={i.id}
            className="rounded-mk-md border px-3 py-2"
            style={{
              borderColor: picked === i.id ? DONE.solid : "var(--mk-border)",
              background: merging.includes(i.id)
                ? tone("peach").bg
                : picked === i.id
                  ? DONE.bg
                  : undefined,
              boxShadow:
                picked === i.id
                  ? `0 0 0 2px color-mix(in srgb, ${DONE.solid} 30%, transparent)`
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
                style={{ color: picked === i.id ? DONE.solid : "var(--mk-secondary)" }}
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
          className="mt-2 w-full rounded-mk-md border border-dashed py-2 text-mk-small"
          style={{ borderColor: tone("peach").solid, color: tone("peach").fg }}
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
