import { useState } from "react";
import type { WritingProjection, WritingReviewItem } from "../state";
import { WorkOrderItem, type WorkOrderRow } from "../WorkOrder";

// WritingProjection + the callbacks Task 9 (buffer/commit) and Task 10
// (review/disposition/citations attest) produce.
export type WritingViewProps = WritingProjection & {
  onBufferChange?: (text: string) => void;
  // May be async (StudioContainer's onCommit is commitSnapshot → refetch) —
  // the view awaits it (spec §7: committing switches to preview, but only
  // once the commit has actually landed).
  onCommit?: (text: string) => void | Promise<void>;
  // Task 10: triggers 整稿体检 over the given committed snapshot. Slice 8b
  // Task 7 adds the examiner-voice arg — the currently-selected pill.
  onOrderReview?: (snapshotId: string, voice: ReviewVoice) => void;
  // Task 10: the three-key disposition on one review-item intervention.
  onReviewDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
  // Task 10: the S5 student-written citations_matched gate item.
  onAttestCitations?: (confirmed: boolean) => void;
};

const TAB_BASE: React.CSSProperties = {
  border: "none",
  fontFamily: "inherit",
  fontSize: 13,
  fontWeight: 700,
  padding: "7px 14px",
  borderRadius: 7,
  cursor: "pointer",
};

function tabStyle(active: boolean): React.CSSProperties {
  return {
    ...TAB_BASE,
    background: active ? "#fff" : "transparent",
    color: active ? "#1C2333" : "#6B7384",
    boxShadow: active ? "0 1px 2px rgba(28,35,51,0.08)" : "none",
  };
}

// Slice 8b Task 7: the examiner-voice switcher. Kept as a view-local type
// (not imported from apps/web/src/api/writing.ts) since views are pure
// presentation and take no dependency on the API layer — the literal union
// is structurally identical to that module's ReviewVoice and to contracts'
// WritingReviewItem.voice enum.
export type ReviewVoice = "board" | "sceptic" | "layperson" | "executioner";
const VOICES: Array<{ voice: ReviewVoice; label: string }> = [
  { voice: "board", label: "考官" },
  { voice: "sceptic", label: "怀疑" },
  { voice: "layperson", label: "外行" },
  { voice: "executioner", label: "字数" },
];

// Mirrors the backend's own paragraph unit (paragraphSpanIndex /
// snapshotParagraphs in apps/api/internal/api/writing.go): split on blank
// lines, trim, drop empties.
function paragraphsOf(text: string): string[] {
  return text
    .split("\n\n")
    .map((p) => p.trim())
    .filter((p) => p.length > 0);
}

// The binding design (docs/design/思维印记_工作区.dc.html:2334) reads
// "第 3 版快照 · 10-14 提交 · 只读" — version · MM-DD · 提交 · 只读. `committedAt`
// is an ISO datetime string (YYYY-MM-DDTHH:mm:ssZ); slice the MM-DD straight
// out of it rather than round-tripping through `Date`, which would re-derive
// the calendar day in the *local* timezone and could shift it a day off from
// what the server actually recorded as committed.
function monthDayOf(committedAt: string): string {
  return committedAt.slice(5, 10);
}

function EditPane({ buffer, onBufferChange, onCommit }: { buffer: string; onBufferChange?: (t: string) => void; onCommit?: (t: string) => void }) {
  return (
    <>
      <div style={{ fontSize: 11.5, color: "#9AA1B0", marginBottom: 8 }}>
        直接在这里写，也可以在别处写好后粘进来。写作时印记不会打断你——想听意见，点「整稿体检」。
      </div>
      <textarea
        value={buffer}
        onChange={(e) => onBufferChange?.(e.target.value)}
        style={{
          width: "100%",
          minHeight: 440,
          border: "1px solid #E7EAF1",
          borderRadius: 14,
          padding: "24px 26px",
          fontSize: 15.5,
          lineHeight: 2,
          color: "#2B3346",
          background: "#fff",
          outline: "none",
          resize: "vertical",
          fontFamily: "inherit",
        }}
      />
      <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 12 }}>
        <button
          type="button"
          onClick={() => onCommit?.(buffer)}
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 7,
            background: "#EDEFF9",
            color: "#2A3B7A",
            border: "none",
            fontSize: 13,
            fontWeight: 700,
            padding: "9px 15px",
            borderRadius: 10,
            cursor: "pointer",
            fontFamily: "inherit",
          }}
        >
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M12 3v12M7 8l5-5 5 5M5 21h14" />
          </svg>
          提交快照 · 定格这一稿
        </button>
      </div>
    </>
  );
}

