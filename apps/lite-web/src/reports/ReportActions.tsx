import { Download, Link2 } from "lucide-react";
import { Icon } from "@/ui";

/**
 * ReportActions — 导出图片 and 分享链接, as two SMALL icon buttons in the
 * report's upper-right corner.
 *
 * They started life at the very bottom of a long page (a text button under a
 * share panel — where you put something you'd rather nobody used), then
 * overshot into two large full-width cards at the top. The instruction that
 * settled it: "small icons on the right upper corner." Two icon buttons, no
 * labels, sitting where a document's controls live.
 *
 * **Icon-only, but never nameless.** Each carries an `aria-label` AND a
 * `title`, so a screen reader announces it and a mouse gets a tooltip. An
 * unlabelled icon button is a guess for everyone who isn't the person who
 * drew it.
 *
 * 分享 is a TOGGLE, not the act of sharing. It opens `SharePanel` below,
 * which is where the real consequence is stated and the real 生成分享链接
 * button lives — publishing a minor's schoolwork to a public URL is never one
 * click from arriving on a page. `aria-expanded` carries that fact.
 *
 * `shared` drives a small live dot on the share icon: a report that is
 * ALREADY published needs to say so at a glance, because the state she cannot
 * see is the one that matters.
 */
export function ReportActions({
  exporting,
  onExport,
  shareOpen,
  shared,
  onToggleShare,
}: {
  exporting: boolean;
  onExport: () => void;
  shareOpen: boolean;
  /** True when a public link is live right now. */
  shared: boolean;
  onToggleShare: () => void;
}) {
  return (
    <div className="mk-rp-actions">
      <button
        type="button"
        onClick={onExport}
        disabled={exporting}
        className="mk-rp-action"
        aria-label={exporting ? "导出图片，生成中" : "导出图片"}
        title={exporting ? "生成图片中…" : "导出图片"}
      >
        <Icon icon={Download} size={17} />
      </button>

      <button
        type="button"
        onClick={onToggleShare}
        aria-expanded={shareOpen}
        aria-label={shared ? "分享链接，已经在分享中" : "分享链接"}
        title={shared ? "分享链接（正在分享）" : "分享链接"}
        className={`mk-rp-action ${shareOpen ? "mk-rp-action--on" : ""}`}
      >
        <Icon icon={Link2} size={17} />
        {shared && <span className="mk-rp-action__dot" aria-hidden="true" />}
      </button>
    </div>
  );
}
