// PlacementPicker — 来源归位选择器 (铁律②: 印记 suggests, the student taps). Main
// questions are top-level chips; sub-questions indent under their parent. The
// 印记-suggested question is pre-highlighted with its one-line reason. A final
// 「先放进未归类」 lands the source in the 未归类 node instead. Pure/presentational;
// shared by the add-time modal (ReadingBlock) and the 未归类 panel (ExplorationView).

export type PlacementQuestion = { id: string; text: string; parentId: string | null };

export function PlacementPicker({
  questions,
  suggestedLeadId,
  reason,
  busy,
  onPick,
}: {
  questions: PlacementQuestion[];
  suggestedLeadId: string | null;
  reason: string;
  busy?: boolean;
  onPick: (leadId: string | null) => void;
}) {
  // Group: roots in order, each followed by its sub-questions.
  const roots = questions.filter((q) => q.parentId == null);
  const subsOf = (id: string) => questions.filter((q) => q.parentId === id);

  const row = (q: PlacementQuestion, indented: boolean) => {
    const suggested = q.id === suggestedLeadId;
    return (
      <button
        key={q.id}
        type="button"
        data-suggested={suggested ? "true" : "false"}
        disabled={busy}
        onClick={() => onPick(q.id)}
        className={
          "flex w-full items-center gap-2 rounded-mk border px-3 py-2 text-left text-[13.5px] transition disabled:opacity-50 " +
          (suggested ? "border-mk-accent bg-mk-accent-50 text-mk-ink" : "border-mk-border bg-mk-surface text-mk-ink hover:border-mk-accent") +
          (indented ? " ml-4" : "")
        }
      >
        <span className="min-w-0 flex-1 truncate">{q.text}</span>
        {suggested && <span className="flex-none rounded-full bg-mk-accent px-2 py-0.5 text-[11px] font-bold text-white">印记建议</span>}
      </button>
    );
  };

  return (
    <div className="flex flex-col gap-2">
      {suggestedLeadId && reason && (
        <p className="rounded-mk bg-mk-accent-50 px-3 py-2 text-[12.5px] leading-relaxed text-mk-muted">
          印记：<span>{reason}</span>
        </p>
      )}
      {roots.length === 0 && (
        <p className="text-[12.5px] text-mk-faint">还没有研究问题——先把它放进未归类，之后再挂。</p>
      )}
      <div className="flex flex-col gap-1.5">
        {roots.map((r) => (
          <div key={r.id} className="flex flex-col gap-1.5">
            {row(r, false)}
            {subsOf(r.id).map((s) => row(s, true))}
          </div>
        ))}
      </div>
      <button
        type="button"
        disabled={busy}
        onClick={() => onPick(null)}
        className="mt-1 rounded-mk border border-dashed border-mk-border px-3 py-2 text-[13px] font-semibold text-mk-muted hover:border-mk-accent hover:text-mk-accent disabled:opacity-50"
      >
        先放进未归类
      </button>
    </div>
  );
}
