import { useEffect } from "react";
import { Button } from "@/ui";
import { EDITION_LABELS, type Edition, type EditionDecision } from "./editionRouting";

/**
 * EditionRedirectNotice — what a student sees when they have signed in on the
 * wrong edition's host.
 *
 * Deliberately NOT the shared `Modal` primitive. `Modal` gives every dialog a
 * close button, a backdrop that dismisses, and Escape-to-close — all correct
 * for a dialog you can meaningfully decline, and all wrong here: behind this
 * panel is an app in which every one of this student's API calls answers 404.
 * Dismissing it would only strand them in front of a broken screen. So this
 * is a plain blocking overlay with exactly one way forward.
 *
 * It announces the move rather than performing it silently (product decision,
 * 2026-08-27): a page that swaps itself for a different domain with no
 * explanation reads as a glitch, and the student is left unsure which address
 * is actually theirs. Telling them means the next time they can type the
 * right one.
 *
 * The jump uses `location.replace`, not `assign`: the wrong-edition page must
 * not stay in history, or Back lands the student right back in the app that
 * does not work for them.
 */
export function EditionRedirectNotice({
  decision,
  appEdition,
  delayMs = 4000,
  redirect = (url: string) => window.location.replace(url),
}: {
  decision: Exclude<EditionDecision, { kind: "stay" }>;
  appEdition: Edition;
  /** How long the explanation stays up before the jump happens on its own. */
  delayMs?: number;
  /** Injectable for tests — jsdom cannot navigate. */
  redirect?: (url: string) => void;
}) {
  const url = decision.kind === "redirect" ? decision.url : null;

  useEffect(() => {
    if (url === null) return;
    const timer = setTimeout(() => redirect(url), delayMs);
    return () => clearTimeout(timer);
  }, [url, delayMs, redirect]);

  const here = EDITION_LABELS[appEdition];
  const there = EDITION_LABELS[decision.target];

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-label="版本不匹配"
    >
      <div className="absolute inset-0 bg-black/40" aria-hidden="true" />
      <div className="relative z-10 flex w-full max-w-[420px] flex-col gap-3 rounded-mk-lg bg-mk-surface p-6 shadow-mk-lg">
        <h2 className="text-mk-h3 text-mk-ink">这个入口不是你的版本</h2>
        {url !== null ? (
          <>
            <p className="text-mk-body text-mk-secondary">
              这里是{here}的入口，而你的学校用的是{there}。我们这就把你送过去——你已经登录好了，不用再登一次。
            </p>
            <div className="flex justify-end pt-1">
              <Button variant="primary" onClick={() => redirect(url)}>
                立即前往{there}
              </Button>
            </div>
          </>
        ) : (
          <p className="text-mk-body text-mk-secondary">
            这里是{here}的入口，而你的学校用的是{there}。我们暂时没法把你送过去，请把这个情况告诉老师或管理员。
          </p>
        )}
      </div>
    </div>
  );
}
