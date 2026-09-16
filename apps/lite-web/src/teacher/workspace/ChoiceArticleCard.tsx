import type { ChoiceArticle } from "./workspaceLogic";

// teacher/workspace/ChoiceArticleCard.tsx — an ask_choice option that means an
// article, shown as a card instead of a bare text button. The whole
// card is the control: a native <button> so it is keyboard-accessible for
// free, disabled the same way the plain choice buttons already are.
//
// A thumbnail-sized reuse of the student shelf's ArticleCardBody would need a
// full LibraryArticle (title, tags, levels) that liteworkspace.ChoiceArticle
// does not carry — the wire shape here is only what §6 already lets the
// server hand the teacher: slug/zhTitle/coverUrl/reason. This is its own,
// smaller card rather than a shim that fakes the missing fields.

export function ChoiceArticleCard({
  article,
  disabled,
  onClick,
}: {
  article: ChoiceArticle;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className="flex w-full items-center gap-3 rounded-mk-lg border border-mk-border bg-mk-surface p-2 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 disabled:opacity-50"
    >
      {article.coverUrl ? (
        <img
          src={article.coverUrl}
          alt={article.zhTitle}
          loading="lazy"
          className="h-14 w-14 shrink-0 rounded-mk-md object-cover"
        />
      ) : (
        <span className="h-14 w-14 shrink-0 rounded-mk-md bg-mk-paper" aria-hidden="true" />
      )}
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate text-mk-body font-semibold text-mk-ink">{article.zhTitle}</span>
        <span className="line-clamp-1 text-mk-small text-mk-secondary">{article.reason}</span>
      </span>
    </button>
  );
}
