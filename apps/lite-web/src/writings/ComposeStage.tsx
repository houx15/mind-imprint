import { useEffect, useState } from "react";
import { Layers, MessageSquareText, Check } from "lucide-react";
import { Button, Icon } from "@/ui";
import { ApiError } from "../api/client";
import {
  composeWritingDraft,
  putWritingDraft,
  reviewWritingDraft,
  finishWriting,
  type WritingDraft,
} from "../api/writingRoom";
import type { Writing } from "../api/writings";

/**
 * ComposeStage — 成稿: "compose → the student can keep editing → request
 * feedback." `composeWritingDraft` is a deterministic assembly of her own
 * already-written snippets (writing_compose.go's `composeSnippetsIntoDraft`
 * — string concatenation, no model call); everything past that point is her
 * own editing. `reviewWritingDraft` returns commentary only — it is never
 * written back into the textarea, so a review can never silently rewrite
 * what she has.
 */

export function ComposeStage({
  writingId,
  draft,
  onDraftChange,
  onFinished,
}: {
  writingId: string;
  draft: WritingDraft;
  onDraftChange: (next: WritingDraft) => void;
  onFinished: (writing: Writing) => void;
}) {
  const [body, setBody] = useState(draft.body);
  const [composing, setComposing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [reviewing, setReviewing] = useState(false);
  const [finishing, setFinishing] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => setBody(draft.body), [draft.body]);

  async function compose() {
    setComposing(true);
    setError(null);
    try {
      const next = await composeWritingDraft(writingId);
      onDraftChange(next);
      setBody(next.body);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "拼合失败，请重试。");
    } finally {
      setComposing(false);
    }
  }

  async function save() {
    if (body === draft.body) return;
    setSaving(true);
    setError(null);
    try {
      const next = await putWritingDraft(writingId, body);
      onDraftChange(next);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "保存失败，请重试。");
    } finally {
      setSaving(false);
    }
  }

  async function review() {
    setReviewing(true);
    setError(null);
    setFeedback(null);
    try {
      await save();
      const { feedback: text } = await reviewWritingDraft(writingId);
      setFeedback(text);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "这次体检没成功，请重试。");
    } finally {
      setReviewing(false);
    }
  }

  async function finish() {
    setFinishing(true);
    setError(null);
    try {
      await save();
      const writing = await finishWriting(writingId);
      onFinished(writing);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "完成失败，请重试。");
    } finally {
      setFinishing(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-mk-h2 text-mk-ink">成稿</h2>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => void compose()}
          loading={composing}
          iconStart={<Icon icon={Layers} size={14} />}
        >
          从段落拼出初稿
        </Button>
      </div>

      <textarea
        value={body}
        onChange={(e) => setBody(e.target.value)}
        onBlur={() => void save()}
        placeholder="拼出来的初稿会出现在这里——你也可以直接在这儿写、改。"
        className="min-h-[320px] w-full resize-y rounded-mk-md border border-mk-input-border bg-mk-surface px-4 py-3 text-mk-body-lg text-mk-ink outline-none placeholder:text-[#B8ADA2] focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      />
      {saving && <span className="text-mk-label text-mk-faint">保存中…</span>}
      {error && <p className="text-mk-small text-mk-danger">{error}</p>}

      <div className="flex items-center gap-2">
        <Button
          variant="secondary"
          onClick={() => void review()}
          loading={reviewing}
          disabled={!body.trim()}
          iconStart={<Icon icon={MessageSquareText} size={14} />}
        >
          请印记看看
        </Button>
        <Button onClick={() => void finish()} loading={finishing} disabled={!body.trim()} iconStart={<Icon icon={Check} size={14} />}>
          完成这篇
        </Button>
      </div>

      {feedback && (
        <div className="rounded-mk-md border border-mk-border bg-mk-paper p-4">
          <h3 className="mb-2 text-mk-label text-mk-faint">印记的反馈</h3>
          <p className="whitespace-pre-wrap text-mk-body text-mk-ink">{feedback}</p>
        </div>
      )}
    </div>
  );
}
