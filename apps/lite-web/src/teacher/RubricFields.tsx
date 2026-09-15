import { Button } from "@/ui";
import type { RubricDimension } from "../api/gradings";
import { Field, INPUT_CLS, Segmented } from "./AssignmentForm";
import type { RubricDraft } from "./rubricLogic";

const MAX_DIMENSIONS = 6;

/**
 * 评分标准 for a writing homework. AI 批改 drafts grades per dimension on this
 * scale. Only rendered once a homework exists (there is no rubric editor on
 * the create form); stays editable after students start, and each grading
 * keeps the rubric it was drafted with.
 */
export function RubricFields({
  value,
  onChange,
}: {
  value: RubricDraft;
  onChange: (update: (d: RubricDraft) => RubricDraft) => void;
}) {
  const setDimension = (index: number, patch: Partial<RubricDimension>) =>
    onChange((d) => ({ ...d, dimensions: d.dimensions.map((x, i) => (i === index ? { ...x, ...patch } : x)) }));

  return (
    <fieldset className="flex flex-col gap-4 rounded-mk-md border border-mk-border p-4">
      <legend className="px-1 text-mk-small font-bold text-mk-ink">评分标准</legend>
      <p className="text-mk-small text-mk-muted">AI 批改按评分标准起草等级和评语；学生开始后仍可修改。</p>
      <Segmented
        label="评分方式"
        options={[
          { value: "letter", label: "等级" },
          { value: "points", label: "分数" },
        ]}
        value={value.scale}
        onChange={(scale) => onChange((d) => ({ ...d, scale }))}
      />
      {value.scale === "points" && (
        <div className="sm:w-[200px]">
          <Field label="满分">
            <input
              type="number"
              min={1}
              max={100}
              step={1}
              inputMode="numeric"
              value={value.max}
              onChange={(e) => onChange((d) => ({ ...d, max: e.target.value }))}
              className={INPUT_CLS}
            />
          </Field>
        </div>
      )}
      <div className="flex flex-col gap-3">
        <span className="text-mk-label font-bold text-mk-muted">维度</span>
        {value.dimensions.map((dim, i) => (
          <div key={i} className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-3 sm:flex-row sm:items-start">
            <div className="sm:w-[220px]">
              <Field label="名称">
                <input value={dim.name} maxLength={40} onChange={(e) => setDimension(i, { name: e.target.value })} className={INPUT_CLS} />
              </Field>
            </div>
            <div className="min-w-0 flex-1">
              <Field label="说明">
                <textarea value={dim.note} rows={2} maxLength={200} onChange={(e) => setDimension(i, { note: e.target.value })} className={INPUT_CLS} />
              </Field>
            </div>
            <Button
              variant="ghost"
              size="sm"
              disabled={value.dimensions.length <= 1}
              onClick={() => onChange((d) => ({ ...d, dimensions: d.dimensions.filter((_, j) => j !== i) }))}
            >
              删除
            </Button>
          </div>
        ))}
        <div>
          <Button
            variant="secondary"
            size="sm"
            disabled={value.dimensions.length >= MAX_DIMENSIONS}
            onClick={() => onChange((d) => ({ ...d, dimensions: [...d.dimensions, { name: "", note: "" }] }))}
          >
            添加维度
          </Button>
        </div>
      </div>
      <Field label="批改重点">
        <textarea value={value.focus} rows={2} maxLength={500} onChange={(e) => onChange((d) => ({ ...d, focus: e.target.value }))} className={INPUT_CLS} />
      </Field>
    </fieldset>
  );
}
