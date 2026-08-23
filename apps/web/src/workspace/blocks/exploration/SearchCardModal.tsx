import type { ReactNode } from "react";

// SearchCardModal — 检索卡 · a static teaching modal (no API, no state). Opened
// from the exploration controls' idle/directions pages. It teaches WHERE to look,
// HOW to judge a source, and what "相关" means (the 兔子洞 mental model) — never
// searches or reads for the student (铁律①). Plain, warm copy; three sections.
export function SearchCardModal({ onClose }: { onClose: () => void }) {
  return (
    <div data-tour="search-card" className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" onClick={onClose}>
      <div
        className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-mk-lg bg-mk-surface shadow-mk-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 border-b border-mk-border px-5 py-3">
          <span className="rounded-full bg-mk-accent px-2 py-0.5 text-[12px] font-bold text-white">检索卡</span>
          <h2 className="font-sans text-[15px] font-bold text-mk-ink">怎么找资料、怎么判断可不可靠</h2>
          <button type="button" onClick={onClose} className="ml-auto text-[14px] font-bold text-mk-faint hover:text-mk-ink">
            ✕
          </button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-5 py-4">
          <Section title="去哪找可靠的资料">
            <ul className="flex flex-col gap-1.5">
              <Bullet>
                <b className="font-bold text-mk-ink">印记</b>（在这里直接检索、让印记陪你读）
              </Bullet>
              <Bullet>
                <b className="font-bold text-mk-ink">知网</b>（中文期刊、学位论文）
              </Bullet>
              <Bullet>
                <b className="font-bold text-mk-ink">Google Scholar</b>（英文文献、看被引次数）
              </Bullet>
            </ul>
          </Section>

          <Section title="怎么判断可靠？">
            <ul className="flex flex-col gap-1.5">
              <Bullet>
                看<b className="font-bold text-mk-ink">来源</b>：谁写的、发在哪、是不是同行评审的期刊
              </Bullet>
              <Bullet>
                看<b className="font-bold text-mk-ink">数据</b>：有没有原始数据/方法，还是只有结论
              </Bullet>
              <Bullet>
                看<b className="font-bold text-mk-ink">出处</b>：它引用了谁、又被谁引用
              </Bullet>
              <Bullet>拿不准就把这篇带来，让印记和你一起读、一起挑毛病</Bullet>
            </ul>
          </Section>

          <Section title="什么算「相关」？——兔子洞">
            <ul className="flex flex-col gap-1.5">
              <Bullet>从一篇你感兴趣的文章出发，顺着它引用的、引用它的、和它相似的往下追</Bullet>
              <Bullet>每一步问自己：这条线还在回答我的问题吗？</Bullet>
              <Bullet>别追偏——兔子洞是用来加深，不是用来跑题</Bullet>
            </ul>
          </Section>
        </div>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section>
      <h3 className="text-[13px] font-bold uppercase tracking-wider text-mk-faint">{title}</h3>
      <div className="mt-2 text-[13.5px] leading-relaxed text-mk-muted">{children}</div>
    </section>
  );
}

function Bullet({ children }: { children: ReactNode }) {
  return (
    <li className="flex gap-2">
      <span className="flex-none text-mk-accent">·</span>
      <span className="min-w-0 flex-1">{children}</span>
    </li>
  );
}
