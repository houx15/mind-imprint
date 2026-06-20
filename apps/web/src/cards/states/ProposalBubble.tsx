type Props = {
  status: "proposed" | "completed" | "skipped";
  category: string;
  name: string;
  nudge: string;
  onOpen: () => void;
  onSkip: () => void;
};

export function ProposalBubble({ status, category, name, nudge, onOpen, onSkip }: Props) {
  return (
    <div className="w-full max-w-[520px] rounded-[5px_16px_16px_16px] border border-mk-border bg-white p-4 shadow-sm">
      <div className="mb-3 flex items-center gap-2.5">
        <span className="rounded-full bg-mk-accent-tint px-2.5 py-1 text-[11px] font-bold text-mk-accent">建议工具卡</span>
        <span className="rounded-full bg-mk-primary-tint px-2.5 py-1 text-[11px] font-semibold text-mk-primary">{category}</span>
      </div>
      <div className="mb-1.5 text-[15px] font-bold text-mk-ink">{name}</div>
      <div className="text-sm leading-relaxed text-[#5B6373]">{nudge}</div>

      {status === "proposed" && (
        <div className="mt-3.5 flex items-center gap-3.5">
          <button type="button" onClick={onOpen} className="rounded-[11px] bg-mk-accent px-5 py-2.5 text-sm font-bold text-white">打开卡</button>
          <button type="button" onClick={onSkip} className="text-[13px] font-semibold text-mk-muted-2">暂不，先继续</button>
        </div>
      )}
      {status === "completed" && (
        <div className="mt-3 inline-flex items-center gap-1.5 rounded-[9px] bg-mk-green-tint px-3 py-1.5 text-[13px] font-semibold text-mk-green">
          已完成 · 已钉到过程树
        </div>
      )}
      {status === "skipped" && (
        <div className="mt-3 flex items-center gap-3">
          <span className="rounded-[9px] bg-[#F1F2F5] px-3 py-1.5 text-[13px] font-semibold text-mk-muted-2">已跳过（已记录为信号）</span>
          <button type="button" onClick={onOpen} className="text-[13px] font-semibold text-mk-primary underline underline-offset-2">仍可打开</button>
        </div>
      )}
    </div>
  );
}
