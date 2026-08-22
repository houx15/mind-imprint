import { Modal, Pebble } from "@/ui";

export function WelcomeModal({ open, displayName, onPick, onDismiss }: {
  open: boolean;
  displayName: string;
  onPick: (start: "courses" | "projects") => void;
  onDismiss: () => void;
}) {
  const name = displayName?.trim() || "同学";
  return (
    <Modal open={open} onClose={onDismiss} title={null}>
      <div className="flex flex-col items-center text-center">
        <Pebble size={48} />
        <div className="mt-3 text-mk-h3 font-semibold text-mk-ink">{name}你好呀 👋</div>
        <p className="mt-2 text-mk-body text-mk-muted">
          欢迎来到思维印记 AI 思辨力成长平台。我是印记，你的 AI 小伙伴。要不让我先带你逛逛这里吧！想先看看什么呢？
        </p>
        <div className="mt-5 flex w-full flex-col gap-2">
          <button type="button" onClick={() => onPick("courses")}
            className="rounded-mk-md bg-mk-accent px-4 py-2.5 text-mk-body font-semibold text-white hover:opacity-90">课程</button>
          <button type="button" onClick={() => onPick("projects")}
            className="rounded-mk-md bg-mk-accent-50 px-4 py-2.5 text-mk-body font-semibold text-mk-accent-700 hover:bg-mk-accent-100">项目</button>
          <button type="button" onClick={onDismiss}
            className="mt-1 text-mk-small text-mk-muted hover:text-mk-ink">稍后再说</button>
        </div>
        <p className="mt-3 text-mk-small text-mk-muted">以后想再逛，可以点左边导航底部的“重新开始引导”。</p>
      </div>
    </Modal>
  );
}
