export type Theme = "light" | "dark";

const KEY = "mk-theme";

/**
 * Apply a theme by setting (or clearing) `data-theme` on the root element.
 *
 * Light REMOVES the attribute rather than setting `data-theme="light"`, so the
 * light palette keeps exactly one definition — the bare `:root` block in
 * `apps/web/src/index.css` — and cannot drift from what the dark block assumes
 * about it. It also means every existing surface in both editions renders
 * untouched for anyone who never opts in.
 */
export function applyTheme(theme: Theme): void {
  if (theme === "dark") document.documentElement.setAttribute("data-theme", "dark");
  else document.documentElement.removeAttribute("data-theme");
}

/**
 * Read the stored preference.
 *
 * Reading `localStorage` does not merely return null in some contexts — it
 * THROWS (a browser set to block site data, some embedded webviews). A theme
 * preference is never worth failing a render over, so the whole access is
 * guarded and an unreadable store means light.
 */
export function readStoredTheme(): Theme {
  try {
    return localStorage.getItem(KEY) === "dark" ? "dark" : "light";
  } catch {
    return "light";
  }
}

export function storeTheme(theme: Theme): void {
  try {
    localStorage.setItem(KEY, theme);
  } catch {
    // A preference we cannot persist is still worth applying for this session.
  }
}

/** Apply the stored preference. Called once at boot, before first paint. */
export function bootTheme(): void {
  applyTheme(readStoredTheme());
}
