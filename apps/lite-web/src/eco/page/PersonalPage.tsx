import { useEco } from "../store";
import { buildSite, siteTheme } from "../data/site";
import { BuiltSite } from "../site/BuiltSite";
import { go } from "../route";
import { Btn, Empty } from "../ui";

/**
 * 她的个人主页 — the public site, at `/eco/p/:handle`.
 *
 * No app shell: whoever opens this is a friend, a parent or a teacher with no
 * account, and a visitor should not see her navigation. Same reasoning as
 * lite's `/s/:token`.
 *
 * The page itself is `BuiltSite`, the same component the 页面构建 step previews
 * and the 我的主页 tab renders. One renderer is the whole point — a preview
 * that is a different component eventually ships something she never approved.
 */
export function PersonalPage({ handle }: { handle: string }) {
  const { state } = useEco();

  if (!isLive(state.homepage.published, state.projects)) {
    return (
      <div className="mx-auto max-w-[640px] px-8 py-24">
        <Empty
          title="这个主页还没建好"
          body={`/p/${handle} 还没有发布。如果这是你的页面，去项目里把「我自己的主页」做完——七步，最后一步就是发布。`}
          action={<Btn onClick={() => go({ name: "projects" })}>去建我的主页</Btn>}
        />
      </div>
    );
  }

  return (
    <BuiltSite
      site={buildSite({ projects: state.projects, sections: state.homepage.sections })}
      theme={siteTheme(state.projects, state.homepage.style)}
    />
  );
}

/** 上线了没有 — either door counts: the old studio's 发布, or a finished
 *  project, since a finished project is what this page is made of. */
export function isLive(
  studioPublished: boolean,
  projects: { phase: string }[],
): boolean {
  return studioPublished || projects.some((p) => p.phase === "published");
}
