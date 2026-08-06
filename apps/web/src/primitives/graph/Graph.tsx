import { useState } from "react";
import type { GraphState } from "@mind-imprint/contracts";
import type { Slot } from "./serialize";

export type LockedSource = { id: string; name: string };

export type GraphProps = {
  slots: Slot[];
  state: GraphState;
  lockedSources: LockedSource[];
  onChange: (state: GraphState) => void;
  onLock: () => void;
  onSkip: () => void;
};

// The backend's completion gate (slotComplete in card_completion.go): a
// trimmed answer of >=12 Unicode runes. Array.from() iterates by code point,
// matching Go's utf8.RuneCountInString for the Chinese/mixed text these
// slots hold (both diverge from astral-plane surrogate pairs the same way).
function runeLength(text: string): number {
  return Array.from(text).length;
}

function textOf(state: GraphState, slotId: string): string {
  return state.nodes.find((n) => n.id === slotId)?.text ?? "";
}

function sourcesOf(state: GraphState, slotId: string): string[] {
  return state.edges.filter((e) => e.type === "cites" && e.from === slotId).map((e) => e.to);
}

function hasText(state: GraphState, slotId: string): boolean {
  return runeLength(textOf(state, slotId).trim()) >= 12;
}

function hasSource(state: GraphState, slotId: string): boolean {
  return sourcesOf(state, slotId).length >= 1;
}

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="var(--mk-accent)" strokeWidth={2.6} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

