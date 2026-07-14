import { useEffect, useState } from "react";
import type { Anchor, CardInstance, CardSpec, MaterialSource, TraceEvent } from "@mind-imprint/contracts";
import { newEnvelope } from "../cards/envelopeReducer";

export type StudioCompareCardProps = {
  spec: CardSpec;
  anchors: Anchor[];
  // The project's materials — SIFT is the first card whose anchors span TWO
  // materials, and the server has no channel that tells the client which
  // material a compare card was surfaced against (unlike `annotate`, whose
  // AI-authored anchors already carry material_id at surface time). Rather
  // than guess, the student explicitly names both: which source she is
  // checking, and which independent source she found — the same restraint
  // rule everywhere else in this product (the AI never decides for her).
  materials: MaterialSource[];
  // Opens 6b's existing 添加信源 entry point (never ingests here — RL-2).
  onAddLateralSource: () => void;
  onSubmit: (env: CardInstance) => void;
  onSkip: (eventTrace: TraceEvent[]) => void;
};

type SiftParams = { lateral_dimension?: string };

const FONT = "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif";

function pillStyle(selected: boolean): React.CSSProperties {
  return {
    display: "inline-flex",
    alignItems: "center",
    fontSize: 12,
    fontWeight: 700,
    padding: "5px 12px",
    borderRadius: 999,
    cursor: "pointer",
    fontFamily: "inherit",
    border: selected ? "1px solid #5C4A8A" : "1px solid #E1E4ED",
    color: selected ? "#fff" : "#3A4256",
    background: selected ? "#5C4A8A" : "#fff",
  };
}

function PlusIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path d="M12 5v14M5 12h14" stroke="#5C4A8A" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.6} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

function LockIcon() {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="5" y="11" width="14" height="10" rx="2" />
      <path d="M8 11V7a4 4 0 018 0v4" />
    </svg>
  );
}

