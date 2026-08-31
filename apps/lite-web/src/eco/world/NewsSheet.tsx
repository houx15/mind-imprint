import { BookOpen, Bookmark, BookmarkCheck, MessageCircle, X } from "lucide-react";
import { useEco } from "../store";
import { DOMAIN_META } from "../data/news";
import { READINGS } from "../data/library";
import { FIELDS } from "../data/tree";
import { go } from "../route";
import type { NewsItem } from "../data/types";
import { Drawer, Sys } from "../ui";

/**
 * A planet, opened. **A read-first journey.**
 *
 * The order of this panel is the argument, and it changed on 2026-08-31:
 *
 *   导读 → 这条新闻本身 → 来源 → 三个问题 → 开始探索
 *
 * 导读 comes first because it is the only line on the panel written FOR HER —
 * one sentence in the second person telling her what to watch for. Then the
 * story. Then the questions, which is where the panel earns its keep: the
 * point of a news planet is not that she knows a fact, it is that she leaves
 * holding a question she wants to chase.
 *
 * Every question is a BUTTON. Clicking one carries it straight into 印记 as
 * her opening line, so the distance between "that's interesting" and
 * "I'm working on it" is one click.
 *
 * Three exits, not four. 「做成项目」 was removed: a project has to come from
 * something she has actually chewed on, and offering it thirty seconds after
 * a headline taught the opposite lesson. Projects begin from 我的兴趣树, where
 * there is evidence behind the interest.
 */
