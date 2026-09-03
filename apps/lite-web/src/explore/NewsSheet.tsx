import { useState } from "react";
import { BookmarkCheck, ExternalLink, Loader2, Sprout, X } from "lucide-react";
import { savePlanet, type ExplorePlanet } from "../api/explore";
import { fieldById } from "../tree/geometry";
import type { FieldId } from "../tree/types";
import { Drawer, Sys } from "../tree/ui";
import type { Lang } from "./Planet";

/**
 * 一颗星球，被打开。**先读，再问，最后才是收藏。**
 *
 * 这一面板的顺序就是它的论证：
 *
 *   这条新闻 → 它想问你 → 出处与原文 → 学科透镜 → 收进我的树
 *
 * 「它想问你」排在新闻本身后面、所有动作前面，因为这一屏的意义不是让她**知道
 * 一件事**，而是让她带着一个**想追下去的问题**离开。标题只是让她不用瞎猜；
 * 问题才是钩子。
 *
 * ## 只有一个动作，不是四个
 *
 * 原型这里有四个出口（去读 / 去写 / 做项目 / 问印记）。四个动作摆在一条三十秒
 * 前才见到的新闻后面，教的是「什么都可以马上开始」，而那不是真的。现在只有
 * **收进我的树** —— 它是一件轻的、诚实的事：这条新闻在你身上留下了一个词。
 * 项目要从树上长出来，那里有证据撑着。
 *
 * ## 收藏不是书签
 *
 * 按下去会往她的兴趣树上种一个真的关键词，evidence 就是上面那个问题 —— 她按下
 * 收藏时看着的就是它。所以**没有「取消收藏」**：那个词来自一件真的发生过的事。
 * 界面必须把这件事说清楚，而不是画一个可以来回切的书签图标。
 */
