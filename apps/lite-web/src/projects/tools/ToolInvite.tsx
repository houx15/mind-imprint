import { useState } from "react";
import { Footprints, PanelRight } from "lucide-react";
import { Icon } from "@/ui";
import type { ToolInstance } from "../../api/tools";
import { TOOL_TASKS } from "./registry";

/**
 * ToolInvite —— 印记把一件工具递到对话里。
 *
 * 邀请，不是弹窗（铁律②）。她可以不打开，而且**不打开必须和打开一样容易**：
 * 「跳过」是一次点击，不问理由。如果拒绝比接受费事，那就不是一个真的选择。
 *
 * world 工具（出门做的事）长得不一样：打开它不会跳出一个面板，而是把事情
 * 交给她，然后让开。
 */
export function ToolInvite({
  tool,
  onOpen,
  onDecline,
  busy,
}: {
  tool: ToolInstance;
  onOpen: () => void;
  onDecline: (note: string) => void;
  busy?: boolean;
}) {
  const [note, setNote] = useState("");
  const [declining, setDeclining] = useState(false);
  const away = tool.kind === "world";
  const task = TOOL_TASKS[tool.tool] ?? "";

  if (declining) {
    return (
      <div className="mt-2 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2.5">
        <p className="text-mk-small text-mk-secondary">请阐述原因（可不填）</p>
        <div className="mt-2 flex items-end gap-2">
          <input
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="跳过的原因"
            className="flex-1 rounded-mk-md border border-mk-input-border bg-mk-surface px-2.5 py-1.5 text-mk-small text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
          />
          <button
            type="button"
            disabled={busy}
            onClick={() => onDecline(note.trim())}
            className="shrink-0 rounded-mk-full border border-mk-border px-3 py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
          >
            确认
          </button>
        </div>
      </div>
    );
  }

  return (
    <div
      // e2e 靠它找到这张卡。按文字找会随着文案改动而碎，而这张卡上每一句话
      // 都是会改的。
      data-testid={`tool-invite-${tool.tool}`}
      className="mt-2 rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2.5"
    >
      <div className="flex items-start gap-2">
        <Icon
          icon={away ? Footprints : PanelRight}
          size={15}
          className="mt-0.5 shrink-0 text-mk-accent-500"
        />
        <div className="min-w-0">
          <p className="text-mk-small">
            <span className="font-semibold text-mk-ink">{tool.label}</span>
            {task && <span className="ml-1.5 text-mk-secondary">{task}</span>}
          </p>
          {/* 印记自己的话，不是模板。 */}
          <p className="mt-1 text-mk-small text-mk-muted">{tool.reason}</p>
          {away && (
            <p className="mt-1 text-mk-small text-mk-faint">请在合适的地方完成这项任务，完成后回来继续项目</p>
          )}
        </div>
      </div>
      <div className="mt-2.5 flex gap-2">
        <button
          type="button"
          disabled={busy}
          onClick={onOpen}
          className="rounded-mk-full px-3.5 py-1.5 text-mk-small text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          {away ? "接受任务" : "开始任务"}
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={() => setDeclining(true)}
          className="rounded-mk-full border border-mk-border px-3.5 py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
        >
          跳过
        </button>
      </div>
    </div>
  );
}

/**
 * AwayCard —— 她答应了、人不在的那件事。
 *
 * 🚨 这张卡不能长得像待办。一件出门做的事挂上三天是正常的，显示成"未完成"
 * 就变成了催促，而催促正是这个产品说自己不做的事。
 */
export function AwayCard({
  tool,
  onBack,
  busy,
}: {
  tool: ToolInstance;
  onBack: () => void;
  busy?: boolean;
}) {
  return (
    <div className="rounded-mk-md border border-mk-border bg-mk-surface px-3 py-2.5">
      <div className="flex items-start gap-2">
        <Icon icon={Footprints} size={15} className="mt-0.5 shrink-0 text-mk-accent-500" />
        <div className="min-w-0">
          <p className="text-mk-small font-semibold text-mk-ink">{tool.label}</p>
          <p className="mt-1 text-mk-small text-mk-muted">{tool.reason}</p>
          {/* 🚨 这一句原来只印在邀请卡上，她一答应就没了——而这张卡正是她几天
              后回来时唯一还能看到的东西。要她做的事必须一直留在上面。 */}
          <p className="mt-1 text-mk-small text-mk-faint">
            请在合适的地方完成这项任务，完成后回来记录结果
          </p>
        </div>
      </div>
      <button
        type="button"
        disabled={busy}
        onClick={onBack}
        className="mt-2.5 w-full rounded-mk-full border border-mk-border py-1.5 text-mk-small text-mk-secondary disabled:opacity-40"
      >
        {/* 🚨「已完成」是个状态词，印在一个还没做完的任务上，读起来像是这件事
            已经结束了。按钮要写她按下去会发生什么。 */}
        记录结果
      </button>
    </div>
  );
}
