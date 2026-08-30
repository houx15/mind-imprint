import { useEffect, useState } from "react";
import { Check, Copy, ExternalLink, Hexagon } from "lucide-react";
import { useEco } from "../../store";
import { STUDENT } from "../../data/library";
import { styleById } from "../../data/homepage";
import { go } from "../../route";
import { Btn, Panel, Sys } from "../../ui";

/**
 * Step 6 · 分享.
 *
 * The ask is deliberately small and specific: **把链接发给一个人**, and say why
 * you picked that person. "Share with everyone" produces nothing; one named
 * reader produces a real first response, which is the thing that makes the
 * page feel alive.
 *
 * The QR block is drawn, not fetched — a deterministic pattern seeded from the
 * handle. It is honestly labelled as a prototype placeholder rather than
 * pretending to be a scannable code.
 */
export function ShareStep() {
  const { state, hpShare } = useEco();
  const [copied, setCopied] = useState(false);
  const style = styleById(state.homepage.style ?? "morning");
  const url = `mind.im/p/${STUDENT.handle}`;

  useEffect(() => {
    hpShare();
    // Reaching this step IS the completion signal for PBL#0 — it is what opens
    // the other tracks. Running once on mount keeps that fact in one place.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function copy() {
    try {
      await navigator.clipboard.writeText(`https://${url}`);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      /* clipboard blocked — the link is on screen and selectable */
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_340px]">
      <div>
        <Sys>做完了 · PROJECT 00 COMPLETE</Sys>
        <h2 className="mt-1.5 text-mk-display text-mk-ink">你有一个家了</h2>
        <p className="mt-3 max-w-[58ch] text-mk-body-lg leading-[1.95] text-mk-secondary">
          现在做一件小事，比发给一百个人有用：
          <strong className="font-semibold text-mk-ink">挑一个人，把链接发给他</strong>
          ，并且告诉他你为什么挑他。第一个真实的回应，会决定你还想不想继续做下去。
        </p>

        <Panel className="mt-6 p-5">
          <Sys>你的链接</Sys>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-mk-md px-4 py-3 font-mono text-mk-body text-mk-ink"
                  style={{ background: "var(--mk-paper)", border: "1px solid var(--mk-border)" }}>
              https://{url}
            </code>
            <Btn variant="outline" iconStart={copied ? <Check size={15} /> : <Copy size={15} />} onClick={copy}>
              {copied ? "已复制" : "复制"}
            </Btn>
            <Btn
              variant="quiet"
              iconStart={<ExternalLink size={15} />}
              onClick={() => go({ name: "page", handle: STUDENT.handle })}
            >
              打开
            </Btn>
          </div>
        </Panel>

        <Panel className="mt-4 p-5">
          <Sys>接下来</Sys>
          <ul className="mt-3 space-y-3">
            {[
              { t: "五条赛道打开了", d: "设计 / 网站 / 游戏 / 调查报告 / 其他。下一个项目做完会自动出现在这一页上。" },
              { t: "回来改它", d: "第一个项目发布之后，你多半会想重写那句自我介绍。那是好事。" },
              { t: "把读过的补上去", d: "你的书架一直在长。挑三篇最能说明你是谁的放上来。" },
            ].map((x) => (
              <li key={x.t} className="flex gap-3">
                <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-mk-full" style={{ background: "var(--mk-accent)" }} />
                <span>
                  <span className="block text-mk-h3 text-mk-ink">{x.t}</span>
                  <span className="mt-0.5 block text-mk-body leading-[1.8] text-mk-secondary">{x.d}</span>
                </span>
              </li>
            ))}
          </ul>
          <Btn className="mt-5" iconStart={<Hexagon size={16} strokeWidth={1.8} />} onClick={() => go({ name: "projects" })}>
            去挑下一个项目
          </Btn>
        </Panel>
      </div>

      {/* share card */}
      <aside>
        <Sys className="mb-2 block">分享卡</Sys>
        <div
          className="overflow-hidden rounded-mk-lg shadow-mk-md"
          style={{ background: style.paper, color: style.ink, fontFamily: style.font }}
        >
          <div className="px-6 pt-7">
            <p
              className="text-[10px] uppercase tracking-[0.2em]"
              style={{ color: style.accent, fontFamily: 'ui-monospace,"SF Mono",monospace' }}
            >
              思维印记 · 个人主页
            </p>
            <p className="mt-3 text-[28px] font-bold leading-[1.25]">{STUDENT.name}</p>
            <p className="mt-2 text-[14px] leading-[1.8]" style={{ opacity: 0.82 }}>
              {state.homepage.sections.find((s) => s.id === "intro")?.value.trim() || "（一句话介绍还没写）"}
            </p>
          </div>
          <div className="mt-6 flex items-end justify-between gap-4 px-6 pb-6">
            <p className="font-mono text-[11px]" style={{ opacity: 0.6 }}>
              {url}
            </p>
            <QrBlock seed={STUDENT.handle} ink={style.ink} paper={style.paper} />
          </div>
        </div>
        <p className="mt-2 text-mk-small text-mk-muted">
          原型里的码是画出来的示意图，不能扫。真正的二维码在接上后端之后生成。
        </p>
      </aside>
    </div>
  );
}

/** A deterministic 9×9 block pattern from the handle. Decorative and labelled
 *  as such — we never render something that looks scannable but isn't without
 *  saying so. */
function QrBlock({ seed, ink, paper }: { seed: string; ink: string; paper: string }) {
  const n = 9;
  let h = 0;
  for (let i = 0; i < seed.length; i++) h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  const cells: boolean[] = [];
  for (let i = 0; i < n * n; i++) {
    h = (h * 1103515245 + 12345) >>> 0;
    cells.push(((h >>> 16) & 1) === 1);
  }
  // corner finders, so it reads as a code at a glance
  const finder = (r: number, c: number) =>
    (r < 3 && c < 3) || (r < 3 && c > n - 4) || (r > n - 4 && c < 3);

  return (
    <div
      className="grid shrink-0 gap-[2px] rounded-[6px] p-2"
      style={{ gridTemplateColumns: `repeat(${n}, 6px)`, background: paper, border: `1px solid ${ink}22` }}
      aria-hidden
    >
      {cells.map((on, i) => {
        const r = Math.floor(i / n);
        const c = i % n;
        const f = finder(r, c);
        const filled = f ? (r + c) % 2 === 0 || (r % 2 === 0 && c % 2 === 0) : on;
        return (
          <span
            key={i}
            className="h-[6px] w-[6px] rounded-[1px]"
            style={{ background: filled ? ink : "transparent" }}
          />
        );
      })}
    </div>
  );
}
