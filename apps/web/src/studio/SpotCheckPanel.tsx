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

// I2 fix: `orderable` is false whenever the station has no targets yet
// (apps/api/internal/studio/projection.go — `len(targets) == 0`), which is
// the DEFAULT state at both stations: S3 until she adds and evaluates an
// article; S4 until at least one Toulmin slot has non-blank text. Without
// this note, the FIRST thing either station shows is a disabled button
// sitting above the empty state's old copy telling her to press it — an
// instruction the disabled button will not let her follow. This is a plain
// fact about what has to exist first, not a scolding or a score (铁律 2) —
// keyed on `title` since that's the only signal this shared component has
// for which station it is (the two call sites pass it verbatim).
// Deliberately does NOT say "信源档案" — that exact substring is
// SourceDossier's own header ("信源档案 · 已收集 N 篇"), which always renders
// on the same screen as this note (S3's plain branch, its compare-mode
// dossier view), so a getByText(/信源档案/) anywhere on that screen would
// otherwise ambiguously match both.
const NOT_POSSIBLE_YET_COPY: Record<string, string> = {
  "信源体检": "先添加一篇信源、评估过之后，再来体检。",
  "论证体检": "先在上面写下至少一个论证位置，再来体检。",
};

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
  // station that has never been checked shows the "not possible yet" note
  // below instead, not a claim that "nothing changed" when nothing has ever
  // been ordered.
  const showNothingNewNote = !pending && !data.orderable && hasItems;
  // I2 fix: the OTHER reason the button is disabled — no batch has ever
  // existed because the station has no targets yet. Distinct from the note
  // above (that one presupposes a prior check; this one explains why there
  // has never been one).
  const showNotPossibleYetNote = !pending && !data.orderable && !hasItems;

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
      {showNotPossibleYetNote && (
        <div style={{ fontSize: 12, color: "#9AA1B0", marginTop: 10 }}>
          {NOT_POSSIBLE_YET_COPY[title] ?? "现在还没有可以体检的内容，先让内容有进展，再来体检。"}
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
            {/* I2 fix: no longer says "点上面的「title}」" — the button above
                may currently be disabled (orderable false), and an empty
                state must never instruct an action that isn't available. */}
            还没做{title}。{title}会检查一遍现在的内容，指出还缺什么。
            <br />
            一次体检对应当前的内容——想再查一次，先让内容有变化。
          </div>
        </div>
      )}
    </div>
  );
}
