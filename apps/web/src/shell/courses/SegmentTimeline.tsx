import { useEffect, useMemo, useState } from "react";
import type { Interaction, RenderSegment, RenderStepContent, CourseAsset } from "@mind-imprint/contracts";
import { AssetView } from "./AssetView";

// Ported from docs/reference/class-agent/src/main.jsx's buildLessonTimeline /
// renderTimelineItems / selectedAnswerIds / isInteractionCorrect, adapted to
// our simpler RenderStepContent shape: the reference interleaves segments
// against a separate `flowBlocks` plan (teaching/summary/source_submission/
// interaction blocks); our render cache has no such plan, so a segment's own
// `flow_block_id` IS the block id an interaction can target — see
// buildTimeline below.

function cleanId(value: string | undefined | null): string {
  return String(value || "").trim();
}

type TimelineItem =
  | { type: "segment"; key: string; segment: RenderSegment }
  | { type: "interaction"; key: string; interaction: Interaction };

// buildTimeline interleaves `content.interactions` into segment order by
// matching each interaction's `id` against a segment's `flow_block_id`
// (mirrors the reference's resolveInteractionForBlock, minus the
// `block_<id>` alias the reference needs only because its `block.id` is
// itself prefixed — ours never is, since segments carry the id directly).
// Any interaction that matches no segment is appended after all segments,
// mirroring the reference's `missingInteractions` fallback.
export function buildTimeline(content: RenderStepContent): TimelineItem[] {
  const segments = content.segments || [];
  const interactions = content.interactions || [];
  const items: TimelineItem[] = [];
  const used = new Set<string>();

  segments.forEach((segment, index) => {
    items.push({ type: "segment", key: `segment-${index}`, segment });
    const blockId = cleanId(segment.flow_block_id);
    if (!blockId) return;
    const matched = interactions.find((interaction) => {
      const id = cleanId(interaction.id);
      if (!id || used.has(id)) return false;
      return id === blockId || `block_${id}` === blockId;
    });
    if (matched) {
      used.add(cleanId(matched.id));
      items.push({ type: "interaction", key: `interaction-${matched.id}`, interaction: matched });
    }
  });

  interactions.forEach((interaction) => {
    const id = cleanId(interaction.id);
    if (id && used.has(id)) return;
    items.push({ type: "interaction", key: `interaction-tail-${interaction.id}`, interaction });
  });

  return items;
}

