import { Download, Link2 } from "lucide-react";
import { Icon } from "@/ui";

/**
 * ReportActions — 导出图片 and 分享链接, as two big buttons at the TOP of the
 * report.
 *
 * They used to be a small text button and a whole share panel stacked under
 * the very bottom of a long page, which is where you put something you would
 * rather nobody used. The product owner's instruction: "export and share link,
 * should be put at the top, big icons." A report exists to be shown to
 * someone; the two ways of doing that belong where she lands.
 *
 * 分享 is a TOGGLE, not the act of sharing. It opens `SharePanel` underneath,
 * which is where the real consequence is stated and the real button lives —
 * publishing a minor's schoolwork to a public URL is never one click from
 * arriving on a page. The toggle's own label reflects what the panel is
 * currently doing (分享 / 收起分享), and `aria-expanded` carries the same fact
 * to a screen reader.
 *
 * `shared` drives the small live dot: a report that is ALREADY published needs
 * to say so at a glance, before she scrolls anywhere, because the state she
 * cannot see is the one that matters.
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
        className="mk-rp-action mk-rp-action--primary"
      >
        <span className="mk-rp-action__icon" aria-hidden="true">
          <Icon icon={Download} size={22} />
        </span>
        <span className="flex flex-col items-start">
          <span className="text-mk-h3">{exporting ? "生成图片中…" : "导出图片"}</span>
          <span className="text-mk-small opacity-80">存成一张图，发给谁都行</span>
        </span>
      </button>

      <button
        type="button"
        onClick={onToggleShare}
        aria-expanded={shareOpen}
        className="mk-rp-action"
      >
        <span className="mk-rp-action__icon" aria-hidden="true">
          <Icon icon={Link2} size={22} />
        </span>
        <span className="flex flex-col items-start">
          <span className="flex items-center gap-2 text-mk-h3">
            {shareOpen ? "收起分享" : "分享链接"}
            {shared && <span className="mk-rp-action__dot" aria-hidden="true" />}
          </span>
          <span className="text-mk-small text-mk-muted">
            {shared ? "链接已经在用，可以随时收回" : "生成一个链接，不用登录也能打开"}
          </span>
        </span>
      </button>
    </div>
  );
}
