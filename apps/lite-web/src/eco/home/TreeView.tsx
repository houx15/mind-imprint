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
import { STUDENT } from "../data/library";
import { go } from "../route";
import type { FieldId, Keyword } from "../data/types";
import { Sys, cx } from "../ui";
import { useFitScale } from "../fit";
import { ViewSwitch } from "./ViewSwitch";
import { KeywordDrawer } from "./KeywordDrawer";

/**
 * 我的家 · the growing tree.
 *
 * ## The visual ruling (2026-08-31)
 * The first build drew a LITERAL tree — brown trunk, green candy pills — and
 * it read as a children's illustration. It is now **an abstract structure that
 * sits BEHIND the data**: the tree is a semi-transparent diagram (gradient
 * hairlines that fade toward the tips, a mirrored root system, faint growth
 * rings) and each keyword is a **technical annotation** — a diamond marker
 * exactly on the branch, a leader line, and a square-cornered callout carrying
 * a monospace readout. Structure recedes; her words are the foreground.
 *
 * The growth rings deliberately rhyme with 世界's orbit rings: outward,
 * orbits; inward, rings. Same instrument, two directions.
 *
 * ## What the tree claims, and how it earns it
 * *This is a model of you, built from what you actually did.* Two things make
 * that true rather than decorative — every node opens a drawer listing its
 * REAL sources (`KeywordDrawer`), and the growth replay shows the same tree at
 * four earlier moments, so she can see it was grown and not authored.
 *
 * ## Geometry
 * A fixed 1000×780 aspect box, so the SVG `viewBox` maps 1:1 onto percentage
 * positions — which is what lets the annotations be real HTML buttons
 * (selectable text, hover, focus ring, no `<foreignObject>`) sitting exactly
 * on their branches. `pointOnBranch` in `data/tree.ts` is the single source of
 * both the curve and the marker positions.
 *
 * ## No gamification
 * Strength is an instrument reading (`SRC 3 · STR 4/5`), never a level, a
 * streak or a badge. Nothing here rewards frequency (铁律②).
 */
