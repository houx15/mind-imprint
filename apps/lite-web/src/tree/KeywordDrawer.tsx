import { Sparkles, X } from "lucide-react";
import { GROWTH_STOPS, fieldById } from "./geometry";
import type { Keyword, KeywordSource } from "./types";
import { Drawer, Sys, cx } from "./ui";
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
 * ## 继续深挖去哪了
 * 原型的第三节是四个具体的深挖种子（想一想 / 去读 / 去写 / 去做），它们是按
 * mock 关键词的 id 手写在 `eco/data/dig.ts` 里的，对真关键词（uuid）一个都命
 * 不中。与其在一个真的观察后面摆四个空动词，不如先不摆——那一节是 P5 的活，
 * 要由模型按她这个词的真实来源现生成。
 */
export function KeywordDrawer({ kw, onClose }: { kw: Keyword | null; onClose: () => void }) {
  if (!kw) return null;
  const f = fieldById(kw.field);
  // 🚨 用 GROWTH_STOPS 的标签，不是写死的 ["三月","五月","七月","本月"]。那四个
  // 绝对月份对一个二月开始的学生是错的——刻度本来就是**相对她自己的起点**的。
  const bornLabel = GROWTH_STOPS[kw.bornAt]?.label ?? GROWTH_STOPS[GROWTH_STOPS.length - 1]!.label;

  return (
    <Drawer open onClose={onClose} width={560} label={kw.text}>
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
            <span className="font-mono text-mk-small text-[#F0E9E0]">{bornLabel}</span>
          </span>
        </div>

        {/* ── 摘要 ─────────────────────────────────────────────────────── */}
        {kw.note ? (
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
        ) : null}

        {kw.shining ? (
          <div
            className="mt-4 rounded-mk-md p-4"
            style={{ border: "1px solid rgba(201,150,43,.4)", background: "rgba(201,150,43,.1)" }}
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
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]",
          target ? "hover:bg-[rgba(240,233,224,.1)]" : "cursor-default",
        )}
        style={{ border: "1px solid rgba(240,233,224,.13)", background: "rgba(240,233,224,.035)" }}
      >
        <span className="flex items-center gap-2">
          <span
            className="tree-mono rounded-mk-full px-2 py-0.5"
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