function CitationsAttest({ citationsMatched, onAttestCitations }: { citationsMatched: boolean; onAttestCitations?: (confirmed: boolean) => void }) {
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 9, marginTop: 16, paddingTop: 14, borderTop: "1px dashed #ECEEF3" }}>
      <input
        type="checkbox"
        id="citations-matched"
        checked={citationsMatched}
        onChange={(e) => onAttestCitations?.(e.target.checked)}
        style={{ width: 16, height: 16, cursor: "pointer" }}
      />
      <label htmlFor="citations-matched" style={{ fontSize: 12.5, color: "#5B6373", cursor: "pointer" }}>
        我已核对：这一稿里出现的每处引用都能对上信源
      </label>
    </div>
  );
}

function PreviewPane({
  buffer,
  reviewOrdered,
  reviewItems,
  overBudgetDelta,
  citationsMatched,
  onReviewDisposition,
  onAttestCitations,
}: {
  buffer: string;
  reviewOrdered: boolean;
  reviewItems: WritingReviewItem[];
  overBudgetDelta: number | null;
  citationsMatched: boolean;
  onReviewDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
  onAttestCitations?: (confirmed: boolean) => void;
}) {
  const paras = paragraphsOf(buffer);
  return (
    <>
      <div style={{ background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "30px 34px", minHeight: 300 }}>
        {paras.map((p, i) => (
          <p key={i} style={{ fontSize: 15.5, lineHeight: 2.05, color: "#2B3346", margin: "0 0 16px", textIndent: "2em" }}>
            {p}
          </p>
        ))}
      </div>
      {reviewOrdered ? (
        <div style={{ marginTop: 18, background: "#fff", border: "1px solid #EAECF2", borderRadius: 16, padding: "20px 22px" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 9, marginBottom: 4 }}>
            <span style={{ fontSize: 11, fontWeight: 700, letterSpacing: ".05em", color: "#2A3B7A", background: "#EDEFF9", padding: "3px 9px", borderRadius: 999 }}>
              整稿体检 · 段落 ⇄ 评分表
            </span>
            <span style={{ fontSize: 11.5, color: "#9AA1B0", fontWeight: 600 }}>一稿一检 · 只读</span>
          </div>
          {overBudgetDelta !== null && (
            <div style={{ fontSize: 12, fontWeight: 700, color: "#C96F4F", background: "#FBEEE7", borderRadius: 10, padding: "8px 12px", margin: "0 0 14px" }}>
              超预算 {overBudgetDelta} 字 · 删减决策按「这段在向哪张表交证据」来做
            </div>
          )}
          <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.6, margin: "8px 0 16px" }}>
            它只告诉你：哪一段在向哪张表交证据、还缺什么。它从不替你改句子——改，是你自己的事。
          </div>
          {reviewItems.map((item) => {
            const row: WorkOrderRow = {
              interventionId: item.interventionId,
              label: item.criterion,
              band: item.band,
              evidence: item.evidence,
              missing: item.missing,
              fix: item.fix,
              disposition: item.disposition,
            };
            return (
              <WorkOrderItem
                key={item.interventionId}
                row={row}
                onDisposition={onReviewDisposition}
              />
            );
          })}
          <CitationsAttest citationsMatched={citationsMatched} onAttestCitations={onAttestCitations} />
        </div>
      ) : (
        <div style={{ marginTop: 18, border: "1px dashed #DDE1EB", borderRadius: 14, padding: 22, textAlign: "center" }}>
          <div style={{ fontSize: 13.5, color: "#8A92A3", lineHeight: 1.7 }}>
            还没做体检。点右上角「整稿体检」，印记会告诉你每段在向哪张评分表交证据。
            <br />
            一稿一检——想再体检一次，先提交新的快照。
          </div>
        </div>
      )}
    </>
  );
}

