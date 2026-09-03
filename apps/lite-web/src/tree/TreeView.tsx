import { useMemo, useRef, useState } from "react";
import type { MeUser } from "../api/auth";
import {
  FIELDS,
  GROWTH_STOPS,
  stopsFor,
  branchPath,
  fieldById,
  pointOnBranch,
} from "./geometry";
import { outputCount, useInterestTree } from "./useInterestTree";
import type { FieldId, Keyword } from "./types";
import { Hint, Sys, cx } from "./ui";
import { useFitScale } from "./useFitScale";
import { KeywordDrawer } from "./KeywordDrawer";
import { useQuizTaken } from "./quiz/useQuizStatus";
import { liteRoutePath, navigate } from "../routing";
import "./tree.css";

/**
 * 我的兴趣树 · the keyword model, as a lit structure on a dark ground.
 *
 * ## The visual ruling, third pass (2026-08-31)
 * v1 was a literal cartoon tree (brown trunk, green candy pills). v2 replaced
 * it with a technical diagram on paper — correct, and cold. v3 keeps v2's
 * geometry and moves it onto its own night: **the structure GLOWS and the
 * keywords are translucent beads of light hanging on it.**
 *
 * Why it is still an SVG rather than a generated illustration: every bead is
 * placed by `pointOnBranch()` on the exact Bézier the SVG draws. A painted
 * tree has branches the maths knows nothing about, so the beads would float
 * beside twigs instead of hanging on them, and the whole claim — *this
 * picture IS your model* — would quietly become decoration. The picture has
 * to be the data structure. So the glow is built, not imported: luminous
 * gradient strokes, an SVG bloom filter, secondary twigs for density, and
 * motes of light rising through the canopy.
 *
 * The 世界 map and this share one instrument in two directions: out there,
 * orbit rings around a sun; in here, growth rings around an origin.
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
export function TreeView({ user }: { user: MeUser }) {
  // 成长回放的刻度是**这一页的本地状态**。原型里它住在 EcoProvider 的全局
  // store 里，那是因为世界和树共用一个 store；在 lite 里没有别的页面关心她把
  // 回放拖到了哪一格，把它提升到全局只会让一个纯展示的选择跨页面存活。
  const [stop, setStop] = useState(GROWTH_STOPS.length - 1);
  const [openId, setOpenId] = useState<string | null>(null);
  const [hoverField, setHoverField] = useState<FieldId | null>(null);

  const stageRef = useRef<HTMLDivElement>(null);
  // Bead labels are fixed pixel size on a stage that scales with the viewport.
  // Without this they collide on any short window.
  const scale = useFitScale(stageRef, 792, 0.74);

  // 真数据。没有 mock 兜底——一棵回退到示例词的树，会把十六个不属于她的词
  // 挂在一张标着「这就是你的模型」的图上，而她看不出来。见 useInterestTree。
  const live = useInterestTree();
  const all = live.keywords;

  // 觉醒协议（兴趣测试）。三态：null = 还不知道，那时两件事都不做。
  const quizTaken = useQuizTaken();
  const openQuiz = () => navigate(liteRoutePath({ tab: "tree", quiz: true }));
  // 空枝邀请：点一根还没有词的枝，问的是「这根枝上会长什么」。
  const [inviteField, setInviteField] = useState<FieldId | null>(null);

  const visible = useMemo(() => all.filter((k) => k.bornAt <= stop), [all, stop]);
  // Only the stops that actually hold something. A dot that shows the tree she
  // is already looking at is a control that does nothing.
  const liveStops = useMemo(() => stopsFor(all), [all]);
  const maturity = 0.42 + (stop / 3) * 0.58;
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
          <Sys tone="dark">我的兴趣树 · INTEREST TREE</Sys>
          <h1 className="mt-1 text-mk-h1 text-[#F5EFE7]">
            {user.display_name}
            {user.classes[0] ? (
              <span className="ml-2 text-mk-body font-normal text-[#9A8E80]">
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
                const st = GROWTH_STOPS[si]!;
                const active = si === stop;
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
                          background: active ? "var(--mk-accent-400)" : "rgba(240,233,224,.32)",
                          transform: active ? "rotate(45deg) scale(1.5)" : "rotate(45deg)",
                          boxShadow: active ? "0 0 12px var(--mk-accent-400)" : "none",
                        }}
                      />
                      <span
                        className={cx(
                          "whitespace-nowrap text-mk-small transition-colors duration-[160ms]",
                          active ? "font-semibold text-[#F5EFE7]" : "text-[#8E8175]",
                        )}
                      >
                        {st.label}
                      </span>
                    </button>
                    {i < liveStops.length - 1 ? (
                      <span
                        className="mb-5 h-px w-6 shrink-0"
                        style={{ background: "rgba(240,233,224,.18)" }}
                      />
                    ) : null}
                  </div>
                );
              })}
              <span className="mb-5 ml-3 text-mk-small text-[#8E8175]">
                {stop === GROWTH_STOPS.length - 1 ? `${total} 个关键词` : `那时候 ${total} 个`}
              </span>
            </div>
          ) : (
            // 只在真的读到了「她还没有词」时才说这句。读取失败时说「你的树刚
            // 开始长」，是在替她断言一件我们并不知道的事。
            known && (
              <p className="mt-3 max-w-[46ch] text-mk-small leading-[1.8] text-[#8E8175]">
                你的树刚开始长。每读完一篇、写完一篇、做完一个项目，它就会多一个词。
              </p>
            )
          )}
        </div>
        <div className="flex items-center gap-5">
          {/* 兴趣测试的常驻入口。树不空时也留着，因为**重做是再长几个词**
              （服务端给同一个词再添一条来源，强度上升），不是清空重来。
              和空树上那条邀请一样，状态未知（null）时不显示。 */}
          {quizTaken !== null ? (
            <button
              type="button"
              onClick={openQuiz}
              className="rounded-full border px-3.5 py-1.5 text-mk-small transition-colors duration-[120ms]
                         hover:bg-[rgba(240,233,224,.1)] focus-visible:outline-none
                         focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
              style={{ borderColor: "rgba(85,230,255,.42)", color: "#9fdcf0" }}
            >
              {quizTaken ? "再做一次兴趣测试" : "兴趣测试"}
            </button>
          ) : null}
          <span className="text-right">
            <span className="flex items-center justify-end gap-1.5">
              <Sys tone="dark">关键词</Sys>
              <Hint
                tone="dark"
                text="根据你读过、收藏过、写过、做过的东西自动生成的兴趣关键词。每一个都可以点开，看它到底是从哪几件事来的。"
              />
            </span>
            <span className="block font-mono text-mk-h2 tabular-nums text-[#F5EFE7]">
              {live.status === "ready" || live.status === "empty" ? total : "—"}
            </span>
          </span>
          <span className="h-8 w-px" style={{ background: "rgba(240,233,224,.2)" }} />
          <span className="text-right">
            <span className="flex items-center justify-end gap-1.5">
              <Sys tone="dark">成果数</Sys>
              <Hint
                tone="dark"
                text="你已经完成的阅读、写作和已发布项目的总数。没做完的不算——这个数字只数你真的做出来的东西。"
              />
            </span>
            <span className="block font-mono text-mk-h2 tabular-nums text-[#F5EFE7]">
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
            onClick={() => (known && countFor(f.id) === 0 ? setInviteField(f.id) : undefined)}
            className="inline-flex items-center gap-1.5 rounded-mk-full px-2.5 py-1 text-mk-small
                       transition-colors duration-[120ms] hover:bg-[rgba(240,233,224,.1)]
                       focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
            style={{ border: "1px solid rgba(240,233,224,.16)", color: "#C0B4A6" }}
          >
            <span
              className="h-1.5 w-1.5 rotate-45"
              style={{ background: f.hue, boxShadow: `0 0 8px ${f.hue}` }}
            />
            <span className="tree-mono text-[#7C7166]">{String(i + 1).padStart(2, "0")}</span>
            {f.label}
            <span className="font-mono text-[11px] tabular-nums text-[#7C7166]">
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
            aspectRatio: "1000 / 780",
            height: "min(792px, calc(100vh - 300px))",
            width: "auto",
            maxWidth: "1120px",
          }}
        >
          <TreeSvg maturity={maturity} hoverField={hoverField} />

          {/* 四个状态，说清楚是哪一个。绝不用示例关键词填满一棵空树——那会把
              十六个不属于她的词挂在一张写着「这就是你的模型」的图上。 */}
          <TreeState
            status={live.status}
            error={live.error}
            onRetry={live.reload}
            quizTaken={quizTaken}
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
              onOpen={() => setOpenId(k.id)}
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
        <Sys tone="dark" className="mb-2 block">
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
                  onClick={() => (known && n === 0 ? setInviteField(f.id) : undefined)}
                  className="flex w-full items-center gap-2 border-b px-1 py-2 text-left transition-colors
                             duration-[120ms] hover:bg-[rgba(240,233,224,.07)] focus-visible:outline-none
                             focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
                  style={{ borderColor: "rgba(240,233,224,.1)" }}
                >
                  <span
                    className="h-1.5 w-1.5 shrink-0 rotate-45"
                    style={{ background: f.hue, boxShadow: `0 0 8px ${f.hue}` }}
                  />
                  <span className="tree-mono shrink-0 text-[#7C7166]">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-mk-small text-[#C0B4A6]">{f.label}</span>
                  {known && n === 0 ? (
                    <span className="shrink-0 text-[11px]" style={{ color: "var(--mk-accent-400)" }}>
                      还没有
                    </span>
                  ) : (
                    <span className="font-mono text-[11px] tabular-nums text-[#7C7166]">
                      {known ? n : "—"}
                    </span>
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      </div>

      <KeywordDrawer kw={openKw} onClose={() => setOpenId(null)} />
      <BranchInvite
        field={inviteField}
        onClose={() => setInviteField(null)}
        onExplore={() => navigate(liteRoutePath({ tab: "explore" }))}
        onQuiz={openQuiz}
      />
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
function TreeSvg({ maturity, hoverField }: { maturity: number; hoverField: FieldId | null }) {
  return (
    <svg
      viewBox="0 0 1000 780"
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
          <stop offset="0%" stopColor="#FFE9CF" stopOpacity="0.5" />
          <stop offset="55%" stopColor="#FFD9B0" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#FFD9B0" stopOpacity="0.05" />
        </linearGradient>
        <radialGradient id="tree-canopy" cx="50%" cy="46%" r="52%">
          <stop offset="0%" stopColor="#FFC9A0" stopOpacity="0.16" />
          <stop offset="60%" stopColor="#FFB98C" stopOpacity="0.05" />
          <stop offset="100%" stopColor="#FFB98C" stopOpacity="0" />
        </radialGradient>
        {/* The bloom. `stdDeviation` is deliberately large: this is a halo
            around the strokes, not a soft edge on them. */}
        <filter id="tree-bloom" x="-30%" y="-30%" width="160%" height="160%">
          <feGaussianBlur stdDeviation="7" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

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
          stroke="#F0E9E0"
          strokeOpacity={0.1 - i * 0.018}
          strokeDasharray="2 7"
        />
      ))}

      {/* horizon + ticks */}
      <line x1="90" y1="742" x2="910" y2="742" stroke="#F0E9E0" strokeOpacity="0.16" />
      {[...Array(21)].map((_, i) => (
        <line
          key={i}
          x1={100 + i * 40}
          y1="742"
          x2={100 + i * 40}
          y2={i % 5 === 0 ? 752 : 747}
          stroke="#F0E9E0"
          strokeOpacity="0.13"
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
              fill="#7C7166"
            >
              {`FIELD ${String(idx + 1).padStart(2, "0")}`}
            </text>
            <text
              x={tip.x + (left ? -20 : 20)}
              y={tip.y - 19}
              textAnchor={left ? "end" : "start"}
              fontSize="12.5"
              fontWeight="600"
              fill="#C0B4A6"
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
        filter="url(#tree-bloom)"
      />
      <path
        d="M 500 748 C 497 640, 503 560, 500 470 C 498 400, 502 350, 500 292"
        stroke="#FFF0DC"
        strokeOpacity="0.4"
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
          filter="url(#tree-bloom)"
        />
      ))}

      {/* the origin */}
      <circle cx="500" cy="742" r="17" fill="#0B0907" stroke="#F0E9E0" strokeOpacity="0.26" />
      <circle cx="500" cy="742" r="4" fill="var(--mk-accent-400)" filter="url(#tree-bloom)" />
      <text
        x="500"
        y="712"
        textAnchor="middle"
        fontSize="9.5"
        fontFamily="ui-monospace, SF Mono, monospace"
        letterSpacing="1.4"
        fill="#7C7166"
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
  // stops the branch is short, so the bead slides in with it.
  const p = pointOnBranch(kw.field, Math.min(kw.at.t, maturity), kw.at.spread);

  return (
    <Bead
      x={p.x}
      y={p.y}
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
  const d = (17 + strength * 6) * scale;

  return (
    <div
      className="tree-node absolute"
      style={{
        left: `${x / 10}%`,
        top: `${y / 7.8}%`,
        width: 0,
        height: 0,
        animationDelay: `${delay}ms`,
        opacity: dim ? 0.16 : 1,
        transition: "opacity 200ms cubic-bezier(.2,0,0,1)",
        zIndex: hot ? 20 : 6,
      }}
    >
      {/* the bead */}
      <span
        aria-hidden
        className="tree-bead absolute block"
        style={{
          left: -d / 2,
          top: -d / 2,
          width: d,
          height: d,
          background: `radial-gradient(circle at 36% 32%, color-mix(in srgb, ${hue} 82%, #FFFFFF), color-mix(in srgb, ${hue} 62%, transparent) 62%, color-mix(in srgb, ${hue} 22%, transparent))`,
          border: `1px solid color-mix(in srgb, ${hue} 60%, transparent)`,
          boxShadow: hot
            ? `0 0 0 2px color-mix(in srgb, ${hue} 40%, transparent), 0 0 34px color-mix(in srgb, ${hue} 70%, transparent)`
            : `0 0 ${12 + strength * 4}px color-mix(in srgb, ${hue} 46%, transparent)`,
          // Periods spread by strength so the canopy shimmers instead of
          // pulsing as one organism.
          ["--bead-period" as string]: `${5 + strength * 0.9}s`,
          ["--bead-delay" as string]: `${delay}ms`,
        }}
      />

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
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]",
          onClick ? "cursor-pointer" : "cursor-default",
        )}
        style={{
          padding: `${5 * scale}px ${9 * scale}px`,
          top: -17 * scale,
          [left ? "right" : "left"]: d / 2 + 8 * scale,
          borderRadius: 3,
          background: hot ? "rgba(28,23,19,.97)" : "rgba(18,15,12,.62)",
          border: hot
            ? `1px solid color-mix(in srgb, ${hue} 58%, transparent)`
            : "1px solid rgba(240,233,224,.12)",
          boxShadow: hot ? "0 18px 44px rgba(0,0,0,.6)" : "none",
          backdropFilter: "blur(3px)",
        }}
      >
        <span
          className="tree-mono block"
          style={{
            color: fresh ? hue : "#8A7F72",
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
            color: strength >= 4 ? "#FBF5EC" : "#DCD2C6",
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
          borderColor: "rgba(245,239,231,0.14)",
          background: "rgba(20,16,12,0.72)",
        }}
      >
        {status === "loading" && (
          <>
            <Sys tone="dark">处理中 · GROWING</Sys>
            <p className="mt-2 text-mk-body text-[#F5EFE7]">正在读取你的兴趣树</p>
            {/* 服务端会先把已完成、还没采过的阅读与写作补采一遍（最多三个，
                并行），所以第一次打开可能要几秒。说出来，别让她以为卡住了。 */}
            <p className="mt-1 text-mk-small text-[#9A8E80]">
              正在从你最近完成的阅读与写作里提取关键词，需要几秒。
            </p>
          </>
        )}

        {status === "error" && (
          <>
            <Sys tone="dark">读取失败 · ERROR</Sys>
            <p className="mt-2 text-mk-body text-[#F5EFE7]">兴趣树读取失败</p>
            {/* 后台原话原样给出：她和我们看到的是同一句（界面文案 §8）。 */}
            <p className="mt-1 break-words text-mk-small text-[#9A8E80]">{error}</p>
            <button
              type="button"
              onClick={onRetry}
              className="mt-4 rounded-full border px-4 py-1.5 text-mk-small text-[#F5EFE7] transition hover:opacity-80"
              style={{ borderColor: "rgba(245,239,231,0.3)" }}
            >
              重试
            </button>
          </>
        )}

        {status === "empty" && (
          <>
            <Sys tone="dark">空 · NO KEYWORDS YET</Sys>
            <p className="mt-2 text-mk-body text-[#F5EFE7]">这棵树还没有关键词</p>
            <p className="mt-1 text-mk-small text-[#9A8E80]">
              关键词由你完成的阅读、写作与项目自动生成。完成一篇后回到这里，
              它会长出来。
            </p>
            {/* 🚨 空树上那条邀请。**只在明确知道她没做过时出现** —— 读不到
                状态（null）时不显示，因为在一个上个月已经做过的学生面前每次
                都闪一下「来做个测试」，比不显示糟。
                这也是产品负责人要的那条：第一次看见这棵树时邀请她做测试。 */}
            {quizTaken === false ? (
              <>
                <div className="my-4 h-px" style={{ background: "rgba(245,239,231,0.14)" }} />
                <p className="text-mk-small leading-[1.8] text-[#C0B4A6]">
                  也可以先做一次兴趣测试，五分钟，树上会长出第一批词。
                </p>
                <button
                  type="button"
                  onClick={onStartQuiz}
                  className="mt-3 rounded-full px-5 py-2 text-mk-body font-semibold text-[#04121d] transition hover:opacity-90"
                  style={{ background: "linear-gradient(135deg,#55e6ff,#1ca5dc)" }}
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
        style={{ background: "rgba(10,8,6,.62)" }}
      />
      <div
        className="tree-in relative w-full max-w-[460px] rounded-[18px] p-6"
        style={{ background: "#1C1713", border: `1px solid color-mix(in srgb, ${f.hue} 38%, transparent)` }}
      >
        <div className="flex items-center gap-2">
          <span
            className="h-2.5 w-2.5 rotate-45"
            style={{ background: f.hue, boxShadow: `0 0 12px ${f.hue}` }}
          />
          <Sys tone="dark">{f.en}</Sys>
        </div>
        <h2 className="mt-1.5 text-mk-h2 text-[#F5EFE7]">你的树上还没有{f.label}这根枝</h2>
        <p className="mt-3 text-mk-body leading-[1.9] text-[#C0B4A6]">{blurb}</p>
        <p className="mt-3 text-mk-small leading-[1.85] text-[#8E8175]">
          关键词只从你真的做完的事情上长出来，所以这根枝空着，只是说明你还没往这边走过。
        </p>

        <div className="mt-5 flex flex-wrap gap-2.5">
          <button
            type="button"
            onClick={onExplore}
            className="rounded-mk-full px-4 py-2 text-mk-small font-semibold text-[#17130F] transition hover:opacity-90"
            style={{ background: f.hue }}
          >
            去今日探索地图看看
          </button>
          <button
            type="button"
            onClick={onQuiz}
            className="rounded-mk-full border px-4 py-2 text-mk-small text-[#F0E9E0] transition hover:opacity-80"
            style={{ borderColor: "rgba(240,233,224,.3)" }}
          >
            做一次兴趣测试
          </button>
          <button
            type="button"
            onClick={onClose}
            className="rounded-mk-full px-4 py-2 text-mk-small text-[#8E8175] transition hover:text-[#C0B4A6]"
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
