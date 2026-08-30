import { BookOpen, Hexagon, MessageCircle, Sprout, X } from "lucide-react";
import { useEco } from "../store";
import { DOMAIN_META } from "../data/news";
import { READINGS } from "../data/library";
import { FIELDS } from "../data/tree";
import { go } from "../route";
import type { NewsItem } from "../data/types";
import { Drawer, Sys } from "../ui";

/**
 * A planet, opened.
 *
 * The panel's order is the argument: **the question first**, then what
 * happened, then where it came from. A summary-first layout would let her
 * decide she is done before the question ever lands.
 *
 * Four exits, and they are the whole ecosystem in miniature — this is the
 * screen that proves the world is not a reading app with a starfield:
 *   读这篇 → 阅读   ·  问印记 → 对话  ·  收进我的树 → 关键词模型  ·  做成项目 → PBL
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
   *  「收进我的树」 label honest if that ever stops being true. */
  const seedKeyword = item.keywords[0] ?? "新发现";

  return (
    <Drawer open onClose={onClose} tone="dark" width={560} label={item.title[lang]}>
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
        {/* the hook, largest thing on the panel */}
        <p className="mt-5 text-[24px] font-semibold leading-[1.55] text-[#F7F1E9]">
          {item.hook[lang]}
        </p>

        <div className="eco-scanline my-5" />

        <h3 className="text-mk-h2 text-[#EFE7DC]">{item.title[lang]}</h3>
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
          <Sys tone="dark">编辑权重 {item.weight.toFixed(2)}</Sys>
        </div>

        <div className="mt-3 flex flex-wrap gap-1.5">
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

        {/* ── four exits ────────────────────────────────────────────────── */}
        <div className="eco-scanline my-6" />
        <Sys tone="dark" className="mb-3 block">
          你可以拿它做什么
        </Sys>

        <div className="grid grid-cols-2 gap-2.5">
          <Exit
            icon={<BookOpen size={17} strokeWidth={1.8} />}
            title="读这篇"
            sub={related ? `《${related.title}》· ${related.minutes} 分钟` : "去阅读室找一篇"}
            onClick={() => {
              onClose();
              go(related ? { name: "readings", id: related.id } : { name: "readings" });
            }}
          />
          <Exit
            icon={<MessageCircle size={17} strokeWidth={1.8} />}
            title="问印记"
            sub="带着这个问题去问"
            onClick={() => {
              onClose();
              openCoach("world", item.hook.zh);
            }}
          />
          <Exit
            icon={<Sprout size={17} strokeWidth={1.8} />}
            title={kept ? "已经在你的树上" : "收进我的树"}
            sub={kept ? `长在「${field.label}」这根枝上` : `会长成关键词「${seedKeyword}」`}
            disabled={kept}
            onClick={() => keep(item.id, seedKeyword, field.id)}
          />
          <Exit
            icon={<Hexagon size={17} strokeWidth={1.8} />}
            title="做成项目"
            sub="从这条新闻开一个 PBL"
            onClick={() => {
              if (!kept) keep(item.id, seedKeyword, field.id);
              onClose();
              go({ name: "project-new" });
            }}
          />
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
}: {
  icon: React.ReactNode;
  title: string;
  sub: string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="flex items-start gap-3 rounded-mk-md p-3.5 text-left transition-all duration-[140ms] ease-mk
                 hover:bg-[rgba(240,233,224,.1)] disabled:cursor-default disabled:opacity-55
                 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
      style={{ border: "1px solid rgba(240,233,224,.15)", background: "rgba(240,233,224,.04)" }}
    >
      <span className="mt-0.5 shrink-0" style={{ color: "#E0D4C4" }}>
        {icon}
      </span>
      <span className="min-w-0">
        <span className="block text-mk-body font-semibold text-[#F0E9E0]">{title}</span>
        <span className="mt-0.5 block text-mk-small leading-snug text-[#9A8E80]">{sub}</span>
      </span>
    </button>
  );
}

