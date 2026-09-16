import { useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import type { ClassSummary } from "@/api";
import { AssignmentStatusChip } from "../inbox/AssignmentStrip";
import { beijingInputToISO, formatDeadline, STATUS_LABEL } from "../shared/deadline";
import type { AssignmentDraft } from "./assignmentLogic";
import { KIND_OPTIONS } from "./labels";

// teacher/StudentViewPreview.tsx — 「学生看到的样子」: a collapsed preview at
// the bottom of the AI-mode homework card, showing how the draft
// will read on the student's inbox row / 作业 strip once published
// (`AssignmentInboxItem`, see `inbox/AssignmentStrip.tsx`). A draft has no
// id, unread flag, atomId or return info yet — those cells are left out
// rather than filled with placeholders. Status is fixed 未开始, since a
// draft has never been started.
//
// Kind label comes from `labels.ts`'s `KIND_OPTIONS` — the same table
// `AssignmentForm.tsx` uses for its 类型 selector, and the one
// `labels_test.go`'s `TestLabelsMatchTheWebTables` pins on the Go side.

function kindLabel(kind: AssignmentDraft["kind"]): string {
  return KIND_OPTIONS.find((o) => o.value === kind)?.label ?? kind;
}

export interface StudentPreviewItem {
  kindLabel: string;
  /** Never blank — an empty draft title shows a plain "未填写标题" placeholder,
   * since this preview is teacher-facing (not shown to a student). */
  title: string;
  instructions: string;
  /** `null` when `dueInput` is empty or not yet a full Beijing timestamp. */
  dueLabel: string | null;
  /** "" when the draft's class can't be found in `classes` (e.g. no class
   * selected yet); the caller leaves the row out in that case. */
  className: string;
}

/** Draft → the preview's read-only data. Pure so it can be tested without
 * rendering (AGENTS.md: only logic tests, no render tests). */
export function draftPreviewItem(draft: AssignmentDraft, classes: ClassSummary[]): StudentPreviewItem {
  const iso = beijingInputToISO(draft.dueInput);
  return {
    kindLabel: kindLabel(draft.kind),
    title: draft.title.trim() || "未填写标题",
    instructions: draft.instructions.trim(),
    dueLabel: iso ? formatDeadline(iso) : null,
    className: classes.find((c) => c.id === draft.classId)?.name ?? "",
  };
}

/**
 * Collapsed by default, sits at the bottom of the AI-mode homework card,
 * above 发布作业. Traditional mode does not get this preview: spec §12.3 puts
 * it on the AI card, and the traditional form stays as it was.
 */
export function StudentViewPreview({ draft, classes }: { draft: AssignmentDraft; classes: ClassSummary[] }) {
  const [open, setOpen] = useState(false);
  const item = draftPreviewItem(draft, classes);
  const Chevron = open ? ChevronDown : ChevronRight;

  return (
    <div className="border-t border-mk-border pt-4">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        className="flex items-center gap-1 text-mk-small font-semibold text-mk-secondary"
      >
        <Chevron size={16} aria-hidden="true" />
        学生看到的样子
      </button>

      {open && (
        <div className="mt-3 rounded-mk-lg border border-mk-border bg-mk-surface p-3 shadow-mk-xs">
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2 px-1 py-1">
            <div className="min-w-0 flex-1">
              <p className="text-mk-label font-semibold text-mk-secondary">{item.kindLabel}</p>
              <p className="mt-0.5 truncate text-mk-body font-semibold text-mk-ink">{item.title}</p>
              {item.instructions && (
                <p className="mt-0.5 line-clamp-2 text-mk-small text-mk-muted">{item.instructions}</p>
              )}
              {item.dueLabel && <p className="mt-0.5 text-mk-small text-mk-muted">截止 {item.dueLabel}</p>}
              {item.className && <p className="mt-0.5 text-mk-small text-mk-muted">{item.className}</p>}
            </div>
            <AssignmentStatusChip status="not_started" label={STATUS_LABEL.not_started} />
          </div>
        </div>
      )}
    </div>
  );
}
