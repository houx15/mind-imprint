/**
 * editionRouting — deciding whether the signed-in student is standing in the
 * right app, and where to send them if not.
 *
 * WHY THIS EXISTS. 思维印记 ships as two frontends: the full version
 * (apps/web) and the lite version (apps/lite-web). Which one a student
 * belongs to is an ORGANISATION fact — `schools.edition`, migration 0093 —
 * not a per-account setting, and the API already gates every route group on
 * it (`requireEdition`, authz.go), answering 404 for the other edition's
 * surface.
 *
 * The frontends need the same rule, because a student who lands on the wrong
 * host sees an app whose every request 404s. Rather than duplicate the
 * judgement in two shells, both apps show the SAME auth screen and then run
 * THIS module against `user.school.edition`.
 *
 * The redirect is safe to perform because the frontends and the API all sit
 * under one registrable domain (`uni-robot.cn`), so the session cookie —
 * host-only on the API, `SameSite=Lax` — is same-site for both apps. The
 * student arrives at the other host already signed in; there is no second
 * login and no cookie change anywhere in this design.
 *
 * This module is deliberately PURE, and the app's own edition is passed in as
 * a compile-time constant by whichever shell imports it, never read from the
 * environment. An env var that failed to be set would make an app believe it
 * is the other edition and bounce every one of its own students away forever
 * — the exact failure this indirection removes.
 */

export type Edition = "pro" | "lite";

/** What each edition is called to a student. Used in the notice copy. */
export const EDITION_LABELS: Record<Edition, string> = {
  pro: "完整版",
  lite: "轻量版",
};

/**
 * Query flag stamped on the URL we send a student TO. If that host bounces
 * them straight back — two deployments misconfigured to the same edition, or
 * one DNS record pointing both names at one app — the flag is already there
 * and we stop, showing a dead end instead of ping-ponging the browser
 * between two hosts forever.
 */
export const EDITION_REDIRECT_FLAG = "edition_redirect";

/**
 * Turn a configured app URL into the address to send a student to, or null
 * if it isn't usable. Null is a real outcome, not an error to swallow: an
 * unset or malformed URL means we know the student is in the wrong place but
 * cannot say where the right one is, and the notice then says exactly that
 * rather than offering a button that goes nowhere.
 */
export function normalizeEditionUrl(raw: unknown): string | null {
  if (typeof raw !== "string") return null;
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return null;
  }
  if (url.protocol !== "https:" && url.protocol !== "http:") return null;
  url.searchParams.set(EDITION_REDIRECT_FLAG, "1");
  return url.toString();
}

/** The configured home of the other edition, or null if none is configured. */
export function editionHomeUrl(target: Edition): string | null {
  const raw =
    target === "lite" ? import.meta.env.VITE_LITE_APP_URL : import.meta.env.VITE_PRO_APP_URL;
  return normalizeEditionUrl(raw);
}

/** True if we arrived here as the result of an edition redirect. */
export function arrivedByRedirect(search: string): boolean {
  return new URLSearchParams(search).get(EDITION_REDIRECT_FLAG) === "1";
}

export type EditionDecision =
  | { kind: "stay" }
  /** Wrong app, and we know where the right one is. */
  | { kind: "redirect"; target: Edition; url: string }
  /** Wrong app, but we cannot send them — show the dead end honestly. */
  | { kind: "stranded"; target: Edition };

/**
 * Decide what to do with a signed-in student.
 *
 * Note the first rule: an edition we do not recognise means STAY. A student
 * is never locked out of the app in front of them because the field arrived
 * empty, misspelled, or from an older API that predates it. Being in the
 * wrong app degrades gracefully — routes 404 and they can ask a teacher —
 * whereas being ejected from the RIGHT one on a parse failure leaves them
 * nowhere at all.
 */
export function decideEdition(args: {
  appEdition: Edition;
  userEdition: string | null | undefined;
  targetUrl: string | null;
  arrivedByRedirect: boolean;
}): EditionDecision {
  const { appEdition, userEdition, targetUrl } = args;
  if (userEdition !== "pro" && userEdition !== "lite") return { kind: "stay" };
  if (userEdition === appEdition) return { kind: "stay" };
  if (args.arrivedByRedirect) return { kind: "stranded", target: userEdition };
  if (targetUrl === null) return { kind: "stranded", target: userEdition };
  return { kind: "redirect", target: userEdition, url: targetUrl };
}

/**
 * The wiring `decideEdition` deliberately does not do: read the configured
 * URLs out of the build environment and the redirect flag off the current
 * location. Kept as a separate, trivial wrapper so the rules above stay
 * testable with neither a DOM nor an env.
 */
export function resolveEditionDecision(
  appEdition: Edition,
  userEdition: string | null | undefined,
): EditionDecision {
  const target: Edition = userEdition === "lite" ? "lite" : "pro";
  return decideEdition({
    appEdition,
    userEdition,
    targetUrl: editionHomeUrl(target),
    arrivedByRedirect:
      typeof window === "undefined" ? false : arrivedByRedirect(window.location.search),
  });
}
