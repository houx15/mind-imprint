import type { AnnotateState } from "@mind-imprint/contracts";
import { segmentBlock } from "./segment";

export type AnnotateProps = {
  blocks: { id: string; text: string }[];
  state: AnnotateState;
  activeSpanId: string | null;
  onSelectSpan: (id: string | null) => void;
};

const AUTHOR_MARK_STYLE: Record<"ai" | "student" | "imported", { background: string; border: string }> = {
  ai: { background: "#F0ECF8", border: "#7C6BB5" },
  student: { background: "#EAF3EE", border: "#4E9A6F" },
  imported: { background: "#F3F0E8", border: "#B79A4C" },
};

export function Annotate({ blocks, state, activeSpanId, onSelectSpan }: AnnotateProps) {
  const activeSpan = activeSpanId ? state.spans.find((s) => s.id === activeSpanId) ?? null : null;

  return (
    <div style={{ fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif" }}>
      <div>
        {blocks.map((block) => {
          const runs = segmentBlock(block.id, block.text, state.spans);
          return (
            <p key={block.id} style={{ fontSize: 15, lineHeight: 2.1, color: "#2B3346", margin: "0 0 14px" }}>
              {runs.map((run, i) => {
                if (run.spanId == null) {
                  return <span key={i}>{run.text}</span>;
                }
                const tone = AUTHOR_MARK_STYLE[run.author ?? "ai"];
                const active = run.spanId === activeSpanId;
                return (
                  <mark
                    key={i}
                    onClick={() => onSelectSpan(run.spanId)}
                    style={{
                      background: tone.background,
                      color: "#1C2333",
                      borderBottom: `2px solid ${tone.border}`,
                      borderRadius: 3,
                      padding: "1px 2px",
                      cursor: "pointer",
                      outline: active ? `2px solid ${tone.border}` : "none",
                      outlineOffset: 1,
                    }}
                  >
                    {run.text}
                  </mark>
                );
              })}
            </p>
          );
        })}
      </div>

      {activeSpan && (
        <div style={{ marginTop: 14, background: "#F7F5FB", border: "1px solid #E3DCF2", borderRadius: 12, padding: "13px 15px" }}>
          <div style={{ fontSize: 12, fontWeight: 700, color: "#5C4A8A", marginBottom: 6 }}>{activeSpan.tag}</div>
          <div style={{ fontSize: 13.5, lineHeight: 1.6, color: "#3A4256" }}>{activeSpan.note}</div>
        </div>
      )}
    </div>
  );
}
