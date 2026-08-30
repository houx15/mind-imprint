import { ExternalLink, Globe } from "lucide-react";
import { useEco } from "../../store";
import { STUDENT } from "../../data/library";
import { go } from "../../route";
import { PersonalPage } from "../../page/PersonalPage";
import { Btn, Panel, Sys } from "../../ui";

/**
 * Step 5 · 发布.
 *
 * The preview is the REAL page component (`PersonalPage` with `preview`), not
 * a mock of it — what she approves here is exactly what a visitor gets.
 *
 * Before the button, a short honest checklist of what publishing means. A
 * 14-year-old putting her name and a way to reach her on a public URL deserves
 * to be told plainly what that does, and that she can take it down.
 */
export function PublishStep() {
  const { state, hpPublish, hpGo } = useEco();
  const hp = state.homepage;
  const url = `mind.im/p/${STUDENT.handle}`;
  const filled = hp.sections.filter(
    (s) => s.enabled && (s.picker ? (s.picked?.length ?? 0) > 0 : s.value.trim().length > 0),
  ).length;
  const total = hp.sections.filter((s) => s.enabled).length;

  return (
    <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div>
        <Sys className="mb-2 block">预览 · 这就是别人会看到的</Sys>
        <div className="overflow-hidden rounded-mk-lg border border-mk-border shadow-mk-sm">
          <div
            className="flex items-center gap-2 border-b border-mk-border px-4 py-2.5"
            style={{ background: "var(--mk-paper)" }}
          >
            <span className="flex gap-1.5">
              {["#E8837C", "#E9C46A", "#7FB88A"].map((c) => (
                <span key={c} className="h-2.5 w-2.5 rounded-mk-full" style={{ background: c }} />
              ))}
            </span>
            <span className="ml-2 flex-1 truncate rounded-mk-full bg-mk-surface px-3 py-1 font-mono text-[11px] text-mk-muted">
              {url}
            </span>
          </div>
          <PersonalPage handle={STUDENT.handle} preview />
        </div>
      </div>

      <aside className="space-y-4">
        <Panel className="p-5">
          <Sys>发布前</Sys>
          <p className="mt-2 text-mk-body-lg leading-[1.85] text-mk-ink">
            已经填好 <span className="font-mono font-bold">{filled}</span> / {total} 块。
            {filled < total ? "剩下的空着也能发布，之后随时能补。" : "全都填好了。"}
          </p>
          <ul className="mt-4 space-y-2.5">
            {[
              "发布后，任何拿到链接的人都能打开它——不需要账号。",
              "上面的每一句都是你写的，我们不替你改。",
              "你随时可以回来改，或者把它下线。",
            ].map((t) => (
              <li key={t} className="flex gap-2.5 text-mk-body leading-[1.8] text-mk-secondary">
                <span className="mt-2 h-1 w-1 shrink-0 rounded-mk-full" style={{ background: "var(--mk-accent)" }} />
                {t}
              </li>
            ))}
          </ul>

          {hp.published ? (
            <div className="mt-5 space-y-2.5">
              <div
                className="flex items-center gap-2 rounded-mk-md p-3"
                style={{ background: "var(--mk-success-bg)", color: "var(--mk-success)" }}
              >
                <Globe size={16} strokeWidth={1.9} />
                <span className="text-mk-body font-medium">已发布</span>
              </div>
              <Btn className="w-full" onClick={() => hpGo(5)}>
                去分享
              </Btn>
              <Btn
                variant="quiet"
                className="w-full"
                iconStart={<ExternalLink size={15} />}
                onClick={() => go({ name: "page", handle: STUDENT.handle })}
              >
                打开真的页面
              </Btn>
            </div>
          ) : (
            <Btn className="mt-5 w-full" iconStart={<Globe size={16} strokeWidth={1.9} />} onClick={hpPublish}>
              发布我的主页
            </Btn>
          )}
        </Panel>

        <Panel className="p-5">
          <Sys>发布之后会发生什么</Sys>
          <ul className="mt-2.5 space-y-2 text-mk-body leading-[1.8] text-mk-secondary">
            <li>· 另外五条赛道打开了。</li>
            <li>· 以后每做完一个项目，它会出现在这一页上。</li>
            <li>· 你的树上会长出「把想法做出来」这个词。</li>
          </ul>
        </Panel>
      </aside>
    </div>
  );
}
