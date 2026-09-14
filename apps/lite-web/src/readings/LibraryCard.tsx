import { useState } from "react";
import { Check } from "lucide-react";
import { fieldById } from "../tree/geometry";
import type { FieldId } from "../tree/types";
import type { LibraryArticle, LibraryLevel } from "../api/library";

/**
 * LibraryCard —— 分级阅读库里的一张卡。
 *
 * 一张卡要在两个位置上都成立：落地页的推荐位（四张一排），和「全部文章」那一
 * 页的网格。所以卡自己不管排版，只管一件事：把一篇文章说清楚，并让她挑一档。
 *
 * 卡上的东西，按她真正会用到的顺序：
 *
 *   照片    这是新闻报道，照片是它的一部分；一书架灰框子谁也不想翻。
 *   中文标题 英文标题在下面一行、小一号 —— 她在中文界面里扫标题。
 *   一句话   这篇里有什么。不是摘要，是理由。
 *   学科标签 兴趣树上那七根主枝的颜色。同一门学科在树上和在这里是同一个颜色，
 *           所以「这篇属于我关心的那一块」不需要文字解释。
 *   五档     点开才展开。默认那一档已经选好（服务端按她读过的档位给的），
 *           所以最快的路径是：点卡片 → 点「开始读」。
 *
 * 颜色靠 `.mk-branch-hues` 作用域里的七个变量（见 tree/branchHues.css）。
 * 🚨 挂这张卡的容器必须带上那个类，否则七个变量一声不响地全部失效 —— 不报错，
 * 只是颜色没了。
 */

export interface LibraryCardProps {
  article: LibraryArticle;
  /** 默认选中的档位（1..5）。 */
  defaultTier: number;
  /** 命中的学科中文名。有值时卡上多一行「因为你关心 X」。 */
  why?: string[];
  busy?: boolean;
  onStart: (slug: string, tier: number) => void;
  onResume: (readingId: string) => void;
}

