import { useState } from "react";
import type { AssembledFace, AssembledCategory, AssembledDim, ScoredLevel } from "@mind-imprint/contracts";
import { SOLO_LABELS } from "@mind-imprint/contracts";

export function CoverageChip({ scored, na }: { scored: number; na: number }) {
  const text = na > 0 ? `${scored} 项已评 · ${na} 未涉及` : `${scored} 项已评`;
  return (
    <span style={{ flex: "none", fontSize: "12px", fontWeight: 600, color: "#8A92A3" }}>{text}</span>
  );
}

export function DimRow({ dim, last }: { dim: AssembledDim; last: boolean }) {
  const isNA = dim.level === "NA";
  return (
    <div style={{ padding: "11px 0", borderBottom: last ? "none" : "1px solid #F2F3F7" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px", marginBottom: isNA ? 0 : "8px" }}>
        <span style={{ fontSize: "14px", fontWeight: 700, color: isNA ? "#A7AEBC" : "#1C2333" }}>{dim.name}</span>
        {isNA ? (
          <span style={{ fontSize: "12px", fontWeight: 600, color: "#A7AEBC", background: "#F2F3F7", padding: "2px 10px", borderRadius: "999px", flexShrink: 0 }}>本次未涉及</span>
        ) : (
          <span style={{ fontSize: "12.5px", fontWeight: 700, color: "#D98263", background: "#FBEEE7", padding: "2px 10px", borderRadius: "999px", flexShrink: 0 }}>
            {dim.level} · {SOLO_LABELS[dim.level as ScoredLevel]}
          </span>
        )}
      </div>
      {!isNA && (
        <>
          <div style={{ display: "flex", gap: "5px", marginBottom: "7px" }}>
            {[1, 2, 3, 4].map((i) => (
              <span key={i} aria-hidden="true" style={{ flex: "1", height: "6px", borderRadius: "3px", background: i <= LEVEL_NUMBER[dim.level as ScoredLevel] ? "#D98263" : "#ECEEF4" }} />
            ))}
          </div>
          <div style={{ fontSize: "12.5px", color: "#8A92A3" }}>{dim.note}</div>
        </>
      )}
    </div>
  );
}

const LEVEL_NUMBER: Record<ScoredLevel, number> = { L1: 1, L2: 2, L3: 3, L4: 4 };

export function CategorySection({ category }: { category: AssembledCategory }) {
  const [open, setOpen] = useState(false);
  return (
    <div style={{ marginTop: "10px" }}>
      <button type="button" onClick={() => setOpen((v) => !v)}
        style={{ width: "100%", display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px", background: "none", border: "none", padding: "8px 0", cursor: "pointer", fontFamily: "inherit" }}>
        <span style={{ fontSize: "13.5px", fontWeight: 700, color: "#2A3B7A" }}>{category.label}</span>
        <CoverageChip scored={category.scored} na={category.na} />
      </button>
      {open && (
        <div style={{ paddingLeft: "12px", borderLeft: "2px solid #EDEFF6" }}>
          {category.dims.map((d, i) => <DimRow key={d.dimId} dim={d} last={i === category.dims.length - 1} />)}
        </div>
      )}
    </div>
  );
}

export function FaceSection({ face }: { face: AssembledFace }) {
  // Faces start expanded to show their categories + coverage (the headline structure).
  return (
    <div style={{ padding: "14px 0", borderBottom: "1px solid #EEF0F4" }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px" }}>
        <span style={{ fontSize: "16px", fontWeight: 800, color: "#1C2333" }}>
          <span aria-hidden="true">{face.icon}</span>{" "}
          <span>{face.label}</span>
        </span>
        <CoverageChip scored={face.scored} na={face.na} />
      </div>
      {face.categories.map((c) => <CategorySection key={c.id} category={c} />)}
    </div>
  );
}
