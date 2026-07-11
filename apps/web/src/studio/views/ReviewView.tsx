import type { GaugeFx } from "../state";

export type ReviewViewProps = {
  gauges: GaugeFx[];
};

const LEVEL_COLOR: Record<GaugeFx["level"], string> = {
  full: "#4C9A82",
  partial: "#D9A23D",
  empty: "#AEB4C2",
};

function GaugeCard({ g }: { g: GaugeFx }) {
  const color = LEVEL_COLOR[g.level];
  const lamps = Array.from({ length: g.total }, (_, i) => i < g.lit);

  return (
    <div
      data-testid={`gauge-${g.table}`}
      style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "14px 16px" }}
    >
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
        <span style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>{g.table}</span>
        <span style={{ fontSize: 11.5, fontWeight: 800, color }}>
          {g.lit}/{g.total}
        </span>
      </div>
      <div style={{ display: "flex", gap: 5, margin: "11px 0 9px" }}>
        {lamps.map((lit, i) => (
          <span
            key={i}
            data-lamp={lit ? "lit" : "empty"}
            style={{
              flex: 1,
              height: 6,
              borderRadius: 4,
              background: lit ? color : "#EEF0F5",
            }}
          />
        ))}
      </div>
      <div style={{ fontSize: 11.5, color: "#8A92A3", lineHeight: 1.55 }}>{g.note}</div>
    </div>
  );
}

export function ReviewView({ gauges }: ReviewViewProps) {
  return (
    <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "22px 30px 40px" }}>
      <div style={{ maxWidth: 720, margin: "0 auto" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 16, marginBottom: 8 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 7 }}>
            <div style={{ fontSize: 17, fontWeight: 800, color: "#1C2333" }}>就绪度</div>
            <span
              title="就绪度显示你的草稿现在落在评分表的哪一格，用来定位下一步，不是预估分数"
              style={{
                width: 16,
                height: 16,
                borderRadius: "50%",
                background: "#EEF0F5",
                color: "#9AA1B0",
                fontSize: 11,
                fontWeight: 700,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                cursor: "help",
              }}
            >
              ?
            </span>
          </div>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 12, marginTop: 14 }}>
          {gauges.map((g) => (
            <GaugeCard key={g.table} g={g} />
          ))}
        </div>
      </div>
    </div>
  );
}