export function WritingView({
  buffer,
  latestSnapshot,
  review,
  citationsMatched,
  onBufferChange,
  onCommit,
  onOrderReview,
  onReviewDisposition,
  onAttestCitations,
}: WritingViewProps) {
  const [mode, setMode] = useState<"edit" | "preview">("edit");
  const isEdit = mode === "edit";
  const [voice, setVoice] = useState<ReviewVoice>("board");
  const items = review?.items ?? [];
  const cachedVoices = new Set(items.map((i) => i.voice));
  const itemsForVoice = items.filter((i) => i.voice === voice);
  const reviewOrdered = itemsForVoice.length > 0;
  const budget = latestSnapshot?.budget;

  // spec §7: committing a snapshot switches the view to preview — but only
  // once the commit has actually succeeded. onCommit may be async
  // (StudioContainer's onCommit is commitSnapshot → refetch); awaiting it
  // here means a failed commit (which StudioContainer surfaces as its own
  // sync-error banner and rethrows) leaves the view on 编辑 · 安静 instead of
  // switching to a preview of a draft that was never actually captured.
  async function handleCommit(text: string) {
    try {
      await onCommit?.(text);
    } catch {
      return;
    }
    setMode("preview");
  }
  const budgetClause = budget
    ? budget.state === "over"
      ? ` · 超出 ${budget.delta} 字`
      : budget.state === "under"
        ? ` · 还差 ${budget.delta} 字`
        : " · 在预算内"
    : "";
  const snapshotMeta = latestSnapshot
    ? `第 ${latestSnapshot.seq} 版快照 · ${monthDayOf(latestSnapshot.committedAt)} 提交 · 只读${budgetClause}`
    : "还没有提交过快照";

  return (
    <div style={{ display: "flex", flexDirection: "column", flex: 1, minHeight: 0 }}>
      <div style={{ flex: "none", display: "flex", alignItems: "center", gap: 12, padding: "12px 30px 0" }}>
        <div style={{ display: "flex", gap: 3, background: "#EBEDF2", borderRadius: 9, padding: 3 }}>
          <button type="button" onClick={() => setMode("edit")} style={tabStyle(isEdit)}>
            编辑 · 安静
          </button>
          <button type="button" onClick={() => setMode("preview")} style={tabStyle(!isEdit)}>
            预览 · 批注
          </button>
        </div>
        <span style={{ fontSize: 11.5, color: "#AEB4C2", fontWeight: 600 }}>{snapshotMeta}</span>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 10 }}>
          <div style={{ display: "flex", gap: 3, background: "#EBEDF2", borderRadius: 9, padding: 3 }}>
            {VOICES.map((v) => {
              const active = voice === v.voice;
              const cached = cachedVoices.has(v.voice);
              return (
                <button key={v.voice} type="button" onClick={() => setVoice(v.voice)} style={tabStyle(active)}>
                  {v.label}{cached ? " ·" : ""}
                </button>
              );
            })}
          </div>
          <button
            type="button"
            disabled={!latestSnapshot}
            onClick={() => latestSnapshot && onOrderReview?.(latestSnapshot.id, voice)}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 7,
              background: latestSnapshot ? "#2A3B7A" : "#B9C0D6",
              border: "none",
              color: "#fff",
              fontSize: 13,
              fontWeight: 700,
              padding: "9px 15px",
              borderRadius: 10,
              cursor: latestSnapshot ? "pointer" : "not-allowed",
              fontFamily: "inherit",
            }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="#fff" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
              <path d="M9 11l3 3L22 4M21 12v7a2 2 0 01-2 2H5a2 2 0 01-2-2V5a2 2 0 012-2h11" />
            </svg>
            整稿体检
          </button>
        </div>
      </div>
      <div style={{ flex: 1, minHeight: 0, overflowY: "auto", padding: "16px 30px 40px" }}>
        <div style={{ maxWidth: 720, margin: "0 auto" }}>
          {isEdit ? (
            <EditPane buffer={buffer} onBufferChange={onBufferChange} onCommit={handleCommit} />
          ) : (
            <PreviewPane
              buffer={buffer}
              reviewOrdered={reviewOrdered}
              reviewItems={itemsForVoice}
              overBudgetDelta={budget?.state === "over" ? budget.delta : null}
              citationsMatched={citationsMatched}
              onReviewDisposition={onReviewDisposition}
              onAttestCitations={onAttestCitations}
            />
          )}
        </div>
      </div>
    </div>
  );
}
