import { useMemo, useRef, useState } from "react";
import type { MeUser } from "../api/auth";
import {
  FIELDS,
  stopsFor,
  branchPath,
  fieldById,
  pointOnBranch,
} from "./geometry";
import { growthStops } from "./liveTree";
import { outputCount, type LiveTree } from "./useInterestTree";
import type { FieldId, Keyword } from "./types";
import { Hint, Sys, cx } from "./ui";
import { useFitScale } from "./useFitScale";
import {
  GROUND_Y,
  ROOT_NODES,
  ROOT_NODE_BY_ID,
  STAGE_H,
  leafShape,
  rootPath,
  subRootPaths,
  threadPath,
} from "./roots";
import { KeywordDrawer } from "./KeywordDrawer";
import { useQuizTaken } from "./quiz/useQuizStatus";
import { liteRoutePath, navigate } from "../routing";
import "./tree.css";

/**
 * 我的兴趣树 · the keyword model, drawn on the same paper as the rest of lite.
 *
 * ## The visual ruling, fourth pass (2026-09-12)
 * v1 was a literal cartoon tree (brown trunk, green candy pills). v2 was a
 * technical diagram on paper — correct, and cold. v3 kept v2's geometry and
 * moved it onto its own night, which worked on its own terms but made this
 * the one screen in lite that is not the product's paper: every other page is
 * `--mk-paper` + white cards + the 朱砂 nav rail, and a student coming from
 * the reading room landed on what looked like a different app. v4 keeps every
 * line of v3's geometry and puts it back on that paper.
 *
 * **What v4 changes is the instrument, not the drawing: 发光 becomes 落墨.**
 * Branches are deeper, more saturated strokes of the same hue (the second set
 * in `branchHues.css` — the night values are near-invisible on #FBF8F4), the
 * bloom narrows to a thin bleed, and a keyword label is a white card with a
 * hairline border, the same card language as everywhere else. Colours all come
 * from the `--tree-*` block at the top of `tree.css`; nothing here should hold
 * a hex.
 *
 * Why it is still an SVG rather than a generated illustration: every leaf is
 * placed by `pointOnBranch()` on the exact Bézier the SVG draws. A painted
 * tree has branches the maths knows nothing about, so the leaves would float
 * beside twigs instead of hanging on them, and the whole claim — *this
 * picture IS your model* — would quietly become decoration. The picture has
 * to be the data structure.
 *
 * The 世界 map and this share one instrument in two directions: out there,
 * orbit rings around a sun; in here, growth rings around an origin. The map
 * stays a night — it is the sky, and it is the half she has NOT walked.
 *
 * ## What the tree claims, and how it earns it
 * *This is a model of you, built from what you actually did.* Two things make
 * that true rather than decorative — every bead opens a drawer listing its
 * REAL sources (`KeywordDrawer`), and the timeline shows the same tree at four
 * earlier moments, so she can see it was grown and not authored.
 *
 * ## No gamification
 * Two numbers, both defined on hover, neither of them a score: 关键词 (how
 * many words the model holds) and 成果数 (how many finished things they were
 * built from). Nothing rewards frequency (铁律②).
 */
/** 选中的是一片叶子（领域），还是一条根（学科）。 */
type Pick = { kind: "leaf" | "discipline"; id: string };

