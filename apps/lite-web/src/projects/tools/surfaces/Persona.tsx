import { useCallback, useEffect, useState } from "react";
import { Check, Loader2, RefreshCw, Sparkles, User } from "lucide-react";
import { Icon } from "@/ui";
import {
  choosePersona,
  drawPersonaPortrait,
  generatePersonas,
  listPersonas,
  type Persona as PersonaRow,
} from "../../../api/personas";
import { apiErrorText } from "../../../api/errorText";
import { ToolFrame } from "../ToolFrame";
import type { ToolSurfaceProps } from "../registry";

/**
 * Persona —— 主页项目第一关：这一页给谁看。
 *
 * 产品负责人 2026-09-03：「think about my story from audience's view … draw a
 * persona board with ai generated photos, and tagged keywords.」
 *
 * ## 这一屏一个输入框都没有
 *
 * 印记先做出两三个可能的读者（从她**真做过的事**里推，不是凭空编），每个带一张
 * 生成的画像和一组关键词。她的动作全是判断：挑一个人、划掉不同意的关键词、
 * 或者说「都不像」让它重来。
 *
 * ## 画像是一张一张来的
 *
 * 实测一张 69 秒。三张一起要就是一个转三分钟的圈，所以三张卡各自请求、各自转。
 *
 * 🚨 每张画像下面都写着「AI 生成」。这是一个虚构的读者，她任何时候都不该分不清
 * 屏幕上哪张脸是真人。
 */
