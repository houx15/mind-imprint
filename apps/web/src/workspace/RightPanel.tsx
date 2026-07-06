import { useRef, useState, useEffect } from "react";
import type { CSSProperties, MouseEvent as ReactMouseEvent } from "react";
import type { Anchor } from "@mind-imprint/contracts";
import type { ProcessNode } from "./processTree";
import { TreeBody } from "./TreeBody";
import { MaterialPane } from "./MaterialPane";

type Tab = "material" | "tree";
const MIN_W = 320;
const MAX_W = 640;

export function RightPanel({ nodes = [], taskId, seedUrl, anchors }: { nodes?: ProcessNode[]; taskId: string; seedUrl: string | null; anchors?: Anchor[] }) {
  const [open, setOpen] = useState(true);
  const [tab, setTab] = useState<Tab>("tree");
  const [width, setWidth] = useState(360);
  const drag = useRef<{ startX: number; startW: number } | null>(null);

  const hasAnchors = !!(anchors && anchors.length > 0);
  useEffect(() => {
    if (hasAnchors) setTab("material");
  }, [hasAnchors]);

  function onDragStart(e: ReactMouseEvent) {
    drag.current = { startX: e.clientX, startW: width };
    const onMove = (ev: MouseEvent) => {
      if (!drag.current) return;
      const next = drag.current.startW - (ev.clientX - drag.current.startX);
      setWidth(Math.max(MIN_W, Math.min(MAX_W, next)));
    };
    const onUp = () => {
      drag.current = null;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }

  if (!open) {
    return (
      <button
        type="button"
        aria-label="展开侧栏"
        onClick={() => setOpen(true)}
        style={{ width: 46, flex: "none", background: "#fff", display: "flex", flexDirection: "column", alignItems: "center", paddingTop: 16, gap: 14, cursor: "pointer", border: "none", borderLeft: "1px solid #EAECF2" }}
      >
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#6B7384" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M15 18l-6-6 6-6" /></svg>
        <span style={{ writingMode: "vertical-rl", fontSize: "12.5px", fontWeight: 700, color: "#6B7384", letterSpacing: ".08em" }}>材料 / 过程树</span>
      </button>
    );
  }

  const segBtn = (active: boolean): CSSProperties => ({
    flex: 1, display: "flex", alignItems: "center", justifyContent: "center", gap: 6,
    padding: "6px 10px", borderRadius: 7, fontSize: 12.5, fontWeight: 700, cursor: "pointer",
    fontFamily: "inherit", border: "none",
    background: active ? "#fff" : "transparent", color: active ? "#2A3B7A" : "#8A92A3",
    boxShadow: active ? "0 1px 2px rgba(20,30,60,.08)" : "none",
  });

  return (
    <>
      <div onMouseDown={onDragStart} style={{ width: 7, flex: "none", cursor: "col-resize", display: "flex", alignItems: "center", justifyContent: "center", background: "transparent" }}>
        <div style={{ width: 3, height: 34, borderRadius: 2, background: "#D6DAE4" }} />
      </div>
      <div style={{ width: `${width}px`, flex: "none", background: "#fff", borderLeft: "1px solid #EAECF2", display: "flex", flexDirection: "column" }}>
        <div style={{ height: 54, flex: "none", display: "flex", alignItems: "center", gap: 10, padding: "0 14px 0 16px", borderBottom: "1px solid #EFF0F5" }}>
          <div style={{ flex: 1, display: "flex", gap: 3, background: "#F1F2F5", borderRadius: 9, padding: 3 }}>
            <button type="button" role="tab" aria-selected={tab === "material"} onClick={() => setTab("material")} style={segBtn(tab === "material")}>材料</button>
            <button type="button" role="tab" aria-selected={tab === "tree"} onClick={() => setTab("tree")} style={segBtn(tab === "tree")}>过程树</button>
          </div>
          <button type="button" aria-label="折叠侧栏" onClick={() => setOpen(false)} style={{ flex: "none", cursor: "pointer", color: "#9AA1B0", padding: 6, borderRadius: 7, display: "flex", background: "none", border: "none" }}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M9 18l6-6-6-6" /></svg>
          </button>
        </div>
        {tab === "tree" ? <TreeBody nodes={nodes} /> : <MaterialPane taskId={taskId} seedUrl={seedUrl} anchors={anchors} />}
      </div>
    </>
  );
}
