import { useState } from "react";
import { Modal, toast } from "@/ui";
import { api } from "@/api";

// FeedbackModal — Task 10 · nav rail's 反馈 button. Deliberately minimal: a
// single free-text box, no structure, no rubric weight. Not a card, not a
// process node — a plain landing pad for "something's off" / "I wish this did X".
export function FeedbackModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [text, setText] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const trimmed = text.trim();

  async function handleSubmit() {
    if (!trimmed || submitting) return;
    setSubmitting(true);
    try {
      await api.submitFeedback(trimmed);
      toast("感谢反馈，我们会认真看看。");
      setText("");
      onClose();
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="给我们提反馈"
      footer={
        <button
          type="button"
          onClick={handleSubmit}
          disabled={!trimmed || submitting}
          className="rounded-mk-md bg-mk-accent px-4 py-2 text-mk-body font-semibold text-white disabled:opacity-40 hover:opacity-90 disabled:hover:opacity-40"
        >
          提交
        </button>
      }
    >
      <textarea
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder="有什么想法、卡住的地方，或者哪里不好用，都可以告诉我们……"
        rows={5}
        className="w-full resize-none rounded-mk-md border border-mk-border bg-mk-paper p-3 text-mk-body text-mk-ink placeholder:text-mk-muted focus:outline-none"
      />
    </Modal>
  );
}