// splitTeachingTextAroundAssets ported from the reference: a long teaching
// paragraph reads better with its assets dropped in mid-flow, at the first
// sentence that cues "look at this" rather than always after the whole
// paragraph. Short text or text with no assets to place is left whole.
function splitTeachingTextAroundAssets(text: string, hasAssets: boolean): { before: string; after: string } {
  if (!hasAssets || !text || text.length < 120) return { before: text, after: "" };
  const sentences = text
    .split(/(?<=[。!?！？])/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (sentences.length < 3) return { before: text, after: "" };
  const cueIndex = sentences.findIndex(
    (sentence, index) => index < sentences.length - 1 && /(请看|先看|来看|看这|观察|截图|图表|材料|页面|链接)/.test(sentence)
  );
  const splitIndex = cueIndex >= 0 ? cueIndex + 1 : Math.min(2, sentences.length - 1);
  return {
    before: sentences.slice(0, splitIndex).join(""),
    after: sentences.slice(splitIndex).join(""),
  };
}

// orderMatches / isInteractionCorrect ported verbatim (semantics-wise) from
// the reference's same-named functions — exported so Task 10 (and this
// file's own tests) can unit-test answer-checking without mounting a
// component.
export function orderMatches(order: string[], correctAnswer: string[]): boolean {
  return order.length === correctAnswer.length && correctAnswer.every((id, index) => id === order[index]);
}

export function isInteractionCorrect(interaction: Interaction, answerIds: string[]): boolean {
  const expected: string[] = interaction.correct_answer || [];
  if (interaction.type === "ordering") return orderMatches(answerIds, expected);
  const answerSet = new Set(answerIds);
  return expected.length === answerSet.size && expected.every((id: string) => answerSet.has(id));
}

// `assets` is resolved and de-duplicated by the parent SegmentTimeline (an
// image referenced by many segments in a step is shown only on its first
// occurrence — mirrors the reference's `displayedAssetIds` set in main.jsx),
// so SegmentBlock just renders whatever it's handed.
function SegmentBlock({ segment, assets }: { segment: RenderSegment; assets: CourseAsset[] }) {
  if (segment.kind === "structure") {
    const items = (segment.items || []).filter((item) => item.label || item.text);
    return (
      <section
        aria-label="板书"
        style={{
          marginBottom: 20,
          background: "linear-gradient(135deg,#1C2333 0%,#2A3B7A 100%)",
          borderRadius: 16,
          padding: "20px 22px",
        }}
      >
        {segment.text && <p style={{ margin: "0 0 12px", fontSize: 13.5, lineHeight: 1.7, color: "#D8DCEE" }}>{segment.text}</p>}
        {items.length > 0 && (
          <div style={{ display: "grid", gridTemplateColumns: `repeat(${Math.min(items.length, 3)}, 1fr)`, gap: 12 }}>
            {items.map((item, index) => (
              <div
                key={`${item.label}-${index}`}
                style={{ background: "rgba(255,255,255,0.08)", borderRadius: 10, padding: "12px 14px" }}
              >
                {item.label && (
                  <span style={{ display: "inline-block", fontSize: 11, fontWeight: 700, color: "#B7C1E8", letterSpacing: ".03em", marginBottom: 6 }}>
                    {item.label}
                  </span>
                )}
                {item.text && <p style={{ margin: 0, fontSize: 13.5, lineHeight: 1.65, color: "#fff" }}>{item.text}</p>}
              </div>
            ))}
          </div>
        )}
      </section>
    );
  }

  const { before, after } = splitTeachingTextAroundAssets(segment.text || "", assets.length > 0);

  return (
    <section style={{ marginBottom: 20 }}>
      {before && <p style={{ margin: "0 0 12px", fontSize: 15.5, lineHeight: 1.85, color: "#2B3346" }}>{before}</p>}
      {assets.length > 0 && (
        <div style={{ display: "flex", flexDirection: assets.length > 1 ? "row" : "column", gap: 12, margin: "0 0 12px", flexWrap: "wrap" }}>
          {assets.map((asset) => (
            <div key={asset.id} style={{ flex: assets.length > 1 ? "1 1 220px" : "none" }}>
              <AssetView asset={asset} />
            </div>
          ))}
        </div>
      )}
      {after && <p style={{ margin: 0, fontSize: 15.5, lineHeight: 1.85, color: "#2B3346" }}>{after}</p>}
    </section>
  );
}

export type QuizAnswerEvent = { stepId: string; interactionId: string; selected: string[]; correct: boolean };

function InteractionBlock({
  interaction,
  stepId,
  onQuizAnswer,
}: {
  interaction: Interaction;
  stepId: string;
  onQuizAnswer: (event: QuizAnswerEvent) => void;
}) {
  // `Interaction` is a recursive Zod type declared `z.ZodType<any>` in
  // contracts (remediation_questions references itself), so its TS inference
  // collapses to `any` — pin the two fields this component reads to their
  // authored shape once, here, so every use below type-checks normally
  // instead of re-triggering noImplicitAny at each call site.
  const options: { id: string; text: string }[] = interaction.options || [];
  const correctAnswer: string[] = interaction.correct_answer || [];

  const isOrdering = interaction.type === "ordering";
  const [selected, setSelected] = useState<string[]>([]);
  const [order, setOrder] = useState<string[]>(() => options.map((o) => o.id));
  const [submitted, setSubmitted] = useState(false);
  const [correct, setCorrect] = useState(false);

  // A fresh interaction (different id) resets local answer state — otherwise
  // a remediation question or the next quiz on the page would inherit the
  // previous one's picks.
  useEffect(() => {
    setSelected([]);
    setOrder(options.map((o) => o.id));
    setSubmitted(false);
    setCorrect(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [interaction.id]);

  function toggleOption(optionId: string) {
    if (submitted) return;
    if (interaction.type === "single_choice") {
      setSelected([optionId]);
    } else {
      setSelected((prev) => (prev.includes(optionId) ? prev.filter((id) => id !== optionId) : [...prev, optionId]));
    }
  }

  function moveOption(index: number, direction: -1 | 1) {
    if (submitted) return;
    setOrder((prev) => {
      const target = index + direction;
      if (target < 0 || target >= prev.length) return prev;
      const next = [...prev];
      const tmp = next[index]!;
      next[index] = next[target]!;
      next[target] = tmp;
      return next;
    });
  }

  function handleSubmit() {
    const answerIds = isOrdering ? order : selected;
    const result = isInteractionCorrect(interaction, answerIds);
    setCorrect(result);
    setSubmitted(true);
    onQuizAnswer({ stepId, interactionId: interaction.id, selected: answerIds, correct: result });
  }

  const canSubmit = isOrdering ? order.length > 0 : selected.length > 0;
  const optionById = new Map(options.map((o): [string, string] => [o.id, o.text]));

  return (
    <section
      style={{ marginBottom: 20, background: "#fff", border: "1px solid #ECEEF3", borderRadius: 14, padding: "16px 18px" }}
    >
      <div style={{ fontSize: 15, fontWeight: 700, color: "#1C2333", marginBottom: 12 }}>{interaction.prompt}</div>

      {isOrdering ? (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {order.map((optionId, index) => (
            <div
              key={optionId}
              style={{
                display: "flex", alignItems: "center", gap: 10, border: "1px solid #E1E4ED", borderRadius: 10,
                padding: "9px 12px", background: submitted ? "#F6F7FA" : "#fff",
              }}
            >
              <span style={{ fontSize: 13.5, fontWeight: 700, color: "#6B7384", width: 18 }}>{index + 1}.</span>
              <span style={{ flex: 1, fontSize: 14, color: "#1C2333" }}>{optionById.get(optionId) || optionId}</span>
              <button
                type="button"
                aria-label="上移"
                disabled={submitted || index === 0}
                onClick={() => moveOption(index, -1)}
                style={{
                  border: "1px solid #E1E4ED", background: "#fff", borderRadius: 6, width: 26, height: 26,
                  cursor: submitted || index === 0 ? "default" : "pointer", opacity: submitted || index === 0 ? 0.4 : 1,
                }}
              >
                ↑
              </button>
              <button
                type="button"
                aria-label="下移"
                disabled={submitted || index === order.length - 1}
                onClick={() => moveOption(index, 1)}
                style={{
                  border: "1px solid #E1E4ED", background: "#fff", borderRadius: 6, width: 26, height: 26,
                  cursor: submitted || index === order.length - 1 ? "default" : "pointer",
                  opacity: submitted || index === order.length - 1 ? 0.4 : 1,
                }}
              >
                ↓
              </button>
            </div>
          ))}
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {options.map((option) => {
            const isSelected = selected.includes(option.id);
            const isCorrectOption = correctAnswer.includes(option.id);
            let indicator = "";
            let color = "#1C2333";
            let border = "#E1E4ED";
            let background = "#fff";
            if (submitted) {
              if (isCorrectOption) {
                indicator = " ✓";
                color = "#2B4A3E";
                border = "#D3E9DF";
                background = "#E7F3EE";
              } else if (isSelected) {
                indicator = " ✗";
                color = "#7A3B2E";
                border = "#F3D9CE";
                background = "#FBEEE7";
              }
            } else if (isSelected) {
              border = "#2A3B7A";
              background = "#EDEFF9";
            }
            return (
              <button
                key={option.id}
                type="button"
                disabled={submitted}
                onClick={() => toggleOption(option.id)}
                style={{
                  textAlign: "left", border: `1px solid ${border}`, background, color, borderRadius: 10,
                  padding: "10px 13px", fontSize: 14, cursor: submitted ? "default" : "pointer", fontFamily: "inherit",
                }}
              >
                {option.text}
                {indicator}
              </button>
            );
          })}
        </div>
      )}

      {!submitted ? (
        <button
          type="button"
          disabled={!canSubmit}
          onClick={handleSubmit}
          style={{
            marginTop: 12, background: canSubmit ? "#2A3B7A" : "#C7CBDA", color: "#fff", border: "none",
            padding: "9px 16px", borderRadius: 9, fontSize: 13.5, fontWeight: 700,
            cursor: canSubmit ? "pointer" : "default", fontFamily: "inherit",
          }}
        >
          提交
        </button>
      ) : (
        <div
          style={{
            marginTop: 12, background: correct ? "#E7F3EE" : "#FBEEE7", border: `1px solid ${correct ? "#D3E9DF" : "#F3D9CE"}`,
            borderRadius: 10, padding: "11px 14px", fontSize: 13.5, lineHeight: 1.7, color: correct ? "#2B4A3E" : "#7A3B2E",
          }}
        >
          <div style={{ fontWeight: 700, marginBottom: interaction.explanation ? 4 : 0 }}>{correct ? "✓ 回答正确" : "✗ 再想想"}</div>
          {interaction.explanation && <div>{interaction.explanation}</div>}
        </div>
      )}
    </section>
  );
}

export function SegmentTimeline({
  content,
  assetsById,
  stepId,
  onQuizAnswer,
}: {
  content: RenderStepContent;
  assetsById: Record<string, CourseAsset>;
  stepId: string;
  onQuizAnswer: (event: QuizAnswerEvent) => void;
}) {
  const items = useMemo(() => buildTimeline(content), [content]);
  // Reveal-on-click (铁律 3 一次只问一个 in spirit — the page paces itself
  // rather than dumping the whole step at once): starts revealing just the
  // first item, same as the reference's `revealedSegmentCount`.
  const [revealedCount, setRevealedCount] = useState(1);

  useEffect(() => {
    setRevealedCount(1);
  }, [content]);

  const visibleItems = items.slice(0, revealedCount);
  const hasMore = revealedCount < items.length;

  function handleReveal(event: React.MouseEvent<HTMLDivElement>) {
    if (!hasMore) return;
    const target = event.target as HTMLElement;
    if (target.closest("button, a, input, textarea, select")) return;
    setRevealedCount((value) => Math.min(value + 1, items.length));
  }

  // De-duplicate assets across the whole step: an image referenced by many
  // segments (the render cache does this — e.g. doc_img_06 in 10 segments)
  // must render only on its first occurrence. `shown` is rebuilt each render
  // and the map callback runs synchronously in order, so the first segment to
  // reference an id wins deterministically (mirrors main.jsx's
  // assetsForSegment/displayedAssetIds).
  const shown = new Set<string>();

  return (
    <div data-testid="segment-timeline" onClick={handleReveal} style={{ cursor: hasMore ? "pointer" : "default" }}>
      {visibleItems.map((item) => {
        if (item.type !== "segment") {
          return <InteractionBlock key={item.key} interaction={item.interaction} stepId={stepId} onQuizAnswer={onQuizAnswer} />;
        }
        const assets = (item.segment.asset_ids || [])
          .map((id) => assetsById[id])
          .filter((a): a is CourseAsset => Boolean(a))
          .filter((a) => {
            if (shown.has(a.id)) return false;
            shown.add(a.id);
            return true;
          });
        return <SegmentBlock key={item.key} segment={item.segment} assets={assets} />;
      })}
      {hasMore && (
        <div aria-hidden="true" style={{ textAlign: "center", fontSize: 12.5, fontWeight: 600, color: "#AEB4C2", padding: "6px 0 18px" }}>
          点击页面继续
        </div>
      )}
    </div>
  );
}
