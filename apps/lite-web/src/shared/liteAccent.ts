import type { AccentId } from "@/ui";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";

/** Lite's default accent for an account that has no saved choice. */
export const LITE_DEFAULT_ACCENT: AccentId = "teal";

/**
 * Coerce a stored accent (the server's `avatar_color`, or the `mk-accent`
 * localStorage key) into a lite preset id, else undefined. Students and
 * teachers both use `LITE_ACCENT_PRESETS`; a value that is not one of them (an
 * old hex colour, a removed preset) is dropped rather than applied raw.
 */
export function coerceLiteAccent(value: string | null | undefined): AccentId | undefined {
  return LITE_ACCENT_PRESETS.some((p) => p.id === value) ? (value as AccentId) : undefined;
}

/**
 * The accent a lite account starts with: the server's saved choice, then the
 * browser's, then the lite default. `readStored` wraps localStorage and may
 * throw (private mode); a throw counts as nothing stored.
 */
export function initialLiteAccent(saved: string | null | undefined, readStored: () => string | null): AccentId {
  const fromServer = coerceLiteAccent(saved);
  if (fromServer) return fromServer;
  try {
    return coerceLiteAccent(readStored()) ?? LITE_DEFAULT_ACCENT;
  } catch {
    return LITE_DEFAULT_ACCENT;
  }
}
