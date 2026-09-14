import { useEffect } from "react";

/**
 * useNoIndex — mounts `<meta name="robots" content="noindex, nofollow, noarchive">`
 * while the page is on screen and removes it on unmount.
 *
 * For the public pages opened by a share link (`/p/:token`, `/r/:token`). The
 * server already sends `X-Robots-Tag`; the meta covers the same rule at the
 * page layer. She is a minor, and these links are for people, not search
 * engines.
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
