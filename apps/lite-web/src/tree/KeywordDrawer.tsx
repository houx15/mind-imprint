import { Sparkles, X } from "lucide-react";
import { fieldById } from "./geometry";
import type { Keyword, KeywordSource } from "./types";
import { Drawer, Sys, cx } from "./ui";
import { DigSection } from "./DigSection";
import { liteRoutePath, navigate } from "../routing";

/**
 * 一个关键词，被打开。两节，顺序就是这个顺序：
 *
 *   摘要 · 相关活动
 *
 * ## 为什么「相关活动」是第二节而不是脚注
 * 一个学生面对一份生成出来的、关于她自己的模型，真正会有的问题是
 * **「你凭什么这么说我？」**。所以抽屉立刻回答它：这个词是什么（印记的一句
 * 话），然后是它由哪些痕迹长出来的——可点、有日期，并且尽可能带着**她自己
 * 写的那句话**。一个拿不出证据的模型是星座运势。
 *
 * ## 促成这次搬家的那件事
 * 原型里这份列表是**死的**：「只有项目活在这个原型里，阅读和写作作为证据显示，
 * 但在这里不通向任何地方」。搬进 lite 之后 `/readings/:id`、`/writings/:id`、
 * `/projects/:id` 都是真页面，所以每一条来源现在**真的能点回去**。这正是这次
 * 搬家的意义：证据从「一句声称」变成「一条路」。
 *
 * ## 继续深挖（P5，2026-09-03 补上）
 * 原型的第三节是按 mock 关键词 id 手写在 `eco/data/dig.ts` 里的四颗种子，对真
 * 关键词（uuid）一个都命不中，所以搬家时没跟过来。现在它回来了，并且是**按她
 * 在这个词上留下的原话现生成**的（`DigSection` → `GET /interest/keywords/:id/dig`），
 * 其中三颗能一键变成 lite 里一个真的房间。
 */
export function KeywordDrawer({ kw, onClose }: { kw: Keyword | null; onClose: () => void }) {
  if (!kw) return null;
  const f = fieldById(kw.field);
  // 🚨 写**它是哪天第一次出现的**，不是成长轴上那一格的名字（2026-09-07）。
  // 那一格的名字是相对她整段跨度算出来的，随着她继续用还会变 —— 同一个词今天
  // 写着「4 个月前」，下个月写着「5 个月前」。而「什么时候加进来的」只有一个
  // 答案，就是这个日期。
  const bornLabel = kw.firstSeenAt ? kw.firstSeenAt.slice(0, 10) : "不知道";

  return (
    <Drawer open onClose={onClose} width={560} label={kw.text} tone="light">
      <div className="flex items-start justify-between gap-4 px-6 pt-5">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className="h-2.5 w-2.5 rounded-mk-full"
              style={{ background: f.hue }}
            />
            <Sys>{f.label} · KEYWORD</Sys>
          </div>
          <h2 className="mt-1.5 text-mk-h1 text-mk-ink">{kw.text}</h2>
          <p className="mt-0.5 font-mono text-mk-small text-mk-faint">{kw.en}</p>
        </div>
        <button
          type="button"
          onClick={onClose}
          className="rounded-mk-full p-2 transition-colors duration-[120ms] hover:bg-[rgba(51,48,46,.05)]
                     focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-300"
          aria-label="关闭"
        >
          <X size={18} strokeWidth={1.8} color="var(--tree-ink-3)" />
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
            <Sys>第一次出现</Sys>
            <span className="font-mono text-mk-small text-mk-ink">{bornLabel}</span>
          </span>
        </div>

        {/* ── 摘要 ─────────────────────────────────────────────────────── */}
        {kw.note ? (
          <div
            className="mt-5 rounded-mk-md p-4"
            style={{
              background: `color-mix(in srgb, ${f.hue} 8%, var(--tree-card))`,
              border: `1px solid color-mix(in srgb, ${f.hue} 28%, transparent)`,
            }}
          >
            <Sys>摘要</Sys>
            <p className="mt-1.5 text-mk-body-lg leading-[1.85] text-mk-ink">{kw.note}</p>
          </div>
        ) : null}

        {kw.shining ? (
          <div
            className="mt-4 rounded-mk-md p-4"
            style={{ border: "1px solid rgba(189,146,43,.4)", background: "rgba(189,146,43,.09)" }}
          >
            <div className="flex items-center gap-2">
              <Sparkles size={15} strokeWidth={2} color="#A8801F" />
              <Sys className="!text-[#8A6A18]">
                做得最好的一次 · {kw.shining.date}
              </Sys>
            </div>
            <p className="mt-2 text-mk-h3 text-mk-ink">{kw.shining.title}</p>
            <p className="mt-1.5 text-mk-body leading-[1.85] text-mk-secondary">{kw.shining.body}</p>
          </div>
        ) : null}

        {/* ── 相关活动 ─────────────────────────────────────────────────── */}
        <h3 className="mt-7 text-mk-h3 text-mk-ink">相关活动</h3>
        <ul className="mt-3 space-y-2">
          {kw.sources.map((s) => (
            <SourceRow key={`${s.kind}-${s.id}`} source={s} onNavigate={onClose} />
          ))}
        </ul>

        {/* ── 继续深挖 ─────────────────────────────────────────────────── */}
        <DigSection keywordId={kw.id} />
      </div>
    </Drawer>
  );
}

const KIND_LABEL: Record<KeywordSource["kind"], { label: string; hue: string }> = {
  reading: { label: "阅读", hue: "var(--mk-lake)" },
  writing: { label: "写作", hue: "var(--mk-peach)" },
  project: { label: "项目", hue: "var(--mk-taro)" },
  news: { label: "新闻", hue: "var(--mk-mist)" },
  course: { label: "测试", hue: "var(--mk-matcha)" },
};

/**
 * 来源在 lite 里的落点。
 *
 * `news` 和 `course`（兴趣测试）没有可打开的页面：一条新闻还没有自己的房间
 * （P4），一次测试是一个已经结束的时刻。它们照常显示为证据，只是不可点——
 * 一个点了什么都不发生的按钮，比一个明摆着不能点的行更糟。
 */
function pathFor(source: KeywordSource): string | null {
  if (!source.id) return null;
  switch (source.kind) {
    case "reading":
      return liteRoutePath({ tab: "readings", readingId: source.id });
    case "writing":
      return liteRoutePath({ tab: "writings", writingId: source.id });
    case "project":
      return liteRoutePath({ tab: "projects", projectId: source.id });
    default:
      return null;
  }
}

function SourceRow({ source, onNavigate }: { source: KeywordSource; onNavigate: () => void }) {
  const meta = KIND_LABEL[source.kind];
  const target = pathFor(source);

  return (
    <li>
      <button
        type="button"
        disabled={!target}
        onClick={() => {
          if (!target) return;
          onNavigate();
          navigate(target);
        }}
        className={cx(
          "w-full rounded-mk-md p-3 text-left transition-colors duration-[120ms] ease-mk",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-300",
          target ? "hover:bg-[rgba(51,48,46,.05)]" : "cursor-default",
        )}
        style={{ border: "1px solid var(--tree-line)", background: "var(--mk-paper)" }}
      >
        <span className="flex items-center gap-2">
          <span
            className="tree-mono rounded-mk-full px-2 py-0.5"
            style={{
              background: `color-mix(in srgb, ${meta.hue} 16%, transparent)`,
              color: `color-mix(in srgb, ${meta.hue} 72%, var(--tree-ink))`,
              letterSpacing: 0,
            }}
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
