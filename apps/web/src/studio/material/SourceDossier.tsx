import { useState } from "react";
import type { Anchor, AnnotateState, StudioEvent } from "@mind-imprint/contracts";
import { Annotate } from "../../primitives/annotate";
import type { SourceFixture } from "./fixtures";

export type SourceDossierProps = {
  sources: SourceFixture[];
  onEvent?: (e: StudioEvent) => void;
  anchors?: Anchor[];
};

type AnnotateSpan = AnnotateState["spans"][number];

// Live card anchors are presentation-only here: they highlight the same spans
// the coach rail is asking about, but the mint never depends on this mapping.
// A source-level anchor (empty block_id + a 0..0 range, e.g. risk_note) has
// nowhere to highlight in the article body, so it's skipped.
function anchorToSpan(anchor: Anchor): AnnotateSpan | null {
  const blockRef = anchor.block_id || undefined;
  const hasRange = anchor.end > anchor.start;
  if (!blockRef && !hasRange) return null;
  return {
    id: anchor.id,
    ...(hasRange ? { range: { start: anchor.start, end: anchor.end } } : {}),
    ...(blockRef ? { block_ref: blockRef } : {}),
    tag: anchor.dimension,
    note: anchor.answer || anchor.question,
    author: anchor.author,
  };
}

const CRAAP_TONE: Record<string, { background: string; color: string }> = {
  可信: { background: "#EAF3EE", color: "#2E7D4F" },
  存疑: { background: "#FBF0E4", color: "#B5762A" },
};

function LockIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <rect x="4" y="11" width="16" height="9" rx="2" stroke="#8A93A6" strokeWidth="2" />
      <path d="M8 11V7a4 4 0 0 1 8 0v4" stroke="#8A93A6" strokeWidth="2" />
    </svg>
  );
}

function BackIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M15 18l-6-6 6-6" stroke="#5C4A8A" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function SourceDossier({ sources, onEvent, anchors }: SourceDossierProps) {
  const [openId, setOpenId] = useState<string | null>(null);
  const [activeSpanId, setActiveSpanId] = useState<string | null>(null);

  const lockedCount = sources.filter((s) => s.locked).length;
  const openSource = openId ? sources.find((s) => s.id === openId) ?? null : null;

  const liveSpans = openSource
    ? (anchors ?? [])
        .filter((a) => a.material_id === openSource.annotate.material_id)
        .map(anchorToSpan)
        .filter((s): s is AnnotateSpan => s !== null)
    : [];
  const annotateState = openSource
    ? liveSpans.length > 0
      ? { ...openSource.annotate, spans: [...openSource.annotate.spans, ...liveSpans] }
      : openSource.annotate
    : null;

  const openSourceView = (source: SourceFixture) => {
    setOpenId(source.id);
    setActiveSpanId(null);
    onEvent?.({ type: "source_opened", surface: "studio", url: source.id, time_spent_s: 0 });
  };

  const backToList = () => {
    setOpenId(null);
    setActiveSpanId(null);
  };

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      {!openSource && (
        <div>
          <div style={{ marginBottom: 14 }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>信源档案 · 已收集 {sources.length} 篇</div>
            <div style={{ fontSize: 12.5, color: "#7A8296", marginTop: 2 }}>
              已锁定 {lockedCount}/{sources.length}
            </div>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {sources.map((source) => {
              const tone = CRAAP_TONE[source.craapLabel] ?? { background: "#F1F2F6", color: "#5A6178" };
              return (
                <button
                  key={source.id}
                  type="button"
                  onClick={() => openSourceView(source)}
                  style={{
                    display: "block",
                    textAlign: "left",
                    background: "#fff",
                    border: "1px solid #E4E6EE",
                    borderRadius: 12,
                    padding: "11px 13px",
                    cursor: "pointer",
                  }}
                >
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    {source.locked && <LockIcon />}
                    <span style={{ fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{source.name}</span>
                    <span
                      style={{
                        marginLeft: "auto",
                        fontSize: 11,
                        fontWeight: 700,
                        borderRadius: 999,
                        padding: "2px 8px",
                        background: tone.background,
                        color: tone.color,
                      }}
                    >
                      {source.craapLabel}
                    </span>
                  </div>
                  <div style={{ fontSize: 12, color: "#8A93A6", marginTop: 4 }}>
                    {source.type} · {source.tier}
                  </div>
                  <div style={{ fontSize: 12.5, color: "#5A6178", marginTop: 6, lineHeight: 1.5 }}>
                    作用与风险：{source.role}
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      )}

      {openSource && (
        <div>
          <button
            type="button"
            onClick={backToList}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 4,
              background: "transparent",
              border: "none",
              padding: 0,
              marginBottom: 12,
              color: "#5C4A8A",
              fontSize: 13,
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            <BackIcon />
            返回信源列表
          </button>

          {openSource.view === "article" && (
            <div>
              <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>{openSource.name}</div>
              {openSource.meta && <div style={{ fontSize: 12, color: "#8A93A6", margin: "2px 0 12px" }}>{openSource.meta}</div>}
              <Annotate
                blocks={openSource.blocks}
                state={annotateState!}
                activeSpanId={activeSpanId}
                onSelectSpan={setActiveSpanId}
              />
              <div style={{ marginTop: 16, fontSize: 12, color: "#A4ABBD" }}>
                点亮的段落是 AI 标出的可疑处——追问会出现在旁边的陪练轨道。
              </div>
            </div>
          )}

          {openSource.view === "summary" && (
            <div>
              <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333" }}>{openSource.name}</div>
              <div style={{ fontSize: 12, color: "#8A93A6", margin: "2px 0 12px" }}>
                {openSource.type} · {openSource.tier}
                {openSource.meta ? ` · ${openSource.meta}` : ""}
              </div>
              <div style={{ fontSize: 13, color: "#5A6178", lineHeight: 1.6 }}>{openSource.role}</div>
              {openSource.takeaway && (
                <div
                  style={{
                    marginTop: 14,
                    background: "#F7F5FB",
                    border: "1px solid #E3DCF2",
                    borderRadius: 12,
                    padding: "13px 15px",
                  }}
                >
                  <div style={{ fontSize: 12, fontWeight: 700, color: "#5C4A8A", marginBottom: 6 }}>一句话摘要</div>
                  <div style={{ fontSize: 13.5, lineHeight: 1.6, color: "#3A4256" }}>{openSource.takeaway}</div>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
