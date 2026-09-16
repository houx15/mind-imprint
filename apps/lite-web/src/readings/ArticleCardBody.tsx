import type { ReactNode } from "react";
import { fieldById } from "../tree/geometry";
import type { FieldId } from "../tree/types";
import type { LibraryArticle } from "../api/library";

/**
 * ArticleCardBody — the display half of a library card: cover, Chinese
 * title, English title, one-line reason, discipline tags.
 *
 * Shared by the student card (`LibraryCard`, which adds the tier picker and
 * the start/resume buttons as `footer`) and the teacher's `LibraryPicker`
 * (which wraps it in a selectable card). With `compact` unset the markup is
 * exactly what `LibraryCard` rendered before the split, so the student shelf
 * looks the same.
 *
 * The caller supplies the outer element (it decides whether the card is an
 * `<article>` or a selectable control) and must sit inside a
 * `.mk-branch-hues` scope, or the tag colours resolve to nothing.
 */
export function ArticleCardBody({
  article,
  badge,
  note,
  footer,
  compact = false,
}: {
  article: LibraryArticle;
  /** Shown to the right of the title (已完成 / 已选). */
  badge?: ReactNode;
  /** One line under the tags (the recommendation reason). */
  note?: ReactNode;
  /** Pinned to the bottom of the card. */
  footer?: ReactNode;
  /** Teacher picker: clamps the English title and the reason so cards in a
   *  row keep the same height. */
  compact?: boolean;
}) {
  return (
    <>
      {article.coverUrl && (
        <img
          src={article.coverUrl}
          alt={article.zhTitle}
          loading="lazy"
          className="reading-library-cover w-full object-cover"
        />
      )}

      <div className="reading-library-content flex flex-1 flex-col gap-2">
        <div className="flex items-start justify-between gap-2">
          <h3 className="text-mk-body font-semibold leading-snug text-mk-ink">{article.zhTitle || article.title}</h3>
          {badge}
        </div>
        <p className={"reading-library-english text-mk-muted" + (compact ? " line-clamp-1" : "")}>{article.title}</p>
        <p className={"reading-library-description text-mk-secondary" + (compact ? " line-clamp-2" : "")}>
          {article.reason}
        </p>

        <div className="mt-0.5 flex flex-wrap gap-1.5">
          {(Array.isArray(article.tags) ? article.tags : []).map((t) => (
            <span
              key={t.id}
              className="rounded-mk-full px-2 py-0.5 text-mk-label"
              style={{
                background: `color-mix(in srgb, ${fieldById(t.field as FieldId).hue} 16%, transparent)`,
                color: `color-mix(in srgb, ${fieldById(t.field as FieldId).hue} 72%, black)`,
              }}
            >
              {t.zh}
            </span>
          ))}
        </div>

        {note}

        {footer !== undefined && <div className="reading-library-footer mt-auto">{footer}</div>}
      </div>
    </>
  );
}