export function TreeView() {
  const { state, setGrowth, openCoach } = useEco();
  const [openId, setOpenId] = useState<string | null>(null);
  const [hoverField, setHoverField] = useState<FieldId | null>(null);

  const stageRef = useRef<HTMLDivElement>(null);
  // Callouts are fixed pixel size on a stage that scales with the viewport.
  // Without this they collide on any short window — which is what made this
  // screen unreadable at 700px tall.
  const scale = useFitScale(stageRef, 792, 0.74);

  const stop = state.growth;
  const visible = useMemo(() => KEYWORDS.filter((k) => k.bornAt <= stop), [stop]);
  const maturity = 0.42 + (stop / 3) * 0.58;
  // Words she kept from 世界 are collected NOW, so they exist only at the
  // present stop. Showing them while the replay is rewound put a keyword on a
  // tree that also claimed to have zero keywords.
  const grown = stop === GROWTH_STOPS.length - 1 ? state.grownKeywords : [];
  const shining = visible.filter((k) => k.shining);
  const openKw = openId ? (keywordById(openId) ?? null) : null;

  // ONE count, used by the header, the field index and the caption. Three
  // places computing it independently is how the screen came to show 17 / 16 / 0
  // at the same time.
  const countFor = (fieldId?: FieldId) =>
    (fieldId ? visible.filter((k) => k.field === fieldId) : visible).length +
    (fieldId ? grown.filter((g) => g.field === fieldId) : grown).length;
  const total = countFor();

  return (
    <div className="relative min-h-full">
      <header className="flex flex-wrap items-center justify-between gap-4 px-7 pt-6">
        <div className="min-w-0">
          <Sys>我的树 · KEYWORD MODEL</Sys>
          <h1 className="mt-1 text-mk-h1 text-mk-ink">
            {STUDENT.name}
            <span className="ml-2 text-mk-body font-normal text-mk-muted">{STUDENT.grade}</span>
          </h1>
        </div>
        <ViewSwitch view="tree" />
        <div className="flex items-center gap-5">
          <span className="text-right">
            <Sys>关键词</Sys>
            <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">{total}</span>
          </span>
          <span className="h-8 w-px" style={{ background: "var(--mk-border)" }} />
          <span className="text-right">
            <Sys>高光时刻</Sys>
            <span className="block font-mono text-mk-h2 tabular-nums text-mk-ink">{shining.length}</span>
          </span>
        </div>
      </header>

      {/* ── growth replay ─────────────────────────────────────────────── */}
      <div className="mt-5 flex flex-wrap items-center gap-4 px-7">
        <Sys>成长回放</Sys>
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
                      background: active ? "var(--mk-accent)" : past ? "var(--mk-secondary)" : "transparent",
                      border: active || past ? "none" : "1px solid var(--mk-input-border)",
                      transform: active ? "rotate(45deg) scale(1.5)" : "rotate(45deg)",
                    }}
                  />
                  <span
                    className={cx(
                      "whitespace-nowrap text-mk-small transition-colors duration-[160ms]",
                      active ? "font-semibold text-mk-ink" : "text-mk-muted",
                    )}
                  >
                    {s.label}
                  </span>
                </button>
                {i < GROWTH_STOPS.length - 1 ? (
                  <span className="h-px w-8 shrink-0" style={{ background: "var(--mk-border)" }} />
                ) : null}
              </div>
            );
          })}
        </div>
        <p className="text-mk-small text-mk-muted">
          {stop === GROWTH_STOPS.length - 1
            ? `现在：${total} 个词，${shining.length} 个高光时刻。`
            : `${GROWTH_STOPS[stop]?.sub ?? ""} —— 那时候树上有 ${total} 个词。`}
        </p>
      </div>

      {/* ── field index (chip row, below xl) ───────────────────────────────────────────────── */}
      {/* Below xl there is no room for the floating column, so the same index
          becomes a chip row under the replay — it is the key to the colour
          coding and used to vanish entirely on a laptop. */}
      <div className="mb-2 flex flex-wrap gap-1.5 px-7 xl:hidden">
        {FIELDS.map((f, i) => (
          <button
            key={f.id}
            type="button"
            onMouseEnter={() => setHoverField(f.id)}
            onMouseLeave={() => setHoverField(null)}
            onFocus={() => setHoverField(f.id)}
            onBlur={() => setHoverField(null)}
            className="inline-flex items-center gap-1.5 rounded-mk-full border border-mk-border bg-mk-surface
                       px-2.5 py-1 text-mk-small text-mk-secondary transition-colors duration-[120ms]
                       hover:border-mk-accent-200 focus-visible:outline-none focus-visible:ring-2
                       focus-visible:ring-mk-accent-200"
          >
            <span className="h-1.5 w-1.5 rotate-45" style={{ background: f.hue }} />
            <span className="eco-mono text-mk-faint">{String(i + 1).padStart(2, "0")}</span>
            {f.label}
            <span className="font-mono text-[11px] tabular-nums text-mk-faint">{countFor(f.id)}</span>
          </button>
        ))}
      </div>


      {/* ── the structure ─────────────────────────────────────────────── */}
      <div className="px-7 pb-28 pt-2">
        {/* Height-first, width derived. The model has to fit ON SCREEN: a
            picture of yourself you must scroll to see is a document, not a
            picture. */}
        <div
          ref={stageRef}
          className="relative mx-auto"
          style={{
            aspectRatio: "1000 / 780",
            height: "min(792px, calc(100vh - 268px))",
            width: "auto",
            maxWidth: "1120px",
          }}
        >
          <svg
            viewBox="0 0 1000 780"
            className="absolute inset-0 h-full w-full"
            aria-hidden
            preserveAspectRatio="xMidYMid meet"
          >
            <defs>
              {/* Every branch fades toward its tip: the structure is most
                  certain near the trunk and dissolves where it is still
                  growing. That is the honest picture of a keyword model. */}
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
                    <stop offset="0%" stopColor={f.hue} stopOpacity="0.9" />
                    <stop offset="62%" stopColor={f.hue} stopOpacity="0.62" />
                    <stop offset="100%" stopColor={f.hue} stopOpacity="0.16" />
                  </linearGradient>
                );
              })}
              <linearGradient id="eco-spine" x1="0" y1="1" x2="0" y2="0">
                <stop offset="0%" stopColor="var(--mk-ink)" stopOpacity="0.4" />
                <stop offset="70%" stopColor="var(--mk-ink)" stopOpacity="0.22" />
                <stop offset="100%" stopColor="var(--mk-ink)" stopOpacity="0.04" />
              </linearGradient>
              <radialGradient id="eco-canopy" cx="50%" cy="46%" r="52%">
                <stop offset="0%" stopColor="var(--mk-accent-200)" stopOpacity="0.15" />
                <stop offset="100%" stopColor="var(--mk-accent-200)" stopOpacity="0" />
              </radialGradient>
            </defs>

            {/* canopy glow — the only soft thing on the page */}
            <ellipse cx="500" cy="360" rx="470" ry="300" fill="url(#eco-canopy)" />

            {/* growth rings, centred on the origin */}
            {[210, 340, 470, 600].map((r, i) => (
              <circle
                key={r}
                cx="500"
                cy="742"
                r={r}
                fill="none"
                stroke="var(--mk-ink)"
                strokeOpacity={0.07 - i * 0.01}
                strokeDasharray="2 7"
              />
            ))}

            {/* horizon + ticks */}
            <line x1="90" y1="742" x2="910" y2="742" stroke="var(--mk-ink)" strokeOpacity="0.13" />
            {[...Array(21)].map((_, i) => (
              <line
                key={i}
                x1={100 + i * 40}
                y1="742"
                x2={100 + i * 40}
                y2={i % 5 === 0 ? 752 : 747}
                stroke="var(--mk-ink)"
                strokeOpacity="0.11"
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
                  strokeOpacity="0.22"
                  strokeWidth="1.5"
                  strokeDasharray="3 5"
                />
              );
            })}

            {/* the spine — a gradient wash with a hairline through it, not a
                brown trunk */}
            <path
              d="M 500 742 C 496 640, 504 560, 500 470 C 497 400, 503 360, 500 296"
              stroke="url(#eco-spine)"
              strokeWidth="7"
              strokeLinecap="round"
              fill="none"
            />
            <path
              d="M 500 742 C 496 640, 504 560, 500 470 C 497 400, 503 360, 500 296"
              stroke="var(--mk-ink)"
              strokeOpacity="0.2"
              strokeWidth="1"
              fill="none"
            />

            {/* branches */}
            {FIELDS.map((f, idx) => {
              const on = hoverField === null || hoverField === f.id;
              const tip = pointOnBranch(f.id, maturity, 0);
              const left = tip.x < 500;
              return (
                <g key={f.id} style={{ transition: "opacity 200ms", opacity: on ? 1 : 0.22 }}>
                  <path
                    d={branchPath(f.id)}
                    stroke={`url(#eco-br-${f.id})`}
                    strokeWidth={hoverField === f.id ? 5 : 3.2}
                    strokeLinecap="round"
                    fill="none"
                    pathLength={1}
                    strokeDasharray={`${maturity} 1`}
                    style={{ transition: "stroke-width 200ms" }}
                  />
                  <circle cx={tip.x} cy={tip.y} r="3" fill={f.hue} fillOpacity="0.55" />
                  <text
                    x={tip.x + (left ? -20 : 20)}
                    y={tip.y - 34}
                    textAnchor={left ? "end" : "start"}
                    fontSize="9.5"
                    fontFamily="ui-monospace, SF Mono, monospace"
                    letterSpacing="1.4"
                    fill="var(--mk-faint)"
                  >
                    {`FIELD ${String(idx + 1).padStart(2, "0")}`}
                  </text>
                  <text
                    x={tip.x + (left ? -20 : 20)}
                    y={tip.y - 19}
                    textAnchor={left ? "end" : "start"}
                    fontSize="12.5"
                    fontWeight="600"
                    fill="var(--mk-secondary)"
                  >
                    {f.label}
                  </text>
                </g>
              );
            })}

            {/* the origin */}
            <circle cx="500" cy="742" r="17" fill="var(--mk-paper)" stroke="var(--mk-ink)" strokeOpacity="0.18" />
            <circle cx="500" cy="742" r="3.5" fill="var(--mk-accent)" />
            <text
              x="500"
              y="712"
              textAnchor="middle"
              fontSize="9.5"
              fontFamily="ui-monospace, SF Mono, monospace"
              letterSpacing="1.4"
              fill="var(--mk-faint)"
            >
              ORIGIN · 我
            </text>
          </svg>

          {/* keyword annotations */}
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
              <Annotation
                key={g.id}
                x={p.x}
                y={p.y + i * 34}
                hue={f.hue}
                meta="NEW · 来自世界"
                name={g.text}
                fresh
                scale={scale}
              />
            );
          })}
        </div>
      </div>

      {/* ── field index (floating column, xl and up) ─────────────────── */}
      <div className="pointer-events-auto absolute left-7 top-[188px] hidden w-[176px] xl:block">
        <Sys className="mb-2 block">主枝 · INDEX</Sys>
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
                             duration-[120ms] hover:bg-mk-surface focus-visible:outline-none
                             focus-visible:ring-2 focus-visible:ring-mk-accent-200"
                  style={{ borderColor: "color-mix(in srgb, var(--mk-ink) 8%, transparent)" }}
                >
                  <span className="h-1.5 w-1.5 shrink-0 rotate-45" style={{ background: f.hue }} />
                  <span className="eco-mono shrink-0 text-mk-faint">{String(i + 1).padStart(2, "0")}</span>
                  <span className="min-w-0 flex-1 truncate text-mk-small text-mk-secondary">{f.label}</span>
                  <span className="font-mono text-[11px] tabular-nums text-mk-faint">{n}</span>
                </button>
              </li>
            );
          })}
        </ul>
      </div>

      {/* ── action dock ───────────────────────────────────────────────── */}
      <div className="fixed bottom-6 left-1/2 z-30 -translate-x-1/2">
        <div
          className="flex items-center gap-1 rounded-mk-full p-1.5 shadow-mk-lg"
          style={{ background: "var(--mk-surface)", border: "1px solid var(--mk-border)" }}
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
  // stops the branch is short, so the annotation slides in with it.
  const p = pointOnBranch(kw.field, Math.min(kw.at.t, maturity), kw.at.spread);

  return (
    <Annotation
      x={p.x}
      y={p.y}
      hue={f.hue}
      meta={`SRC ${kw.sources.length} · STR ${kw.strength}/5`}
      name={kw.text}
      shining={Boolean(kw.shining)}
      strong={kw.strength >= 4}
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
 * The annotation primitive — a diamond marker on the curve, a leader, and a
 * square-cornered callout. Shared by tree keywords and by words she kept from
 * 世界, so a grown keyword looks like a member of the model rather than a
 * sticker on top of it.
 */
function Annotation({
  x,
  y,
  hue,
  meta,
  name,
  shining = false,
  strong = false,
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
  shining?: boolean;
  strong?: boolean;
  fresh?: boolean;
  dim?: boolean;
  /** Stage-fit factor — see `useFitScale`. Shrinks the callout so a short
   *  window does not turn the model into overlapping text. */
  scale?: number;
  delay?: number;
  title?: string;
  onClick?: () => void;
  onHover?: (on: boolean) => void;
}) {
  const [hot, setHot] = useState(false);
  // Callouts sit on the outboard side so leader lines never cross the
  // structure they annotate.
  const left = x < 500;

  return (
    <div
      className="eco-node absolute"
      style={{
        left: `${x / 10}%`,
        top: `${y / 7.8}%`,
        width: 0,
        height: 0,
        animationDelay: `${delay}ms`,
        opacity: dim ? 0.2 : 1,
        transition: "opacity 200ms cubic-bezier(.2,0,0,1)",
        zIndex: hot ? 20 : 6,
      }}
    >
      <span
        aria-hidden
        className="absolute"
        style={{
          left: -4,
          top: -4,
          width: 8,
          height: 8,
          transform: "rotate(45deg)",
          background: hot || strong ? hue : "var(--mk-paper)",
          border: `1.5px solid ${hue}`,
        }}
      />
      <span
        aria-hidden
        className="absolute"
        style={{
          top: -0.5,
          [left ? "right" : "left"]: 6,
          width: 14 * scale,
          height: 1,
          background: `color-mix(in srgb, ${hue} 70%, transparent)`,
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
          "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200",
          onClick ? "cursor-pointer" : "cursor-default",
        )}
        style={{
          padding: `${6 * scale}px ${10 * scale}px`,
          top: -19 * scale,
          [left ? "right" : "left"]: 20 * scale,
          // Square corners, one coloured edge on the leader side. An
          // annotation, not a bubble.
          borderRadius: 2,
          background: hot ? "var(--mk-surface)" : "color-mix(in srgb, var(--mk-paper) 78%, transparent)",
          boxShadow: hot ? "var(--mk-shadow-md)" : "none",
          [left ? "borderRight" : "borderLeft"]: `2px solid ${hue}`,
          backdropFilter: "blur(2px)",
        }}
      >
        <span
          className="eco-mono block"
          style={{
            color: fresh || shining ? hue : "var(--mk-faint)",
            fontSize: 10 * scale,
            letterSpacing: `${0.14 * scale}em`,
          }}
        >
          {shining ? "✳ " : ""}
          {meta}
        </span>
        <span
          className="mt-0.5 block leading-tight text-mk-ink"
          style={{ fontSize: (strong ? 15 : 13.5) * scale, fontWeight: strong ? 700 : 500 }}
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
                 hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
    >
      <span className="text-mk-accent-700">{icon}</span>
      <span className="text-left">
        <span className="block text-mk-body font-semibold leading-tight text-mk-ink">{label}</span>
        <span className="block text-[11px] leading-tight text-mk-muted">{sub}</span>
      </span>
    </button>
  );
}