// Controlled component in the annotate/compare mold: props in, onChange out,
// no internal persistence, no model calls. It renders the design's five-slot
// list and nothing else (StructureView.tsx L64-180 RoleCard is the exact
// visual target) — it does not know the word "Toulmin"; the roles, their
// questions, and their needSrc flags all arrive via `slots` from the card
// spec (schema-driven: a new card = new JSON, no renderer edit). Which row
// is expanded is pure UI navigation state, not part of the persisted
// GraphState, so it stays local.
export function Graph({ slots, state, lockedSources, onChange, onLock, onSkip }: GraphProps) {
  const [activeSlotId, setActiveSlotId] = useState<string | null>(slots[0]?.id ?? null);

  // Mirrors the backend's graph_slots_complete predicate exactly: every slot
  // needs >=12 trimmed runes of text, and every needSrc slot needs >=1
  // selected source. Diverging from this — even by one edge case — reopens
  // the stuck-active-card failure this slice's earlier fixes closed: a
  // student locks here, the server's completion check disagrees, and the
  // card is left active with nothing minted.
  const canLock = slots.every((s) => hasText(state, s.id) && (!s.needSrc || hasSource(state, s.id)));

  function handleTextChange(slotId: string, value: string) {
    const exists = state.nodes.some((n) => n.id === slotId);
    const nodes = exists
      ? state.nodes.map((n) => (n.id === slotId ? { ...n, text: value, author: "student" as const } : n))
      : [...state.nodes, { id: slotId, type: slotId, text: value, author: "student" as const }];
    onChange({ nodes, edges: state.edges });
  }

  function handleSourceToggle(slotId: string, sourceId: string) {
    const selected = state.edges.some((e) => e.type === "cites" && e.from === slotId && e.to === sourceId);
    const edges = selected
      ? state.edges.filter((e) => !(e.type === "cites" && e.from === slotId && e.to === sourceId))
      : [...state.edges, { id: `cites_${slotId}_${sourceId}`, from: slotId, to: sourceId, type: "cites" }];
    onChange({ nodes: state.nodes, edges });
  }

  function handleLock() {
    if (!canLock) return;
    onLock();
  }

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {slots.map((slot) => {
        const active = activeSlotId === slot.id;
        const text = textOf(state, slot.id);
        const done = hasText(state, slot.id) && (!slot.needSrc || hasSource(state, slot.id));
        const statusLabel = done ? "✓ 已写" : slot.needSrc ? "待写 · 需素材" : "待写";
        // done/pending is a genuine 2-state semantic distinction (done vs.
        // still-needed), not an arbitrary category — success/warning tokens,
        // not macarons.
        const statusColor = done ? "var(--mk-success)" : "var(--mk-warning)";
        const statusBg = done ? "var(--mk-success-bg)" : "var(--mk-warning-bg)";
        const selectedSources = sourcesOf(state, slot.id);

        return (
          <div
            key={slot.id}
            style={{
              background: "var(--mk-surface)",
              border: "1px solid " + (active ? "var(--mk-accent-200)" : done ? "var(--mk-success-bg)" : "var(--mk-border)"),
              borderRadius: 14,
              padding: active ? "16px 18px" : "13px 16px",
              marginBottom: 11,
            }}
          >
            <div
              role="button"
              tabIndex={0}
              onClick={() => setActiveSlotId(slot.id)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") setActiveSlotId(slot.id);
              }}
              style={{ display: "flex", alignItems: "center", gap: 8, cursor: active ? "default" : "pointer" }}
            >
              <span style={{ fontSize: 12, fontWeight: 700, color: "var(--mk-secondary)", background: "var(--mk-paper)", padding: "3px 10px", borderRadius: 999 }}>
                {slot.role}
              </span>
              <span style={{ fontSize: 11.5, fontWeight: 700, color: statusColor, background: statusBg, padding: "3px 10px", borderRadius: 999 }}>
                {statusLabel}
              </span>
              {slot.needSrc && (
                <span style={{ marginLeft: "auto", fontSize: 11, fontWeight: 700, color: "var(--mk-info)" }}>{selectedSources.length} 条素材</span>
              )}
            </div>

            {!active && (
              <div style={{ marginTop: 8, fontSize: 13, lineHeight: 1.55, color: text.trim() ? "var(--mk-ink)" : "var(--mk-faint)" }}>
                {text.trim() || "轻点展开，选素材、写这一步"}
              </div>
            )}

            {active && (
              <>
                <div style={{ marginTop: 13, marginBottom: 14, fontSize: 13, lineHeight: 1.6, color: "var(--mk-secondary)", background: "var(--mk-paper)", border: "1px solid var(--mk-border)", borderRadius: 10, padding: "9px 12px" }}>
                  {slot.q}
                </div>

                {slot.needSrc && (
                  <>
                    <div style={{ fontSize: 11.5, fontWeight: 700, color: "var(--mk-muted)", marginBottom: 8 }}>
                      ① 选择相关素材（信源评估里已锁定的）
                    </div>
                    {lockedSources.length === 0 ? (
                      // A slot with no offered sources at all can never satisfy needSrc
                      // (StudioAnnotateCard's dead-button-explanation pattern) — say why
                      // instead of leaving an empty picker unexplained.
                      <div style={{ marginBottom: 14, fontSize: 12, lineHeight: 1.6, color: "var(--mk-warning)" }}>
                        还没有锁定的素材可选——先去信源评估锁一条，再回来接上这一步。
                      </div>
                    ) : (
                      <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 14 }}>
                        {lockedSources.map((src) => {
                          const on = selectedSources.includes(src.id);
                          return (
                            <button
                              key={src.id}
                              type="button"
                              onClick={() => handleSourceToggle(slot.id, src.id)}
                              style={{
                                display: "inline-flex",
                                alignItems: "center",
                                gap: 6,
                                fontSize: 12,
                                fontWeight: 700,
                                cursor: "pointer",
                                padding: "7px 11px",
                                borderRadius: 9,
                                color: on ? "var(--mk-accent)" : "var(--mk-secondary)",
                                background: on ? "var(--mk-accent-50)" : "var(--mk-paper)",
                                border: "1px solid " + (on ? "var(--mk-accent-200)" : "var(--mk-border)"),
                                fontFamily: "inherit",
                              }}
                            >
                              {on && <CheckIcon />}
                              {src.name}
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </>
                )}

                <div style={{ fontSize: 11.5, fontWeight: 700, color: "var(--mk-muted)", margin: "6px 0 6px" }}>
                  {slot.needSrc && "② "}基于素材，把这一步写成句子
                </div>
                <textarea
                  value={text}
                  onChange={(e) => handleTextChange(slot.id, e.target.value)}
                  rows={3}
                  placeholder="用你自己的话写……"
                  style={{
                    width: "100%",
                    border: "1px solid var(--mk-input-border)",
                    borderRadius: 10,
                    padding: "10px 12px",
                    fontSize: 13.5,
                    lineHeight: 1.65,
                    color: "var(--mk-ink)",
                    background: "var(--mk-surface)",
                    outline: "none",
                    resize: "vertical",
                    fontFamily: "inherit",
                  }}
                />
              </>
            )}
          </div>
        );
      })}

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 6 }}>
        <button
          type="button"
          onClick={onSkip}
          style={{ background: "none", border: "none", color: "var(--mk-faint)", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: "6px 0", fontFamily: "inherit" }}
        >
          跳过这张卡
        </button>
        <button
          type="button"
          onClick={handleLock}
          disabled={!canLock}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            fontSize: 13,
            fontWeight: 700,
            cursor: canLock ? "pointer" : "not-allowed",
            opacity: canLock ? 1 : 0.5,
            padding: "9px 15px",
            borderRadius: 10,
            color: "var(--mk-surface)",
            background: "var(--mk-accent)",
            border: "1px solid var(--mk-accent)",
            fontFamily: "inherit",
          }}
        >
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <rect x="5" y="11" width="14" height="10" rx="2" />
            <path d="M8 11V7a4 4 0 018 0v4" />
          </svg>
          全部锁定，完成论证
        </button>
      </div>
    </div>
  );
}
