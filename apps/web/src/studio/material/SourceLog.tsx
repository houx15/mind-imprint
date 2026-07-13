import type { MaterialSource } from "@mind-imprint/contracts";

export type SourceLogProps = {
  sources: MaterialSource[];
};

// The visible ledger proving the student did the finding (RL-2) — a source
// only shows up here once it actually carries a log entry (a takeaway and/or
// a tier the student chose). Nothing is invented for a source that hasn't
// been opened/logged yet.
function hasLogEntry(source: MaterialSource): boolean {
  return source.takeaway !== "" || source.tier !== "";
}

// Whole minutes, rounded to the nearest — but a source with nothing logged
// (0s) renders nothing rather than the misleading "停留 0m".
function readingMinutesLabel(timeSpentS: number): string | null {
  if (timeSpentS <= 0) return null;
  return `停留 ${Math.round(timeSpentS / 60)}m`;
}

export function SourceLog({ sources }: SourceLogProps) {
  const logged = sources.filter(hasLogEntry);

  return (
    <div style={{ marginTop: 16 }}>
      <div style={{ fontSize: 13.5, fontWeight: 700, color: "#1C2333" }}>
        检索日志 · 已记录 {logged.length} 条
      </div>
      <div style={{ fontSize: 12, color: "#8A93A6", marginTop: 2, marginBottom: 10 }}>
        每一条你打开过的来源都在这里——引用只能从这里来。
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {logged.map((source) => (
          <div
            key={source.id}
            style={{
              background: "#fff",
              border: "1px solid #E4E6EE",
              borderRadius: 12,
              padding: "10px 13px",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
              {source.sourceUrl ? (
                <a
                  href={source.sourceUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{ fontSize: 13, fontWeight: 700, color: "#1C2333", textDecoration: "none" }}
                >
                  {source.title}
                </a>
              ) : (
                <span style={{ fontSize: 13, fontWeight: 700, color: "#1C2333" }}>{source.title}</span>
              )}
              {source.tier !== "" && (
                <span
                  style={{
                    fontSize: 10.5,
                    fontWeight: 700,
                    padding: "2px 9px",
                    borderRadius: 999,
                    background: "#F1F2F6",
                    color: "#5A6178",
                  }}
                >
                  {source.tier}
                </span>
              )}
              {readingMinutesLabel(source.timeSpentS) && (
                <span style={{ fontSize: 11.5, color: "#8A93A6" }}>{readingMinutesLabel(source.timeSpentS)}</span>
              )}
            </div>
            {source.takeaway !== "" && (
              <div style={{ fontSize: 12.5, color: "#5A6178", marginTop: 6, lineHeight: 1.5 }}>{source.takeaway}</div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
