import { describe, expect, it } from "vitest";
import {
  EDITION_REDIRECT_FLAG,
  arrivedByRedirect,
  decideEdition,
  normalizeEditionUrl,
} from "@/shell/edition/editionRouting";

/**
 * The rules that decide whether a signed-in student is standing in the right
 * app. Pure by construction — no DOM, no env — which is the whole reason
 * `decideEdition` takes its inputs rather than reading them.
 */

describe("normalizeEditionUrl", () => {
  it("stamps the redirect flag onto a configured URL", () => {
    const url = normalizeEditionUrl("https://mind-lite.uni-robot.cn");
    expect(url).not.toBeNull();
    expect(new URL(url!).searchParams.get(EDITION_REDIRECT_FLAG)).toBe("1");
  });

  it("preserves an existing path and query", () => {
    const url = new URL(normalizeEditionUrl("https://mind-lite.uni-robot.cn/readings?x=1")!);
    expect(url.pathname).toBe("/readings");
    expect(url.searchParams.get("x")).toBe("1");
  });

  it.each([
    ["unset", undefined],
    ["empty", ""],
    ["whitespace", "   "],
    ["not a URL", "mind-lite"],
    ["a non-http scheme", "javascript:alert(1)"],
    ["a non-string", 42],
  ])("returns null for %s", (_label, raw) => {
    expect(normalizeEditionUrl(raw)).toBeNull();
  });
});

describe("arrivedByRedirect", () => {
  it("is true only when the flag is present", () => {
    expect(arrivedByRedirect(`?${EDITION_REDIRECT_FLAG}=1`)).toBe(true);
    expect(arrivedByRedirect("?other=1")).toBe(false);
    expect(arrivedByRedirect("")).toBe(false);
  });
});

describe("decideEdition", () => {
  const URL_LITE = "https://mind-lite.uni-robot.cn/?edition_redirect=1";

  it("stays when the student's edition matches this app", () => {
    expect(
      decideEdition({
        appEdition: "pro",
        userEdition: "pro",
        targetUrl: URL_LITE,
        arrivedByRedirect: false,
      }),
    ).toEqual({ kind: "stay" });
  });

  it("redirects a lite student who signed in on the pro host", () => {
    expect(
      decideEdition({
        appEdition: "pro",
        userEdition: "lite",
        targetUrl: URL_LITE,
        arrivedByRedirect: false,
      }),
    ).toEqual({ kind: "redirect", target: "lite", url: URL_LITE });
  });

  it("redirects in the other direction too", () => {
    const proUrl = "https://mind-web.uni-robot.cn/?edition_redirect=1";
    expect(
      decideEdition({
        appEdition: "lite",
        userEdition: "pro",
        targetUrl: proUrl,
        arrivedByRedirect: false,
      }),
    ).toEqual({ kind: "redirect", target: "pro", url: proUrl });
  });

  // The graceful-degradation rule. A student must never be ejected from an
  // app that was working for them because a field arrived empty or from an
  // older API — being in the wrong app is recoverable, being in neither is
  // not.
  it.each([
    ["missing", undefined],
    ["null", null],
    ["empty", ""],
    ["unrecognised", "enterprise"],
  ])("stays put when the edition is %s", (_label, userEdition) => {
    expect(
      decideEdition({
        appEdition: "pro",
        userEdition,
        targetUrl: URL_LITE,
        arrivedByRedirect: false,
      }),
    ).toEqual({ kind: "stay" });
  });

  it("is stranded, not redirecting, when no target URL is configured", () => {
    expect(
      decideEdition({
        appEdition: "pro",
        userEdition: "lite",
        targetUrl: null,
        arrivedByRedirect: false,
      }),
    ).toEqual({ kind: "stranded", target: "lite" });
  });

  // The loop breaker. If the host we were sent to sends us straight back —
  // two deployments misconfigured to the same edition, or one DNS name
  // pointing at the other's app — the flag on our own URL stops the bounce.
  // Without this, the browser ping-pongs between two hosts forever and the
  // student can neither read the page nor press Back.
  it("stops instead of bouncing again when we arrived by redirect", () => {
    expect(
      decideEdition({
        appEdition: "pro",
        userEdition: "lite",
        targetUrl: URL_LITE,
        arrivedByRedirect: true,
      }),
    ).toEqual({ kind: "stranded", target: "lite" });
  });
});
