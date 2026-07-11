import { useState } from "react";
import type { OnboardingFx, Station } from "../state";

export type OnboardingViewProps = {
  station: Station;
  data: OnboardingFx;
};

const WRAP: React.CSSProperties = { flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" };
const COL: React.CSSProperties = { maxWidth: 720, margin: "0 auto" };
const CARD: React.CSSProperties = { background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "20px 22px" };

function S0View({ data }: { data: OnboardingFx }) {
  const [weak, setWeak] = useState<Set<number>>(() => new Set(data.rubricRows.flatMap((r, i) => (r.weak ? [i] : []))));

  function toggle(i: number) {
    setWeak((prev) => {
      const next = new Set(prev);
      if (next.has(i)) next.delete(i);
      else next.add(i);
      return next;
    });
  }

  return (
    <div style={WRAP}>
      <div style={COL}>
        {/* Card 1: restate */}
        <div style={CARD}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333" }}>用自己的话，说清这份任务在考什么</div>
          </div>
          <div style={{ fontSize: 12.5, color: "#8A92A3", margin: "4px 0 12px" }}>
            死记评分标准没用——先用你自己的话复述一遍，印记只看你是不是真读懂了。
          </div>
          <textarea
            rows={3}
            placeholder={data.restatePrompt}
            style={{
              width: "100%",
              border: "1px solid #E1E4ED",
              borderRadius: 12,
              padding: "12px 14px",
              fontSize: 14,
              lineHeight: 1.7,
              color: "#1C2333",
              background: "#fff",
              outline: "none",
              resize: "vertical",
            }}
          />
        </div>

        {/* Card 2: rubric */}
        <div style={{ ...CARD, marginTop: 14 }}>
          <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333" }}>评分表 · 翻成人话</div>
          <div style={{ fontSize: 12.5, color: "#8A92A3", margin: "4px 0 10px" }}>
            点两项你最没底的——之后印记会在这两块盯得更紧。
            <span style={{ color: "#C96F4F", fontWeight: 700 }}> 已选 {weak.size}</span>
          </div>
          {data.rubricRows.map((r, i) => {
            const selected = weak.has(i);
            return (
              <div
                key={r.official}
                onClick={() => toggle(i)}
                role="button"
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 10,
                  cursor: "pointer",
                  borderRadius: 12,
                  padding: "10px 8px",
                  background: selected ? "#FCF5F1" : "transparent",
                }}
              >
                <span style={{ flex: "none", width: 220, fontSize: 12, color: "#9AA1B0", fontWeight: 600 }}>{r.official}</span>
                <span style={{ flex: 1, fontSize: 14, fontWeight: 700, color: "#1C2333" }}>{r.plain}</span>
                {selected && (
                  <span
                    style={{
                      flex: "none",
                      fontSize: 10.5,
                      fontWeight: 700,
                      color: "#C96F4F",
                      background: "#fff",
                      border: "1px solid #F1D6C8",
                      padding: "2px 9px",
                      borderRadius: 999,
                    }}
                  >
                    待加强
                  </span>
                )}
              </div>
            );
          })}
        </div>

        {/* Card 3: plan tracker */}
        <div style={{ background: "#fff", border: "1px solid #EAECF2", borderRadius: 14, padding: "18px 20px", marginTop: 14 }}>
          <div style={{ fontSize: 13, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>我的写作计划</div>
          <div style={{ display: "flex", alignItems: "center", gap: 0 }}>
            {data.planSteps.map((name, i) => (
              <div key={name} style={{ display: "flex", alignItems: "center", gap: 8, flex: 1 }}>
                <span
                  style={{
                    flex: "none",
                    width: 8,
                    height: 8,
                    borderRadius: "50%",
                    background: i === 0 ? "#2A3B7A" : "#D6DBE8",
                  }}
                />
                <span style={{ fontSize: 12, fontWeight: 700, color: i === 0 ? "#1C2333" : "#9AA1B0" }}>{name}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function ShellView({ station }: { station: Station }) {
  return (
    <div style={WRAP}>
      <div style={COL}>
        <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333", marginBottom: 14 }}>{station.name}</div>
        <div style={{ ...CARD, textAlign: "center", color: "#8A92A3", fontSize: 13.5, fontWeight: 600 }}>
          此环节的深入交互将在后续切片接入
        </div>
      </div>
    </div>
  );
}

export function OnboardingView({ station, data }: OnboardingViewProps) {
  if (station.code === "S0") {
    return <S0View data={data} />;
  }
  return <ShellView station={station} />;
}
