import { useState } from "react";
import { BookOpen, Clock, ExternalLink, Loader2, X } from "lucide-react";
import { savePlanet, type ExplorePlanet } from "../api/explore";
import { putReadingSource } from "../api/readings";
import { fieldById } from "../tree/geometry";
import { liteRoutePath, navigate } from "../routing";
import type { FieldId } from "../tree/types";
import { Drawer, Sys } from "../tree/ui";
import type { Lang } from "./Planet";

/**
 * 一颗星球，被打开。**先读，再问，然后才是去哪读。**
 *
 * 这一面板的顺序就是它的论证：
 *
 *   这条新闻 → 它想问你 → 出处 → 学科透镜 → 现在读 / 稍后读
 *
 * 「它想问你」排在新闻本身后面、所有动作前面，因为这一屏的意义不是让她**知道
 * 一件事**，而是让她带着一个**想追下去的问题**离开。标题只是让她不用瞎猜；
 * 问题才是钩子。
 *
 * ## 出口从「收进我的树」换成「去读」（2026-09-07）
 *
 * 原来这里的唯一动作是收藏，而收藏会直接往树上种一个词。产品负责人指出这一步
 * 来得太早 —— 她看到的只有一个标题和两句摘要，还没读过任何东西，树上就多了一
 * 个词。词该长在她真的读完之后：读完会有报告，报告上再提出候选词让她自己认。
 *
 * 所以现在是两个动作，都落到阅读室的同一篇上（服务端幂等，迁移 0138）：
 *
 *  - **现在读** —— 原文另开一页，同时进阅读室那一篇。我们先替她试一次抓正文；
 *    抓不到就是空的，阅读室本来就有粘贴框。
 *  - **稍后读** —— 只建那一篇，她留在地图上。
 *
 * ## 为什么原文要另开一页
 *
 *   > we have a link to 读原文, but what I really hope is to read in our platform.
 *   > but I understand that, on our platform, it is difficult to fetch the original
 *   > content. then, maybe we can jump to the reading room, and also open a new
 *   > page, and invite students to paste here?
 *
 * 那篇文章在别人的网站上，抓不抓得到不由我们决定。抓到了她就在我们这儿读；
 * 抓不到，旁边那一页就是她能复制的那份。两种情况下她都已经在阅读室里了。
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
  const [busy, setBusy] = useState<"now" | "later" | null>(null);
  const [error, setError] = useState("");

  if (!item) return null;
  const meta = fieldById(item.field as FieldId);
  const title = lang === "zh" ? item.titleZh : item.titleEn || item.titleZh;

  /**
   * 落到阅读室的那一篇。服务端幂等 —— 同一颗星球永远是同一篇，所以「稍后读」
   * 之后再「现在读」不会多出第二篇。
   */
  async function ensureReading(): Promise<string> {
    if (!item) return "";
    if (item.readingId) return item.readingId;
    const saved = await savePlanet(item.id);
    onSaved(saved);
    return saved.readingId;
  }

  async function readNow() {
    if (!item) return;
    // 🚨 `window.open` 必须在 await 之前，否则浏览器不认它是点击引起的，
    // 直接当弹窗拦掉。
    if (item.url) window.open(item.url, "_blank", "noopener,noreferrer");
    setBusy("now");
    setError("");
    try {
      const id = await ensureReading();
      if (!id) throw new Error("阅读室没有返回这一篇的编号。");
      // 替她试一次抓正文。抓不到是**正常结果**，不是错误：阅读室会摆出粘贴框，
      // 而她要复制的那一页刚刚已经开在旁边了。
      if (item.url) {
        await putReadingSource(id, { title: item.titleZh, url: item.url }).catch(() => undefined);
      }
      navigate(liteRoutePath({ tab: "readings", readingId: id }));
    } catch (e: unknown) {
      // 动词 + 失败，再接后台原话（AGENTS.md §8）。
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
    }
  }

  async function readLater() {
    setBusy("later");
    setError("");
    try {
      await ensureReading();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(null);
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
          {/* 主枝名，没有编号。「NO.3」和球上那个角标是同一件事，而它读起来
              是一个未读计数。 */}
          <Sys tone="dark">{meta.label}</Sys>
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

        {/* ── 去读 ─────────────────────────────────────────────────────── */}
        <div className="mt-7">
          <div className="flex flex-wrap gap-2.5">
            <button
              type="button"
              onClick={() => void readNow()}
              disabled={busy !== null}
              className="inline-flex items-center gap-2 rounded-mk-full px-5 py-2.5 text-mk-body font-semibold
                         text-[#17130F] transition hover:opacity-90 disabled:opacity-45"
              style={{ background: meta.hue }}
            >
              {busy === "now" ? (
                <>
                  <Loader2 size={16} className="animate-spin" />
                  处理中
                </>
              ) : (
                <>
                  <BookOpen size={16} strokeWidth={1.9} />
                  现在读
                </>
              )}
            </button>

            {item.saved ? null : (
              <button
                type="button"
                onClick={() => void readLater()}
                disabled={busy !== null}
                className="inline-flex items-center gap-2 rounded-mk-full border px-5 py-2.5 text-mk-body
                           text-[#F0E9E0] transition-colors hover:bg-[rgba(240,233,224,.1)] disabled:opacity-45"
                style={{ borderColor: "rgba(240,233,224,.28)" }}
              >
                {busy === "later" ? (
                  <>
                    <Loader2 size={16} className="animate-spin" />
                    处理中
                  </>
                ) : (
                  <>
                    <Clock size={16} strokeWidth={1.9} />
                    稍后读
                  </>
                )}
              </button>
            )}
          </div>

          <p className="mt-2.5 max-w-[52ch] text-mk-small leading-[1.8] text-[#9A8E80]">
            {item.saved
              ? "这一篇已经在阅读室里了。「现在读」会直接打开它。"
              : "两个都会在阅读室里建这一篇；「现在读」还会另开一页放原文，让你把正文粘进来。读完之后，报告上会提出可以加进你树里的词。"}
          </p>

          {error ? (
            <p
              className="mt-3 rounded-mk-md p-3 text-mk-small leading-[1.8] text-[#F0D5D9]"
              style={{ background: "rgba(255,113,137,.12)", border: "1px solid rgba(255,113,137,.4)" }}
            >
              打开失败：{error}
            </p>
          ) : null}
        </div>
      </div>
    </Drawer>
  );
}
