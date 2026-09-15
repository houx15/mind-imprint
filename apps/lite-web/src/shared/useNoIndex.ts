import { useEffect } from "react";

/**
 * useNoIndex — mounts `<meta name="robots" content="noindex, nofollow, noarchive">`
 * while the page is on screen and removes it on unmount.
 *
 * For the public page opened by her homepage link (`/p/:token`). The server
 * already sends `X-Robots-Tag`; the meta covers the same rule at the page
 * layer. She is a minor, and the link is for people, not search engines.
 */
export function useNoIndex(): void {
  useEffect(() => {
    const meta = document.createElement("meta");
    meta.name = "robots";
    meta.content = "noindex, nofollow, noarchive";
    document.head.appendChild(meta);
    return () => {
      meta.remove();
    };
  }, []);
}
