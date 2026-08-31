import { useMemo, useRef, useState } from "react";
import { BookOpen, Hexagon, MessageCircle, PenLine } from "lucide-react";
import { useEco } from "../store";
import {
  FIELDS,
  GROWTH_STOPS,
  KEYWORDS,
  branchPath,
  fieldById,
  keywordById,
  pointOnBranch,
} from "../data/tree";
import { READINGS, STUDENT, WRITINGS } from "../data/library";
import { go } from "../route";
import type { FieldId, Keyword } from "../data/types";
import { Hint, Sys, cx } from "../ui";
import { useFitScale } from "./../fit";
import { ViewSwitch } from "./ViewSwitch";
import { KeywordDrawer } from "./KeywordDrawer";

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
export function TreeView() {
  const { state, setGrowth, openCoach } = useEco();
  const [openId, setOpenId] = useState<string | null>(null);
  const [hoverField, setHoverField] = useState<FieldId | null>(null);

  const stageRef = useRef<HTMLDivElement>(null);
  // Bead labels are fixed pixel size on a stage that scales with the viewport.
  // Without this they collide on any short window.
  const scale = useFitScale(stageRef, 792, 0.74);

  const stop = state.growth;
  const visible = useMemo(() => KEYWORDS.filter((k) => k.bornAt <= stop), [stop]);
  const maturity = 0.42 + (stop / 3) * 0.58;
  // Words she kept from 世界 are collected NOW, so they exist only at the
  // present stop. Showing them while the replay is rewound put a keyword on a
  // tree that also claimed to have zero keywords.
  const grown = stop === GROWTH_STOPS.length - 1 ? state.grownKeywords : [];
  const openKw = openId ? (keywordById(openId) ?? null) : null;

  // ONE count, used by the header, the field index and the caption. Three
  // places computing it independently is how the screen came to show 17 / 16 / 0
  // at the same time.
  const countFor = (fieldId?: FieldId) =>
    (fieldId ? visible.filter((k) => k.field === fieldId) : visible).length +
    (fieldId ? grown.filter((g) => g.field === fieldId) : grown).length;
  const total = countFor();

  // 成果数 — finished things, not activity. Readings + writings + projects she
  // actually published. A project still in progress is not a 成果, and calling
  // it one would be the same lie as a streak counter.
  const published = state.projects.filter((p) => p.status === "published").length;
  const outputs = READINGS.length + WRITINGS.length + published;

  return (
    <div className="eco-grove eco-motes relative min-h-full">
      <header className="relative z-20 flex flex-wrap items-center justify-between gap-4 px-7 pt-6">
        <div className="min-w-0">
          <Sys tone="dark">我的兴趣树 · INTEREST TREE</Sys>
          <h1 className="mt-1 text-mk-h1 text-[#F5EFE7]">
            {STUDENT.name}
            <span className="ml-2 text-mk-body font-normal text-[#9A8E80]">{STUDENT.grade}</span>
          </h1>
        </div>
        <ViewSwitch view="tree" />
        <div className="flex items-center gap-5">
          <span className="text-right">
            <span className="flex items-center justify-end gap-1.5">
              <Sys tone="dark">关键词</Sys>
              <Hint
                tone="dark"
                text="根据你读过、收藏过、写过、做过的东西自动生成的兴趣关键词。每一个都可以点开，看它到底是从哪几件事来的。"
              />
            </span>
            <span className="block font-mono text-mk-h2 tabular-nums text-[#F5EFE7]">{total}</span>
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
            <span className="block font-mono text-mk-h2 tabular-nums text-[#F5EFE7]">{outputs}</span>
          </span>
        </div>
      </header>

      {/* ── timeline ──────────────────────────────────────────────────── */}
      <div className="relative z-20 mt-5 flex flex-wrap items-center gap-4 px-7">
        <Sys tone="dark">时间轴</Sys>
        <div className="flex items-center">
          {GROWTH_STOPS.map((s, i) => {
            const active = i === stop;
            const past = i < stop;
            return (
              <div key={s.label} className="flex items-center">
                <button
                  type="button"
                  onClick={() => setGrowth(i)}
                  className="group flex flex-col items-center gap-1.5 px-3 py-1 focus-visible:outline-none"
                  aria-pressed={active}
                >
                  <span
                    className="h-2 w-2 transition-all duration-[200ms] ease-mk"
                    style={{
                      background: active
                        ? "var(--mk-accent-400)"
                        : past
                          ? "rgba(240,233,224,.5)"
                          : "transparent",
                      border: active || past ? "none" : "1px solid rgba(240,233,224,.3)",
                      transform: active ? "rotate(45deg) scale(1.5)" : "rotate(45deg)",
                      boxShadow: active ? "0 0 12px var(--mk-accent-400)" : "none",
                    }}
                  />
                  <span
                    className={cx(
                      "whitespace-nowrap text-mk-small transition-colors duration-[160ms]",
                      active ? "font-semibold text-[#F5EFE7]" : "text-[#9A8E80]",
                    )}
                  >
                    {s.label}
                  </span>
                </button>
                {i < GROWTH_STOPS.length - 1 ? (
                  <span className="h-px w-8 shrink-0" style={{ background: "rgba(240,233,224,.18)" }} />
                ) : null}
              </div>
            );
          })}
        </div>
        <p className="text-mk-small text-[#9A8E80]">
          {stop === GROWTH_STOPS.length - 1
            ? `现在：${total} 个关键词。`
            : `${GROWTH_STOPS[stop]?.sub ?? ""} —— 那时候树上有 ${total} 个词。`}
        </p>
      </div>

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
            className="inline-flex items-center gap-1.5 rounded-mk-full px-2.5 py-1 text-mk-small
                       transition-colors duration-[120ms] hover:bg-[rgba(240,233,224,.1)]
                       focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
            style={{ border: "1px solid rgba(240,233,224,.16)", color: "#C0B4A6" }}
          >
            <span
              className="h-1.5 w-1.5 rotate-45"
              style={{ background: f.hue, boxShadow: `0 0 8px ${f.hue}` }}
            />
            <span className="eco-mono text-[#7C7166]">{String(i + 1).padStart(2, "0")}</span>
            {f.label}
            <span className="font-mono text-[11px] tabular-nums text-[#7C7166]">{countFor(f.id)}</span>
          </button>
        ))}
      </div>

      {/* ── the structure ─────────────────────────────────────────────── */}
      <div className="relative z-10 px-7 pb-28 pt-2">
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

          {/* Kept-from-世界 words get their own lane low on the trunk, so a new
              arrival can never land on top of an existing keyword. */}
          {grown.map((g, i) => {
            const f = fieldById(g.field as FieldId);
            const p = pointOnBranch(f.id, 0.1, i % 2 === 0 ? 44 : -44);
            return (
              <Bead
                key={g.id}
                x={p.x}
                y={p.y + i * 34}
                hue={f.hue}
                meta="新收藏"
                name={g.text}
                strength={2}
                fresh
                scale={scale}
              />
            );
          })}
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
                  className="flex w-full items-center gap-2 border-b px-1 py-2 text-left transition-colors
                             duration-[120ms] hover:bg-[rgba(240,233,224,.07)] focus-visible:outline-none
                             focus-visible:ring-2 focus-visible:ring-[#8A7F72]"
                  style={{ borderColor: "rgba(240,233,224,.1)" }}
                >
                  <span
                    className="h-1.5 w-1.5 shrink-0 rotate-45"
                    style={{ background: f.hue, boxShadow: `0 0 8px ${f.hue}` }}
                  />
                  <span className="eco-mono shrink-0 text-[#7C7166]">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <span className="min-w-0 flex-1 truncate text-mk-small text-[#C0B4A6]">{f.label}</span>
                  <span className="font-mono text-[11px] tabular-nums text-[#7C7166]">{n}</span>
                </button>
              </li>
            );
          })}
        </ul>
      </div>

      {/* ── action dock ───────────────────────────────────────────────── */}
      <div className="fixed bottom-6 left-1/2 z-30 -translate-x-1/2">
        <div
          className="flex items-center gap-1 rounded-mk-full p-1.5"
          style={{
            background: "rgba(28,23,19,.86)",
            border: "1px solid rgba(240,233,224,.16)",
            backdropFilter: "blur(14px)",
            boxShadow: "0 24px 60px rgba(0,0,0,.5)",
          }}
        >
          <Dock icon={<BookOpen size={17} strokeWidth={1.8} />} label="读" sub="找一篇" onClick={() => go({ name: "readings" })} />
          <Dock icon={<PenLine size={17} strokeWidth={1.8} />} label="写" sub="写一篇" onClick={() => go({ name: "writings" })} />
          <Dock icon={<Hexagon size={17} strokeWidth={1.8} />} label="做" sub="开项目" onClick={() => go({ name: "projects" })} />
          <Dock
            icon={<MessageCircle size={17} strokeWidth={1.8} />}
            label="聊"
            sub="问印记"
            onClick={() => openCoach("tree")}
          />
        </div>
      </div>

      <KeywordDrawer kw={openKw} onClose={() => setOpenId(null)} />
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
              id={`eco-br-${f.id}`}
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
        <linearGradient id="eco-spine" x1="0" y1="1" x2="0" y2="0">
          <stop offset="0%" stopColor="#FFE9CF" stopOpacity="0.5" />
          <stop offset="55%" stopColor="#FFD9B0" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#FFD9B0" stopOpacity="0.05" />
        </linearGradient>
        <radialGradient id="eco-canopy" cx="50%" cy="46%" r="52%">
          <stop offset="0%" stopColor="#FFC9A0" stopOpacity="0.16" />
          <stop offset="60%" stopColor="#FFB98C" stopOpacity="0.05" />
          <stop offset="100%" stopColor="#FFB98C" stopOpacity="0" />
        </radialGradient>
        {/* The bloom. `stdDeviation` is deliberately large: this is a halo
            around the strokes, not a soft edge on them. */}
        <filter id="eco-bloom" x="-30%" y="-30%" width="160%" height="160%">
          <feGaussianBlur stdDeviation="7" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      {/* canopy glow */}
      <ellipse cx="500" cy="360" rx="480" ry="310" fill="url(#eco-canopy)" />
      <ellipse cx="500" cy="470" rx="250" ry="230" fill="url(#eco-canopy)" />

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
                    filter="url(#eco-bloom)"
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
              filter="url(#eco-bloom)"
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
              stroke={`url(#eco-br-${f.id})`}
              strokeWidth={hoverField === f.id ? 5 : 3}
              strokeLinecap="round"
              fill="none"
              pathLength={1}
              strokeDasharray={`${maturity} 1`}
              filter="url(#eco-bloom)"
              style={{ transition: "stroke-width 200ms" }}
            />
            <circle cx={tip.x} cy={tip.y} r="3.5" fill={f.hue} filter="url(#eco-bloom)" />
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
        fill="url(#eco-spine)"
        filter="url(#eco-bloom)"
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
          stroke="url(#eco-spine)"
          strokeWidth="5"
          strokeLinecap="round"
          fill="none"
          filter="url(#eco-bloom)"
        />
      ))}

      {/* the origin */}
      <circle cx="500" cy="742" r="17" fill="#0B0907" stroke="#F0E9E0" strokeOpacity="0.26" />
      <circle cx="500" cy="742" r="4" fill="var(--mk-accent-400)" filter="url(#eco-bloom)" />
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
      className="eco-node absolute"
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
        className="eco-bead absolute block"
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
          className="eco-mono block"
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

function Dock({
  icon,
  label,
  sub,
  onClick,
}: {
  icon: React.ReactNode;
  label: string;
  sub: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-2.5 rounded-mk-full px-4 py-2 transition-colors duration-[120ms] ease-mk
                 hover:bg-[rgba(240,233,224,.1)] focus-visible:outline-none focus-visible:ring-2
                 focus-visible:ring-[#8A7F72]"
    >
      <span style={{ color: "var(--mk-accent-300)" }}>{icon}</span>
      <span className="text-left">
        <span className="block text-mk-body font-semibold leading-tight text-[#F0E9E0]">{label}</span>
        <span className="block text-[11px] leading-tight text-[#8A7F72]">{sub}</span>
      </span>
    </button>
  );
}
