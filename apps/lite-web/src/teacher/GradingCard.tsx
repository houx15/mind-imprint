import { useEffect, useRef, useState } from "react";
import { Button } from "@/ui";
import { LETTER_GRADES, type GradingContent, type GradingPoint, type Rubric } from "../api/gradings";
import { NumberField } from "./controls/NumberField";
import { Select } from "./controls/Select";
import { INPUT_CLS } from "./formParts";
import { gradingContentReducer, gradingPointLabel, pointHasBasis, previewValue, type GradingMode } from "./gradingLogic";

/**
 * GradingCard — the right column of `GradingPage`: one card holding the
 * grading itself, with two views.
 *
 * 编辑 is the form (grades, comments, points). 预览 lays the same content out
 * as running text, the way the student reads it in her finished writing —
 * which is what a teacher opening an already-sent grading actually wants
 * (2026-09-18). Both views read the SAME `content` state, so 预览 shows
 * unsaved edits too; neither view is a separate copy of the grading.
 *
 * A quote in either view scrolls the essay on the left to that quote
 * (`onShowQuote`), so the two columns stay one screen.
 */
export function GradingCard({
  rubric,
  content,
  mode,
  onMode,
  unmarked,
  onEdit,
  onPickQuote,
  onShowQuote,
}: {
  rubric: Rubric;
  content: GradingContent;
  mode: GradingMode;
  onMode: (m: GradingMode) => void;
  unmarked: ReadonlySet<number>;
  onEdit: (a: Parameters<typeof gradingContentReducer>[1]) => void;
  onPickQuote: (index: number) => void;
  onShowQuote: (index: number) => void;
}) {
  // The two views are different lengths, so the scroll offset from the one
  // she left means nothing in the one she opened: switching views would drop
  // her into the middle of a sentence.
  const body = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (body.current) body.current.scrollTop = 0;
  }, [mode]);

  // Which point's 依据 modal is open, by index — null when none is. Shared
  // across both views: whichever one is mounted opens the same modal.
  const [basisIndex, setBasisIndex] = useState<number | null>(null);
  const basisPoint = basisIndex !== null ? content.points[basisIndex] : undefined;

  return (
    <div className="teacher-grading-card min-h-0 min-[900px]:flex-1">
      <header className="teacher-grading-card-head">
        <h2 className="teacher-grading-h">批改内容</h2>
        <div className="teacher-grading-tabs" role="group" aria-label="批改视图">
          {(
            [
              ["edit", "编辑"],
              ["preview", "预览"],
            ] as const
          ).map(([value, label]) => (
            <button key={value} type="button" aria-pressed={mode === value} onClick={() => onMode(value)}>
              {label}
            </button>
          ))}
        </div>
      </header>
      <div ref={body} className="teacher-grading-editor mk-scroll min-h-0 min-[900px]:flex-1">
        {mode === "preview" ? (
          <GradingPreview content={content} unmarked={unmarked} onShowQuote={onShowQuote} onShowBasis={setBasisIndex} />
        ) : (
          <GradingEditor
            rubric={rubric}
            content={content}
            unmarked={unmarked}
            onEdit={onEdit}
            onPickQuote={onPickQuote}
            onShowQuote={onShowQuote}
            onShowBasis={setBasisIndex}
          />
        )}
      </div>
      {basisPoint && <PointBasisDialog point={basisPoint} onClose={() => setBasisIndex(null)} />}
    </div>
  );
}

/**
 * PointBasisDialog — 依据: where one point came from (维度/对应毛病/学生原句),
 * for the teacher who otherwise has no way to tell. Centred overlay + Esc to
 * close, same pattern as ReturnDialog.tsx.
 *
 * Only rendered when `pointHasBasis` is true, so every row here is filled —
 * no "待填写"/em-dash placeholders, unlike 预览's Value.
 */
function PointBasisDialog({ point, onClose }: { point: GradingPoint; onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="point-basis-title"
        className="flex max-h-full w-full max-w-[440px] flex-col gap-4 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
      >
        <h2 id="point-basis-title" className="text-mk-h2 text-mk-ink">
          依据
        </h2>
        <dl className="flex flex-col gap-3">
          {point.dimension && (
            <div className="flex flex-col gap-1">
              <dt className="text-mk-small text-mk-muted">维度</dt>
              <dd className="text-mk-body text-mk-ink">{point.dimension}</dd>
            </div>
          )}
          {point.symptom && (
            <div className="flex flex-col gap-1">
              <dt className="text-mk-small text-mk-muted">对应毛病</dt>
              <dd className="text-mk-body text-mk-ink">{point.symptom}</dd>
            </div>
          )}
          {point.quote && (
            <div className="flex flex-col gap-1">
              <dt className="text-mk-small text-mk-muted">学生原句</dt>
              <dd className="whitespace-pre-wrap text-mk-body text-mk-ink">「{point.quote}」</dd>
            </div>
          )}
        </dl>
        <div className="flex justify-end">
          <Button variant="ghost" size="sm" onClick={onClose}>
            关闭
          </Button>
        </div>
      </div>
    </div>
  );
}