export function LibraryCard({ article, defaultTier, why, busy, onStart, onResume }: LibraryCardProps) {
  // 卡片跟着页面上的「默认难度」走，一张不落。
  //
  // 🚨 上一版在这里加过一条例外：她手上还开着这一篇时，卡片停在她打开的那一
  // 档。那条例外修的是一个真问题（回到书架点下去会开出同一篇的第二条记录），
  // 但它的代价是**这一排难度按钮对她读过的那几篇不起作用** —— 报回来的原话是
  // 「I clicked 入门 but not all papers changed to 入门」。
  //
  // 现在两件事分开做：难度按钮管所有卡片，而她开着的那一次单独出一个
  // 「继续读」按钮（见下面的 openTier）。她想换一档重读同一篇是**正当的意图**，
  // 现在它是一次明确的点击，不再是一次误触。
  const [tier, setTier] = useState(defaultTier);
  const [picking, setPicking] = useState(false);
  const hue = "var(--mk-accent-500)";
  const level = article.levels.find((l) => l.tier === tier) ?? article.levels[1] ?? article.levels[0];
  // 她手上还开着的那一档（没有就是 0）。
  const openTier = (article.readingId && !article.finished ? article.readTier : 0) ?? 0;
  const openLevel = openTier ? article.levels.find((l) => l.tier === openTier) : undefined;

  return (
    <article
      className="reading-library-card flex flex-col overflow-hidden rounded-mk-md border border-mk-border bg-mk-surface shadow-mk-xs transition-shadow duration-[120ms] ease-mk hover:shadow-mk-sm"
    >
      {article.coverUrl && (
        <img
          src={article.coverUrl}
          alt={article.zhTitle}
          loading="lazy"
          className="reading-library-cover w-full object-cover"
        />
      )}

      <div className="reading-library-content flex flex-1 flex-col gap-2">
        <div className="flex items-start justify-between gap-2">
          <h3 className="text-mk-body font-semibold leading-snug text-mk-ink">{article.zhTitle}</h3>
          {article.finished && (
            <span className="mt-0.5 flex shrink-0 items-center gap-1 text-mk-label text-mk-muted">
              <Check size={13} aria-hidden="true" />
              已完成
            </span>
          )}
        </div>
        <p className="reading-library-english text-mk-muted">{article.title}</p>
        <p className="reading-library-description text-mk-secondary">{article.reason}</p>

        <div className="mt-0.5 flex flex-wrap gap-1.5">
          {article.tags.map((t) => (
            <span
              key={t.id}
              className="rounded-mk-full px-2 py-0.5 text-mk-label"
              style={{
                background: `color-mix(in srgb, ${fieldById(t.field as FieldId).hue} 16%, transparent)`,
                color: `color-mix(in srgb, ${fieldById(t.field as FieldId).hue} 72%, black)`,
              }}
            >
              {t.zh}
            </span>
          ))}
        </div>

        {why && why.length > 0 && (
          <p className="text-mk-label text-mk-muted">因为你关心{why.join("、")}</p>
        )}

        <div className="reading-library-footer mt-auto">
          {picking ? (
            <fieldset className="flex flex-col gap-2">
              <legend className="pb-1.5 text-mk-label text-mk-muted">选择难度</legend>
              <div className="flex flex-wrap gap-1.5">
                {article.levels.map((l) => (
                  <LevelChip key={l.tier} level={l} active={l.tier === tier} hue={hue} onPick={() => setTier(l.tier)} />
                ))}
              </div>
              <p className="text-mk-label text-mk-muted">
                {level ? levelSummary(level) : ""}
              </p>
            </fieldset>
          ) : (
            <p className="text-mk-label text-mk-muted">{level ? levelSummary(level) : ""}</p>
          )}

          <div className="mt-2 flex flex-wrap items-center gap-2">
            <button
              type="button"
              disabled={busy}
              onClick={() => {
                if (openTier === tier && article.readingId) {
                  onResume(article.readingId);
                  return;
                }
                onStart(article.slug, tier);
              }}
              className="reading-library-start rounded-mk-full px-3.5 py-1.5 text-mk-small font-medium text-white transition-opacity duration-[120ms] ease-mk disabled:opacity-50"
              style={{ background: `color-mix(in srgb, ${hue} 82%, black)` }}
            >
              {/* 🚨 不叫「开始阅读」：粘贴框那个提交按钮已经叫这个名字了，
                  同一页上两个同名按钮，她点哪个都说不清。 */}
              {openTier === tier ? "继续读" : "读这一篇"}
            </button>
            <button
              type="button"
              onClick={() => setPicking((v) => !v)}
              className="reading-library-change rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700"
            >
              {picking ? "收起难度" : "换一档"}
            </button>
          </div>
          {/* 她开着的那一次，摆在自己的位置上。主按钮已经被页面的默认难度占了，
              这一行是那一次阅读的唯一入口 —— 少了它，「读这一篇」就会在她背后
              开出同一篇文章的第二条记录。 */}
          {openTier > 0 && openTier !== tier && article.readingId && (
            <button
              type="button"
              onClick={() => onResume(article.readingId!)}
              className="mt-2 text-mk-label text-mk-secondary underline decoration-mk-border underline-offset-2 transition-colors duration-[120ms] ease-mk hover:text-mk-accent-700"
            >
              继续读你开着的那一档 · {openLevel ? openLevel.name : ""}
            </button>
          )}
        </div>
      </div>
    </article>
  );
}

function LevelChip({
  level,
  active,
  hue,
  onPick,
}: {
  level: LibraryLevel;
  active: boolean;
  hue: string;
  onPick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onPick}
      aria-pressed={active}
      className="rounded-mk-full border px-2.5 py-1 text-mk-label transition-colors duration-[120ms] ease-mk"
      style={
        active
          ? { borderColor: hue, background: `color-mix(in srgb, ${hue} 18%, transparent)`, color: `color-mix(in srgb, ${hue} 74%, black)` }
          : { borderColor: "var(--mk-border)", color: "var(--mk-secondary)" }
      }
    >
      {level.name}
    </button>
  );
}

/** 一档的二行说明。Lexile 是一个真实测量，改名成「进阶」之后仍然留着 ——
 *  原文那一档没有分级值，就不显示一个编出来的数字。 */
export function levelSummary(level: LibraryLevel): string {
  const lex = level.lexile > 0 ? `${level.lexile}L · ` : "";
  return `${level.name} · ${lex}${level.words} 词 · 约 ${level.minutes} 分钟`;
}
