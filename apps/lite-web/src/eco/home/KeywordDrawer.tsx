import { Sparkles, X } from "lucide-react";
import { useEco } from "../store";
import { fieldById } from "../data/tree";
import { DIG_META, digFor, type DigSeed } from "../data/dig";
import { READINGS } from "../data/library";
import { go } from "../route";
import type { Keyword, KeywordSource } from "../data/types";
import { Drawer, Sys, cx } from "../ui";

/**
 * A keyword, opened. Three sections, in this order:
 *
 *   摘要 · 相关活动 · 继续深挖
 *
 * ## Why 相关活动 comes second and is not a footnote
 * The question a student actually has in front of a generated model of herself
 * is **「你凭什么这么说我？」**. So the drawer answers it immediately: what the
 * keyword is (印记's one-line read), then every trace it was built from —
 * clickable, dated, and where possible carrying HER OWN SENTENCE. A model that
 * cannot show its evidence is a horoscope. The old copy explained this in a
 * sentence above the list ("这个词不是猜的…"); the list makes the point better
 * than the sentence did, so the sentence is gone.
 *
 * ## Why 继续深挖 is bubbles and not four verbs
 * It used to end with 再读一篇 / 写一篇 / 做个项目 / 问印记 — the same four on
 * every keyword. Four empty verbs after a real observation teach a student
 * that the model has an opinion about her and no idea what to do about it.
 * Now every keyword carries four CONCRETE seeds from `data/dig.ts`: a question
 * she cannot answer yet, a specific next reading, a piece she could write, and
 * a project that could actually exist. Clicking one carries its text where it
 * belongs.
 */
