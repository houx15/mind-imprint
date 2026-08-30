import { useState } from "react";
import { Button, Modal } from "@/ui";

/**
 * NamePieceModal — 给这篇起个名字, asked once, at 完成这篇.
 *
 * ## Why this moment and not another
 *
 * A writing's title starts life as her raw 「我想写：…」 sentence, up to 200
 * characters of it (createWriting, writings.go). That is fine while the piece
 * is hers alone — `EditableTitle` sits in the room header the whole time, so
 * she can change it whenever she likes. It stops being fine the instant she
 * finishes: the title goes into display type at the top of the report, onto
 * the exported poster, and out through the share link to someone with no
 * account, who reads her private note-to-self where the piece's name should
 * be.
 *
 * 完成这篇 is therefore the one right moment to ask — the piece is written, so
 * there is something real to name, and it is the last moment before the title
 * becomes public.
 *
 * ## Asked once, and never to someone who already answered
 *
 * The server decides (`suggestWritingTitles` → `needsName`) by comparing the
 * title against her stored opening turn, character for character. Anyone who
 * used `EditableTitle` never sees this dialog at all, and no model call is
 * made for them.
 *
 * ## 铁律②: this is not a gate
 *
 * 用原来的 finishes with the title untouched, and it is a real button of equal
 * standing, not a greyed-out link in the corner. She can also close the dialog
 * outright. Nothing here blocks 完成这篇, because a piece she chose not to
 * rename is a legitimate outcome — the failure this fixes is never being
 * ASKED, not "having the wrong title".
 *
 * ## The suggestions are offers
 *
 * Tapping one fills the box; it does not commit anything. She can edit it
 * afterwards, or ignore all of them and type her own — the input is the
 * primary control and the chips sit under it, the same "named, explained,
 * tappable, inert until she chooses" posture `GuideBox` takes with its
 * methods. What gets saved is whatever is in the box when she presses the
 * button, which is always something she has looked at.
 */
export function NamePieceModal({
  ideas,
  onName,
  onKeep,
  onClose,
  saving,
  error,
}: {
  /** 印记's candidates. May be empty — she pressed 完成这篇 on a draft with
   *  nothing in it, and there was nothing to name from. The box still works. */
  ideas: string[];
  /** Save this title, then finish. */
  onName: (title: string) => void;
  /** Finish with the title as it stands. */
  onKeep: () => void;
  onClose: () => void;
  saving: boolean;
  error: string | null;
}) {
  const [value, setValue] = useState(ideas[0] ?? "");
  const trimmed = value.trim();

  return (
    <Modal
      open
      onClose={onClose}
      title="给这篇起个名字"
      footer={
        <>
          {/* Equal standing, not a get-out link: see 铁律② in this file's
              header. Left of the primary button so it reads as a choice. */}
          <Button variant="ghost" onClick={onKeep} disabled={saving}>
            用原来的
          </Button>
          <Button onClick={() => onName(trimmed)} loading={saving} disabled={trimmed === ""}>
            就叫这个，完成
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-5">
        <p className="text-mk-body text-mk-muted">
          现在这一栏里放的还是你最开始写的那句「我想写…」。报告、导出的图片和你发出去的链接上，写的都是它。
        </p>

        <div className="flex flex-col gap-2">
          <label htmlFor="mk-piece-name" className="text-mk-small text-mk-secondary">
            这篇文章叫
          </label>
          <input
            id="mk-piece-name"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            placeholder="写一个你想让别人看到的名字"
            autoFocus
            className="w-full rounded-mk-sm border border-mk-border bg-mk-paper px-3 py-2 text-mk-body-lg text-mk-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
          />
        </div>

        {ideas.length > 0 && (
          <div className="flex flex-col gap-2">
            <span className="text-mk-small text-mk-secondary">印记读了你这篇，想到几个名字：</span>
            <div className="flex flex-wrap gap-2">
              {ideas.map((idea) => {
                const on = trimmed === idea;
                return (
                  <button
                    key={idea}
                    type="button"
                    onClick={() => setValue(idea)}
                    aria-pressed={on}
                    className="rounded-mk-full border px-3 py-1.5 text-mk-body"
                    style={{
                      borderColor: on ? "var(--mk-accent-500)" : "var(--mk-border)",
                      background: on ? "var(--mk-accent-100)" : "var(--mk-paper)",
                      color: on ? "var(--mk-accent-700)" : "var(--mk-ink)",
                    }}
                  >
                    {idea}
                  </button>
                );
              })}
            </div>
            <span className="text-mk-caption text-mk-faint">点一个填进上面的框，还能再改。</span>
          </div>
        )}

        {error && (
          <p role="alert" className="text-mk-small text-mk-danger">
            {error}
          </p>
        )}
      </div>
    </Modal>
  );
}
