import { useEffect, useState } from "react";
import { Button } from "@/ui";
import { previewPersonalizedReading } from "../api/assignments";
import { getLibraryShelf, type LibraryArticle } from "../api/library";
import { Chip } from "./formParts";
import { LibraryPicker } from "./LibraryPicker";
import {
  disciplineOptions,
  errorText,
  keptFromRows,
  keptFromSaved,
  mergePickRows,
  personalTierLabel,
  pickTierText,
  swapPick,
  toggleId,
  visiblePickRows,
  type PickRow,
  type SettingsDraft,
} from "./assignmentLogic";

/**
 * PersonalizedPicker — 个性化 reading homework: a filter (学科筛选, 难度),
 * the class preview (one article per checked student) and 更换 per row.
 *
 * The preview runs again when the filter or the class changes; rows the
 * teacher swapped, and picks saved on this homework, are kept (mergePickRows).
 * Classes are inlined rather than imported from AssignmentForm, which imports
 * this file.
 */
export function PersonalizedPicker({
  value,
  onChange,
  classId,
  recipientIds,
}: {
  value: SettingsDraft;
  onChange: (update: (d: SettingsDraft) => SettingsDraft) => void;
  classId: string;
  recipientIds: string[];
}) {
  const [articles, setArticles] = useState<LibraryArticle[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [nonce, setNonce] = useState(0);
  const [swapping, setSwapping] = useState<PickRow | null>(null);

  // The shelf gives the discipline options and the titles of saved picks. On
  // failure the filter is hidden and titles fall back to slugs.
  useEffect(() => {
    let cancelled = false;
    getLibraryShelf()
      .then((shelf) => {
        if (!cancelled) setArticles(Array.isArray(shelf?.articles) ? shelf.articles : []);
      })
      .catch(() => {
        if (!cancelled) setArticles([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const disciplinesKey = value.disciplines.join(",");
  const tier = value.personalTier;
  const ready = articles !== null;
  useEffect(() => {
    if (!classId || !ready) return;
    let cancelled = false;
    setLoading(true);
    setError(null);
    previewPersonalizedReading(classId, { disciplines: disciplinesKey ? disciplinesKey.split(",") : [], tier })
      .then((rows) => {
        if (cancelled) return;
        onChange((d) => ({
          ...d,
          picks: mergePickRows(rows, d.personalTier, { ...keptFromSaved(d.savedPicks, articles ?? []), ...keptFromRows(d.picks) }),
        }));
      })
      .catch((e: unknown) => {
        if (!cancelled) setError(errorText(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // onChange is a new function on every render; the preview depends only on
    // the class, the filter and a retry.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [classId, disciplinesKey, tier, nonce, ready]);

  const options = articles ? disciplineOptions(articles) : [];
  const rows = value.picks ? visiblePickRows(value.picks, recipientIds) : null;

  return (
    <div className="flex flex-col gap-4">
      {options.length > 0 && (
        <div className="flex flex-col gap-1.5">
          <span className="text-mk-label font-bold text-mk-muted">学科筛选</span>
          <div className="flex flex-wrap gap-2" role="group" aria-label="学科筛选">
            {options.map((t) => (
              <Chip
                key={t.id}
                active={value.disciplines.includes(t.id)}
                onClick={() => onChange((d) => ({ ...d, disciplines: toggleId(d.disciplines, t.id) }))}
              >
                {t.zh}
              </Chip>
            ))}
          </div>
          <span className="text-mk-small text-mk-muted">不选表示不限学科</span>
        </div>
      )}

      <div className="flex flex-col gap-1.5">
        <span className="text-mk-label font-bold text-mk-muted">难度</span>
        <div className="flex flex-wrap gap-2" role="group" aria-label="难度">
          {[null, 1, 2, 3, 4, 5].map((t) => (
            <Chip key={t ?? "auto"} active={tier === t} onClick={() => onChange((d) => ({ ...d, personalTier: t }))}>
              {personalTierLabel(t)}
            </Chip>
          ))}
        </div>
      </div>

      {error ? (
        <div className="text-mk-small font-semibold text-mk-danger">
          加载失败：{error}{" "}
          <button type="button" onClick={() => setNonce((n) => n + 1)} className="cursor-pointer underline">
            重试
          </button>
        </div>
      ) : !classId ? (
        <div className="text-mk-small text-mk-muted">请选择班级</div>
      ) : rows === null ? (
        <div className="text-mk-small text-mk-muted">加载中…</div>
      ) : rows.length === 0 ? (
        <div className="text-mk-small text-mk-muted">暂无学生</div>
      ) : (
        <div className="overflow-x-auto rounded-mk-lg border border-mk-border bg-mk-surface">
          <table className="w-full min-w-[640px] border-collapse">
            <thead>
              <tr>
                {["学生", "文章", "难度", "原因", "操作"].map((h) => (
                  <th key={h} className="whitespace-nowrap border-b border-mk-border px-3 py-2.5 text-left text-mk-label font-bold text-mk-muted">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={r.userId}>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small font-bold text-mk-ink">{r.name}</td>
                  <td className="border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{r.title}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small text-mk-ink">{pickTierText(r, value.personalTier)}</td>
                  <td className="border-b border-mk-border px-3 py-3 text-mk-small text-mk-muted">{r.reason}</td>
                  <td className="whitespace-nowrap border-b border-mk-border px-3 py-3 text-mk-small">
                    <Button variant="link" size="sm" onClick={() => setSwapping(r)}>
                      更换
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {loading && rows !== null && <span className="text-mk-small text-mk-muted">处理中</span>}

      {swapping && (
        <SwapDialog
          row={swapping}
          onClose={() => setSwapping(null)}
          onConfirm={(next) => {
            const userId = swapping.userId;
            onChange((d) => ({ ...d, picks: d.picks ? swapPick(d.picks, userId, next, articles ?? []) : d.picks }));
            setSwapping(null);
          }}
        />
      )}
    </div>
  );
}

function SwapDialog({
  row,
  onClose,
  onConfirm,
}: {
  row: PickRow;
  onClose: () => void;
  onConfirm: (next: { slug: string; tier: number | null }) => void;
}) {
  const [choice, setChoice] = useState<{ slug: string; tier: number | null }>({ slug: row.slug, tier: row.tier });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="swap-dialog-title"
        className="flex max-h-full w-full max-w-[640px] flex-col gap-5 overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
      >
        <div className="flex flex-col gap-1.5">
          <h2 id="swap-dialog-title" className="text-mk-h2 text-mk-ink">
            更换文章
          </h2>
          <p className="text-mk-small text-mk-muted">{row.name}</p>
        </div>
        <LibraryPicker slug={choice.slug} tier={choice.tier} onChange={setChoice} />
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={onClose}>
            取消
          </Button>
          <Button variant="primary" size="sm" onClick={() => onConfirm(choice)} disabled={!choice.slug}>
            确认选择
          </Button>
        </div>
      </div>
    </div>
  );
}