export function KeywordDrawer({ kw, onClose }: { kw: Keyword | null; onClose: () => void }) {
  const { openCoach } = useEco();
  if (!kw) return null;
  const f = fieldById(kw.field);
  const seeds = digFor(kw.id);

  function follow(seed: DigSeed) {
    onClose();
    switch (seed.kind) {
      case "question":
        openCoach("tree", seed.text);
        break;
      case "reading":
        go(seed.ref ? { name: "readings", id: seed.ref } : { name: "readings" });
        break;
      case "writing":
        go({ name: "writings" });
        break;
      case "project":
        go({ name: "project-new" });
        break;
    }
  }

  return (
    <Drawer open onClose={onClose} tone="dark" width={560} label={kw.text}>
      <div className="flex items-start justify-between gap-4 px-6 pt-5">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className="h-2.5 w-2.5 rounded-mk-full"
              style={{ background: f.hue, boxShadow: `0 0 12px ${f.hue}` }}
            />
            <Sys tone="dark">{f.label} · KEYWORD</Sys>
          </div>
          <h2 className="mt-1.5 text-mk-h1 text-[#F5EFE7]">{kw.text}</h2>
          <p className="mt-0.5 font-mono text-mk-small text-[#7C7166]">{kw.en}</p>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="rounded-mk-full p-2 transition-colors duration-[120ms] hover:bg-[rgba(240,233,224,.1)]
                     focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
          aria-label="关闭"
        >
          <X size={18} strokeWidth={1.8} color="#C6B9AA" />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-6">
        <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2">
          <span className="inline-flex items-baseline gap-1.5">
            <Sys tone="dark">强度</Sys>
            <span className="font-mono text-mk-small tabular-nums text-[#F0E9E0]">
              {kw.strength} / 5
            </span>
          </span>
          <span className="inline-flex items-baseline gap-1.5">
            <Sys tone="dark">来源</Sys>
            <span className="font-mono text-mk-small tabular-nums text-[#F0E9E0]">
              {kw.sources.length}
            </span>
          </span>
          <span className="inline-flex items-baseline gap-1.5">
            <Sys tone="dark">出现于</Sys>
            <span className="font-mono text-mk-small text-[#F0E9E0]">
              {["三月", "五月", "七月", "本月"][kw.bornAt]}
            </span>
          </span>
        </div>

        {/* ── 摘要 ─────────────────────────────────────────────────────── */}
        <div
          className="mt-5 rounded-mk-md p-4"
          style={{
            background: `color-mix(in srgb, ${f.hue} 12%, rgba(240,233,224,.04))`,
            border: `1px solid color-mix(in srgb, ${f.hue} 26%, transparent)`,
          }}
        >
          <Sys tone="dark">摘要</Sys>
          <p className="mt-1.5 text-mk-body-lg leading-[1.85] text-[#F0E9E0]">{kw.note}</p>
        </div>

        {kw.shining ? (
          <div
            className="mt-4 rounded-mk-md p-4"
            style={{
              border: "1px solid rgba(201,150,43,.4)",
              background: "rgba(201,150,43,.1)",
            }}
          >
            <div className="flex items-center gap-2">
              <Sparkles size={15} strokeWidth={2} color="#E5B65A" />
              <Sys tone="dark" className="!text-[#E5B65A]">
                做得最好的一次 · {kw.shining.date}
              </Sys>
            </div>
            <p className="mt-2 text-mk-h3 text-[#F5E7C8]">{kw.shining.title}</p>
            <p className="mt-1.5 text-mk-body leading-[1.85] text-[#DBCBA6]">{kw.shining.body}</p>
          </div>
        ) : null}

        {/* ── 相关活动 ─────────────────────────────────────────────────── */}
        <h3 className="mt-7 text-mk-h3 text-[#EFE7DC]">相关活动</h3>
        <ul className="mt-3 space-y-2">
          {kw.sources.map((s) => (
            <SourceRow key={`${s.kind}-${s.id}`} source={s} onNavigate={onClose} />
          ))}
        </ul>

        {/* ── 继续深挖 ─────────────────────────────────────────────────── */}
        <div className="eco-scanline my-7" />
        <h3 className="text-mk-h3 text-[#EFE7DC]">继续深挖</h3>
        <p className="mt-1 text-mk-small text-[#8E8175]">
          点一个，它会带着这句话去到该去的地方。
        </p>
        <div className="mt-3.5 flex flex-wrap gap-2.5">
          {seeds.map((seed, i) => {
            const meta = DIG_META[seed.kind];
            const related =
              seed.kind === "reading" && seed.ref
                ? READINGS.find((r) => r.id === seed.ref)
                : undefined;
            return (
              <button
                key={seed.text}
                type="button"
                onClick={() => follow(seed)}
                className="eco-in group max-w-full text-left transition-all duration-[160ms] ease-mk
                           hover:-translate-y-0.5 focus-visible:outline-none focus-visible:ring-2
                           focus-visible:ring-[#8A7F72]"
                style={{
                  ["--i" as string]: i,
                  // A bubble: fully round ends, so it reads as something to
                  // pick up rather than a row in a list.
                  borderRadius: 20,
                  padding: "12px 18px",
                  border: `1px solid color-mix(in srgb, ${meta.hue} 46%, transparent)`,
                  background: `color-mix(in srgb, ${meta.hue} 13%, rgba(240,233,224,.03))`,
                  boxShadow: `0 0 0 0 ${meta.hue}`,
                }}
              >
                <span className="flex items-center gap-1.5">
                  <span
                    className="eco-mono"
                    style={{ color: meta.hue, letterSpacing: "0.1em" }}
                  >
                    {meta.glyph} {meta.label}
                  </span>
                  {related ? (
                    <span className="eco-mono text-[#7C7166]" style={{ letterSpacing: 0 }}>
                      {related.minutes} 分钟
                    </span>
                  ) : null}
                </span>
                <span className="mt-1.5 block max-w-[46ch] text-mk-body leading-[1.7] text-[#EDE4D9]">
                  {seed.text}
                </span>
                {seed.why ? (
                  <span className="mt-1.5 block max-w-[46ch] text-mk-small leading-[1.7] text-[#8E8175]">
                    {seed.why}
                  </span>
                ) : null}
              </button>
            );
          })}
        </div>
      </div>
    </Drawer>
  );
}

const KIND_LABEL: Record<KeywordSource["kind"], { label: string; hue: string }> = {
  reading: { label: "阅读", hue: "var(--mk-lake)" },
  writing: { label: "写作", hue: "var(--mk-peach)" },
  project: { label: "项目", hue: "var(--mk-taro)" },
  news: { label: "新闻", hue: "var(--mk-mist)" },
  course: { label: "课程", hue: "var(--mk-matcha)" },
};

function SourceRow({ source, onNavigate }: { source: KeywordSource; onNavigate: () => void }) {
  const meta = KIND_LABEL[source.kind];
  const target =
    source.kind === "reading"
      ? { name: "readings" as const, id: source.id }
      : source.kind === "writing"
        ? { name: "writings" as const, id: source.id }
        : source.kind === "project"
          ? { name: "project" as const, id: source.id }
          : null;

  return (
    <li>
      <button
        type="button"
        disabled={!target}
        onClick={() => {
          if (!target) return;
          onNavigate();
          go(target);
        }}
        className={cx(
          "w-full rounded-mk-md p-3 text-left transition-colors duration-[120ms] ease-mk",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]",
          target ? "hover:bg-[rgba(240,233,224,.1)]" : "cursor-default",
        )}
        style={{
          border: "1px solid rgba(240,233,224,.13)",
          background: "rgba(240,233,224,.035)",
        }}
      >
        <span className="flex items-center gap-2">
          <span
            className="eco-mono rounded-mk-full px-2 py-0.5"
            style={{
              background: `color-mix(in srgb, ${meta.hue} 26%, transparent)`,
              color: "#E0D6C9",
              letterSpacing: 0,
            }}
          >
            {meta.label}
          </span>
          <span className="min-w-0 flex-1 truncate text-mk-body font-medium text-[#F0E9E0]">
            {source.label}
          </span>
          <span className="font-mono text-[11px] text-[#7C7166]">{source.date}</span>
        </span>
        {source.evidence ? (
          <span
            className="mt-2 block border-l-2 pl-3 text-mk-small italic leading-[1.75] text-[#C0B4A6]"
            style={{ borderColor: meta.hue }}
          >
            「{source.evidence}」
            <span className="mt-1 block not-italic text-[11px] text-[#7C7166]">你自己写的</span>
          </span>
        ) : null}
      </button>
    </li>
  );
}
