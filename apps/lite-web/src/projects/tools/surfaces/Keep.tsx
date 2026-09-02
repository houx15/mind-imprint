import { apiErrorText } from "../../../api/errorText";
import { useCallback, useEffect, useState } from "react";
import { MessageCircle, Plus } from "lucide-react";
import { Icon } from "@/ui";
import {
  KEEP_KINDS,
  KEEP_METRICS,
  KEEP_STAGES,
  addKeepEntry,
  keepStage,
  listKeepEntries,
  openKeepSession,
  type KeepEntry,
  type KeepKind,
  type KeepStage,
} from "../../../api/lookback";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Keep —— 上线之后。
 *
 * 产品负责人 2026-09-01：东西放出去之后还有事情发生，而那些事情才是真的。
 * 「we can use a colorful diagram to teach students about the step」——上面那
 * 条彩色的圈就是这个：放出去 → 看数据 → 读出意思 → 改一件事 → 再放出去。
 *
 * 🚨 每一条数据旁边都有「深入讨论」。按下去会给这个项目开**一轮新的思考**
 * （「one project may have several sessions」）。这一下就是维持和归档的分界：
 * 数字看过就算了，那是归档；数字让她重新想一遍，这个项目还活着。
 *
 * 我们不替她保管成品——网站活在她自己的世界里。我们保管的是她从成品那里学到
 * 的东西。
 */
export function Keep({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [entries, setEntries] = useState<KeepEntry[]>([]);
  const [kind, setKind] = useState<KeepKind>("stat");
  const [stage, setStage] = useState<KeepStage>("observe");
  const [draft, setDraft] = useState("");
  const [metric, setMetric] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setEntries(await listKeepEntries(projectId));
  }, [projectId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const at = keepStage(entries);

  async function add() {
    const body = draft.trim();
    if (!body) return;
    setDraft("");
    try {
      const made = await addKeepEntry(projectId, { kind, body, stage });
      setEntries((prev) => [made, ...prev]);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  async function think(entry: KeepEntry) {
    try {
      const { sessionId } = await openKeepSession(projectId, entry.id);
      setEntries((prev) =>
        prev.map((e) => (e.id === entry.id ? { ...e, sessionId } : e)),
      );
      onFinish({ entryId: entry.id, sessionId }, "");
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  return (
    <ToolFrame
      title={tool.label}
      task="收集数据 - 思考原因 - 进行优化，是产品不断变好的关键"
      why={tool.reason}
      todo={entries.length === 0 ? "记录关于产品的实际反馈 / 使用情况" : ""}
      finishLabel="完成"
      onFinish={() => onFinish({ entries: entries.length }, "")}
      onClose={onClose}
    >
      {error && (
        <p className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          {error}
        </p>
      )}

      {/* 🚨 先说清这件事为什么值得做，再请她交数据（产品负责人 2026-09-02：
          「we should explain to students why iterate and collect data is
          important, then invite students to give back data and we discuss
          together」）。上来就要数据，她只会觉得又是一项作业。 */}
      <div className="rounded-mk-md px-3 py-2.5" style={{ background: "var(--mk-paper)" }}>
        <p className="text-mk-small text-mk-ink">
          东西做出来只是开始。真正让它变好的，是放出去之后你能看见什么、又据此改了什么。
        </p>
        <p className="mt-1 text-mk-small text-mk-secondary">
          把你观察到的带回来，我们一起看它说明了什么。
        </p>
      </div>

      {/* 循环。她现在停在哪一步。 */}
      <div className="flex items-stretch gap-1">
        {KEEP_STAGES.map((s, i) => (
          <div key={s.stage} className="flex min-w-0 flex-1 items-stretch">
            <button
              type="button"
              onClick={() => setStage(s.stage)}
              className="min-w-0 flex-1 rounded-mk-md px-1.5 py-2 text-center"
              style={{
                background:
                  stage === s.stage
                    ? s.hue
                    : `color-mix(in srgb, ${s.hue} 14%, transparent)`,
                color: stage === s.stage ? "#fff" : "var(--mk-ink)",
                outline: at === s.stage ? `2px solid ${s.hue}` : undefined,
              }}
            >
              <span className="block truncate text-mk-small">{s.label}</span>
            </button>
            {i < KEEP_STAGES.length - 1 && (
              <span className="self-center px-0.5 text-mk-faint">›</span>
            )}
          </div>
        ))}
      </div>
      <p className="mt-1.5 text-mk-small text-mk-muted">
        {KEEP_STAGES.find((s) => s.stage === stage)?.hint}
        {at === "ship" && entries.length > 0 && " · 再次发布，持续收集反馈"}
      </p>

      {/* 记一条 */}
      <div className="mt-3">
        <div className="flex flex-wrap gap-1">
          {KEEP_KINDS.map((k) => (
            <button
              key={k.kind}
              type="button"
              onClick={() => setKind(k.kind)}
              className="rounded-mk-full px-2.5 py-1 text-mk-small"
              style={
                kind === k.kind
                  ? { background: "var(--mk-accent-500)", color: "#fff" }
                  : { border: "1px solid var(--mk-border)", color: "var(--mk-secondary)" }
              }
            >
              {k.label}
            </button>
          ))}
        </div>
        {kind === "stat" && (
          <div className="mt-2">
            <div className="flex flex-wrap gap-1">
              {KEEP_METRICS.map((m) => (
                <button
                  key={m.name}
                  type="button"
                  onClick={() => {
                    if (metric === m.name) {
                      setDraft(`${m.name}：`);
                      setMetric(null);
                    } else {
                      setMetric(m.name);
                    }
                  }}
                  className="rounded-mk-full border border-mk-border px-2.5 py-0.5 text-mk-small"
                  style={
                    metric === m.name
                      ? { borderColor: "var(--mk-accent-500)", color: "var(--mk-ink)" }
                      : { color: "var(--mk-secondary)" }
                  }
                >
                  {m.name}
                </button>
              ))}
            </div>
            {metric && (
              <p className="mt-1.5 text-mk-small text-mk-muted">
                {KEEP_METRICS.find((m) => m.name === metric)?.what}
                <span className="ml-1 text-mk-faint">再点一下就填进去。</span>
              </p>
            )}
          </div>
        )}

        <div className="mt-2 flex items-end gap-2">
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && void add()}
            placeholder={KEEP_KINDS.find((k) => k.kind === kind)?.placeholder ?? "写一条"}
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
      </div>

      <div className="mt-3 space-y-2">
        {entries.map((e) => {
          const meta = KEEP_STAGES.find((s) => s.stage === e.stage);
          return (
            <div
              key={e.id}
              className="rounded-mk-md px-3 py-2"
              style={{ borderLeft: `3px solid ${meta?.hue ?? "var(--mk-border)"}`,
                       background: "var(--mk-paper)" }}
            >
              <p className="text-mk-small text-mk-ink">{e.body}</p>
              <div className="mt-1.5 flex items-center gap-2">
                <span className="text-mk-small text-mk-faint">{meta?.label}</span>
                <button
                  type="button"
                  onClick={() => void think(e)}
                  className="flex items-center gap-1 text-mk-small"
                  style={{ color: "var(--mk-accent-500)" }}
                >
                  <Icon icon={MessageCircle} size={12} />
                  {e.sessionId ? "继续讨论" : "深入讨论"}
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </ToolFrame>
  );
}
