import type { SpotCheckFx } from "@mind-imprint/contracts";
import { WorkOrderItem, type WorkOrderRow } from "./WorkOrder";

// SpotCheckPanel.tsx — N3f Task 7. One component, two call sites: 信源体检
// at S3 (素材) and 论证体检 at S4 (结构). Mirrors WritingView's 整稿体检
// trigger/work-order shape (dc.html:1101-1104 button, :1120-1151 work order +
// empty state) but with NO examiner-voice switcher — voices are a
// whole-draft-review affordance with no design here — and rows built with NO
// `band` key at all (a spot-check row has no band by design, see
// agent.SpotCheckItem's own comment).
//
// `data.orderable` is computed server-side (studio.projectSpotChecks) and is
// never recomputed here — it encodes whether pressing the button would
// actually cost a model call.

function CheckIcon({ color }: { color: string }) {
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 11l3 3L22 4M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11" />
    </svg>
  );
}

export function SpotCheckPanel({
  title,
  data,
  onOrder,
  onDisposition,
  pending,
}: {
  title: string;
  data: SpotCheckFx;
  onOrder: () => void;
  onDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
  pending: boolean;
}) {
  const hasItems = data.items.length > 0;
  const disabled = pending || !data.orderable;
  // "There's nothing new to check yet" is a fact about state, never a
  // scolding (铁律 2) — and it only applies once a first batch exists; a
  // station that has never been checked shows the plain empty state instead,
  // not a claim that "nothing changed" when nothing has ever been ordered.
  const showNothingNewNote = !pending && !data.orderable && hasItems;

  return (
    <div style={{ marginTop: 18, background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "20px 22px" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 9 }}>
        <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".05em", color: "#2A3B7A", background: "#EDEFF9", padding: "3px 9px", borderRadius: 999 }}>
          {title}
        </span>
        <div style={{ marginLeft: "auto" }}>
          <button
            type="button"
            disabled={disabled}
            onClick={onOrder}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              background: disabled ? "#B9C0D6" : "#2A3B7A",
              border: "none",
              color: "#fff",
              fontSize: 13,
              fontWeight: 700,
              padding: "9px 15px",
              borderRadius: 10,
              cursor: disabled ? "not-allowed" : "pointer",
              fontFamily: "inherit",
            }}
          >
            <CheckIcon color="#fff" />
            {title}
          </button>
        </div>
      </div>
      {showNothingNewNote && (
        <div style={{ fontSize: 12, color: "#9AA1B0", marginTop: 10 }}>
          还没有新的变化——内容跟上次体检时一样，暂时没有新的可以查。
        </div>
      )}
      {hasItems ? (
        <div style={{ marginTop: 14 }}>
          {data.items.map((item) => {
            const row: WorkOrderRow = {
              interventionId: item.interventionId,
              label: item.targetName,
              evidence: item.evidence,
              missing: item.missing,
              fix: item.fix,
              disposition: item.disposition,
            };
            return <WorkOrderItem key={item.interventionId} row={row} onDisposition={onDisposition} />;
          })}
        </div>
      ) : (
        <div style={{ marginTop: 18, border: "1px dashed #DDE1EB", borderRadius: 14, padding: 22, textAlign: "center" }}>
          <div style={{ fontSize: 13.5, color: "#8A92A3", lineHeight: 1.7 }}>
            还没做{title}。点上面的「{title}」，印记会检查一遍现在的内容，指出还缺什么。
            <br />
            一次体检对应当前的内容——想再查一次，先让内容有变化。
          </div>
        </div>
      )}
    </div>
  );
}
