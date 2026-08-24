import { Button, Modal, Pebble } from "@/ui";

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
        <span className="mk-pebble-bounce inline-flex">
          <Pebble size={48} />
        </span>
        <div className="mt-3 text-mk-h3 font-semibold text-mk-ink">{name}你好呀 👋</div>
        <p className="mt-2 text-mk-body text-mk-muted leading-relaxed">
          欢迎来到思维印记 AI 思辨力成长平台。我是印记，你的 AI 小伙伴。要不让我先带你逛逛这里吧！想先看看什么呢？
        </p>
        <div className="mt-5 flex w-full flex-col gap-2">
          <Button type="button" variant="primary" size="md" className="w-full" onClick={() => onPick("courses")}>课程</Button>
          <Button type="button" variant="secondary" size="md" className="w-full" onClick={() => onPick("projects")}>项目</Button>
          <Button type="button" variant="link" size="sm" className="mt-1" onClick={onDismiss}>稍后再说</Button>
        </div>
        <p className="mt-3 text-mk-small text-mk-muted">以后想再逛，可以点左边导航底部的“重新开始引导”。</p>
      </div>
    </Modal>
  );
}
