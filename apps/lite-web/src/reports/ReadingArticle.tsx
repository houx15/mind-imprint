import { useEffect, useState } from "react";
import { getReadingSource, type ReadingSource } from "../api/readings";
import { useAlive } from "../shared/useAlive";
import { apiErrorText } from "../api/errorText";

/**
 * 读完之后，回头看一眼那篇文章。
 *
 * 🚨 一篇读完的文章，她**再也打不开了**：读完的阅读点进去是报告不是房间
 *（那是刻意的），而报告上原来没有任何通向正文的路。走查里她连着说了两次：
 *   「屏幕上没有英文正文了，只有一个总结页面」
 *   「正文不见了，我没法回头看了」
 * 写作那一侧读完之后是留得住的（成稿本来就在报告里），阅读这一侧留不住 ——
 * 这是两个房间不一致，不是一条新规矩。
 *
 * ## 为什么是**现取**，不是塞进报告那个 blob
 *
 * 报告会被分享出去（`atom_report_share.go`）。写作那一篇是**她自己写的字**，
 * 分享出去没问题；而这里的正文是**别人的文章** —— 把它写进报告的 blob，
 * 等于每一条分享链接都在公开转载一篇原文。
 *
 * 现取走的是那条要登录、要归属的老接口（GET /readings/{id}/source），
 * 所以「只有她自己看得到」是接口本身保证的，不是我们记得去删某个字段。
 * 顺带也不用为已经存在的报告做回填。
 *
 * 她不点就不取：一次多余的请求也没有。
 */
export function ReadingArticle({ atomId, defaultOpen = false }: { atomId: string; defaultOpen?: boolean }) {
  // `defaultOpen` 是给完成页那一格用的：在一个叫「原文」的页签底下还要她先按
  // 一颗「再看一遍这篇文章」，是让她为同一件事点两次。折叠那一版留着 —— 它
  // 本来是摆在报告里的形态。
  const [open, setOpen] = useState(defaultOpen);
  const [source, setSource] = useState<ReadingSource | null>(null);
  const [error, setError] = useState<string | null>(null);
  const alive = useAlive();

  useEffect(() => {
    if (!open || source) return;
    getReadingSource(atomId)
      .then((s) => {
        if (alive.current) setSource(s);
      })
      .catch((err) => {
        if (alive.current) setError(apiErrorText(err));
      });
  }, [open, source, atomId, alive]);

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="self-start rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small text-mk-secondary transition-colors duration-[120ms] ease-mk hover:border-mk-accent-200 hover:text-mk-accent-700"
      >
        再看一遍这篇文章
      </button>
    );
  }

  return (
    <section className="mk-rp-section">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-mk-body-lg font-semibold text-mk-ink">{source?.title ?? "这篇文章"}</h3>
        {!defaultOpen && (
          <button
            type="button"
            onClick={() => setOpen(false)}
            className="shrink-0 rounded-mk-full border border-mk-border px-3 py-1 text-mk-small text-mk-secondary"
          >
            收起
          </button>
        )}
      </div>
      {error && <p className="mt-2 text-mk-small text-mk-danger">{error}</p>}
      {!source && !error && <p className="mt-2 text-mk-small text-mk-muted">正在取这篇文章…</p>}
      {source && (
        // 只读。她在这里不标注、不划句 —— 这一篇已经读完了，这是回头看一眼。
        <div className="mt-3 flex flex-col gap-3">
          {source.blocks.map((b, i) => (
            <p key={b.id} className="text-mk-body leading-relaxed text-mk-ink">
              <span className="mr-2 select-none text-mk-caption text-mk-muted">{i + 1}</span>
              {b.text}
            </p>
          ))}
        </div>
      )}
    </section>
  );
}
