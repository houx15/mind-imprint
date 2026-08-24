import { Button, Modal } from "@/ui";

/**
 * DemoProjectGuardModal — the "guide-or-leave" gate a student hits when they
 * manually click the shared, read-only demo project card in the directory
 * (Task 9). The demo exists to be TOURED, not free-roamed: the studio behind
 * it is real data the backend 403s all writes to, so a manual open would just
 * land the student in a workspace where every action silently fails.
 *
 * Two ways out, both closing the modal: 好，带我逛一遍 hands off to the guided
 * tour (`onGuide`, wired by the caller to `journeyStarting("projects")`) which
 * drives its OWN, separate open path (`initialProjectId`) straight into the
 * studio; 不用了 just closes and leaves the student on the project list.
 */
export function DemoProjectGuardModal({
  open,
  onGuide,
  onClose,
}: {
  open: boolean;
  /** 好，带我逛一遍 — hand off to the guided tour. The modal always closes
   * itself right after, so callers don't also need to close on this path. */
  onGuide: () => void;
  onClose: () => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title="示例项目">
      <div className="flex flex-col gap-4">
        <p className="text-mk-body text-mk-muted leading-relaxed">
          这是一个只读的<strong className="font-semibold text-mk-ink">示例项目</strong>，用来给你演示「项目」是怎么用的——你不能在里面编辑。要我带你逛一遍吗？
        </p>
        <div className="flex flex-col gap-2">
          <Button
            type="button"
            variant="primary"
            size="md"
            className="w-full"
            onClick={() => {
              onGuide();
              onClose();
            }}
          >
            好，带我逛一遍
          </Button>
          <Button type="button" variant="secondary" size="md" className="w-full" onClick={onClose}>
            不用了
          </Button>
        </div>
      </div>
    </Modal>
  );
}