export function TreeView({
  user,
  live,
  readOnly = false,
}: {
  user: MeUser;
  live: LiveTree;
  /** 教师看学生的树：拿掉一切会创建/改动学生数据的入口（兴趣测试、继续深挖、
   *  空枝邀请），其余——枝、叶、根、成长轴、相关活动——照常渲染。默认
   *  `false`，学生自己那面因此一字不变。 */
  readOnly?: boolean;
}) {
  // 成长回放的刻度是**这一页的本地状态**。原型里它住在 EcoProvider 的全局
  // store 里，那是因为世界和树共用一个 store；在 lite 里没有别的页面关心她把
  // 回放拖到了哪一格，把它提升到全局只会让一个纯展示的选择跨页面存活。
  // 落在最后一格（现在）。真正有几格由 `growthStops` 按她的跨度算，见下面。
  const [stop, setStop] = useState(3);
  const [openId, setOpenId] = useState<string | null>(null);
  const [hoverField, setHoverField] = useState<FieldId | null>(null);
  // 点一片叶子看它扎在哪几条根上；点一条根看哪几片叶子共用它。连线只在这时出现。
  const [pick, setPick] = useState<Pick | null>(null);

  const stageRef = useRef<HTMLDivElement>(null);
  // Bead labels are fixed pixel size on a stage that scales with the viewport.
  // Without this they collide on any short window.
  const scale = useFitScale(stageRef, 1000, 0.74);

  // 真数据。没有 mock 兜底——一棵回退到示例词的树，会把十六个不属于她的词
  // 挂在一张标着「这就是你的模型」的图上，而她看不出来。见 useInterestTree。
  //
  // 这棵树**由外面传进来**（`SkyTab`）：探索地图和这一屏在同一个 tab 下，两屏
  // 各拉一次会在她来回切时各发一次请求，而且推荐算出来的星可能和树上的词对不上。
  const all = live.keywords;

  // 觉醒协议（兴趣测试）。三态：null = 还不知道，那时两件事都不做。
  // 🚨 `readOnly`（教师视角）下这个 hook 仍然照常调用——hook 顺序不能因为一个
  // prop 分支——只是它的结果被忽略：下面每处用到 `quizTaken` 的地方都先判
  // `readOnly`，从不把它喂给 TreeState 或渲染那颗按钮。
  const quizTaken = useQuizTaken();
  const openQuiz = () => navigate(liteRoutePath({ tab: "tree", quiz: true }));
  // 空枝邀请：点一根还没有词的枝，问的是「这根枝上会长什么」。
  const [inviteField, setInviteField] = useState<FieldId | null>(null);

  // 🚨 这条轴有几格，是**她的树活了多久**决定的（`growthStops`）：三周以内根本
  // 不画，之后从两格长到四格。上一版永远是四格，标签写死成「半年前 / 近两个月」，
  // 对一个上周才开始的学生那六个月不存在。
  const axis = useMemo(() => growthStops(all), [all]);
  const lastStop = axis.length - 1;
  // 刻度数会随数据变（她今天跨过了三个月，轴就从两格变三格），所以选中的那一格
  // 要夹住，否则会停在一个不存在的下标上，界面看起来是「一格都没选中」。
  const atStop = Math.min(stop, lastStop);
  const visible = useMemo(() => all.filter((k) => k.bornAt <= atStop), [all, atStop]);
  // Only the stops that actually hold something. A dot that shows the tree she
  // is already looking at is a control that does nothing.
  const liveStops = useMemo(() => stopsFor(all, axis.length), [all, axis.length]);
  const maturity = 0.42 + (lastStop === 0 ? 1 : atStop / lastStop) * 0.58;
  const openKw = openId ? (all.find((k) => k.id === openId) ?? null) : null;

  // ONE count, used by the header, the field index and the caption. Three
  // places computing it independently is how the screen came to show 17 / 16 / 0
  // at the same time.
  // 读不到数据时，一切计数都是「不知道」，不是 0。写 0 等于替她断言她什么都
  // 没有——而这个界面最不能撒的谎，就是关于她自己有什么。
  const known = live.status === "ready" || live.status === "empty";
  const countFor = (fieldId?: FieldId) =>
    (fieldId ? visible.filter((k) => k.field === fieldId) : visible).length;
  const total = countFor();

  // 成果数 — finished things, not activity. Readings + writings + projects she
  // actually published. A project still in progress is not a 成果, and calling
  // it one would be the same lie as a streak counter.
  // 从她的词自己的来源里数，按 (类型, id) 去重：一篇阅读长出三个词，它仍然是
  // 一件事。原来这里数的是 mock 书架的长度，那个数字和树上的词毫无关系。
  const outputs = outputCount(all);

  return (
    <div className="tree-grove tree-motes relative min-h-full">
      <header className="relative z-20 flex flex-wrap items-center justify-between gap-4 px-7 pt-6">
        <div className="min-w-0">
          <Sys>我的兴趣树 · INTEREST TREE</Sys>
          <h1 className="mt-1 text-mk-h1 text-mk-ink">
            {user.display_name}
            {user.classes[0] ? (
              <span className="ml-2 text-mk-body font-normal text-mk-muted">
                {user.classes[0].name}
              </span>
            ) : null}
          </h1>

          {/* The growth axis, top-left under the name — same position and
              behaviour as the world's date axis, because it answers the same
              shape of question: 「我在看哪个时候」.

              🚨 Only the stops that hold something (`stopsFor`). With one stop
              there is nothing to scrub, so it becomes a sentence about what
              makes the tree grow instead of a scrubber that does nothing. */}
          {liveStops.length > 1 ? (
            <div className="mt-3 flex items-center">
              {liveStops.map((si, i) => {
                const st = axis[si]!;
                const active = si === atStop;
                return (
                  <div key={st.label} className="flex items-center">
                    <button
                      type="button"
                      onClick={() => setStop(si)}
                      aria-pressed={active}
                      className="group flex flex-col items-center gap-1.5 px-2.5 py-1 focus-visible:outline-none"
                      title={st.sub}
                    >
                      <span
                        className="h-2 w-2 transition-all duration-[200ms] ease-mk"
                        style={{
                          background: active ? "var(--mk-accent-500)" : "var(--tree-ink-4)",
                          transform: active ? "rotate(45deg) scale(1.5)" : "rotate(45deg)",
                          boxShadow: active ? "0 0 12px var(--mk-accent-400)" : "none",
                        }}
                      />
                      <span
                        className={cx(
                          "whitespace-nowrap text-mk-small transition-colors duration-[160ms]",
                          active ? "font-semibold text-mk-ink" : "text-mk-muted",
                        )}
                      >
                        {st.label}
                      </span>
                    </button>
                    {i < liveStops.length - 1 ? (
                      <span
                        className="mb-5 h-px w-6 shrink-0"
                        style={{ background: "var(--tree-line-strong)" }}
                      />
                    ) : null}
                  </div>
                );
              })}
              <span className="mb-5 ml-3 text-mk-small text-mk-muted">
                {atStop === lastStop ? `${total} 个关键词` : `那时候 ${total} 个`}
              </span>
            </div>
          ) : (
            // 只在真的读到了「她还没有词」时才说这句。读取失败时说「你的树刚
            // 开始长」，是在替她断言一件我们并不知道的事。
            known && (
              <p className="mt-3 max-w-[46ch] text-mk-small leading-[1.8] text-mk-muted">
                你的树刚开始长。每读完一篇、写完一篇、做完一个项目，它就会多一个词。
              </p>
            )
          )}
        </div>
        <div className="flex items-center gap-5">
          {/* 兴趣测试的常驻入口。树不空时也留着，因为**重做是再长几个词**
              （服务端给同一个词再添一条来源，强度上升），不是清空重来。
              和空树上那条邀请一样，状态未知（null）时不显示。
              🚨 只读（教师）视角下整个入口收起——这是学生自己的测试。 */}
          {!readOnly && quizTaken !== null ? (
            <button
              type="button"
              onClick={openQuiz}
              className="rounded-full border px-3.5 py-1.5 text-mk-small transition-colors duration-[120ms]
                         hover:bg-[rgba(51,48,46,.05)] focus-visible:outline-none
                         focus-visible:ring-2 focus-visible:ring-mk-accent-300"
              style={{ borderColor: "var(--mk-accent-300)", color: "var(--mk-accent-500)" }}
            >
              {quizTaken ? "再做一次兴趣测试" : "兴趣测试"}
            </button>
          ) : null}
          <span className="text-right">
            <span className="flex items-center justify-end gap-1.5">
              <Sys>关键词</Sys>
              <Hint
                text="根据你读过、收藏过、写过、做过的东西自动生成的兴趣关键词。每一个都可以点开，看它到底是从哪几件事来的。"
              />
            </span>
            <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
              {live.status === "ready" || live.status === "empty" ? total : "—"}
            </span>
          </span>
          <span className="h-8 w-px" style={{ background: "var(--tree-line-strong)" }} />
          <span className="text-right">
            <span className="flex items-center justify-end gap-1.5">
              <Sys>成果数</Sys>
              <Hint
                text="你已经完成的阅读、写作和已发布项目的总数。没做完的不算——这个数字只数你真的做出来的东西。"
              />
            </span>
            <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">
              {live.status === "ready" || live.status === "empty" ? outputs : "—"}
            </span>
          </span>
        </div>
      </header>

      {/* ── field index (chip row, below xl) ───────────────────────────── */}
      <div className="relative z-20 mb-2 mt-3 flex flex-wrap gap-1.5 px-7 xl:hidden">
        {FIELDS.map((f, i) => (
          <button
            key={f.id}
            type="button"
            onMouseEnter={() => setHoverField(f.id)}
            onMouseLeave={() => setHoverField(null)}
            onFocus={() => setHoverField(f.id)}
            onBlur={() => setHoverField(null)}
            // 窄屏走的是这一行 chip，空枝邀请在这里也要能点开 —— 只给宽屏那一列
            // 加上，等于让小屏幕的学生永远碰不到这个入口。
            // 🚨 只读（教师）视角不开这个邀请——见 readOnly 的整体说明。
            onClick={() => (!readOnly && known && countFor(f.id) === 0 ? setInviteField(f.id) : undefined)}
            className="inline-flex items-center gap-1.5 rounded-mk-full px-2.5 py-1 text-mk-small
                       transition-colors duration-[120ms] hover:bg-[rgba(51,48,46,.05)]
                       focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-300"
            style={{ border: "1px solid var(--tree-line)", color: "var(--tree-ink-2)" }}
          >
            <span
              className="h-1.5 w-1.5 rotate-45"
              style={{ background: f.hue }}
            />
            <span className="tree-mono text-mk-faint">{String(i + 1).padStart(2, "0")}</span>
            {f.label}
            <span className="font-mono text-[11px] tabular-nums text-mk-faint">
              {known ? countFor(f.id) : "—"}
            </span>
          </button>
        ))}
      </div>

      {/* ── the structure ─────────────────────────────────────────────── */}
      <div className="relative z-10 px-7 pb-10 pt-2">
        {/* Height-first, width derived. The model has to fit ON SCREEN: a
            picture of yourself you must scroll to see is a document, not a
            picture. */}
        <div
          ref={stageRef}
          className="relative mx-auto"
          style={{
            // 加了根之后这张图从 780 高变成 1100 —— 树冠那 780 一个像素没动，
            // 多出来的全在地面（y=748）以下。
            aspectRatio: `1000 / ${STAGE_H}`,
            height: "min(1000px, calc(100vh - 232px))",
            width: "auto",
            maxWidth: "1120px",
          }}
        >
          <TreeSvg
            maturity={maturity}
            hoverField={hoverField}
            keywords={visible}
            pick={pick}
            onPickDiscipline={(id) => setPick({ kind: "discipline", id })}
            onClearPick={() => setPick(null)}
          />

          {/* 四个状态，说清楚是哪一个。绝不用示例关键词填满一棵空树——那会把
              十六个不属于她的词挂在一张写着「这就是你的模型」的图上。 */}
          <TreeState
            status={live.status}
            error={live.error}
            onRetry={live.reload}
            quizTaken={readOnly ? null : quizTaken}
            onStartQuiz={openQuiz}
          />

          {/* keyword beads */}
          {visible.map((k, i) => (
            <Node
              key={k.id}
              kw={k}
              maturity={maturity}
              index={i}
              dim={hoverField !== null && hoverField !== k.field}
              scale={scale}
              onOpen={() => {
                // 点一片叶子做两件事：亮起它的根，并打开抽屉。抽屉里是她的
                // 原话与来源，根上是学校管这件事叫什么 —— 两半合起来才是
                // 「这个词为什么在你树上」的完整答案。
                setPick({ kind: "leaf", id: k.id });
                setOpenId(k.id);
              }}
              onHoverField={setHoverField}
            />
          ))}

          {/* 原型在这里还有一条「从世界收藏来的词」的独立轨道。它没有跟过来：
              那些词住在 EcoProvider 的内存里，刷新即消失。P4 之后，从新闻星图
              收藏一条会走 `plantKeywords` 的同一条路（`kind='news'`），于是它
              就是树上一个真正的关键词，而不是另开一条轨道。 */}
        </div>
      </div>

      {/* ── field index (floating column, xl and up) ─────────────────── */}
      <div className="pointer-events-auto absolute left-7 top-[200px] z-20 hidden w-[176px] xl:block">
        <Sys className="mb-2 block">
          主枝 · INDEX
        </Sys>
        <ul>
          {FIELDS.map((f, i) => {
            const n = countFor(f.id);
            return (
              <li key={f.id}>
                <button
                  type="button"
                  onMouseEnter={() => setHoverField(f.id)}
                  onMouseLeave={() => setHoverField(null)}
                  onFocus={() => setHoverField(f.id)}
                  onBlur={() => setHoverField(null)}
                  // 🚨 只有**空枝**可以点开。有词的枝上，那个数字自己说完了话；
                  // 空枝上，那个 0 什么也没说 —— 它该变成一句邀请。
                  // 只读（教师）视角不开这个邀请。
                  onClick={() => (!readOnly && known && n === 0 ? setInviteField(f.id) : undefined)}
                  className="flex w-full items-center gap-2 border-b px-1 py-2 text-left transition-colors
                             duration-[120ms] hover:bg-[rgba(51,48,46,.04)] focus-visible:outline-none
                             focus-visible:ring-2 focus-visible:ring-mk-accent-300"
                  style={{ borderColor: "var(--tree-line)" }}
                >
                  <span
                    className="h-1.5 w-1.5 shrink-0 rotate-45"
                    style={{ background: f.hue }}
                  />
                  <span className="tree-mono shrink-0 text-mk-faint">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-mk-small text-mk-secondary">{f.label}</span>
                  {known && n === 0 ? (
                    <span className="shrink-0 text-[11px]" style={{ color: "var(--mk-accent-400)" }}>
                      还没有
                    </span>
                  ) : (
                    <span className="font-mono text-[11px] tabular-nums text-mk-faint">
                      {known ? n : "—"}
                    </span>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      </div>

      <KeywordDrawer kw={openKw} onClose={() => setOpenId(null)} readOnly={readOnly} />
      {/* 空枝邀请整个收起：`inviteField` 在只读视角下从不会被设成非 null（见上面
          两处 onClick 的 `!readOnly` 守卫），这里再显式收起一次，两者互为保险。 */}
      {!readOnly && (
        <BranchInvite
          field={inviteField}
          onClose={() => setInviteField(null)}
          onExplore={() => navigate(liteRoutePath({ tab: "explore" }))}
          onQuiz={openQuiz}
        />
      )}
    </div>
  );
}

/**
 * The lit structure.
 *
 * Everything here is atmosphere and geometry; no text she needs to read lives
 * inside the SVG except the field names at the branch tips. Drawn back to
 * front: canopy glow → growth rings → horizon → roots → twigs → branches →
 * trunk → origin.
 */
function TreeSvg({
  maturity,
  hoverField,
  keywords,
  pick,
  onPickDiscipline,
  onClearPick,
}: {
  maturity: number;
  hoverField: FieldId | null;
  keywords: Keyword[];
  pick: Pick | null;
  onPickDiscipline: (id: string) => void;
  onClearPick: () => void;
}) {
  // 这一轮该亮的是哪些叶子、哪些学科、哪些线。三件事一起算，好过在三个地方
  // 各判断一次「现在选中的是什么」。
  const litLeaves = new Set<string>();
  const litDiscs = new Set<string>();
  const threads: { d: string; hue: string }[] = [];
  if (pick) {
    const byId = new Map(keywords.map((k) => [k.id, k]));
    const involved: Keyword[] =
      pick.kind === "leaf"
        ? [byId.get(pick.id)].filter((k): k is Keyword => !!k)
        : keywords.filter((k) => k.disciplineIds.includes(pick.id));
    for (const k of involved) {
      litLeaves.add(k.id);
      const targets = pick.kind === "leaf" ? k.disciplineIds : [pick.id];
      const leaf = leafShape(k.field, Math.min(k.at.t, maturity), k.at.spread);
      for (const did of targets) {
        const node = ROOT_NODE_BY_ID.get(did);
        if (!node) continue;
        litDiscs.add(did);
        threads.push({
          d: threadPath({ x: leaf.mx, y: leaf.my }, node),
          hue: fieldById(node.field).hue,
        });
      }
    }
  }
  const dimmed = pick !== null;

  // 一门学科要不要具名，取决于**她的词有没有连过来** —— 学科在那之前只是土里
  // 的一个位置。sharedDiscs 数的是有几个领域扎在同一条根上：大于一的带一圈环，
  // 因为「你的游戏和金融，底下是同一根」是这张图最值得一眼看见的东西。
  const namedDiscs = new Set<string>();
  const sharedDiscs = new Map<string, number>();
  for (const k of keywords) {
    for (const did of k.disciplineIds) {
      namedDiscs.add(did);
      sharedDiscs.set(did, (sharedDiscs.get(did) ?? 0) + 1);
    }
  }

  return (
    <svg
      viewBox={`0 0 1000 ${STAGE_H}`}
      className="absolute inset-0 h-full w-full"
      aria-hidden
      preserveAspectRatio="xMidYMid meet"
    >
      <defs>
        {/* Every branch is brightest at the trunk and dissolves toward its
            tip: the structure is most certain where it has been fed the most,
            and fades where it is still growing. That is the honest picture of
            a keyword model. */}
        {FIELDS.map((f) => {
          const tip = pointOnBranch(f.id, 1);
          return (
            <linearGradient
              key={f.id}
              id={`tree-br-${f.id}`}
              gradientUnits="userSpaceOnUse"
              x1={500}
              y1={480}
              x2={tip.x}
              y2={tip.y}
            >
              <stop offset="0%" stopColor={f.hue} stopOpacity="1" />
              <stop offset="58%" stopColor={f.hue} stopOpacity="0.72" />
              <stop offset="100%" stopColor={f.hue} stopOpacity="0.12" />
            </linearGradient>
          );
        })}
        <linearGradient id="tree-spine" x1="0" y1="1" x2="0" y2="0">
          <stop offset="0%" stopColor="var(--tree-trunk)" stopOpacity="0.78" />
          <stop offset="52%" stopColor="var(--tree-trunk)" stopOpacity="0.34" />
          <stop offset="100%" stopColor="var(--tree-trunk)" stopOpacity="0.06" />
        </linearGradient>
        <radialGradient id="tree-canopy" cx="50%" cy="46%" r="52%">
          <stop offset="0%" stopColor="#E8A87C" stopOpacity="0.11" />
          <stop offset="60%" stopColor="#E8A87C" stopOpacity="0.035" />
          <stop offset="100%" stopColor="#E8A87C" stopOpacity="0" />
        </radialGradient>
        {/* The bloom. `stdDeviation` is deliberately large: this is a halo
            around the strokes, not a soft edge on them. */}
        {/* 🚨 纸上这层不能按夜里那个强度来。原来是 stdDeviation 7 再把模糊层
            叠两遍 —— 在黑底上那是发光，在 #FBF8F4 上是把每一条彩线都糊出一圈
            脏边。收到 3.5、只叠一层，读起来是墨在纸上洇开的那一点点边。 */}
        <filter id="tree-bloom" x="-30%" y="-30%" width="160%" height="160%">
          <feGaussianBlur stdDeviation="3.5" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* 点空白处清除选中。 */}
      <rect
        x="0"
        y="0"
        width="1000"
        height={STAGE_H}
        fill="transparent"
        onClick={onClearPick}
      />

      {/* canopy glow */}
      <ellipse cx="500" cy="360" rx="480" ry="310" fill="url(#tree-canopy)" />
      <ellipse cx="500" cy="470" rx="250" ry="230" fill="url(#tree-canopy)" />

      {/* growth rings, centred on the origin */}
      {[210, 340, 470, 600].map((r, i) => (
        <circle
          key={r}
          cx="500"
          cy="742"
          r={r}
          fill="none"
          stroke="var(--tree-struct)"
          strokeOpacity={0.26 - i * 0.045}
          strokeDasharray="2 7"
        />
      ))}

      {/* horizon + ticks */}
      <line x1="90" y1="742" x2="910" y2="742" stroke="var(--tree-struct)" strokeOpacity="0.4" />
      {[...Array(21)].map((_, i) => (
        <line
          key={i}
          x1={100 + i * 40}
          y1="742"
          x2={100 + i * 40}
          y2={i % 5 === 0 ? 752 : 747}
          stroke="var(--tree-struct)"
          strokeOpacity="0.34"
        />
      ))}

      {/* roots — mirrored, dashed, barely there. They say the structure
          continues below what is shown. */}
      {FIELDS.map((f) => {
        const tip = pointOnBranch(f.id, 0.66);
        const mx = 500 + (500 - tip.x) * 0.4;
        return (
          <path
            key={`root-${f.id}`}
            d={`M 500 742 C ${500 + (mx - 500) * 0.4} 760, ${mx} 768, ${mx} 776`}
            fill="none"
            stroke={f.hue}
            strokeOpacity="0.3"
            strokeWidth="1.5"
            strokeDasharray="3 5"
          />
        );
      })}

      {/* twigs — three short offshoots per branch. They carry no data; they
          exist because a six-line diagram reads as a schematic and a tree
          needs enough small structure to read as grown. */}
      {FIELDS.map((f) => {
        const on = hoverField === null || hoverField === f.id;
        return (
          <g key={`tw-${f.id}`} style={{ transition: "opacity 200ms", opacity: on ? 0.85 : 0.12 }}>
            {[0.3, 0.44, 0.58, 0.72, 0.86].map((t, j) => {
              if (t > maturity) return null;
              const a = pointOnBranch(f.id, t);
              const side = j % 2 === 0 ? 1 : -1;
              // Twigs shorten toward the tip, the way real ones do.
              const reach = 58 - j * 8;
              const b = pointOnBranch(f.id, Math.min(1, t + 0.15), reach * side);
              const c = pointOnBranch(f.id, Math.min(1, t + 0.06), reach * 0.36 * side);
              return (
                <g key={t}>
                  <path
                    d={`M ${a.x} ${a.y} Q ${c.x} ${c.y}, ${b.x} ${b.y}`}
                    fill="none"
                    stroke={f.hue}
                    strokeOpacity="0.6"
                    strokeWidth="1.6"
                    strokeLinecap="round"
                    filter="url(#tree-bloom)"
                  />
                  {/* a spark at the twig's end — light caught on a leaf */}
                  <circle cx={b.x} cy={b.y} r="1.8" fill={f.hue} fillOpacity="0.8" />
                </g>
              );
            })}
          </g>
        );
      })}

      {/* branches */}
      {FIELDS.map((f, idx) => {
        const on = hoverField === null || hoverField === f.id;
        const tip = pointOnBranch(f.id, maturity, 0);
        const left = tip.x < 500;
        return (
          <g key={f.id} style={{ transition: "opacity 200ms", opacity: on ? 1 : 0.18 }}>
            {/* A branch is drawn TWICE: a short thick stub where it leaves the
                trunk, then the full thin line. SVG strokes cannot taper, and
                without the stub every branch is the same width end to end —
                which is exactly what made v3 read as a network diagram
                instead of a tree. */}
            <path
              d={branchPath(f.id)}
              stroke={f.hue}
              strokeOpacity="0.5"
              strokeWidth={hoverField === f.id ? 12 : 9}
              strokeLinecap="round"
              fill="none"
              pathLength={1}
              strokeDasharray={`${Math.min(0.14, maturity)} 1`}
              filter="url(#tree-bloom)"
              style={{ transition: "stroke-width 200ms" }}
            />
            <path
              d={branchPath(f.id)}
              stroke={f.hue}
              strokeOpacity="0.42"
              strokeWidth={hoverField === f.id ? 7 : 5.5}
              strokeLinecap="round"
              fill="none"
              pathLength={1}
              strokeDasharray={`${Math.min(0.34, maturity)} 1`}
              style={{ transition: "stroke-width 200ms" }}
            />
            <path
              d={branchPath(f.id)}
              stroke={`url(#tree-br-${f.id})`}
              strokeWidth={hoverField === f.id ? 5 : 3}
              strokeLinecap="round"
              fill="none"
              pathLength={1}
              strokeDasharray={`${maturity} 1`}
              filter="url(#tree-bloom)"
              style={{ transition: "stroke-width 200ms" }}
            />
            <circle cx={tip.x} cy={tip.y} r="3.5" fill={f.hue} filter="url(#tree-bloom)" />
            <text
              x={tip.x + (left ? -20 : 20)}
              y={tip.y - 34}
              textAnchor={left ? "end" : "start"}
              fontSize="9.5"
              fontFamily="ui-monospace, SF Mono, monospace"
              letterSpacing="1.4"
              fill="var(--tree-ink-4)"
            >
              {`FIELD ${String(idx + 1).padStart(2, "0")}`}
            </text>
            <text
              x={tip.x + (left ? -20 : 20)}
              y={tip.y - 19}
              textAnchor={left ? "end" : "start"}
              fontSize="12.5"
              fontWeight="600"
              fill="var(--tree-ink-2)"
            >
              {f.label}
            </text>
          </g>
        );
      })}

      {/* The trunk — a FILLED tapered shape, wide at the ground and closing to
          a point in the canopy. It was a 9px stroke, which is a mast, not a
          trunk: the whole difference between "diagram" and "tree" is that a
          tree gets thinner as it rises. */}
      <path
        d={
          "M 476 748 " +
          "C 486 660, 488 566, 494 470 " +
          "C 497 402, 499 350, 500 292 " +
          "C 501 350, 503 402, 506 470 " +
          "C 512 566, 514 660, 524 748 Z"
        }
        fill="url(#tree-spine)"
      />
      <path
        d="M 500 748 C 497 640, 503 560, 500 470 C 498 400, 502 350, 500 292"
        stroke="#FFFFFF"
        strokeOpacity="0.5"
        strokeWidth="1"
        fill="none"
      />
      {/* two low forks, so the trunk meets the ground like a thing that grew */}
      {[-1, 1].map((side) => (
        <path
          key={side}
          d={`M 500 690 C ${500 + side * 14} 712, ${500 + side * 30} 730, ${500 + side * 44} 748`}
          stroke="url(#tree-spine)"
          strokeWidth="5"
          strokeLinecap="round"
          fill="none"
        />
      ))}

      {/* ── 地面以下 ──────────────────────────────────────────────────────
          树冠是她的（叶子只有她做过一件事才长出来）；根是世界的（42 门学科，
          固定的、完整的）。这个分工是根系比原来那棵树强的地方：原来没有词就是
          七根空枝，一个刚注册的学生打开自己的树看到七根什么都没有的线。 */}
      <line
        x1="40"
        y1={GROUND_Y}
        x2="960"
        y2={GROUND_Y}
        stroke="var(--tree-struct)"
        strokeOpacity="0.4"
        strokeWidth="1.5"
      />
      {FIELDS.map((f) => (
        <g key={`root-${f.id}`}>
          {subRootPaths(f.id).map((d, i) => (
            <path
              key={i}
              d={d}
              stroke={f.hue}
              strokeOpacity={dimmed ? 0.08 : 0.2}
              strokeWidth="1"
              strokeLinecap="round"
              fill="none"
              style={{ transition: "stroke-opacity 200ms" }}
            />
          ))}
          <path
            d={rootPath(f.id)}
            stroke={f.hue}
            strokeOpacity={dimmed ? 0.14 : 0.38}
            strokeWidth="2.2"
            strokeLinecap="round"
            fill="none"
            style={{ transition: "stroke-opacity 200ms" }}
          />
        </g>
      ))}

      {/* 42 个学科节点。她连过的那些具名并且亮着；其余是暗点 —— 学科在有叶子
          连到它之前只是土里的一个位置，不是一个她要读的词。 */}
      {ROOT_NODES.map((n) => {
        const hue = fieldById(n.field).hue;
        const named = namedDiscs.has(n.id);
        const lit = litDiscs.has(n.id);
        const shared = sharedDiscs.get(n.id) ?? 0;
        return (
          <g
            key={n.id}
            onClick={() => onPickDiscipline(n.id)}
            style={{
              cursor: "pointer",
              opacity: dimmed ? (lit ? 1 : 0.13) : named ? 0.95 : 0.3,
              transition: "opacity 200ms",
            }}
          >
            <title>{`${n.zh} · ${n.asks}`}</title>
            {/* 44px 的命中区。节点本身只有几个像素，直接点它在触摸屏上不可能命中。 */}
            <circle cx={n.x} cy={n.y} r="22" fill="transparent" />
            <circle cx={n.x} cy={n.y} r={lit ? 15 : named ? 9 : 5} fill={hue} opacity="0.16" />
            <circle cx={n.x} cy={n.y} r={named ? 4 : 2.4} fill={hue} />
            {shared > 1 ? (
              // 被两个以上领域共用的根带一圈细环，不点也看得出来。
              <circle cx={n.x} cy={n.y} r="10" fill="none" stroke={hue} strokeWidth="1" opacity="0.5" />
            ) : null}
            {named ? (
              <text
                x={n.x}
                y={n.labelBelow ? n.y + 17 : n.y - 11}
                textAnchor="middle"
                fontSize="11"
                fontWeight="500"
                fill="var(--tree-ink-2)"
                opacity="0.9"
              >
                {n.zh}
              </text>
            ) : null}
          </g>
        );
      })}

      {/* ── 叶子 ─────────────────────────────────────────────────────────
          叶柄落在枝那条贝塞尔上，叶尖沿法向伸出去。它们画在 SVG 里而不是 HTML
          里，理由和光珠当年一样：一片飘在细枝旁边的叶子会把「这张图就是你的
          模型」悄悄降级成装饰。 */}
      {keywords.map((k) => {
        const hue = fieldById(k.field).hue;
        const leaf = leafShape(k.field, Math.min(k.at.t, maturity), k.at.spread);
        const on = dimmed ? litLeaves.has(k.id) : hoverField === null || hoverField === k.field;
        return (
          <g
            key={`leaf-${k.id}`}
            style={{ opacity: on ? 1 : 0.14, transition: "opacity 200ms" }}
            pointerEvents="none"
          >
            <circle cx={leaf.mx} cy={leaf.my} r={22 + k.strength * 2} fill={hue} opacity="0.1" />
            <path
              d={leaf.blade}
              fill={hue}
              fillOpacity="0.34"
              stroke={hue}
              strokeWidth="1.4"
              strokeLinejoin="round"
              filter="url(#tree-bloom)"
            />
            <path d={leaf.rib} fill="none" stroke="var(--tree-rib)" strokeWidth="1" opacity="0.34" />
          </g>
        );
      })}

      {/* 连线**只在点了之后才出现**。四个词乘三门学科等于十二条线穿过树干，
          常驻的话那是一团乱。线顺着树干往下走，读起来才是「这个词的根扎在
          那里」，而不是一条从树梢拉到根尖的直线。 */}
      {threads.map((t, i) => (
        <path
          key={i}
          d={t.d}
          fill="none"
          stroke={t.hue}
          strokeWidth="2"
          strokeLinecap="round"
          opacity="0.95"
        />
      ))}

      {/* the origin */}
      <circle cx="500" cy="742" r="17" fill="var(--tree-card)" stroke="var(--tree-struct)" strokeOpacity="0.5" />
      <circle cx="500" cy="742" r="4" fill="var(--mk-accent-400)" filter="url(#tree-bloom)" />
      <text
        x="500"
        y="712"
        textAnchor="middle"
        fontSize="9.5"
        fontFamily="ui-monospace, SF Mono, monospace"
        letterSpacing="1.4"
        fill="var(--tree-ink-2)"
        stroke="var(--mk-paper)"
        strokeWidth="3"
        paintOrder="stroke"
      >
        ORIGIN · 我
      </text>
    </svg>
  );
}

function Node({
  kw,
  maturity,
  index,
  dim,
  scale,
  onOpen,
  onHoverField,
}: {
  kw: Keyword;
  maturity: number;
  index: number;
  dim: boolean;
  scale: number;
  onOpen: () => void;
  onHoverField: (f: FieldId | null) => void;
}) {
  const f = fieldById(kw.field);
  // A node never sits past the drawn part of its branch — at early growth
  // stops the branch is short, so the leaf slides in with it.
  //
  // 标签挂在**叶尖外侧**（leafShape().lx/ly），不再挂在原来光珠的位置上：叶片
  // 是有长度的，标签压在叶身上会盖掉叶脉。
  const leaf = leafShape(kw.field, Math.min(kw.at.t, maturity), kw.at.spread);

  return (
    <Bead
      x={leaf.lx}
      y={leaf.ly}
      hue={f.hue}
      meta={`${kw.sources.length} 个来源`}
      name={kw.text}
      strength={kw.strength}
      dim={dim}
      scale={scale}
      delay={index * 40}
      title={kw.note}
      onClick={onOpen}
      onHover={(on) => onHoverField(on ? kw.field : null)}
    />
  );
}

/**
 * A keyword bead.
 *
 * Size carries strength (how many times her own work came back to this word)
 * and colour carries field. Both are translucent — a bead is light passing
 * through the structure, so the branch stays visible behind it, which is what
 * keeps the picture reading as ONE object instead of stickers on a drawing.
 *
 * The label is dark glass beside the bead rather than inside it: text inside a
 * glowing circle at these sizes is unreadable at any window height.
 */
function Bead({
  x,
  y,
  hue,
  meta,
  name,
  strength,
  fresh = false,
  dim = false,
  scale = 1,
  delay = 0,
  title,
  onClick,
  onHover,
}: {
  x: number;
  y: number;
  hue: string;
  meta: string;
  name: string;
  /** 1..5 — bead diameter and label weight. */
  strength: number;
  fresh?: boolean;
  dim?: boolean;
  /** Stage-fit factor — see `useFitScale`. */
  scale?: number;
  delay?: number;
  title?: string;
  onClick?: () => void;
  onHover?: (on: boolean) => void;
}) {
  const [hot, setHot] = useState(false);
  // Labels sit on the outboard side so they never cross the structure.
  const left = x < 500;
  // 标签离叶尖多远。原来这是光珠的直径（跟强度走）；叶子已经把强度画出来了，
  // 所以这里只是一个固定的呼吸空间。
  const d = 14 * scale;

  return (
    <div
      className="tree-node absolute"
      style={{
        left: `${(x / 1000) * 100}%`,
        top: `${(y / STAGE_H) * 100}%`,
        width: 0,
        height: 0,
        animationDelay: `${delay}ms`,
        opacity: dim ? 0.16 : 1,
        transition: "opacity 200ms cubic-bezier(.2,0,0,1)",
        zIndex: hot ? 20 : 6,
      }}
    >
      {/* 这里原来是一颗发光的珠子。2026-09-04 换成了**叶子** —— 叶片画在
          SVG 里（`leafShape`），因为它的叶柄必须落在枝那条贝塞尔上，而 HTML
          元素不认识那条曲线。这里只剩下标签，挂在叶尖外面。 */}
      <button
        type="button"
        title={title}
        onClick={onClick}
        onMouseEnter={() => {
          setHot(true);
          onHover?.(true);
        }}
        onMouseLeave={() => {
          setHot(false);
          onHover?.(false);
        }}
        onFocus={() => {
          setHot(true);
          onHover?.(true);
        }}
        onBlur={() => {
          setHot(false);
          onHover?.(false);
        }}
        className={cx(
          "absolute whitespace-nowrap text-left transition-all duration-[160ms] ease-mk",
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-300",
          onClick ? "cursor-pointer" : "cursor-default",
        )}
        style={{
          padding: `${5 * scale}px ${9 * scale}px`,
          top: -17 * scale,
          [left ? "right" : "left"]: d / 2 + 8 * scale,
          borderRadius: 3,
          background: hot ? "var(--tree-card)" : "rgba(255,255,255,.84)",
          border: hot
            ? `1px solid color-mix(in srgb, ${hue} 58%, transparent)`
            : "1px solid var(--tree-line)",
          boxShadow: hot ? "0 14px 32px rgba(51,48,46,.18)" : "0 1px 2px rgba(51,48,46,.05)",
          backdropFilter: "blur(3px)",
        }}
      >
        <span
          className="tree-mono block"
          style={{
            color: fresh ? hue : "var(--tree-ink-3)",
            fontSize: 9.5 * scale,
            letterSpacing: `${0.12 * scale}em`,
          }}
        >
          {meta}
        </span>
        <span
          className="mt-0.5 block leading-tight"
          style={{
            fontSize: (strength >= 4 ? 15 : 13.5) * scale,
            fontWeight: strength >= 4 ? 700 : 500,
            color: strength >= 4 ? "var(--tree-ink)" : "var(--tree-ink-2)",
          }}
        >
          {name}
        </span>
      </button>
    </div>
  );
}

/**
 * TreeState —— 树的四个状态里，除了「有树」之外的那三个。
 *
 * 它盖在树的上面，而不是替换整屏：底下那张空枝仍然看得见，所以「你的树还没长
 * 出东西」是一句关于一棵**存在的**树的话，不是一片白。
 *
 * 文案按 AGENTS.md「界面文案怎么写」：标签是名词，报错是动词+失败再接后台原话，
 * 请她做事用「请」+ 祈使句，不铺垫、不替她减压。
 */
function TreeState({
  status,
  error,
  onRetry,
  quizTaken,
  onStartQuiz,
}: {
  status: "loading" | "error" | "empty" | "ready";
  error: string;
  onRetry: () => void;
  /** null = 还不知道她做过没有。 */
  quizTaken: boolean | null;
  onStartQuiz: () => void;
}) {
  if (status === "ready") return null;

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center px-6">
      <div
        className="max-w-[420px] rounded-[18px] border px-6 py-5 text-center backdrop-blur-sm"
        style={{
          borderColor: "var(--tree-line)",
          background: "rgba(255,255,255,.9)",
          boxShadow: "0 16px 40px rgba(51,48,46,.1)",
        }}
      >
        {status === "loading" && (
          <>
            <Sys>处理中 · GROWING</Sys>
            <p className="mt-2 text-mk-body text-mk-ink">正在读取你的兴趣树</p>
            {/* 服务端会先把已完成、还没采过的阅读与写作补采一遍（最多三个，
                并行），所以第一次打开可能要几秒。说出来，别让她以为卡住了。 */}
            <p className="mt-1 text-mk-small text-mk-muted">
              正在从你最近完成的阅读与写作里提取关键词，需要几秒。
            </p>
          </>
        )}

        {status === "error" && (
          <>
            <Sys>读取失败 · ERROR</Sys>
            <p className="mt-2 text-mk-body text-mk-ink">兴趣树读取失败</p>
            {/* 后台原话原样给出：她和我们看到的是同一句（界面文案 §8）。 */}
            <p className="mt-1 break-words text-mk-small text-mk-muted">{error}</p>
            <button
              type="button"
              onClick={onRetry}
              className="mt-4 rounded-full border px-4 py-1.5 text-mk-small text-mk-ink transition hover:opacity-80"
              style={{ borderColor: "var(--tree-line-strong)" }}
            >
              重试
            </button>
          </>
        )}

        {status === "empty" && (
          <>
            <Sys>空 · NO KEYWORDS YET</Sys>
            <p className="mt-2 text-mk-body text-mk-ink">这棵树还没有关键词</p>
            <p className="mt-1 text-mk-small text-mk-muted">
              关键词由你完成的阅读、写作与项目自动生成。完成一篇后回到这里，
              它会长出来。
            </p>
            {/* 🚨 空树上那条邀请。**只在明确知道她没做过时出现** —— 读不到
                状态（null）时不显示，因为在一个上个月已经做过的学生面前每次
                都闪一下「来做个测试」，比不显示糟。
                这也是产品负责人要的那条：第一次看见这棵树时邀请她做测试。 */}
            {quizTaken === false ? (
              <>
                <div className="my-4 h-px" style={{ background: "var(--tree-line)" }} />
                <p className="text-mk-small leading-[1.8] text-mk-secondary">
                  也可以先做一次兴趣测试，五分钟，树上会长出第一批词。
                </p>
                <button
                  type="button"
                  onClick={onStartQuiz}
                  className="mt-3 rounded-full px-5 py-2 text-mk-body font-semibold text-white transition hover:opacity-90"
                  style={{ background: "linear-gradient(135deg,var(--mk-accent-400),var(--mk-accent-600))" }}
                >
                  开始兴趣测试
                </button>
              </>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}

/**
 * 空枝邀请 —— 「你的树上还没有这根枝」。
 *
 * 一根没有词的枝上，那个 `0` 什么也没说。它其实是这张图上**最有用的一条
 * 信息**：她还没走过的方向。所以点开它不是显示「暂无数据」，而是问一句这根枝
 * 上会长出什么，再给两个真的入口。
 *
 * 🚨 这里**不劝她**。文案说的是这根枝研究什么、以及从哪能碰到它，不说「快去
 * 试试吧」——一根空枝不是一个缺口，一个学生没有义务把七根枝都长满。
 */
function BranchInvite({
  field,
  onClose,
  onExplore,
  onQuiz,
}: {
  field: FieldId | null;
  onClose: () => void;
  onExplore: () => void;
  onQuiz: () => void;
}) {
  if (!field) return null;
  const f = fieldById(field);
  const blurb = BRANCH_BLURB[field];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-6" role="dialog" aria-label={f.label}>
      <button
        type="button"
        aria-label="关闭"
        onClick={onClose}
        className="absolute inset-0 cursor-default"
        style={{ background: "var(--tree-veil)" }}
      />
      <div
        className="tree-in relative w-full max-w-[460px] rounded-[18px] p-6"
        style={{
          background: "var(--tree-card)",
          border: `1px solid color-mix(in srgb, ${f.hue} 38%, transparent)`,
          boxShadow: "0 24px 60px rgba(51,48,46,.16)",
        }}
      >
        <div className="flex items-center gap-2">
          <span
            className="h-2.5 w-2.5 rotate-45"
            style={{ background: f.hue }}
          />
          <Sys>{f.en}</Sys>
        </div>
        <h2 className="mt-1.5 text-mk-h2 text-mk-ink">你的树上还没有{f.label}这根枝</h2>
        <p className="mt-3 text-mk-body leading-[1.9] text-mk-secondary">{blurb}</p>
        <p className="mt-3 text-mk-small leading-[1.85] text-mk-muted">
          关键词只从你真的做完的事情上长出来，所以这根枝空着，只是说明你还没往这边走过。
        </p>

        <div className="mt-5 flex flex-wrap gap-2.5">
          <button
            type="button"
            onClick={onExplore}
            className="rounded-mk-full px-4 py-2 text-mk-small font-semibold text-white transition hover:opacity-90"
            style={{ background: f.hue }}
          >
            去今日探索地图看看
          </button>
          <button
            type="button"
            onClick={onQuiz}
            className="rounded-mk-full border px-4 py-2 text-mk-small text-mk-ink transition hover:opacity-80"
            style={{ borderColor: "var(--tree-line-strong)" }}
          >
            做一次兴趣测试
          </button>
          <button
            type="button"
            onClick={onClose}
            className="rounded-mk-full px-4 py-2 text-mk-small text-mk-muted transition hover:text-mk-ink"
          >
            先不用
          </button>
        </div>
      </div>
    </div>
  );
}

/** 每根主枝一句「它研究什么」。写的是这门学问在问什么，不是一句招徕。 */
const BRANCH_BLURB: Record<FieldId, string> = {
  formal: "数学与形式问的是：一句话为什么一定成立？一个样本凭什么替一群人说话？统计、概率、逻辑、建模都在这根枝上。",
  science: "科学与自然问的是：这件事到底怎么发生的？从气候与海洋到神经科学、天文，都在追同一个「为什么」。",
  making: "技术与创造问的是：这东西能不能做出来、做得更好？算法、材料、工程设计、能源系统都在这里。",
  society: "社会与世界问的是：人和人凑在一起之后会发生什么？经济、社会学、地理与城市、公共卫生都在这根枝上。",
  humanities: "人文与写作问的是：这段话为什么这样打动人、这个说法站不站得住？历史、哲学、修辞与论证、媒介素养都在这里。",
  arts: "艺术与表达问的是：怎么把一个感觉准确地交出去？影像叙事、视觉设计、音乐理论、创意写作都在这根枝上。",
  self: "自我与成长问的是：我是怎么变成现在这样的、又怎么学得更好？认知与发展心理学、伦理学、学习科学都在这里。",
};