function GradeInput({ rubric, value, onChange, label }: { rubric: Rubric; value: string; onChange: (v: string) => void; label: string }) {
  if (rubric.scale === "letter") {
    return (
      <Select
        ariaLabel={label}
        className="!w-28"
        value={value}
        placeholder="—"
        onChange={onChange}
        options={[
          ...(value && !(LETTER_GRADES as readonly string[]).includes(value) ? [{ value, label: value }] : []),
          ...LETTER_GRADES.map((g) => ({ value: g as string, label: g })),
        ]}
      />
    );
  }
  return <NumberField ariaLabel={label} min={0} max={rubric.max} value={value} onChange={onChange} />;
}

/** One value in 预览: her text, or 待填写 in muted type when it is blank. */
function Value({ value, className = "" }: { value: string | null; className?: string }) {
  const v = previewValue(value);
  return v.filled ? (
    <span className={`whitespace-pre-wrap text-mk-body text-mk-ink ${className}`}>{v.text}</span>
  ) : (
    <span className={`text-mk-small text-mk-muted ${className}`}>{v.text}</span>
  );
}

function GradingPreview({
  content,
  unmarked,
  onShowQuote,
  onShowBasis,
}: {
  content: GradingContent;
  unmarked: ReadonlySet<number>;
  onShowQuote: (index: number) => void;
  onShowBasis: (index: number) => void;
}) {
  const grade = previewValue(content.overall.grade);
  return (
    <div className="teacher-grading-preview">
      <section className="flex flex-col gap-2">
        <div className="flex flex-wrap items-baseline gap-3">
          <span className={grade.filled ? "text-mk-h2 font-bold text-mk-ink" : "text-mk-body text-mk-muted"}>{grade.text}</span>
          <span className="text-mk-small text-mk-muted">总评</span>
        </div>
        <Value value={content.overall.comment} />
      </section>

      {content.dimensions.length > 0 && (
        <section className="flex flex-col gap-2">
          <h3 className="teacher-grading-preview-h">维度</h3>
          <dl className="teacher-grading-dims">
            {content.dimensions.map((d) => (
              <div key={d.name} className="contents">
                <dt className="text-mk-small text-mk-muted">{d.name}</dt>
                <dd className="text-mk-small font-bold text-mk-ink">{previewValue(d.grade).text}</dd>
                <dd className="min-w-0">
                  <Value value={d.comment} className="!text-mk-small" />
                </dd>
              </div>
            ))}
          </dl>
        </section>
      )}

      <section className="flex flex-col gap-2">
        <h3 className="teacher-grading-preview-h">意见</h3>
        {content.points.length === 0 ? (
          <p className="text-mk-small text-mk-muted">没有意见</p>
        ) : (
          <ul className="flex flex-col gap-3">
            {content.points.map((p, i) => (
              <li key={i} className="teacher-grading-point">
                <p className="flex flex-wrap items-center gap-2">
                  <span className="teacher-grading-kind" data-kind={p.kind}>
                    {p.kind === "good" ? "优点" : "问题"}
                  </span>
                  <span className="text-mk-label text-mk-muted">{p.source === "ai" ? "AI" : "老师"}</span>
                  {pointHasBasis(p) && (
                    <button type="button" onClick={() => onShowBasis(i)} className="text-mk-label text-mk-secondary underline">
                      依据
                    </button>
                  )}
                </p>
                {p.quote && (
                  <button type="button" onClick={() => onShowQuote(i)} className="text-left text-mk-small text-mk-secondary underline">
                    「{p.quote}」
                  </button>
                )}
                {p.quote && unmarked.has(i) && <p className="text-mk-label text-mk-danger">未在正文中标出</p>}
                <Value value={p.text} className="!text-mk-small" />
                {p.kind === "issue" && p.action && p.action.trim() !== "" && (
                  <p className="whitespace-pre-wrap text-mk-small text-mk-ink">修改建议：{p.action.trim()}</p>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function GradingEditor({
  rubric,
  content,
  unmarked,
  onEdit,
  onPickQuote,
  onShowQuote,
  onShowBasis,
}: {
  rubric: Rubric;
  content: GradingContent;
  unmarked: ReadonlySet<number>;
  onEdit: (a: Parameters<typeof gradingContentReducer>[1]) => void;
  onPickQuote: (index: number) => void;
  onShowQuote: (index: number) => void;
  onShowBasis: (index: number) => void;
}) {
  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-2">
        <h3 className="teacher-grading-preview-h">总评</h3>
        <GradeInput rubric={rubric} label="总评等级" value={content.overall.grade} onChange={(v) => onEdit({ type: "overallGrade", value: v })} />
        <textarea
          aria-label="总评评语"
          rows={3}
          value={content.overall.comment}
          onChange={(e) => onEdit({ type: "overallComment", value: e.target.value })}
          className={INPUT_CLS}
        />
      </section>

      <section className="flex flex-col gap-2">
        <h3 className="teacher-grading-preview-h">维度</h3>
        {content.dimensions.map((d, i) => (
          <div key={d.name} className="flex flex-col gap-1.5">
            <h4 className="text-mk-small font-semibold text-mk-ink">{d.name}</h4>
            <GradeInput rubric={rubric} label={`${d.name}等级`} value={d.grade} onChange={(v) => onEdit({ type: "dimensionGrade", index: i, value: v })} />
            <textarea
              aria-label={`${d.name}评语`}
              rows={2}
              value={d.comment}
              onChange={(e) => onEdit({ type: "dimensionComment", index: i, value: e.target.value })}
              className={INPUT_CLS}
            />
          </div>
        ))}
      </section>

      <section className="flex flex-col gap-3">
        <h3 className="teacher-grading-preview-h">意见</h3>
        {content.points.map((p, i) => (
          <div key={i} className="teacher-grading-point">
            <div className="flex flex-wrap items-center gap-2">
              <Select
                ariaLabel={gradingPointLabel(i, "类型")}
                size="sm"
                className="!min-w-[104px]"
                value={p.kind}
                onChange={(v) => onEdit({ type: "pointKind", index: i, value: v })}
                options={[
                  { value: "good", label: "优点" },
                  { value: "issue", label: "问题" },
                ]}
              />
              <span className="text-mk-label text-mk-muted">{p.source === "ai" ? "AI" : "老师"}</span>
              {pointHasBasis(p) && (
                <Button variant="link" size="sm" aria-label={gradingPointLabel(i, "依据")} onClick={() => onShowBasis(i)}>
                  依据
                </Button>
              )}
              <Button variant="ghost" size="sm" aria-label={gradingPointLabel(i, "删除")} onClick={() => onEdit({ type: "deletePoint", index: i })}>
                删除
              </Button>
            </div>
            <div className="flex flex-wrap items-center gap-2 text-mk-small">
              <span className="text-mk-muted">引文</span>
              {p.quote ? (
                <button type="button" onClick={() => onShowQuote(i)} className="min-w-0 text-left text-mk-ink underline">
                  「{p.quote}」
                </button>
              ) : (
                <span className="text-mk-muted">—</span>
              )}
              <Button variant="link" size="sm" aria-label={gradingPointLabel(i, "选择引文")} onClick={() => onPickQuote(i)}>
                选择引文
              </Button>
              {p.quote && (
                <Button
                  variant="link"
                  size="sm"
                  aria-label={gradingPointLabel(i, "清除引文")}
                  onClick={() => onEdit({ type: "pointQuote", index: i, value: null })}
                >
                  清除
                </Button>
              )}
            </div>
            {p.quote && unmarked.has(i) && <p className="text-mk-small text-mk-danger">未在正文中标出</p>}
            <textarea
              aria-label={gradingPointLabel(i, "说明")}
              rows={2}
              value={p.text}
              onChange={(e) => onEdit({ type: "pointText", index: i, value: e.target.value })}
              className={INPUT_CLS}
            />
            {p.kind === "issue" && (
              <textarea
                aria-label={gradingPointLabel(i, "修改建议")}
                placeholder="修改建议"
                rows={2}
                value={p.action ?? ""}
                onChange={(e) => onEdit({ type: "pointAction", index: i, value: e.target.value })}
                className={INPUT_CLS}
              />
            )}
          </div>
        ))}
        <div>
          <Button variant="secondary" size="sm" onClick={() => onEdit({ type: "addPoint" })}>
            添加意见
          </Button>
        </div>
      </section>
    </div>
  );
}
