import { useState } from "react";
import type { WritingProjection, WritingReviewItem } from "../state";

// WritingProjection + the callbacks Task 9 (buffer/commit) and Task 10
// (review/disposition/citations attest) produce.
export type WritingViewProps = WritingProjection & {
  onBufferChange?: (text: string) => void;
  onCommit?: (text: string) => void;
  // Task 10: triggers 整稿体检 over the given committed snapshot.
  onOrderReview?: (snapshotId: string) => void;
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

// Design (docs/design/思维印记_工作区.dc.html:2259): WBT maps each review
// item's model-produced band string to a tone. These three band strings are
// the ONLY ones the skill's posture prompt is ever asked to choose among
// (apps/api/internal/agent/review.go's tests fix them at "3–4 段"/"5–6
// 段"/"7–8 段") — an unrecognized value (should never happen) falls back to a
// neutral chip rather than guessing a tone.
const BAND_TONE: Record<string, [color: string, background: string]> = {
  "3–4 段": ["#C96F4F", "#FBEEE7"],
  "5–6 段": ["#B8892F", "#FBF4E2"],
  "7–8 段": ["#4C9A82", "#E7F3EE"],
};

function bandChipStyle(band: string): React.CSSProperties {
  const [color, background] = BAND_TONE[band] ?? ["#6B7384", "#EEF0F5"];
  return { fontSize: 11, fontWeight: 700, color, background, padding: "2px 9px", borderRadius: 999 };
}

// Design (docs/design/思维印记_工作区.dc.html:2260): KEY_LABELS =
// [['keep','保持原样'],['revise','我来改'],['explain','说明为什么不改']] — the
// design's own local key ids ('keep'/'revise'/'explain') aren't the wire
// contract's disposition enum, so this maps the verbatim Chinese labels
// straight to the stored action (per the task brief: 保持原样→accept,
// 我来改→rewrite, 说明为什么不改→reject).
const REVIEW_KEYS: Array<{ label: string; action: "accept" | "rewrite" | "reject" }> = [
  { label: "保持原样", action: "accept" },
  { label: "我来改", action: "rewrite" },
  { label: "说明为什么不改", action: "reject" },
];

// Mirrors DispositionCard's own rune-based gate (backend validates on runes,
// not UTF-16 code units) — kept local since this is the only other place a
// review-item disposition's reason is composed.
function runeCount(text: string): number {
  return [...text.trim()].length;
}

function WorkOrderItem({
  item,
  onReviewDisposition,
}: {
  item: WritingReviewItem;
  onReviewDisposition?: (interventionId: string, action: "accept" | "rewrite" | "reject", reason: string) => void;
}) {
  const [choice, setChoice] = useState<"accept" | "rewrite" | "reject" | null>(item.disposition?.action ?? null);
  const [reason, setReason] = useState(item.disposition?.reason ?? "");
  const hasFix = item.fix.trim().length > 0;
  const rlen = runeCount(reason);
  const reasonOk = rlen >= 15;
  const canSubmit = choice !== null && reasonOk;

  function submit() {
    if (!canSubmit || choice === null) return;
    onReviewDisposition?.(item.interventionId, choice, reason);
  }

  return (
    <div style={{ border: "1px solid #ECEEF3", borderRadius: 13, padding: "14px 16px", marginBottom: 11 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 9, marginBottom: 7 }}>
        <span style={{ fontSize: 13, fontWeight: 800, color: "#1C2333" }}>{item.criterion}</span>
        <span style={bandChipStyle(item.band)}>{item.band}</span>
      </div>
      <div style={{ fontSize: 13, lineHeight: 1.65, color: "#5B6373" }}>{item.evidence}</div>
      {item.missing && (
        <div style={{ fontSize: 12.5, lineHeight: 1.6, color: "#8A92A3", marginTop: 6 }}>还缺：{item.missing}</div>
      )}
      {hasFix && (
        <div style={{ marginTop: 11, paddingTop: 11, borderTop: "1px dashed #ECEEF3" }}>
          <div style={{ fontSize: 12, color: "#8A92A3", marginBottom: 8 }}>建议：{item.fix}</div>
          <div style={{ display: "flex", gap: 7 }}>
            {REVIEW_KEYS.map((k) => {
              const picked = choice === k.action;
              return (
                <div
                  key={k.action}
                  role="button"
                  onClick={() => setChoice(k.action)}
                  style={{
                    flex: 1,
                    padding: "7px 6px",
                    borderRadius: 8,
                    fontSize: 11.5,
                    fontWeight: 700,
                    cursor: "pointer",
                    textAlign: "center",
                    fontFamily: "inherit",
                    background: picked ? "#2A3B7A" : "#fff",
                    color: picked ? "#fff" : "#6B7384",
                    border: `1px solid ${picked ? "#2A3B7A" : "#E1E4ED"}`,
                  }}
                >
                  {k.label}
                </div>
              );
            })}
          </div>
          {choice !== null && (
            <div style={{ marginTop: 9 }}>
              <textarea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                rows={2}
                placeholder="写下你的理由（至少 15 字）"
                style={{
                  width: "100%",
                  border: "1px solid #E1E4ED",
                  borderRadius: 10,
                  padding: "9px 11px",
                  fontSize: 12.5,
                  lineHeight: 1.6,
                  color: "#1C2333",
                  background: "#fff",
                  outline: "none",
                  resize: "vertical",
                  fontFamily: "inherit",
                }}
              />
              <div style={{ fontSize: 10.5, fontWeight: 600, marginTop: 6, color: rlen === 0 ? "#AEB4C2" : reasonOk ? "#4C9A82" : "#D9A23D" }}>
                {rlen === 0 ? "留一句理由才算数（≥15 字）" : reasonOk ? `✓ 已记录 · ${rlen} 字` : `再写一点 · ${rlen}/15 字`}
              </div>
              <button
                type="button"
                onClick={submit}
                disabled={!canSubmit}
                style={{
                  marginTop: 8,
                  padding: "7px 12px",
                  borderRadius: 9,
                  border: "none",
                  fontSize: 12,
                  fontWeight: 700,
                  fontFamily: "inherit",
                  cursor: canSubmit ? "pointer" : "not-allowed",
                  background: canSubmit ? "#2A3B7A" : "#E1E4ED",
                  color: canSubmit ? "#fff" : "#AEB4C2",
                }}
              >
                记录处置
              </button>
            </div>
          )}
        </div>
      )}
    </div>
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
  citationsMatched,
  onReviewDisposition,
  onAttestCitations,
}: {
  buffer: string;
  reviewOrdered: boolean;
  reviewItems: WritingReviewItem[];
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
          <div style={{ fontSize: 12.5, color: "#8A92A3", lineHeight: 1.6, margin: "8px 0 16px" }}>
            它只告诉你：哪一段在向哪张表交证据、还缺什么。它从不替你改句子——改，是你自己的事。
          </div>
          {reviewItems.map((item) => (
            <WorkOrderItem key={item.interventionId} item={item} onReviewDisposition={onReviewDisposition} />
          ))}
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
  const snapshotMeta = latestSnapshot
    ? `第 ${latestSnapshot.seq} 版快照 · ${monthDayOf(latestSnapshot.committedAt)} 提交 · 只读`
    : "还没有提交过快照";
  const reviewOrdered = review?.ordered ?? false;
  const reviewItems = review?.items ?? [];

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
        <div style={{ marginLeft: "auto" }}>
          <button
            type="button"
            disabled={!latestSnapshot}
            onClick={() => latestSnapshot && onOrderReview?.(latestSnapshot.id)}
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
            <EditPane buffer={buffer} onBufferChange={onBufferChange} onCommit={onCommit} />
          ) : (
            <PreviewPane
              buffer={buffer}
              reviewOrdered={reviewOrdered}
              reviewItems={reviewItems}
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
