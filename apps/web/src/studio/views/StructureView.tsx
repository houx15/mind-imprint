import type { CardInstance, TraceEvent } from "@mind-imprint/contracts";
import type { StructureCardFx } from "../state";
import type { LiveCard } from "../CoachRail";
import type { LockedSource } from "../../primitives/graph";
import { StudioToulminCard } from "../StudioToulminCard";

export type StructureViewProps = {
  cards: StructureCardFx[];
  // The active `graph`-primitive (Toulmin) card, when one is open — its
  // interactive 5-slot builder is the point of this pane (unlike compare,
  // whose interactive control lives in the rail). Null on every other view
  // state, in which case the deferred placeholder / role-card path renders.
  toulminCard?: LiveCard | null;
  // CRAAP-locked materials the builder's needSrc slots may cite.
  lockedSources?: LockedSource[];
  onSubmitCard?: (env: CardInstance) => void;
  onSkipCard?: (eventTrace: TraceEvent[]) => void;
};

const WRAP: React.CSSProperties = { flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" };
const COL: React.CSSProperties = { maxWidth: 760, margin: "0 auto" };
// Same neutral deferred-shell placeholder style OnboardingView used before
// N3d gave S1/S2 their own routes (ShellView deleted there).
// This is the legitimate PRE-MINT state: the projection returns an empty
// `structure` slice until the student locks the Toulmin card (7b made the
// slot projection live — this is not a stub). It guards against a
// vacuously-true `cards.every(...)` rendering a false green "门禁通过"
// banner on 0 cards.
const DEFERRED_CARD: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "20px 22px" };

// GateBanner's copy is intentionally NOT byte-identical to the binding
// design (docs/design/思维印记_工作区.dc.html:975) — a deliberate deviation,
// not drift. `allClean` here is computed from only the five Toulmin slots
// (StructureView's own `cards`), but build_argument's gate also requires the
// human item `warrant_quality_spot_check` (packages/contracts/skills/
// writing-project.json), whose producer is the 论证体检 panel rendered BELOW
// this banner. The design predates that spot-check (N3f added it), so its
// original "门禁通过，可以进成稿打磨了" claim is no longer true the moment the
// five slots are done — the station rail and the 论证体检 panel can both still
// show the gate as unfinished. This copy states only what finishing the five
// slots achieves; it never claims the gate passed or that S5 is reachable
// (铁律 2: a factual state statement, never praise or a scolding).
function GateBanner({ allClean }: { allClean: boolean }) {
  if (allClean) {
    return (
      <div
        style={{
          display: "flex",
          alignItems: "flex-start",
          gap: 11,
          background: "#E7F3EE",
          border: "1px solid #CDE7DC",
          borderRadius: 13,
          padding: "13px 16px",
          marginBottom: 20,
        }}
      >
        <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }} aria-hidden="true">
          <path d="M20 6L9 17l-5-5" />
        </svg>
        <div style={{ fontSize: 13, lineHeight: 1.65, color: "#2B4A3E" }}>
          五张卡片都写成了句子、该接素材的都接上了。
        </div>
      </div>
    );
  }
  return (
    <div
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 11,
        background: "#FBEEE7",
        border: "1px solid #F1D6C8",
        borderRadius: 13,
        padding: "13px 16px",
        marginBottom: 20,
      }}
    >
      <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="#C96F4F" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" style={{ flex: "none", marginTop: 1 }} aria-hidden="true">
        <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
        <path d="M12 9v4M12 17h.01" />
      </svg>
      <div style={{ fontSize: 13, lineHeight: 1.65, color: "#8A4A32" }}>
        还有卡片没完成——每张都要写成句子，需要素材的卡片至少选一条。
      </div>
    </div>
  );
}

function RoleCard({ card }: { card: StructureCardFx }) {
  const done = card.status === "done";
  const empty = card.status === "empty";

  const statusLabel = done ? "已完成" : "待开始";
  const statusColor = done ? "#4C9A82" : "#AEB4C2";
  const statusBg = done ? "#E7F3EE" : "#F1F2F5";

  return (
    <div
      style={{
        background: "#fff",
        border: "1px solid #ECEEF3",
        borderRadius: 14,
        padding: "14px 16px",
        marginBottom: 12,
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span
          style={{
            fontSize: 12,
            fontWeight: 700,
            color: "#5B6373",
            background: "#F3F4F7",
            padding: "3px 10px",
            borderRadius: 999,
          }}
        >
          {card.role}
        </span>
        <span
          style={{
            fontSize: 11.5,
            fontWeight: 700,
            color: statusColor,
            background: statusBg,
            padding: "3px 10px",
            borderRadius: 999,
          }}
        >
          {statusLabel}
        </span>
      </div>

      {done && card.preview && (
        <div style={{ marginTop: 8, fontSize: 13, lineHeight: 1.55, color: "#2B3346" }}>{card.preview}</div>
      )}

      {empty && (
        <div style={{ marginTop: 8, fontSize: 13, lineHeight: 1.55, color: "#AEB4C2" }}>还没开始——点开这张卡片开始。</div>
      )}
    </div>
  );
}

export function StructureView({ cards, toulminCard, lockedSources = [], onSubmitCard, onSkipCard }: StructureViewProps) {
  // An open Toulmin card takes over the pane with its live builder — keyed by
  // instance id so a fresh card remounts (re-seeding its working GraphState)
  // rather than inheriting the previous one's slots.
  if (toulminCard) {
    return (
      <div style={WRAP}>
        <div style={COL}>
          <StudioToulminCard
            key={toulminCard.cardInstanceId}
            spec={toulminCard.spec}
            cardInstanceId={toulminCard.cardInstanceId}
            anchors={toulminCard.anchors}
            lockedSources={lockedSources}
            onSubmit={(env) => onSubmitCard?.(env)}
            onSkip={(eventTrace) => onSkipCard?.(eventTrace)}
          />
        </div>
      </div>
    );
  }

  if (cards.length === 0) {
    return (
      <div style={WRAP}>
        <div style={COL}>
          <div style={{ ...DEFERRED_CARD, textAlign: "center", color: "#8A92A3", fontSize: 13.5, fontWeight: 600 }}>
            在这里把论证一步步搭成结构——核完来源后，印记会展开这张工具卡。
          </div>
        </div>
      </div>
    );
  }

  const allClean = cards.every((c) => c.status === "done");

  return (
    <div style={WRAP}>
      <div style={COL}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginBottom: 14 }}>
          <span
            style={{
              fontSize: 12.5,
              fontWeight: 700,
              color: "#2A3B7A",
              background: "#EDEFF9",
              padding: "4px 11px",
              borderRadius: 999,
            }}
          >
            论证构建 · 已完成 {cards.filter((c) => c.status !== "empty").length}/{cards.length}
          </span>
        </div>

        <GateBanner allClean={allClean} />

        <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.6, marginBottom: 16 }}>
          逐张卡片来：点开一张，先选相关素材（你在信源评估里锁定的），再基于素材把这一步写成句子。印记只提问、不代笔。
        </div>

        {cards.map((c) => (
          <RoleCard key={c.id} card={c} />
        ))}
      </div>
    </div>
  );
}
