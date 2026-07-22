import { useState } from "react";
import type { PerspectivesFx } from "../state";
import type { MaterialSource } from "@mind-imprint/contracts";
import type { AddMaterialBody } from "../../api/materials";
import { SourceDossier } from "../material/SourceDossier";

// N3d Task 11: S2 视角与素材.
//
// Binding design: docs/design/思维印记_工作区.dc.html:930-958 (markup) +
// :2158-2162 (the JS that computes its levels + coverage rule). Copy,
// colours, radii, spacing below are that design verbatim, with ONE
// documented departure at the 素材 block (see the comment there).
export type PerspectivesViewProps = {
  data: PerspectivesFx;
  material: MaterialSource[];
  onSubmit?: (body: { perspectives: { text: string; level: string }[] }) => Promise<void>;
  onAddSource?: (body: AddMaterialBody) => Promise<void>;
  addSourceError?: string;
  onOpenLogged?: (materialId: string, timeSpentS: number) => void;
  onAttestSourcesPerPerspective?: (confirmed: boolean) => void;
};

// The binding design's own three levels and colours (dc.html:2158).
const LEVELS = [
  { key: "national", label: "国家视角", color: "#4C9A82" },
  { key: "global_for", label: "全球视角 · 支持", color: "#2A3B7A" },
  { key: "global_against", label: "全球视角 · 反方", color: "#C96F4F" },
] as const;

const WRAP: React.CSSProperties = { flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" };
const COL: React.CSSProperties = { maxWidth: 720, margin: "0 auto" };
const CARD: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "15px 17px", marginBottom: 12 };

const ADD_LINK: React.CSSProperties = {
  display: "inline-flex",
  alignItems: "center",
  gap: 7,
  fontSize: 13,
  fontWeight: 700,
  color: "#2A3B7A",
  background: "none",
  border: "none",
  padding: "4px 2px",
  cursor: "pointer",
};

const REMOVE_BTN: React.CSSProperties = {
  marginLeft: "auto",
  flex: "none",
  width: 24,
  height: 24,
  border: "none",
  background: "none",
  borderRadius: 7,
  display: "flex",
  alignItems: "center",
  justifyContent: "center",
  cursor: "pointer",
  color: "#C2C8D6",
  padding: 0,
};

function PlusIcon({ size = 15 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function XIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M18 6L6 18M6 6l12 12" />
    </svg>
  );
}

type DraftRow = { text: string; level: string; editable: boolean };