export function NewsSheet({
  item,
  lang,
  onClose,
  onSaved,
}: {
  item: ExplorePlanet | null;
  lang: Lang;
  onClose: () => void;
  onSaved: (p: ExplorePlanet) => void;
}) {
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  if (!item) return null;
  const meta = fieldById(item.field as FieldId);
  const title = lang === "zh" ? item.titleZh : item.titleEn || item.titleZh;

  async function save() {
    if (!item) return;
    setSaving(true);
    setError("");
    try {
      onSaved(await savePlanet(item.id));
    } catch (e: unknown) {
      // 动词 + 失败，再接后台原话（AGENTS.md §8）。绝不静默地当成功 —— 一个
      // 显示成已收藏、其实没进树的按钮，是这一屏最糟的谎。
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Drawer open onClose={onClose} width={600} label={item.titleZh}>
      <div className="flex items-center justify-between px-6 pt-5">
        <div className="flex items-center gap-2.5">
          <span
            className="h-2.5 w-2.5 rounded-mk-full"
            style={{ background: meta.hue, boxShadow: `0 0 12px ${meta.hue}` }}
          />
          <Sys tone="dark">
            {meta.label} · NO.{item.rank}
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
        {/* ── 这条新闻 ─────────────────────────────────────────────────── */}
        <h2 className="mt-3 text-mk-h1 leading-[1.45] text-[#F5EFE7]">{title}</h2>
        {item.summary ? (
          <p className="mt-3 text-mk-body-lg leading-[1.9] text-[#D9CEC1]">{item.summary}</p>
        ) : null}

        {/* ── 它想问你 ─────────────────────────────────────────────────── */}
        <div
          className="mt-6 rounded-mk-md p-4"
          style={{
            background: `color-mix(in srgb, ${meta.hue} 14%, rgba(240,233,224,.04))`,
            border: `1px solid color-mix(in srgb, ${meta.hue} 32%, transparent)`,
          }}
        >
          <Sys tone="dark">它想问你</Sys>
          <p className="mt-1.5 text-mk-h3 leading-[1.7] text-[#F5EFE7]">{item.hook}</p>
        </div>

        {/* ── 出处 ─────────────────────────────────────────────────────── */}
        <div className="mt-5 flex flex-wrap items-center gap-x-4 gap-y-2">
          <span className="inline-flex items-baseline gap-1.5">
            <Sys tone="dark">出处</Sys>
            <span className="text-mk-small text-[#F0E9E0]">{item.source}</span>
          </span>
          {item.publishedAt ? (
            <span className="inline-flex items-baseline gap-1.5">
              <Sys tone="dark">发布</Sys>
              <span className="font-mono text-mk-small text-[#F0E9E0]">
                {item.publishedAt.slice(0, 10)}
              </span>
            </span>
          ) : null}
        </div>
        {item.url ? (
          <a
            href={item.url}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-3 inline-flex items-center gap-1.5 rounded-mk-full border px-4 py-2 text-mk-small
                       text-[#F0E9E0] transition-colors hover:bg-[rgba(240,233,224,.1)]"
            style={{ borderColor: "rgba(240,233,224,.28)" }}
          >
            读原文
            <ExternalLink size={14} strokeWidth={1.8} />
          </a>
        ) : null}

        {/* ── 学科透镜 ─────────────────────────────────────────────────── */}
        {item.discipline ? (
          <div className="mt-6">
            <Sys tone="dark">这属于哪门学科</Sys>
            <div
              className="mt-2 rounded-mk-md p-4"
              style={{ border: "1px solid rgba(240,233,224,.14)", background: "rgba(240,233,224,.035)" }}
            >
              <strong className="block text-mk-h3 text-[#F0E9E0]">{item.discipline.zh}</strong>
              <span className="mt-0.5 block font-mono text-mk-small text-[#7C7166]">
                {item.discipline.en}
              </span>
              <p className="mt-2 text-mk-small leading-[1.8] text-[#C0B4A6]">{item.discipline.asks}</p>
            </div>
          </div>
        ) : null}

        {/* ── 收进我的树 ───────────────────────────────────────────────── */}
        <div className="mt-7">
          {item.saved ? (
            <div
              className="flex items-start gap-2.5 rounded-mk-md p-4"
              style={{ border: "1px solid rgba(147,192,136,.4)", background: "rgba(147,192,136,.1)" }}
            >
              <BookmarkCheck size={17} strokeWidth={1.9} color="#93C088" className="mt-0.5 shrink-0" />
              <div>
                <p className="text-mk-body text-[#EDE4D9]">
                  已收进你的树{item.keyword ? `：${item.keyword}` : ""}
                </p>
                <p className="mt-1 text-mk-small leading-[1.75] text-[#9A8E80]">
                  上面那个问题成了这个词的来源。你可以在「我的树」里点开它。
                </p>
              </div>
            </div>
          ) : (
            <>
              <button
                type="button"
                onClick={save}
                disabled={saving || !item.keyword}
                className="inline-flex items-center gap-2 rounded-mk-full px-5 py-2.5 text-mk-body font-semibold
                           text-[#17130F] transition hover:opacity-90 disabled:opacity-45"
                style={{ background: meta.hue }}
              >
                {saving ? (
                  <>
                    <Loader2 size={16} className="animate-spin" />
                    处理中
                  </>
                ) : (
                  <>
                    <Sprout size={16} strokeWidth={1.9} />
                    收进我的树
                  </>
                )}
              </button>
              <p className="mt-2.5 max-w-[52ch] text-mk-small leading-[1.8] text-[#9A8E80]">
                {item.keyword
                  ? `会在你的树上加一个词：「${item.keyword}」，并把上面那个问题记为它的来源。收进去之后不能撤销。`
                  : "这颗星球没有带关键词，暂时不能收进树。"}
              </p>
            </>
          )}
          {error ? (
            <p
              className="mt-3 rounded-mk-md p-3 text-mk-small leading-[1.8] text-[#F0D5D9]"
              style={{ background: "rgba(255,113,137,.12)", border: "1px solid rgba(255,113,137,.4)" }}
            >
              收藏失败：{error}
            </p>
          ) : null}
        </div>
      </div>
    </Drawer>
  );
}
