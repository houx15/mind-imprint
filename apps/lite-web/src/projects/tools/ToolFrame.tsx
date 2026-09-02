import type { ReactNode } from "react";
import { Check, X } from "lucide-react";
import { Icon } from "@/ui";

/**
 * ToolFrame —— 每一件工具共用的外壳。
 *
 * 七件工具长得很不一样：一块便签板，一张结构图，一份要审的文档。共用这个外壳
 * 是为了让它们对她**是同一件事**——打开一个东西，知道要做什么，知道为什么，
 * 做完，回到对话。
 *
 * 三样是强制的，因为它们正是产品负责人 2026-09-01 提的三个要求：
 *
 *   title + task  她要做什么，一句话说完（"very clear to know what to do"）
 *   why           印记为什么这时候递它（"tell them why"）
 *   todo          还差什么。空了才能点"做完了"
 *
 * todo 这一项是"清楚"真正落地的地方。没有它，每件工具都得自己想办法暗示
 * 还差什么，七件工具就会有七种暗示法。
 */
export function ToolFrame({
  title,
  task,
  why,
  todo,
  children,
  onFinish,
  onClose,
  finishLabel = "完成",
  busy,
}: {
  title: string;
  /** 一句话：她现在要做什么。 */
  task: string;
  /** 印记递这件工具的理由。它来自模型，是具体的一句话。 */
  why: string;
  /** 还差什么。空字符串 = 齐了，可以收工。 */
  todo?: string;
  children: ReactNode;
  onFinish: () => void;
  onClose: () => void;
  finishLabel?: string;
  busy?: boolean;
}) {
  const ready = !todo;
  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-mk-border px-4 py-3">
        <div className="flex items-start justify-between gap-2">
          <div>
            <h2 className="text-mk-body font-semibold text-mk-ink">{title}</h2>
            <p className="mt-0.5 text-mk-small text-mk-secondary">{task}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="收起"
            className="shrink-0 rounded-mk-full p-1 text-mk-faint hover:text-mk-secondary"
          >
            <Icon icon={X} size={16} />
          </button>
        </div>
        {why && (
          // 理由用印记自己的话，不用模板。一句写得具体的理由是她愿意打开的
          // 原因；一句"这有助于你深入思考"只会被跳过。
          <p
            className="mt-2 rounded-mk-md px-2.5 py-1.5 text-mk-small text-mk-secondary"
            style={{ background: "var(--mk-paper)" }}
          >
            {why}
          </p>
        )}
      </header>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">{children}</div>

      <footer className="border-t border-mk-border px-4 py-3">
        {todo && <p className="mb-2 text-mk-small text-mk-muted">还差：{todo}</p>}
        <button
          type="button"
          onClick={onFinish}
          disabled={!ready || busy}
          className="flex w-full items-center justify-center gap-1.5 rounded-mk-full py-2 text-mk-body text-white disabled:opacity-40"
          style={{ background: "var(--mk-accent-500)" }}
        >
          <Icon icon={Check} size={15} />
          {finishLabel}
        </button>
      </footer>
    </div>
  );
}