export function PerspectivesView({ data, material, onSubmit, onAddSource, addSourceError, onOpenLogged, onAttestSourcesPerPerspective }: PerspectivesViewProps) {
  const [rows, setRows] = useState<DraftRow[]>(data.rows.map((r) => ({ ...r })));
  const [submitting, setSubmitting] = useState(false);
  const [attestConfirmed, setAttestConfirmed] = useState(data.sourcesPerPerspective);

  // dc.html:2160 — coverage requires national AND at least one non-national.
  // A card-minted row carries level === "" (no level at all) — it must not
  // count toward EITHER side of this local badge, or an empty string would
  // silently satisfy "non-national" with nothing behind it. The backend's
  // own evaluate_perspectives gate is the real arbiter of coverage; this
  // chip is cosmetic.
  const hasNational = rows.some((r) => r.level === "national");
  const hasGlobal = rows.some((r) => r.level !== "national" && r.level !== "");
  const covered = hasNational && hasGlobal;

  // 铁律 2 (不操纵): the save button is never disabled for incompleteness —
  // only while a submit is actually in flight.
  const canSubmit = !!onSubmit && !submitting;

  // Card-minted rows are real perspectives too (they count toward the
  // station's gate) — so they count here as well, not just her own
  // hand-written rows. Minor 4 (whole-branch review): a blank row (nothing
  // typed yet) is not a perspective — counting it let two empty rows unlock
  // the sources-per-perspective confirm with nothing behind it.
  const perspectiveCount = rows.filter((r) => r.text.trim() !== "").length;
  // The one explicit attestation in the slice can only be used once there is
  // something real to attest to — 2 perspectives and at least 1 source.
  // Disabled WITH the reason visible (never hidden): an offer is never a wall.
  const attestUnavailableReason =
    perspectiveCount < 2 ? "至少写两条视角后才能确认" : material.length === 0 ? "至少添加一条素材后才能确认" : null;
  const attestDisabled = attestUnavailableReason !== null;

  function updateText(i: number, value: string) {
    setRows((prev) => prev.map((r, idx) => (idx === i ? { ...r, text: value } : r)));
  }
  function setLevel(i: number, level: string) {
    setRows((prev) => prev.map((r, idx) => (idx === i ? { ...r, level } : r)));
  }
  function addRow() {
    // Minor 3 (whole-branch review): match the binding design's own addPersp
    // (dc.html:1891), which defaults to global_for, not national. This isn't
    // just style — a row she never levelled would otherwise persist as 国家
    // 视角, a claim she never made, and it would feed the coverage chip.
    setRows((prev) => [...prev, { text: "", level: "global_for", editable: true }]);
  }
  function removeRow(i: number) {
    setRows((prev) => prev.filter((_, idx) => idx !== i));
  }

  async function handleSubmit() {
    if (!canSubmit || !onSubmit) return;
    setSubmitting(true);
    try {
      // Only editable rows are hers to save — a card-minted row (editable
      // false) has no level and is never sent back up.
      const perspectives = rows.filter((r) => r.editable).map((r) => ({ text: r.text, level: r.level }));
      await onSubmit({ perspectives });
      // I4 re-fix (whole-branch review): the server silently drops any row
      // whose trimmed `text` is blank. The earlier fix filtered the dropped
      // rows out of local state on a successful submit — harmless here today
      // (a dropped row is blank by definition, nothing is lost) but the same
      // shape as FramingView's regression, so it's fixed the same way for one
      // consistent rule: nothing is deleted. Every row stays on screen; an
      // unsaved editable row gets an inline note instead (rendered below,
      // under its textarea).
    } finally {
      setSubmitting(false);
    }
  }

  function handleAttestToggle() {
    if (attestDisabled) return;
    const next = !attestConfirmed;
    setAttestConfirmed(next);
    onAttestSourcesPerPerspective?.(next);
  }

  return (
    <div style={WRAP}>
      <div style={COL}>
        {/* dc.html:933-936 */}
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginBottom: 6 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333" }}>先摆出不同视角，再去找素材</div>
          {covered && (
            <span style={{ fontSize: 11, fontWeight: 700, color: "#4C9A82", background: "#E7F3EE", padding: "3px 10px", borderRadius: 999 }}>
              ✓ 已覆盖 本地/国家 + 全球
            </span>
          )}
        </div>
        {/* dc.html:937 */}
        <div style={{ fontSize: 12, color: "#8A92A3", marginBottom: 14 }}>
          每条视角都由你自己写、自己标层级。0457 要求至少覆盖 本地/国家 与 全球 两层。印记只追问你的检索方向，不替你找来源。
        </div>

        {/* dc.html:938-950 */}
        {rows.map((row, i) => (
          <div key={i} style={CARD}>
            <div style={{ display: "flex", alignItems: "center", gap: 7, marginBottom: 10 }}>
              {row.editable ? (
                <>
                  {LEVELS.map((lv) => {
                    const on = row.level === lv.key;
                    return (
                      <button
                        key={lv.key}
                        type="button"
                        onClick={() => setLevel(i, lv.key)}
                        style={{
                          fontSize: 10.5,
                          fontWeight: 700,
                          cursor: "pointer",
                          padding: "3px 9px",
                          borderRadius: 999,
                          border: "none",
                          fontFamily: "inherit",
                          color: on ? "#fff" : "#8A92A3",
                          background: on ? lv.color : "#F1F2F5",
                        }}
                      >
                        {lv.label}
                      </button>
                    );
                  })}
                  <button type="button" aria-label="删除这条视角" onClick={() => removeRow(i)} style={REMOVE_BTN}>
                    <XIcon />
                  </button>
                </>
              ) : (
                // A row with editable === false was minted by the
                // perspective-matrix tool card, not by this view. It carries
                // no level — rewriting it here would fight the card that
                // minted it, and the backend's write is deliberately scoped
                // so it can never touch these rows. It still counts toward
                // the station's gate, just not toward this view's submit body.
                <span style={{ fontSize: 10.5, fontWeight: 700, color: "#8A92A3", background: "#F1F2F5", padding: "3px 9px", borderRadius: 999 }}>
                  来自工具卡
                </span>
              )}
            </div>
            {row.editable ? (
              <>
                <textarea
                  aria-label="视角"
                  value={row.text}
                  onChange={(e) => updateText(i, e.target.value)}
                  rows={2}
                  placeholder="写一条视角：谁、从什么立场、看到什么……"
                  style={{
                    width: "100%",
                    border: "1px solid #E1E4ED",
                    borderRadius: 10,
                    padding: "9px 12px",
                    fontSize: 13.5,
                    lineHeight: 1.6,
                    color: "#1C2333",
                    background: "#fff",
                    outline: "none",
                    resize: "vertical",
                    fontFamily: "inherit",
                  }}
                />
                {/* I4 re-fix (whole-branch review): the backend drops this
                    row on save when its trimmed text is blank — the row
                    stays on screen either way, so this note is the only
                    thing that tells her it isn't saved. Quiet inline hint,
                    same muted grey as this view's other sub-lines. */}
                {row.text.trim() === "" && (
                  <div style={{ fontSize: 11, color: "#9AA1B0", marginTop: 5 }}>这条还是空的，尚未保存</div>
                )}
              </>
            ) : (
              <div style={{ fontSize: 13.5, lineHeight: 1.6, color: "#1C2333", padding: "9px 12px", background: "#F8F9FB", borderRadius: 10 }}>
                {row.text}
              </div>
            )}
          </div>
        ))}

        <button type="button" onClick={addRow} style={ADD_LINK}>
          <PlusIcon />
          添加一条视角
        </button>

        <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 14, marginBottom: 26 }}>
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit}
            style={{
              fontSize: 12.5,
              fontWeight: 700,
              color: "#fff",
              background: canSubmit ? "#2A3B7A" : "#B7BBCB",
              border: "none",
              borderRadius: 10,
              padding: "8px 16px",
              cursor: canSubmit ? "pointer" : "not-allowed",
              fontFamily: "inherit",
            }}
          >
            {submitting ? "记录中…" : "记下我的视角"}
          </button>
        </div>

        {/* Spec §6.3: the binding design's S2 has no source affordance at all, yet the
            station is 视角与素材 and its own gate demands sources per perspective — as
            drawn the gate is unreachable. This reuses 6b's ingestion path (RL-2: S2
            never ingests on its own). The full dossier stays at S3.

            C1 fix (whole-branch review): a real "open" is what the station's
            own recon_logged gate item is honestly about (attestReconLogged
            fires from logSourceOpen, called only when a source is opened AND
            closed) — adding a source is not opening one. SourceDossier is the
            ONE component that both lists sources and calls onOpenLogged, so
            it renders here too, just without the anchor/locate machinery S3's
            call site needs (no live card is ever active on this station's own
            screen, and there is no "go find this sentence" locate request to
            honor here). */}
        <div style={{ borderTop: "1px solid #EAECF2", paddingTop: 18 }}>
          <div style={{ fontSize: 13, fontWeight: 800, color: "#1C2333", marginBottom: 8 }}>素材</div>
          <SourceDossier
            sources={material}
            anchors={[]}
            onOpenLogged={onOpenLogged}
            onAddSource={onAddSource}
            addSourceError={addSourceError}
          />

          {/* The one explicit attestation in this slice. 铁律 2: reversible
              (unchecking clears it), and when it can't be used yet, disabled
              WITH the reason visible — never hidden. */}
          <label
            style={{
              display: "flex",
              alignItems: "flex-start",
              gap: 9,
              marginTop: 6,
              fontSize: 12.5,
              color: attestDisabled ? "#B7BBCB" : "#3A4256",
              cursor: attestDisabled ? "default" : "pointer",
            }}
          >
            <input
              type="checkbox"
              checked={attestConfirmed}
              disabled={attestDisabled}
              onChange={handleAttestToggle}
              style={{ marginTop: 2 }}
            />
            <span>
              每条视角我都找到了至少一条素材
              {attestUnavailableReason && (
                <span style={{ display: "block", fontSize: 11, color: "#9AA1B0", marginTop: 2 }}>{attestUnavailableReason}</span>
              )}
            </span>
          </label>
        </div>
      </div>
    </div>
  );
}
