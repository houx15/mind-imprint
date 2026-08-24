// WriteModeChoiceMock — a STATIC, non-interactive replica of the write-mode
// choice chat action (`StudioTurnChips`'s `chatAction` block in
// `apps/web/src/studio/ai/StudioCoachChat.tsx:203-221`), used only inside the
// guided tour's `demoModal` step (mirrors `QuestionCardMock`, §Task 2's
// pattern). No handlers, no state, no LLM calls — it exists purely so a new
// student can see 印记 propose the write-mode choice before ever hitting the
// real one in a project's writing room.
//
// 铁律②: AI 绝不替学生定论 — 印记 PROPOSES the choice, the student picks. The
// mock deliberately shows both options with neither pre-selected, matching
// the real `chatAction` rendering exactly (same container/text/button
// classes) so nothing here teaches a different visual language than the
// student will actually meet.

export function WriteModeChoiceMock() {
  return (
    <div className="rounded-mk-lg border border-mk-border bg-mk-surface p-3">
      <p className="whitespace-pre-wrap text-[14px] leading-relaxed text-mk-ink">
        这一部分，你想自己写，还是我一步步带你写？
      </p>
      <div className="mt-2.5 flex flex-wrap gap-2">
        <button
          type="button"
          className="rounded-mk-md border border-mk-border px-3.5 py-1.5 text-[14px] font-bold text-mk-ink hover:border-mk-accent"
        >
          我自己写
        </button>
        <button
          type="button"
          className="rounded-mk-md bg-mk-accent px-4 py-1.5 text-[14px] font-bold text-white hover:bg-mk-accent-600"
        >
          一步步带我写
        </button>
      </div>
    </div>
  );
}
