import { useState } from "react";
import { Button } from "@/ui";
import { ApiError } from "../api/client";
import { setWritingTargetWords } from "../api/writingRoom";
import type { Writing } from "../api/writings";

/**
 * IdeateStage — 构思: the main panel is deliberately thin. The brief's own
 * shape for this stage is "talk first" — the coach rail (always visible,
 * every stage) IS the 构思 surface. This panel only owns the one thing that
 * has nowhere else to live: 目标字数, which — per writing_stage.go's file
 * comment — "is NEVER a precondition for anything… may be set at ANY stage
 * and may stay NULL forever". No button here jumps her anywhere; the stage
 * map (always visible above) already owns navigation.
 */
export function IdeateStage({
  writingId,
  writing,
  onChange,
}: {
  writingId: string;
  writing: Writing;
  onChange: (next: Writing) => void;
}) {
  const [value, setValue] = useState(writing.targetWords != null ? String(writing.targetWords) : "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function save() {
    const n = Number(value);
    if (!Number.isFinite(n) || n <= 0) return;
    setSaving(true);
    setError(null);
    try {
      const next = await setWritingTargetWords(writingId, Math.round(n));
      onChange(next);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败，请重试。");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <h2 className="text-mk-h2 text-mk-ink">构思</h2>
      <p className="text-mk-body text-mk-muted">跟印记聊聊你想写什么、想说服谁、有什么例子——想清楚了，再去列大纲。</p>

      <div className="flex flex-col gap-2 rounded-mk-md border border-mk-border bg-mk-surface p-4">
        <span className="text-mk-label text-mk-faint">目标字数（可以不定，随时能改）</span>
        <div className="flex items-center gap-2">
          <input
            type="number"
            min={1}
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="比如 800"
            className="w-32 rounded-mk-xs border border-mk-input-border bg-mk-paper px-3 py-2 text-mk-body text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          />
          <Button variant="secondary" size="sm" onClick={() => void save()} loading={saving} disabled={!value.trim()}>
            定下来
          </Button>
        </div>
        {error && <p className="text-mk-small text-mk-danger">{error}</p>}
      </div>
    </div>
  );
}
