import { useState } from "react";
import { ArrowUpRight, Monitor, Smartphone } from "lucide-react";
import { useEco } from "../store";
import { buildSite, siteTheme } from "../data/site";
import { BuiltSite } from "../site/BuiltSite";
import { isLive } from "./PersonalPage";
import { ecoPath, go } from "../route";
import { Btn, Empty, Sys, cx } from "../ui";

/**
 * 我的主页 — her own view of the site, inside the app shell.
 *
 * The difference from `/eco/p/:handle` is the audience and nothing else: this
 * one keeps her navigation and adds the one thing an owner needs and a visitor
 * does not — the public link, and a way to see the page as a phone sees it.
 * The page below the bar is the same `BuiltSite` a visitor gets.
 */
export function MyPage() {
  const { state } = useEco();
  const [phone, setPhone] = useState(false);
  const [demo, setDemo] = useState(false);

  const live = isLive(state.homepage.published, state.projects);
  const site = buildSite({ projects: state.projects, sections: state.homepage.sections });
  const theme = siteTheme(state.projects, state.homepage.style);

  if (!live && !demo) {
    return (
      <div className="mx-auto max-w-[640px] px-8 py-24">
        <Empty
          title="你的主页还没建起来"
          body="它是别人认识你的那一页：你做过的、写过的，和你在想的问题。项目里的「我自己的主页」走完七步就会发布到这里。"
          action={
            <span className="flex flex-wrap items-center justify-center gap-2.5">
              <Btn onClick={() => go({ name: "projects" })}>去建我的主页</Btn>
              {/* 🚨 Seven steps is a long way to go before you can see what you
                  are being asked to build. This shows the finished thing with
                  a ribbon saying it is not hers yet. */}
              <Btn variant="outline" onClick={() => setDemo(true)}>
                先看看做完的样子
              </Btn>
            </span>
          }
        />
      </div>
    );
  }

  return (
    <div className="min-h-full">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-mk-border bg-mk-surface px-6 py-3">
        {live ? (
          <>
            <Sys>公开链接</Sys>
            <span className="eco-mono text-mk-secondary">{site.domain}/p/{site.handle}</span>
          </>
        ) : (
          <span className="text-mk-small text-mk-muted">
            样例 · 这是做完之后的样子，还不是你的页面
          </span>
        )}

        <span className="ml-auto flex items-center gap-1">
          <Toggle on={!phone} icon={<Monitor size={14} strokeWidth={1.9} />} label="电脑" onClick={() => setPhone(false)} />
          <Toggle on={phone} icon={<Smartphone size={14} strokeWidth={1.9} />} label="手机" onClick={() => setPhone(true)} />
        </span>

        <Btn
          size="sm"
          variant="outline"
          iconStart={<ArrowUpRight size={14} strokeWidth={2} />}
          // 🚨 A new tab, not a route change. The visitor's view has no
          // navigation by design, so navigating there in place would strand
          // her on her own page with no way back.
          onClick={() =>
            live
              ? window.open(ecoPath({ name: "page", handle: site.handle }), "_blank")
              : go({ name: "projects" })
          }
        >
          {live ? "以访客身份打开" : "去做我自己的"}
        </Btn>
      </div>

      <div className={cx("mx-auto", phone ? "max-w-[420px] px-4 py-6" : "")}>
        <div className={cx(phone && "overflow-hidden rounded-mk-lg border border-mk-border shadow-mk-md")}>
          <BuiltSite site={site} theme={theme} narrow={phone} />
        </div>
      </div>
    </div>
  );
}

function Toggle({
  on,
  icon,
  label,
  onClick,
}: {
  on: boolean;
  icon: React.ReactNode;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={on}
      className={cx(
        "flex items-center gap-1.5 rounded-mk-sm px-2.5 py-1 text-mk-small transition-colors",
        "duration-[120ms] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
        on ? "bg-mk-paper text-mk-ink" : "text-mk-muted hover:text-mk-secondary",
      )}
    >
      {icon}
      {label}
    </button>
  );
}
