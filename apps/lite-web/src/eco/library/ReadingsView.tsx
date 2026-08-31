import { useState } from "react";
import { Bookmark, Clock, MessageCircle, X } from "lucide-react";
import { useEco } from "../store";
import { READINGS } from "../data/library";
import { DOMAIN_META, newsById } from "../data/news";
import { FIELDS, fieldById } from "../data/tree";
import { go } from "../route";
import type { FieldId } from "../data/types";
import { Btn, Chip, SectionHead, Sys, cx } from "../ui";

/**
 * 阅读 · the shelf.
 *
 * Deliberately modest in this prototype: the real reading room already exists
 * in lite (`src/readings/`). What the shelf has to prove HERE is the two
 * connections the ecosystem depends on —
 *   ① a reading feeds keywords onto the tree (每张卡片都标出它喂养了哪几个词),
 *   ② a reading can be picked onto her homepage (the studio's picker reads
 *      this same list).
 * Anything beyond that belongs in the real room, not here.
 */
export function ReadingsView() {
  const { state, openCoach, unkeep } = useEco();
  const [field, setField] = useState<FieldId | null>(null);
  const list = field ? READINGS.filter((r) => r.field === field) : READINGS;
  const totalMin = READINGS.reduce((s, r) => s + r.minutes, 0);
  // 待读 — everything she pressed 先收藏 on in 世界. It sits ABOVE the shelf
  // because a to-read box below what you have already read is a box you never
  // look at. Empty by default, and it says so plainly rather than hiding.
  const toRead = state.kept.map(newsById).filter((n): n is NonNullable<typeof n> => Boolean(n));

  return (
    <div className="mx-auto max-w-[1080px] px-8 py-8">
      {/* ── 待读 ─────────────────────────────────────────────────────── */}
      <section className="mb-9">
        <div className="flex flex-wrap items-baseline justify-between gap-3">
          <div className="flex items-center gap-2">
            <Bookmark size={16} strokeWidth={1.9} className="text-mk-accent-700" />
            <h2 className="text-mk-h2 text-mk-ink">待读</h2>
            <span className="font-mono text-mk-body tabular-nums text-mk-muted">
              {toRead.length}
            </span>
          </div>
          <p className="text-mk-small text-mk-muted">
            在「世界」里按了「先收藏」的，都会掉进这里。
          </p>
        </div>

        {toRead.length === 0 ? (
          <div
            className="mt-3 rounded-mk-lg border border-dashed p-5"
            style={{ borderColor: "var(--mk-input-border)" }}
          >
            <p className="text-mk-body leading-[1.85] text-mk-muted">
              还没有收藏。去
              <button
                type="button"
                onClick={() => go({ name: "home", view: "world" })}
                className="mx-1 font-semibold text-mk-accent-700 underline underline-offset-2
                           focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
              >
                今日探索地图
              </button>
              看看，遇到不想现在读的，先收藏起来。
            </p>
          </div>
        ) : (
          <ul className="mt-3 grid gap-2.5 md:grid-cols-2">
            {toRead.map((n, i) => {
              const meta = DOMAIN_META[n.domain];
              return (
                <li key={n.id} className="eco-in" style={{ ["--i" as string]: i }}>
                  <div
                    className="flex h-full items-start gap-3 rounded-mk-lg border p-4"
                    style={{
                      borderColor: `color-mix(in srgb, ${meta.hue} 40%, transparent)`,
                      background: `color-mix(in srgb, ${meta.hue} 8%, var(--mk-surface))`,
                    }}
                  >
                    <span
                      className="mt-1 h-2 w-2 shrink-0 rounded-mk-full"
                      style={{ background: meta.hue }}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-x-2.5">
                        <Sys>{meta.zh}</Sys>
                        <Sys>{n.source}</Sys>
                        <Sys>{n.date}</Sys>
                      </div>
                      <p className="mt-1.5 text-mk-body font-semibold leading-[1.7] text-mk-ink">
                        {n.lead.zh}
                      </p>
                      <p className="mt-1 text-mk-small leading-[1.7] text-mk-muted">{n.title.zh}</p>
                      <div className="mt-2.5 flex flex-wrap gap-2">
                        <Btn
                          size="sm"
                          variant="outline"
                          onClick={() => openCoach("reading", n.hook.zh)}
                        >
                          和印记聊这条
                        </Btn>
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => unkeep(n.id)}
                      className="shrink-0 rounded-mk-full p-1.5 text-mk-faint transition-colors
                                 duration-[120ms] hover:bg-mk-accent-50 hover:text-mk-secondary
                                 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                      aria-label="从待读里移走"
                      title="从待读里移走"
                    >
                      <X size={14} strokeWidth={2} />
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <SectionHead
        index="我读过的 · READING SHELF"
        title="过往的阅读"
        sub="每一篇都在往你的树上送词。点开可以看它送了哪几个。"
        right={
          <div className="flex items-center gap-5">
            <span className="text-right">
              <Sys>篇数</Sys>
              <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
                {READINGS.length}
              </span>
            </span>
            <span className="h-8 w-px" style={{ background: "var(--mk-border)" }} />
            <span className="text-right">
              <Sys>累计时长</Sys>
              <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
                {Math.round(totalMin / 6) / 10}h
              </span>
            </span>
          </div>
        }
      />

      <div className="mb-5 flex flex-wrap gap-1.5">
        <Chip active={field === null} onClick={() => setField(null)}>
          全部
        </Chip>
        {FIELDS.map((f) => {
          const n = READINGS.filter((r) => r.field === f.id).length;
          if (n === 0) return null;
          return (
            <Chip key={f.id} hue={f.hue} active={field === f.id} onClick={() => setField(field === f.id ? null : f.id)}>
              {f.label} {n}
            </Chip>
          );
        })}
      </div>

      <ul className="grid gap-3 md:grid-cols-2">
        {list.map((r, i) => {
          const f = fieldById(r.field);
          return (
            <li key={r.id} className="eco-in" style={{ ["--i" as string]: i }}>
              <button
                type="button"
                onClick={() => go({ name: "readings", id: r.id })}
                className={cx(
                  "flex h-full w-full flex-col rounded-mk-lg border border-mk-border bg-mk-surface p-5 text-left",
                  "transition-all duration-[160ms] ease-mk hover:-translate-y-0.5 hover:border-mk-accent-200",
                  "hover:shadow-mk-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
                )}
              >
                <div className="flex items-center gap-2">
                  <span className="h-2 w-2 rounded-mk-full" style={{ background: f.hue }} />
                  <Sys>{f.label}</Sys>
                  <span className="ml-auto inline-flex items-center gap-1 font-mono text-[11px] text-mk-faint">
                    <Clock size={11} strokeWidth={2} />
                    {r.minutes}′
                  </span>
                </div>
                <h3 className="mt-2 text-mk-h3 text-mk-ink">{r.title}</h3>
                <p className="mt-1 text-mk-small text-mk-muted">
                  {r.source} · {r.date}
                </p>
                {r.takeaway ? (
                  <p
                    className="mt-3 border-l-2 pl-3 text-mk-body leading-[1.8] text-mk-secondary"
                    style={{ borderColor: f.hue }}
                  >
                    {r.takeaway}
                    <span className="mt-1 block text-[11px] text-mk-faint">你写下的收获</span>
                  </p>
                ) : null}
                <div className="mt-auto flex flex-wrap gap-1.5 pt-4">
                  {r.keywords.map((k) => (
                    <span
                      key={k}
                      className="rounded-mk-full px-2.5 py-1 text-mk-small"
                      style={{ background: "var(--mk-paper)", color: "var(--mk-secondary)" }}
                    >
                      {k}
                    </span>
                  ))}
                </div>
              </button>
            </li>
          );
        })}
      </ul>

      <div className="mt-8 flex items-center justify-center gap-3">
        <Btn variant="outline" iconStart={<MessageCircle size={16} strokeWidth={1.8} />} onClick={() => openCoach("reading")}>
          问印记：接下来该读什么
        </Btn>
        <Btn variant="quiet" onClick={() => go({ name: "home", view: "world" })}>
          去世界找一篇
        </Btn>
      </div>
    </div>
  );
}
