import { useState } from "react";
import { ApiError } from "../api/client";
import { PROJECT_KIND_LABELS, updateProject, type Project } from "../api/projects";
import { COVER_GLYPHS, COVER_GROUNDS, groundById, resolveCover } from "./covers";

/**
 * NameAndCover — the modal that opens the moment a project exists.
 *
 * Two rules this component exists to hold:
 *
 * **The name field starts empty.** 印记 could easily propose a name from her
 * sentence, and she would accept it, and it would be 印记's project with her
 * idea in it. Naming a thing is the cheapest possible act of ownership and we
 * do not spend it for her.
 *
 * **Closing is allowed and costs nothing.** By the time this opens, the
 * project is already on the server — so a modal that traps her would be
 * pretending a decision is required when it is not. Close it and the project
 * sits on the board under her own sentence until she comes back.
 *
 * The cover, by contrast, opens pre-chosen. An unnamed project with no cover
 * would render as a grey nothing, and choosing is easier against something
 * than against a blank.
 */
export function NameAndCover({
  project,
  onDone,
  onClose,
}: {
  project: Project;
  onDone: (p: Project) => void;
  onClose: () => void;
}) {
  // Seeded through the same resolver the board uses, so what she sees here is
  // exactly what her card already shows behind the modal.
  const seed = resolveCover(project.kind, project.coverGround, project.coverGlyph);
  const [name, setName] = useState(project.name);
  const [ground, setGround] = useState(seed.ground.id);
  const [glyph, setGlyph] = useState(seed.glyph);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const chosen = groundById(ground);

  async function save() {
    if (saving) return;
    setSaving(true);
    setError(null);
    try {
      const updated = await updateProject(project.id, {
        name: name.trim(),
        coverGround: ground,
        coverGlyph: glyph,
      });
      onDone(updated);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "没保存上，再试一次。");
      setSaving(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ background: "color-mix(in srgb, var(--mk-ink) 42%, transparent)" }}
      onClick={onClose}
    >
      <div
        className="max-h-[88vh] w-full max-w-[460px] overflow-y-auto rounded-mk-lg border border-mk-border bg-mk-surface p-6 shadow-mk-lg"
        onClick={(e) => e.stopPropagation()}
      >
        <p className="text-mk-label uppercase text-mk-muted">
          {PROJECT_KIND_LABELS[project.kind] ?? project.kind}
        </p>
        <h2 className="mt-2 text-mk-body-lg font-semibold text-mk-ink">给它起个名字</h2>
        <p className="mt-1 text-mk-small text-mk-secondary">「{project.idea}」</p>

        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          maxLength={60}
          autoFocus
          placeholder="你想叫它什么"
          className="mt-4 w-full rounded-mk-md border border-mk-input-border bg-mk-paper px-3 py-2.5 text-mk-body text-mk-ink outline-none placeholder:text-mk-faint focus:border-mk-accent-200"
        />

        <div className="mt-6 flex items-center gap-4">
          <span
            aria-hidden
            className="flex h-16 w-16 shrink-0 items-center justify-center rounded-mk-md text-[26px] leading-none"
            style={{
              background: `linear-gradient(145deg, ${chosen.from}, ${chosen.to})`,
              color: chosen.ink,
            }}
          >
            {glyph}
          </span>
          <div className="flex flex-wrap gap-1.5">
            {COVER_GROUNDS.map((g) => (
              <button
                key={g.id}
                type="button"
                aria-label={g.id}
                onClick={() => setGround(g.id)}
                className="h-7 w-7 rounded-mk-sm transition-transform duration-[120ms] ease-mk"
                style={{
                  background: `linear-gradient(145deg, ${g.from}, ${g.to})`,
                  outline: g.id === ground ? "2px solid var(--mk-ink)" : "none",
                  outlineOffset: "2px",
                }}
              />
            ))}
          </div>
        </div>

        <div className="mt-4 grid grid-cols-8 gap-1.5">
          {COVER_GLYPHS.map((g) => (
            <button
              key={g}
              type="button"
              onClick={() => setGlyph(g)}
              className="flex h-8 items-center justify-center rounded-mk-sm border text-mk-body text-mk-secondary"
              style={{
                borderColor: g === glyph ? "var(--mk-ink)" : "var(--mk-border)",
                background: g === glyph ? "var(--mk-paper)" : "transparent",
              }}
            >
              {g}
            </button>
          ))}
        </div>

        {error && (
          <p className="mt-4 text-mk-small" style={{ color: "var(--mk-danger)" }}>
            {error}
          </p>
        )}

        <div className="mt-6 flex items-center justify-end gap-3">
          <button
            type="button"
            onClick={onClose}
            className="rounded-mk-full px-3 py-2 text-mk-body text-mk-secondary"
          >
            以后再说
          </button>
          <button
            type="button"
            onClick={save}
            disabled={saving}
            className="rounded-mk-full px-5 py-2 text-mk-body font-semibold text-white transition-opacity duration-[120ms] ease-mk disabled:opacity-40"
            style={{ background: "var(--mk-accent-500)" }}
          >
            {saving ? "保存中…" : "就这样"}
          </button>
        </div>
      </div>
    </div>
  );
}
