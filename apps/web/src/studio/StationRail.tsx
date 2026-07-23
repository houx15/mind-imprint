import type { Station, StationCode } from "./state";

export type StationRailProps = {
  stations: Station[];
  active: StationCode;
  focus: boolean;
  onSelect: (code: StationCode) => void;
  // N6-E: re-opens a `waived` station (the journey composer skipped it) —
  // see the `waived` branch below for the 恢复 control that fires this.
  onReopen: (code: StationCode) => void;
};

// Icon paths lifted from the design's STA table (docs/design/思维印记_工作区.dc.html ~L2092).
const STATION_ICON_PATH: Record<StationCode, string> = {
  S0: "M11 4a7 7 0 100 14 7 7 0 000-14zM21 21l-4-4",
  S1: "M5 3v18M5 4h13l-2 4 2 4H5",
  S2: "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7-10-7-10-7zM12 9a3 3 0 100 6 3 3 0 000-6",
  S3: "M12 3l8 3v6c0 5-4 8-8 9-4-1-8-4-8-9V6zM9 12l2 2 4-4",
  S4: "M6 4a2 2 0 100 4 2 2 0 000-4zM6 16a2 2 0 100 4 2 2 0 000-4zM18 10a2 2 0 100 4 2 2 0 000-4zM8 6h6a2 2 0 012 2v2M8 18h6a2 2 0 002-2v-2",
  S5: "M12 20h9M16.5 3.5a2.12 2.12 0 013 3L7 19l-4 1 1-4z",
  S6: "M19 21l-7-5-7 5V5a2 2 0 012-2h10a2 2 0 012 2z",
};

const HELP_TITLE = "需要完成前面环节才可解锁下一环节，您也可以点击上锁环节先行预览";

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.8} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M20 6L9 17l-5-5" />
    </svg>
  );
}

function LockIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#B6BCC9" strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="5" y="11" width="14" height="10" rx="2" />
      <path d="M8 11V7a4 4 0 018 0v4" />
    </svg>
  );
}

function BackflowIcon() {
  return (
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="#4C9A82" strokeWidth={2.4} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M3 12a9 9 0 019-9 9 9 0 016.7 3M21 12a9 9 0 01-9 9 9 9 0 01-6.7-3" />
      <path d="M18 3v3.5h-3.5M6 21v-3.5h3.5" />
    </svg>
  );
}

export function StationRail({ stations, active, focus, onSelect, onReopen }: StationRailProps) {
  const expanded = !focus;

  return (
    <div
      style={{
        width: focus ? 64 : 246,
        flex: "none",
        background: "#fff",
        borderRight: "1px solid #EAECF2",
        display: "flex",
        flexDirection: "column",
        overflowY: "auto",
        transition: "width .18s",
        fontFamily: "'Plus Jakarta Sans','Noto Sans SC',system-ui,sans-serif",
      }}
    >
      <div style={{ padding: "16px 16px 10px", display: "flex", alignItems: "center", gap: 7, minHeight: 22 }}>
        {expanded && (
          <>
            <div style={{ fontSize: 15, fontWeight: 800, color: "#1C2333" }}>任务旅程</div>
            <span
              title={HELP_TITLE}
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
          </>
        )}
      </div>

      <div style={{ padding: "4px 16px 20px" }}>
        {stations.map((st, i) => {
          const cur = st.state === "current";
          const done = st.state === "done";
          const locked = st.state === "locked";
          const waived = st.state === "waived";
          const isActive = active === st.code;

          const numBg = done ? "#4C9A82" : cur ? "#2A3B7A" : isActive ? "#EDEFF9" : "#F1F2F5";
          const iconColor = done || cur ? "#fff" : isActive ? "#2A3B7A" : "#AEB4C2";

          return (
            <div
              key={st.code}
              onClick={() => onSelect(st.code)}
              title={st.name}
              style={{
                display: "flex",
                gap: 10,
                cursor: "pointer",
                borderRadius: 12,
                padding: "8px 8px 0",
                background: isActive ? "#F5F7FE" : "transparent",
              }}
            >
              <div style={{ flex: "none", width: 30, display: "flex", flexDirection: "column", alignItems: "center", alignSelf: "stretch" }}>
                <div
                  style={{
                    flex: "none",
                    width: 30,
                    height: 30,
                    borderRadius: "50%",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    background: numBg,
                    border: cur ? "2px solid #2A3B7A" : "none",
                    boxShadow: cur ? "0 0 0 3px #E4E8F5" : "none",
                  }}
                >
                  <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke={iconColor} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
                    <path d={STATION_ICON_PATH[st.code]} />
                  </svg>
                </div>
                {i < stations.length - 1 && (
                  <div
                    style={{
                      flex: "1",
                      width: 2,
                      minHeight: 14,
                      background: done ? "#CFE4DA" : "#EAECF2",
                      marginTop: 3,
                    }}
                  />
                )}
              </div>

              {expanded && (
                <div style={{ flex: 1, minWidth: 0, paddingBottom: 14 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                    <span style={{ fontSize: 13.5, fontWeight: cur || isActive ? 800 : 700, color: locked || waived ? "#AEB4C2" : "#1C2333" }}>
                      {st.name}
                    </span>
                    {done && <CheckIcon />}
                    {locked && <LockIcon />}
                  </div>

                  {st.backflow && (
                    <div
                      style={{
                        display: "inline-flex",
                        alignItems: "center",
                        gap: 5,
                        marginTop: 7,
                        fontSize: 10.5,
                        fontWeight: 700,
                        color: "#4C9A82",
                        background: "#E7F3EE",
                        padding: "3px 8px",
                        borderRadius: 999,
                      }}
                    >
                      <BackflowIcon />
                      有据修正
                    </div>
                  )}

                  {waived && (
                    <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 7 }}>
                      <span style={{ fontSize: 10.5, fontWeight: 700, color: "#9AA1B0" }}>已跳过</span>
                      <button
                        type="button"
                        onClick={(e) => { e.stopPropagation(); onReopen(st.code); }}
                        style={{
                          fontSize: 10.5, fontWeight: 700, color: "#2A3B7A",
                          background: "#EDEFF9", border: "none", borderRadius: 999,
                          padding: "3px 9px", cursor: "pointer",
                        }}
                      >
                        恢复
                      </button>
                    </div>
                  )}

                  {cur && st.gate && (
                    <div
                      style={{
                        marginTop: 9,
                        background: "#F7F8FD",
                        border: "1px solid #E4E8F5",
                        borderRadius: 11,
                        padding: "10px 11px",
                      }}
                    >
                      <div style={{ fontSize: 11, fontWeight: 700, color: "#2A3B7A" }}>
                        本环节门禁 {st.gate.total} 项，已过 {st.gate.passed}
                      </div>
                      <div style={{ display: "flex", gap: 4, marginTop: 8 }}>
                        {Array.from({ length: st.gate.total }).map((_, k) => (
                          <span
                            key={k}
                            style={{
                              flex: 1,
                              height: 5,
                              borderRadius: 999,
                              background: k < (st.gate?.passed ?? 0) ? "#2A3B7A" : "#D6DBE8",
                            }}
                          />
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
