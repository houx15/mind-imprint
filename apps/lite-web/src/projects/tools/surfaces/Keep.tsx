import { Says, errorMarkdown } from "../../Says";
import { apiErrorText } from "../../../api/errorText";
import { useCallback, useEffect, useRef, useState } from "react";
import { MessageCircle, Plus } from "lucide-react";
import { Icon } from "@/ui";
import {
  KEEP_KINDS,
  KEEP_STAGES,
  addKeepEntry,
  keepMetricsFor,
  keepDelta,
  keepLaps,
  keepStage,
  settlePrediction,
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
export function Keep({
  projectId,
  projectKind,
  tool,
  onFinish,
  onOpenSession,
  onClose,
}: ToolSurfaceProps) {
  const [entries, setEntries] = useState<KeepEntry[]>([]);
  const [kind, setKind] = useState<KeepKind>("feedback");
  const [stage, setStage] = useState<KeepStage>("observe");
  const [draft, setDraft] = useState("");
  const [metric, setMetric] = useState<string | null>(null);
  // 这一条要记的数字和单位。指标选了才问数——一个没名字的数字过两周她也认不出。
  const [num, setNum] = useState("");
  const [unit, setUnit] = useState("");
  // 改一件事时的预期。空着也能提交：不写预测也是一次改动（铁律④）。
  const [expect, setExpect] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const savingRef = useRef(false);

  const reload = useCallback(async () => {
    setEntries(await listKeepEntries(projectId));
  }, [projectId]);

  useEffect(() => {
    void reload().catch((err) => setError(`加载失败：${apiErrorText(err)}`));
  }, [reload]);

  const at = keepStage(entries);
  const laps = keepLaps(entries);
  // 🚨 该看哪几种数据跟着项目类别走。写死的「点击率」问的是一个走廊上的项目
  // 量不出来的东西，等于告诉她这一步跟她无关。见 api/lookback.ts。
  const metrics = keepMetricsFor(projectKind);

  async function add() {
    const body = draft.trim();
    if (!body || savingRef.current) return;
    savingRef.current = true;
    setSaving(true);
    setError(null);
    try {
      const made = await addKeepEntry(projectId, {
        kind, body, stage,
        metric: metric ?? "",
        value: num.trim() === "" ? undefined : Number(num),
        unit: unit.trim(),
        expect: expect.trim(),
      });
      setEntries((prev) => [made, ...prev]);
      setDraft("");
      setNum("");
      setUnit("");
      setExpect("");
    } catch (err) {
      setError(`记录失败：${apiErrorText(err)}`);
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  }

  /** 那次改动的预期兑现了没有。没兑现是最值钱的一次，所以两个都只是记录。 */
  async function settle(entry: KeepEntry, verdict: "met" | "missed") {
    try {
      const got = await settlePrediction(projectId, entry.id, verdict);
      setEntries((prev) => prev.map((e) => (e.id === got.id ? got : e)));
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
      // 🚨 进那条支线，而不是收工。
      //
      // 以前这里走的是 onFinish：工具被标成 done，从标签页里消失，而且再也
      // 打不开——同时那条支线一次也没被进去过，孤零零躺在库里。设计文档写的
      // 是「一个项目可以有好几条 session」，做出来的却是"点一下，这个工具就
      // 永远没了"。正好相反。
      onOpenSession(sessionId);
    } catch (err) {
      setError(apiErrorText(err));
    }
  }

  return (
    <ToolFrame
      title={tool.label}
      task="收集数据 - 思考原因 - 进行优化，是产品不断变好的关键"
      why={tool.reason}
      todo={entries.length === 0 ? "记录反馈、数据或想法" : ""}
      finishLabel="完成"
      onFinish={() => { if (!savingRef.current) onFinish({ entries: entries.length }, ""); }}
      onClose={onClose}
      busy={saving}
    >
      <fieldset disabled={saving} className="min-w-0">
      {error && (
        <div className="mb-2 text-mk-small" style={{ color: "var(--mk-danger)" }}>
          <Says content={errorMarkdown(error)} />
        </div>
      )}

      {/* 🚨 先说清这件事为什么值得做，再请她交数据（产品负责人 2026-09-02：
          「we should explain to students why iterate and collect data is
          important, then invite students to give back data and we discuss
          together」）。上来就要数据，她只会觉得又是一项作业。 */}
      <div className="rounded-mk-md px-3 py-2.5" style={{ background: "var(--mk-paper)" }}>
        <p className="text-mk-small text-mk-ink">
          实际使用中的反馈能帮助你判断成果需要怎样改进。
        </p>
        <p className="mt-1 text-mk-small text-mk-secondary">
          请记录观察结果，并说明准备据此做哪些修改。
        </p>
      </div>

      {entries.length === 0 && (
        <div className="mt-3 rounded-mk-md border border-mk-border px-3 py-2.5">
          <p className="text-mk-small font-medium text-mk-ink">首次反馈</p>
          <p className="mt-1 text-mk-small text-mk-secondary">
            真实使用中的困难能帮助你确定修改方向。请邀请一位目标用户试用成果，记录他想做什么、实际做了什么，以及遇到的困难。
          </p>
          <p className="mt-1 text-mk-small text-mk-secondary">
            尚未测试时，可以收起工具，取得反馈后再回来记录。收起不会将本次任务标记为已完成。
          </p>
          <button type="button" onClick={onClose} className="mt-2 text-mk-small text-mk-accent-500">
            收起，稍后记录
          </button>
        </div>
      )}

      {/* 🚨 圈数。迭代的意思是重复——一张勾一次就完的清单不是迭代。
          走完一轮（记下一条「产品迭代」）就多一圈，让"又转了一圈"这件事看得见。 */}
      {laps > 0 && (
        <div className="flex items-center gap-2">
          <span className="text-mk-small text-mk-secondary">已完成 {laps} 次迭代</span>
          <span className="flex gap-1">
            {Array.from({ length: Math.min(laps, 8) }, (_, i) => (
              <span
                key={i}
                className="h-2 w-2 rounded-mk-full"
                style={{ background: "var(--mk-accent-500)" }}
              />
            ))}
          </span>
        </div>
      )}

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
              {metrics.map((m) => (
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
                {metrics.find((m) => m.name === metric)?.what}
                <span className="ml-1 text-mk-faint">再点一下就填进去。</span>
              </p>
            )}
          </div>
        )}

        {/* 🚨 一个数字本身不说明任何事：「23」是多还是少？只有和上一次比才有
            意思。上一次多少由服务端查——让她手填，填的只会是印象。 */}
        {kind === "stat" && metric && (
          <div className="mt-2 flex items-center gap-2">
            <input
              value={num}
              onChange={(e) => setNum(e.target.value)}
              inputMode="decimal"
              placeholder="这次的数字"
              className="w-28 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
            <input
              value={unit}
              onChange={(e) => setUnit(e.target.value)}
              placeholder="单位"
              className="w-20 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
            <span className="text-mk-small text-mk-muted">{metric}</span>
          </div>
        )}

        {/* 一次改动 = 一个可以被推翻的预测。 */}
        {stage === "change" && (
          <div className="mt-2">
            <input
              value={expect}
              onChange={(e) => setExpect(e.target.value)}
              placeholder="你预期这会让哪个数字怎么变（选填）"
              className="w-full rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
            />
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
            disabled={!draft.trim() || saving}
            aria-label={saving ? "保存中" : "添加"}
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
              style={{
                background: `color-mix(in srgb, ${meta?.hue ?? "var(--mk-border)"} 12%, var(--mk-surface))`,
              }}
            >
              <p className="text-mk-small text-mk-ink">{e.body}</p>

              {/* 数字连着它的变化一起显示。没有上一次就照实说「首次记录」——
                  假装它是个结果，就把「看变化」这件事教反了。 */}
              {e.value !== null && e.metric && (
                <p className="mt-0.5 text-mk-small">
                  <span className="text-mk-secondary">
                    {e.metric} {e.value}
                    {e.unit}
                  </span>
                  {keepDelta(e) === null ? (
                    <span className="ml-1.5 text-mk-faint">首次记录</span>
                  ) : (
                    <span
                      className="ml-1.5"
                      style={{
                        color:
                          keepDelta(e)! > 0 ? "var(--mk-success)" : "var(--mk-warning)",
                      }}
                    >
                      {keepDelta(e)! > 0 ? "↑" : "↓"} {Math.abs(keepDelta(e)!)}
                      {e.unit}（上次 {e.prev}
                      {e.unit}）
                    </span>
                  )}
                </p>
              )}

              {/* 🚨 一次改动 = 一个可以被推翻的预测。
                  没兑现才是最值钱的那一次：它说明她原来想错了，而那正是迭代要
                  教的东西。所以两个按钮一样重，不庆祝也不惩罚。 */}
              {e.expect && (
                <div
                  className="mt-1.5 rounded-mk-sm px-2 py-1.5"
                  style={{ background: "var(--mk-paper)" }}
                >
                  <p className="text-mk-small text-mk-secondary">当时预期：{e.expect}</p>
                  {e.verdict === "" ? (
                    <div className="mt-1 flex gap-2">
                      <button
                        type="button"
                        onClick={() => void settle(e, "met")}
                        className="rounded-mk-full border border-mk-border px-2.5 py-0.5 text-mk-small text-mk-secondary"
                      >
                        兑现了
                      </button>
                      <button
                        type="button"
                        onClick={() => void settle(e, "missed")}
                        className="rounded-mk-full border border-mk-border px-2.5 py-0.5 text-mk-small text-mk-secondary"
                      >
                        没兑现
                      </button>
                    </div>
                  ) : (
                    <p
                      className="mt-0.5 text-mk-small"
                      style={{
                        color:
                          e.verdict === "met" ? "var(--mk-success)" : "var(--mk-warning)",
                      }}
                    >
                      {e.verdict === "met" ? "已兑现" : "未达到预期"}
                    </p>
                  )}
                </div>
              )}
              <div className="mt-1.5 flex items-center gap-2">
                <span className="text-mk-small text-mk-secondary">
                  {KEEP_KINDS.find((k) => k.kind === e.kind)?.label}
                </span>
                <span className="text-mk-small text-mk-faint">{meta?.label}</span>
                <button
                  type="button"
                  onClick={() => void think(e)}
                  className="flex items-center gap-1 text-mk-small"
                  style={{ color: "var(--mk-accent-500)" }}
                >
                  <Icon icon={MessageCircle} size={12} />
                  {e.sessionId ? "查看讨论" : "深入讨论"}
                </button>
              </div>
            </div>
          );
        })}
      </div>
      </fieldset>
    </ToolFrame>
  );
}
