import { BookOpen, Hexagon, MessageCircle, PenLine, Sparkles, X } from "lucide-react";
import { useEco } from "../store";
import { fieldById } from "../data/tree";
import { go } from "../route";
import type { Keyword, KeywordSource } from "../data/types";
import { Drawer, Sys, cx } from "../ui";

/**
 * A keyword, opened.
 *
 * ## Why 来源 is the second section and not a footnote
 * The question a student actually has in front of a generated model of herself
 * is **「你凭什么这么说我？」**. So the drawer answers it immediately: what the
 * keyword is (one line, 印记's read), then every trace it was built from —
 * clickable, dated, and where possible carrying HER OWN SENTENCE. A model that
 * cannot show its evidence is a horoscope.
 *
 * ## Why the four exits repeat 世界's four exits
 * Same four verbs everywhere (读 / 写 / 做 / 聊) means she never has to learn
 * a second vocabulary. What changes is the object: in 世界 they act on a news
 * item, here they act on a part of herself.
 */
export function KeywordDrawer({ kw, onClose }: { kw: Keyword | null; onClose: () => void }) {
  const { openCoach } = useEco();
  if (!kw) return null;
  const f = fieldById(kw.field);

  return (
    <Drawer open onClose={onClose} width={520} label={kw.text}>
      <div className="flex items-start justify-between gap-4 px-6 pt-5">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="h-2.5 w-2.5 rounded-mk-full" style={{ background: f.hue }} />
            <Sys>{f.label} · KEYWORD</Sys>
          </div>
          <h2 className="mt-1.5 text-mk-h1 text-mk-ink">{kw.text}</h2>
          <p className="mt-0.5 font-mono text-mk-small text-mk-faint">{kw.en}</p>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="rounded-mk-full p-2 transition-colors duration-[120ms] hover:bg-mk-accent-50
                     focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          aria-label="关闭"
        >
          <X size={18} strokeWidth={1.8} />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-6">
        <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2">
          <span className="inline-flex items-baseline gap-1.5">
            <Sys>强度</Sys>
            <span className="font-mono text-mk-small tabular-nums text-mk-ink">
              {kw.strength} / 5
            </span>
          </span>
          <span className="inline-flex items-baseline gap-1.5">
            <Sys>来源</Sys>
            <span className="font-mono text-mk-small tabular-nums text-mk-ink">
              {kw.sources.length}
            </span>
          </span>
          <span className="inline-flex items-baseline gap-1.5">
            <Sys>出现于</Sys>
            <span className="font-mono text-mk-small text-mk-ink">
              {["三月", "五月", "七月", "本月"][kw.bornAt]}
            </span>
          </span>
        </div>

        {/* 印记's read */}
        <div
          className="mt-5 rounded-mk-md p-4"
          style={{ background: `color-mix(in srgb, ${f.hue} 12%, var(--mk-surface))` }}
        >
          <Sys>印记 看见的</Sys>
          <p className="mt-1.5 text-mk-body-lg leading-[1.85] text-mk-ink">{kw.note}</p>
        </div>

        {/* 高光时刻 */}
        {kw.shining ? (
          <div
            className="mt-4 rounded-mk-md border p-4"
            style={{ borderColor: "var(--mk-butter)", background: "var(--mk-butter-bg)" }}
          >
            <div className="flex items-center gap-2">
              <Sparkles size={15} strokeWidth={2} color="#C9962B" />
              <Sys className="!text-[#8A6320]">高光时刻 · {kw.shining.date}</Sys>
            </div>
            <p className="mt-2 text-mk-h3 text-[#6B4D14]">{kw.shining.title}</p>
            <p className="mt-1.5 text-mk-body leading-[1.85] text-[#7A5A1D]">{kw.shining.body}</p>
          </div>
        ) : null}

        {/* sources — the evidence */}
        <h3 className="mt-6 text-mk-h3 text-mk-ink">它是从哪来的</h3>
        <p className="mt-1 text-mk-small text-mk-muted">
          这个词不是猜的。下面每一条都是你做过的事，点开可以回去看。
        </p>
        <ul className="mt-3 space-y-2">
          {kw.sources.map((s) => (
            <SourceRow key={`${s.kind}-${s.id}`} source={s} onNavigate={onClose} />
          ))}
        </ul>

        {/* four exits */}
        <hr className="eco-hair my-6" />
        <Sys className="mb-3 block">从这里继续</Sys>
        <div className="grid grid-cols-2 gap-2.5">
          <Next
            icon={<BookOpen size={16} strokeWidth={1.8} />}
            label="再读一篇"
            sub="让它多一个来源"
            onClick={() => {
              onClose();
              go({ name: "readings" });
            }}
          />
          <Next
            icon={<PenLine size={16} strokeWidth={1.8} />}
            label="写一篇"
            sub="写过的词会长得最快"
            onClick={() => {
              onClose();
              go({ name: "writings" });
            }}
          />
          <Next
            icon={<Hexagon size={16} strokeWidth={1.8} />}
            label="做个项目"
            sub="把它变成一件真东西"
            onClick={() => {
              onClose();
              go({ name: "project-new" });
            }}
          />
          <Next
            icon={<MessageCircle size={16} strokeWidth={1.8} />}
            label="问印记"
            sub={`聊聊「${kw.text}」`}
            onClick={() => {
              onClose();
              openCoach("tree", kw.text);
            }}
          />
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
          "w-full rounded-mk-md border border-mk-border bg-mk-surface p-3 text-left transition-colors",
          "duration-[120ms] ease-mk focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
          target ? "hover:border-mk-accent-200 hover:bg-mk-accent-50" : "cursor-default",
        )}
      >
        <span className="flex items-center gap-2">
          <span
            className="eco-mono rounded-mk-full px-2 py-0.5"
            style={{ background: `color-mix(in srgb, ${meta.hue} 26%, transparent)`, color: "var(--mk-secondary)", letterSpacing: 0 }}
          >
            {meta.label}
          </span>
          <span className="min-w-0 flex-1 truncate text-mk-body font-medium text-mk-ink">
            {source.label}
          </span>
          <span className="font-mono text-[11px] text-mk-faint">{source.date}</span>
        </span>
        {source.evidence ? (
          <span
            className="mt-2 block border-l-2 pl-3 text-mk-small italic leading-[1.75] text-mk-secondary"
            style={{ borderColor: meta.hue }}
          >
            「{source.evidence}」
            <span className="mt-1 block not-italic text-[11px] text-mk-faint">你自己写的</span>
          </span>
        ) : null}
      </button>
    </li>
  );
}

function Next({
  icon,
  label,
  sub,
  onClick,
}: {
  icon: React.ReactNode;
  label: string;
  sub: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-start gap-2.5 rounded-mk-md border border-mk-border bg-mk-surface p-3 text-left
                 transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:bg-mk-accent-50
                 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span className="mt-0.5 shrink-0 text-mk-accent-700">{icon}</span>
      <span className="min-w-0">
        <span className="block text-mk-body font-semibold text-mk-ink">{label}</span>
        <span className="mt-0.5 block text-mk-small leading-snug text-mk-muted">{sub}</span>
      </span>
    </button>
  );
}

