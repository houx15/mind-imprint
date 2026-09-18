import { Download, Link2, MessagesSquare } from "lucide-react";
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
 *
 * 🚨 2026-09-16：分享那一颗从「只有图标」改成**带字**的。这条改动推翻的是上面
 * 那句 "small icons on the right upper corner" 的一半 —— 那也是产品负责人定的，
 * 但她这次说得更晚也更直接：报告「cannot ignite my willing of sharing」，而发
 * 出去这件事在屏幕上得先认出一个图标。导出仍然是图标：它不是这一页要请她做的
 * 那件事。
 */
export function ReportActions({
  exporting,
  onExport,
  shareOpen,
  shared,
  onToggleShare,
  onOpenRecord,
}: {
  exporting: boolean;
  onExport: () => void;
  shareOpen: boolean;
  /** True when a public link is live right now. */
  shared: boolean;
  onToggleShare: () => void;
  /** 「查看阅读记录」：她和印记的对话。只有阅读室自己那一面传；公开页不传。 */
  onOpenRecord?: () => void;
}) {
  return (
    <div className="mk-rp-actions">
      {onOpenRecord && (
        <button type="button" onClick={onOpenRecord} className="mk-rp-action mk-rp-action--labelled">
          <Icon icon={MessagesSquare} size={17} />
          <span className="mk-rp-action__label">查看阅读记录</span>
        </button>
      )}
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
        className={`mk-rp-action mk-rp-action--labelled ${shareOpen ? "mk-rp-action--on" : ""}`}
      >
        <Icon icon={Link2} size={17} />
        <span className="mk-rp-action__label">分享</span>
        {shared && <span className="mk-rp-action__dot" aria-hidden="true" />}
      </button>
    </div>
  );
}
