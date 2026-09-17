/**
 * The language a piece of text is written in — the same rule as the server's
 * guessWritingLang (apps/api/internal/api/lite_create_helpers.go): clearly more
 * Latin letters than Han characters is English; anything unclear is Chinese.
 */
export function guessLang(text: string): "zh" | "en" {
  let han = 0;
  let latin = 0;
  for (const ch of text) {
    if (/\p{Script=Han}/u.test(ch)) han++;
    else if (/[A-Za-z]/.test(ch)) latin++;
  }
  return latin >= 12 && latin > han * 4 ? "en" : "zh";
}