// Coach-rail sibling of StudioAnnotateCard (design s3CoachCard), but for the
// `compare` primitive: SIFT's four steps (stop/investigate/find/trace),
// mirrored 1:1 into anchors on submit — dimension = field key, exactly like
// CRAAP's risk_note. Guidance levels beyond L1 may one day pre-seed some of
// these answers via `anchors`; this renderer already seeds from them (same
// pattern as StudioAnnotateCard) so it needs no change when that lands.
export function StudioCompareCard({ spec, anchors, materials, onAddLateralSource, onSubmit, onSkip }: StudioCompareCardProps) {
  const params = (spec.params ?? {}) as SiftParams;
  const lateralDimension = params.lateral_dimension ?? "";

  const [checkedMaterialId, setCheckedMaterialId] = useState(() => materials[0]?.id ?? "");
  const [lateralMaterialId, setLateralMaterialId] = useState(
    () => anchors.find((a) => a.dimension === lateralDimension && a.material_id !== "")?.material_id ?? "",
  );
  const [answers, setAnswers] = useState<Record<string, string>>(() => {
    const seeded: Record<string, string> = {};
    for (const step of spec.steps) {
      for (const field of step.fields) {
        seeded[field.key] = anchors.find((a) => a.dimension === field.key)?.answer ?? "";
      }
    }
    return seeded;
  });

  // materials can arrive after mount (the project's dossier loads
  // asynchronously) — default the checked material the first time a real
  // candidate shows up, without clobbering a student's own later choice.
  useEffect(() => {
    if (!checkedMaterialId && materials[0]) setCheckedMaterialId(materials[0].id);
  }, [materials, checkedMaterialId]);

  function setAnswer(key: string, value: string) {
    setAnswers((prev) => ({ ...prev, [key]: value }));
  }

  const lateralCandidates = materials.filter((m) => m.id !== checkedMaterialId);
  const answerableFields = spec.steps.flatMap((step) =>
    step.fields.filter((f) => f.type === "textarea" || f.type === "single_choice"),
  );

  const allFieldsAnswered = answerableFields.every((f) => !!answers[f.key]?.trim());
  // "A claim of having read laterally is not lateral reading" (design spec
  // §3.1) — the lateral material must be REAL and DIFFERENT from the one
  // under review, never just "some id got typed in".
  const hasRealLateralSource = !!lateralMaterialId && lateralMaterialId !== checkedMaterialId;
  const canLock = !!checkedMaterialId && hasRealLateralSource && allFieldsAnswered;

  function buildAnchors(): Anchor[] {
    return answerableFields.map((field) => {
      // THE declared rule (design spec §2.2 / card_lifecycle.go's
      // checkedMaterialID + lateralAnchor): the field whose key equals
      // params.lateral_dimension carries the LATERAL material; every other
      // field carries the CHECKED one. Never inferred from array position —
      // that is the exact coin-flip Task 3 fixed server-side.
      const isLateral = field.key === lateralDimension;
      return {
        id: field.key,
        material_id: isLateral ? lateralMaterialId : checkedMaterialId,
        block_id: "",
        start: 0,
        end: 0,
        quote: "",
        dimension: field.key,
        author: "student",
        question: field.label,
        answer: answers[field.key] ?? "",
      };
    });
  }

  function handleLock() {
    const env: CardInstance = { ...newEnvelope(spec.id, ""), anchors: buildAnchors() };
    onSubmit(env);
  }

  function handleSkip() {
    const scaffold = newEnvelope(spec.id, "");
    onSkip(scaffold.event_trace);
  }

  return (
    <div style={{ border: "1px solid #E3DCF2", borderRadius: 14, overflow: "hidden", boxShadow: "0 3px 14px rgba(92,74,138,.10)", fontFamily: FONT }}>
      <div style={{ height: 4, background: "#5C4A8A" }} />
      <div style={{ padding: "12px 15px 8px" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#5C4A8A" }}>工具卡 · SIFT</span>
          <span style={{ fontSize: 10.5, fontWeight: 700, color: "#2A3B7A", background: "#EDEFF9", padding: "2px 8px", borderRadius: 999 }}>
            {spec.category}
          </span>
        </div>
        <div style={{ fontSize: 14, fontWeight: 800, color: "#1C2333" }}>{spec.name}</div>
        <div style={{ fontSize: 11.5, color: "#8A92A3", lineHeight: 1.6, marginTop: 3 }}>{spec.purpose}</div>
      </div>

      <div style={{ padding: "0 15px 14px", maxHeight: 420, overflowY: "auto" }}>
        {materials.length > 1 && (
          <div style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 11, fontWeight: 700, color: "#9AA1B0", marginBottom: 6 }}>待查的来源</div>
            <div data-testid="checked-material-picker" style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
              {materials.map((m) => (
                <button key={m.id} type="button" onClick={() => setCheckedMaterialId(m.id)} style={pillStyle(m.id === checkedMaterialId)}>
                  {m.title}
                </button>
              ))}
            </div>
          </div>
        )}

        {spec.steps.map((step) => (
          <div key={step.key}>
            {step.fields.map((field) => {
              if (field.type === "textarea") {
                const answered = !!answers[field.key]?.trim();
                return (
                  <div key={field.key} style={{ border: "1px solid #ECEEF3", borderRadius: 12, padding: "13px 15px", marginBottom: 10, background: "#fff" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 7 }}>
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 11, fontWeight: 700, padding: "3px 10px", borderRadius: 999, color: "#fff", background: "#5C4A8A" }}>
                        {field.key}
                      </span>
                      {answered && <CheckIcon />}
                    </div>
                    <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#2B3346", fontWeight: 500 }}>{field.label}</div>
                    <textarea
                      value={answers[field.key] ?? ""}
                      onChange={(e) => setAnswer(field.key, e.target.value)}
                      placeholder={field.label}
                      rows={field.rows ?? 2}
                      style={{ width: "100%", marginTop: 8, border: "1px solid #E1E4ED", borderRadius: 9, padding: "8px 10px", fontSize: 12, lineHeight: 1.55, color: "#1C2333", background: "#fff", outline: "none", resize: "vertical", fontFamily: "inherit" }}
                    />
                  </div>
                );
              }
              if (field.type === "single_choice") {
                const answered = !!answers[field.key]?.trim();
                return (
                  <div key={field.key} style={{ border: "1px solid #ECEEF3", borderRadius: 12, padding: "13px 15px", marginBottom: 10, background: "#fff" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 7 }}>
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 6, fontSize: 11, fontWeight: 700, padding: "3px 10px", borderRadius: 999, color: "#fff", background: "#5C4A8A" }}>
                        {field.key}
                      </span>
                      {answered && <CheckIcon />}
                    </div>
                    <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#2B3346", fontWeight: 500, marginBottom: 8 }}>{field.label}</div>
                    <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                      {field.options.map((opt) => (
                        <button key={opt} type="button" onClick={() => setAnswer(field.key, opt)} style={pillStyle(answers[field.key] === opt)}>
                          {opt}
                        </button>
                      ))}
                    </div>
                  </div>
                );
              }
              return null;
            })}

            {step.key === lateralDimension && (
              <div style={{ border: "1px dashed #D8CFF0", borderRadius: 12, padding: "13px 15px", marginBottom: 10, background: "#FAF8FE" }}>
                <div style={{ fontSize: 11, fontWeight: 700, color: "#5C4A8A", marginBottom: 6 }}>独立信源</div>
                {lateralCandidates.length === 0 ? (
                  <>
                    <div style={{ fontSize: 12, color: "#7A8296", lineHeight: 1.6, marginBottom: 8 }}>
                      还没有独立来源——去「素材」加一个，再回来选它。
                    </div>
                    <button
                      type="button"
                      onClick={onAddLateralSource}
                      style={{ display: "inline-flex", alignItems: "center", gap: 6, background: "#5C4A8A", color: "#fff", border: "none", borderRadius: 10, fontSize: 12.5, fontWeight: 700, padding: "7px 14px", cursor: "pointer", fontFamily: "inherit" }}
                    >
                      <PlusIcon />
                      添加信源
                    </button>
                  </>
                ) : (
                  <div data-testid="lateral-material-picker" style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                    {lateralCandidates.map((m) => (
                      <button key={m.id} type="button" onClick={() => setLateralMaterialId(m.id)} style={pillStyle(m.id === lateralMaterialId)}>
                        {m.title}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      <div style={{ padding: "10px 15px 14px", borderTop: "1px solid #F3ECE6", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <button
          type="button"
          onClick={handleSkip}
          style={{ background: "none", border: "none", color: "#C2557A", fontSize: 12, fontWeight: 600, cursor: "pointer", padding: "6px 0", fontFamily: "inherit" }}
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
            color: "#fff",
            background: "#5C4A8A",
            border: "1px solid #5C4A8A",
            fontFamily: "inherit",
          }}
        >
          <LockIcon />
          锁定这张卡
        </button>
      </div>
    </div>
  );
}
