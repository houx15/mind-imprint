// Bilingual helpers. zh is the source of truth; en mirrors it.
// Pages render one language server-side (SEO-correct), so `t` emits a single
// string rather than toggling in the DOM.

export const locales = ["zh", "en"] as const;
export type Lang = (typeof locales)[number];

export function getLang(locale: string | undefined): Lang {
  return locale === "en" ? "en" : "zh";
}

/** Pick the string for the current language. */
export function t(lang: Lang, zh: string, en: string): string {
  return lang === "en" ? en : zh;
}

/** Internal link within the current language. `path` is the canonical (zh-form)
 *  path like "/evaluation" or "/". */
export function localePath(lang: Lang, path: string): string {
  const clean = path === "/" ? "" : path.replace(/\/$/, "");
  return lang === "en" ? `/en${clean || "/"}` : clean || "/";
}

/** Given the current pathname, return the same page in the other language. */
export function switchLocaleUrl(pathname: string, lang: Lang): string {
  // Normalise: strip trailing slash, strip leading /en for the canonical key.
  const noEn = pathname.replace(/^\/en(\/|$)/, "/");
  const key = noEn.replace(/\/$/, "") || "/";
  const target: Lang = lang === "zh" ? "en" : "zh";
  return localePath(target, key);
}