export function NewsSheet({ item, onClose }: { item: NewsItem | null; onClose: () => void }) {
  const { state, keep, openCoach } = useEco();
  if (!item) return null;

  const meta = DOMAIN_META[item.domain];
  const lang = state.lang;
  const kept = state.kept.includes(item.id);
  const related = READINGS.find((r) => r.keywords.some((k) => item.keywords.includes(k)));
  const field = FIELDS.find((f) => f.id === (related?.field ?? "society")) ?? FIELDS[0]!;
  /** Every news item carries at least one keyword; the fallback keeps the
   *  收藏 copy honest if that ever stops being true. */
  const seedKeyword = item.keywords[0] ?? "新发现";
  /** The whisper question plus the two that survive the article. */
  const questions = [item.hook, ...item.hooks];

  return (
    <Drawer open onClose={onClose} tone="dark" width={600} label={item.title[lang]}>
      <div className="flex items-center justify-between px-6 pt-5">
        <div className="flex items-center gap-2.5">
          <span className="h-2.5 w-2.5 rounded-mk-full" style={{ background: meta.hue }} />
          <Sys tone="dark">
            {lang === "zh" ? meta.zh : meta.en} · NO.{item.rank} · {item.date}
          </Sys>
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
        {/* ── 导读 — the largest thing on the panel ────────────────────── */}
        <div
          className="mt-5 rounded-mk-lg p-5"
          style={{
            background: `color-mix(in srgb, ${meta.hue} 13%, rgba(240,233,224,.04))`,
            border: `1px solid color-mix(in srgb, ${meta.hue} 30%, transparent)`,
          }}
        >
          <Sys tone="dark">导读</Sys>
          <p className="mt-2 text-[22px] font-semibold leading-[1.6] text-[#F7F1E9]">
            {item.lead[lang]}
          </p>
        </div>

        {/* ── the story itself ─────────────────────────────────────────── */}
        <h3 className="mt-6 text-mk-h2 text-[#EFE7DC]">{item.title[lang]}</h3>
        <p className="mt-3 text-mk-body-lg leading-[1.9] text-[#CDC1B4]">{item.summary[lang]}</p>

        {/* the other language, always available inline — she is in an
            international track; reading the same fact twice in two languages
            is a real study move, not a toggle for show. */}
        <details className="mt-4 rounded-mk-md p-3" style={{ background: "rgba(240,233,224,.05)" }}>
          <summary className="cursor-pointer text-mk-small text-[#A0947F]">
            {lang === "zh" ? "English version" : "中文版本"}
          </summary>
          <p className="mt-2 text-mk-body leading-[1.85] text-[#BDB2A4]">
            <span className="block font-semibold text-[#DCD2C6]">
              {item.title[lang === "zh" ? "en" : "zh"]}
            </span>
            <span className="mt-1.5 block">{item.summary[lang === "zh" ? "en" : "zh"]}</span>
          </p>
        </details>

        <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-2">
          <Sys tone="dark">来源 {item.source}</Sys>
          <div className="flex flex-wrap gap-1.5">
            {item.keywords.map((k) => (
              <span
                key={k}
                className="rounded-mk-full px-2.5 py-1 text-mk-small"
                style={{ background: "rgba(240,233,224,.07)", color: "#C6B9AA" }}
              >
                {k}
              </span>
            ))}
          </div>
        </div>

        {/* ── the questions ───────────────────────────────────────────── */}
        <div className="eco-scanline my-6" />
        <Sys tone="dark">带着这些问题去读</Sys>
        <p className="mt-1.5 text-mk-small text-[#8E8175]">
          点任何一个，直接把它拿去和印记聊。
        </p>
        <ul className="mt-3 space-y-2">
          {questions.map((q, i) => (
            <li key={q.zh}>
              <button
                type="button"
                onClick={() => {
                  onClose();
                  openCoach("world", q.zh);
                }}
                className="group flex w-full items-start gap-3 rounded-mk-lg p-3.5 text-left
                           transition-colors duration-[140ms] ease-mk hover:bg-[rgba(240,233,224,.1)]
                           focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
                style={{
                  border: "1px solid rgba(240,233,224,.15)",
                  background: "rgba(240,233,224,.035)",
                }}
              >
                <span
                  className="eco-mono mt-1 shrink-0"
                  style={{ color: meta.hue, letterSpacing: 0 }}
                >
                  Q{i + 1}
                </span>
                <span className="min-w-0 flex-1 text-mk-body-lg leading-[1.75] text-[#EDE4D9]">
                  {q[lang]}
                </span>
                <MessageCircle
                  size={15}
                  strokeWidth={1.8}
                  className="mt-1 shrink-0 opacity-0 transition-opacity group-hover:opacity-70"
                  color="#C6B9AA"
                />
              </button>
            </li>
          ))}
        </ul>

        {/* ── 开始探索 ─────────────────────────────────────────────────── */}
        <div className="eco-scanline my-6" />
        <h4 className="text-[20px] font-semibold text-[#F5EFE7]">开始探索</h4>
        <p className="mt-1 text-mk-small text-[#8E8175]">
          先读一篇把它读懂，再决定要不要往下走。
        </p>

        <div className="mt-4 space-y-2.5">
          {/* 🚨 阅读 used to be the full-width primary here and it opened the
              prototype's own reading room. That room was dropped — it is
              already built for real in lite, and a rougher copy of a shipped
              screen invites feedback on a design nobody intends to build. So
              this sheet now offers only what the prototype can actually
              honour: take the question to 印记, or keep it for later. */}
          <Exit
            primary
            icon={<MessageCircle size={18} strokeWidth={1.8} />}
            title="与 AI 讨论"
            sub="带着第一个问题去问"
            onClick={() => {
              onClose();
              openCoach("world", item.hook.zh);
            }}
          />
          <div className="grid grid-cols-2 gap-2.5">
            <Exit
              icon={
                kept ? (
                  <BookmarkCheck size={17} strokeWidth={1.8} />
                ) : (
                  <Bookmark size={17} strokeWidth={1.8} />
                )
              }
              title={kept ? "已收藏" : "先收藏"}
              sub={kept ? "已经长到你的兴趣树上" : "留着以后读，顺带长到树上"}
              disabled={kept}
              onClick={() => keep(item.id, seedKeyword, field.id)}
            />
          </div>
        </div>
      </div>
    </Drawer>
  );
}

function Exit({
  icon,
  title,
  sub,
  onClick,
  disabled,
  primary,
}: {
  icon: React.ReactNode;
  title: string;
  sub: string;
  onClick: () => void;
  disabled?: boolean;
  primary?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="flex w-full items-center gap-3 rounded-mk-md p-3.5 text-left transition-all duration-[140ms]
                 ease-mk hover:bg-[rgba(240,233,224,.12)] disabled:cursor-default disabled:opacity-55
                 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
      style={{
        border: primary
          ? "1px solid rgba(240,233,224,.42)"
          : "1px solid rgba(240,233,224,.15)",
        background: primary ? "rgba(240,233,224,.11)" : "rgba(240,233,224,.04)",
      }}
    >
      <span className="shrink-0" style={{ color: "#E0D4C4" }}>
        {icon}
      </span>
      <span className="min-w-0">
        <span
          className="block font-semibold text-[#F0E9E0]"
          style={{ fontSize: primary ? 17 : 15 }}
        >
          {title}
        </span>
        <span className="mt-0.5 block text-mk-small leading-snug text-[#9A8E80]">{sub}</span>
      </span>
    </button>
  );
}
