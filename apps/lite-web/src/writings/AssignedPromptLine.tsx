import { assignedPromptOf, type Writing } from "../api/writings";

/**
 * 题目：… under a writing's title, for a writing started from an assignment.
 * The prompt is the teacher's text, so it is shown as context in the header and
 * never as a message of hers. Renders nothing for a writing she opened herself.
 *
 * Shared by the room header and the 结构 page header (PlanningView), which an
 * assigned writing opens on first. Clamped to two lines; the full prompt is in
 * the title attribute.
 */
export function AssignedPromptLine({ writing }: { writing: Pick<Writing, "assignedPrompt"> }) {
  const prompt = assignedPromptOf(writing);
  if (!prompt) return null;
  return (
    <p className="mt-0.5 line-clamp-2 max-w-[640px] text-mk-small text-mk-muted" title={prompt}>
      题目：{prompt}
    </p>
  );
}
