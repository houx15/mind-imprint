import { useEffect, useRef, useState } from "react";
import { ApiError } from "../api/client";
import { renameWriting, type Writing } from "../api/writings";

/**
 * EditableTitle — the h1, as something she owns.
 *
 * What sat here before was her raw idea sentence, carried straight from the
 * 「我想写：…」 box on the landing page. That is a note to self, not a title:
 * it is long, it is a full sentence, and it is the one line of the room she
 * cannot change. So it becomes click-to-edit, saved on blur, present in every
 * step's header.
 *
 * Her original idea is NOT lost by renaming: it lives on in `atom_message` as
 * her own setup turn, which is what every downstream prompt reads. `title` is
 * therefore free to become a real title without any prompt losing the thing
 * she actually wanted to write about.
 *
 * The server 400s on a blank title, so a cleared box reverts rather than being
 * sent — refusing to save an empty title is not the same as scolding her for
 * one.
 */
export function EditableTitle({
  writingId,
  title,
  onRenamed,
}: {
  writingId: string;
  title: string;
  /** Lifted so the rest of the room (header, list, finished screen) sees the
   *  new title immediately rather than at the next load. */
  onRenamed?: (next: Writing) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState(title);
  const [error, setError] = useState<string | null>(null);
  /**
   * Enter commits, and so does blur. Without this the two fire in sequence on
   * the same keystroke — one PATCH, then a second identical one — because
   * unmounting a focused input blurs it. The flag is reset on every entry into
   * edit mode, so it can never wedge the field shut.
   */
  const committed = useRef(false);

  useEffect(() => {
    if (!editing) setValue(title);
  }, [title, editing]);

  function open() {
    committed.current = false;
    setError(null);
    setValue(title);
    setEditing(true);
  }

  async function commit() {
    if (committed.current) return;
    committed.current = true;
    setEditing(false);
    const next = value.trim();
    if (!next || next === title) {
      setValue(title);
      return;
    }
    try {
      const saved = await renameWriting(writingId, next);
      setValue(saved.title);
      onRenamed?.(saved);
    } catch (err) {
      setValue(title);
      setError(err instanceof ApiError ? err.message : "改名字没成功，再试一次。");
    }
  }

  function cancel() {
    committed.current = true;
    setValue(title);
    setEditing(false);
  }

  if (editing) {
    return (
      <input
        autoFocus
        value={value}
        aria-label="标题"
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => void commit()}
        onKeyDown={(e) => {
          if (e.key === "Enter") void commit();
          if (e.key === "Escape") cancel();
        }}
        className="w-full max-w-[520px] rounded-mk-xs border border-mk-input-border bg-mk-paper px-2 py-1 text-mk-h3 text-mk-ink outline-none focus-visible:border-mk-accent focus-visible:ring-2 focus-visible:ring-mk-accent-200"
      />
    );
  }

  return (
    <div className="min-w-0">
      {/* One font-size class only. Taking a `className` here would let a caller
          stack a second `text-mk-*` on this element, and Tailwind emits
          competing utilities in ALPHABETICAL order rather than className order
          — so the caller's override would silently lose. */}
      <h1 className="min-w-0 text-mk-h3 text-mk-ink">
        <button
          type="button"
          onClick={open}
          title="点一下改标题"
          className="block max-w-full truncate rounded-mk-xs px-1 py-0.5 text-left transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-mk-accent-200"
        >
          {title || "还没起名字的写作"}
        </button>
      </h1>
      {error && (
        <p role="alert" className="px-1 text-mk-small text-mk-danger">
          {error}
        </p>
      )}
    </div>
  );
}
