import { toPng } from "html-to-image";

/**
 * exportPoster — rasterizes `node` to a PNG and triggers a browser download.
 *
 * `node` is expected to be a `ReportPoster`'s own root element, passed
 * directly — never a wrapper div added just to hold a ref. There is nothing
 * for an extra wrapper layer to do here (the poster's own root already
 * carries its offscreen positioning and fixed size — see ReportPoster.tsx),
 * so passing one would only ask html-to-image to walk one more, pointless
 * node.
 *
 * `pixelRatio: 2` for a crisp result on a typical phone/retina screen —
 * this is a picture meant to be sent to a parent or a friend, not viewed at
 * 1x. `cacheBust: true` appends a cache-busting query string to any image
 * requests html-to-image issues while walking the node, so a re-export
 * later in the same tab can't quietly serve a stale cached asset.
 *
 * Never throws into the caller: a failed export is a picture she doesn't
 * get, not a broken room around her. `ReportPanel` treats this call as
 * fire-and-forget beyond its own loading state.
 */
export async function exportPoster(node: HTMLElement | null, filename: string): Promise<void> {
  if (!node) return;
  try {
    const dataUrl = await toPng(node, { pixelRatio: 2, cacheBust: true });
    const a = document.createElement("a");
    a.href = dataUrl;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  } catch {
    /* exporting a picture should never break the room around it */
  }
}