export function Persona({ projectId, tool, onFinish, onClose }: ToolSurfaceProps) {
  const [rows, setRows] = useState<PersonaRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // 她挑中的那个人，以及她还留着的关键词。挑之前是 null。
  const [picked, setPicked] = useState<string | null>(null);
  const [dropped, setDropped] = useState<Set<string>>(new Set());
  // 正在画的那几张，用来在卡片上转圈。
  const [drawing, setDrawing] = useState<Set<string>>(new Set());

  const drawMissing = useCallback(
    async (list: PersonaRow[]) => {
      for (const p of list) {
        if (p.portraitUrl) continue;
        setDrawing((s) => new Set(s).add(p.id));
        try {
          const done = await drawPersonaPortrait(projectId, p.id);
          setRows((cur) => cur.map((x) => (x.id === done.id ? done : x)));
        } catch (err) {
          // 画不出来不该让整块板停下——她仍然可以照着字判断这三个人。
          setError(apiErrorText(err));
        } finally {
          setDrawing((s) => {
            const n = new Set(s);
            n.delete(p.id);
            return n;
          });
        }
      }
    },
    [projectId],
  );

  useEffect(() => {
    let cancelled = false;
    listPersonas(projectId)
      .then((list) => {
        if (cancelled) return;
        setRows(list);
        const chosen = list.find((p) => p.chosen);
        if (chosen) setPicked(chosen.id);
        void drawMissing(list);
      })
      .catch((err) => !cancelled && setError(apiErrorText(err)))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [projectId, drawMissing]);

  const generate = useCallback(async () => {
    setBusy(true);
    setError(null);
    setPicked(null);
    setDropped(new Set());
    try {
      const list = await generatePersonas(projectId);
      setRows(list);
      void drawMissing(list);
    } catch (err) {
      setError(apiErrorText(err));
    } finally {
      setBusy(false);
    }
  }, [projectId, drawMissing]);

  const active = rows.find((p) => p.id === picked) ?? null;
  const keptKeywords = (active?.keywords ?? []).filter((k) => !dropped.has(k));
  const settled = rows.some((p) => p.chosen);

  return (
    <ToolFrame
      title="受众画像"
      task="请挑出一个真会点进你这一页的人"
      why={tool.reason}
      todo={settled ? "" : "还没有定下读者"}
      onFinish={() => {
        const chosen = rows.find((p) => p.chosen);
        onFinish(
          chosen ? { label: chosen.label, keywords: chosen.keywords } : {},
          chosen ? `她定下的读者是「${chosen.label}」` : "她还没定读者",
        );
      }}
      onClose={onClose}
      busy={busy}
    >
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex flex-wrap items-center gap-2 border-b border-mk-border px-4 py-2.5">
          <span className="text-mk-label text-mk-secondary">
            {settled ? "已确定" : "待确定"}
          </span>
          <div className="flex-1" />
          <button
            type="button"
            disabled={busy}
            onClick={() => void generate()}
            className="flex items-center gap-1.5 rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
          >
            <Icon icon={busy ? Loader2 : RefreshCw} size={14} />
            {rows.length ? "都不像，换一批" : "生成"}
          </button>
        </div>

        {error ? (
          <p className="px-4 pt-3 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {error}
          </p>
        ) : null}

        <div className="min-h-0 flex-1 overflow-auto px-4 py-3">
          {loading ? null : rows.length === 0 ? (
            <div className="rounded-mk-lg border border-mk-border p-4">
              <p className="text-mk-body text-mk-secondary">
                一个网站的样子由读它的人决定。先看看谁会点进你这一页。
              </p>
              <button
                type="button"
                disabled={busy}
                onClick={() => void generate()}
                className="mt-3 flex items-center gap-1.5 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
                style={{ background: "var(--mk-accent-500)" }}
              >
                <Icon icon={busy ? Loader2 : Sparkles} size={15} />
                {busy ? "生成中" : "看看有谁"}
              </button>
            </div>
          ) : (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {rows.map((p) => {
                const on = picked === p.id;
                return (
                  <button
                    key={p.id}
                    type="button"
                    onClick={() => {
                      setPicked(p.id);
                      setDropped(new Set());
                    }}
                    className="overflow-hidden rounded-mk-lg border text-left transition-colors"
                    style={{
                      borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)",
                      background: "var(--mk-surface)",
                    }}
                  >
                    <div
                      className="flex h-40 items-center justify-center"
                      style={{ background: "var(--mk-paper)" }}
                    >
                      {p.portraitUrl ? (
                        <img
                          src={p.portraitUrl}
                          alt=""
                          className="h-40 w-full object-cover"
                        />
                      ) : (
                        <Icon
                          icon={drawing.has(p.id) ? Loader2 : User}
                          size={22}
                          className="text-mk-faint"
                        />
                      )}
                    </div>
                    <div className="p-3">
                      <div className="flex items-baseline justify-between gap-2">
                        <h3 className="text-mk-body font-semibold text-mk-ink">{p.label}</h3>
                        {p.chosen ? (
                          <span className="text-mk-small text-mk-accent-600">已确定</span>
                        ) : null}
                      </div>
                      {/* 🚨 说清楚这张脸是生成的。她任何时候都不该分不清哪张是真人。 */}
                      <p className="mt-0.5 text-mk-small text-mk-faint">画像由 AI 生成</p>
                      <dl className="mt-2 space-y-1.5">
                        {[
                          ["怎么找到你", p.whyKnows],
                          ["想看到什么", p.wants],
                          ["该有的感觉", p.feeling],
                        ]
                          .filter(([, v]) => v)
                          .map(([k, v]) => (
                            <div key={k}>
                              <dt className="text-mk-label text-mk-faint">{k}</dt>
                              <dd className="text-mk-small text-mk-secondary">{v}</dd>
                            </div>
                          ))}
                      </dl>
                    </div>
                  </button>
                );
              })}
            </div>
          )}

          {/* 挑中之后：关键词。点一下划掉，再点一下加回来——判断，不是填写。 */}
          {active ? (
            <div className="mt-5 rounded-mk-lg border border-mk-border p-3">
              <h3 className="text-mk-label text-mk-secondary">关键词</h3>
              <p className="mt-1 text-mk-small text-mk-muted">
                请留下你同意的那几个。后面搭结构、挑配色都会用到它们。
              </p>
              <div className="mt-2 flex flex-wrap gap-2">
                {active.keywords.map((k) => {
                  const off = dropped.has(k);
                  return (
                    <button
                      key={k}
                      type="button"
                      onClick={() =>
                        setDropped((s) => {
                          const n = new Set(s);
                          if (n.has(k)) n.delete(k);
                          else n.add(k);
                          return n;
                        })
                      }
                      className="rounded-mk-full border px-3 py-1 text-mk-small transition-colors"
                      style={{
                        borderColor: off ? "var(--mk-border)" : "var(--mk-accent-500)",
                        color: off ? "var(--mk-faint)" : "var(--mk-ink)",
                        textDecoration: off ? "line-through" : "none",
                      }}
                    >
                      {k}
                    </button>
                  );
                })}
              </div>
              <button
                type="button"
                disabled={busy || keptKeywords.length === 0}
                onClick={async () => {
                  setBusy(true);
                  setError(null);
                  try {
                    await choosePersona(projectId, active.id, keptKeywords);
                    setRows(await listPersonas(projectId));
                  } catch (err) {
                    setError(apiErrorText(err));
                  } finally {
                    setBusy(false);
                  }
                }}
                className="mt-3 flex items-center gap-1.5 rounded-mk-full px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40"
                style={{ background: "var(--mk-accent-500)" }}
              >
                <Icon icon={Check} size={15} />
                确认选择
              </button>
              {keptKeywords.length === 0 ? (
                <span className="ml-3 text-mk-small text-mk-muted">请至少留下一个关键词。</span>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </ToolFrame>
  );
}
