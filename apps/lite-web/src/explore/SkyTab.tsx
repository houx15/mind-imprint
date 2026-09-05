import { Sprout, Telescope } from "lucide-react";
import { ExploreView } from "./ExploreView";
import { TreeView } from "../tree/TreeView";
import { useInterestTree } from "../tree/useInterestTree";
import { cx } from "../tree/ui";
import type { MeUser } from "../api/auth";

/**
 * SkyTab —— 探索地图与兴趣树合成的那一格。
 *
 * # 为什么合
 *
 * 这两屏说的是同一件事的两半：**树是她已经有的，地图是她还没走过的。** 分成
 * 两格之后，「你可能还会感兴趣的」那一层在地图上，而它的依据（她的词扎在哪几
 * 门学科上）在树上，学生要在两个 tab 之间自己把这条线接起来。
 *
 * # 一次请求，两屏共用
 *
 * `useInterestTree` 提到这里来了。树要它是显然的；地图也要，因为推荐是从她的词
 * 算出来的。各拉一次会在她来回切时各发一次请求，而且两屏可能拿到不同的快照 ——
 * 地图上推荐「电动车」的同时树上已经长出了「电动车」。
 *
 * # 默认落在地图
 *
 * `/` 落在这一格，而这一格落在地图：每天回来看一眼的理由是今天的五颗星，不是
 * 一棵昨天以来没变过的树。`/tree` 仍然是一条真链接，直接打开树那一屏。
 */
export type SkySurface = "map" | "tree";

const SWITCHES: { key: SkySurface; label: string; icon: typeof Telescope }[] = [
  { key: "map", label: "今日探索地图", icon: Telescope },
  { key: "tree", label: "我的兴趣树", icon: Sprout },
];

export function SkyTab({
  user,
  surface,
  onSwitch,
}: {
  user: MeUser;
  surface: SkySurface;
  onSwitch: (next: SkySurface) => void;
}) {
  const live = useInterestTree();

  return (
    <div className="relative flex h-full min-h-full w-full flex-col">
      {/* 切换器浮在两屏之上，而不是占一条自己的横栏：两屏都是满幅的深色画面，
          一条横栏会把它们各压掉 56px，而这一格最不缺的就是画面。 */}
      <div className="pointer-events-none absolute inset-x-0 top-0 z-30 flex justify-center pt-4">
        <div
          className="pointer-events-auto flex items-center gap-1 rounded-mk-full p-1 backdrop-blur-md"
          style={{
            background: "rgba(18,15,20,.72)",
            border: "1px solid rgba(240,233,224,.16)",
          }}
          role="tablist"
          aria-label="探索与我的树"
        >
          {SWITCHES.map(({ key, label, icon: Ico }) => {
            const active = surface === key;
            return (
              <button
                key={key}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => onSwitch(key)}
                className={cx(
                  "flex items-center gap-2 rounded-mk-full px-4 py-1.5 text-mk-small transition-colors",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/40",
                  active
                    ? "bg-[rgba(245,239,231,.14)] font-semibold text-[#F5EFE7]"
                    : "text-[#9A8E80] hover:text-[#C0B4A6]",
                )}
              >
                <Ico size={14} strokeWidth={1.8} />
                {label}
              </button>
            );
          })}
        </div>
      </div>

      <div className="min-h-0 flex-1">
        {surface === "map" ? <ExploreView tree={live} /> : <TreeView user={user} live={live} />}
      </div>
    </div>
  );
}
