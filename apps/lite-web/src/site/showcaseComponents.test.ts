import { describe, expect, it } from "vitest";
import { componentDocument, isolatedComponentDocument } from "./ShowcaseComponents";

describe("component document isolation",()=>{
  it("keeps hostile document boundaries inside the inner srcdoc attribute",()=>{
    const source=`</iframe><script>parent.document.body.remove()</script><p title="&quot;">学生作品</p>`;
    const doc=new DOMParser().parseFromString(isolatedComponentDocument(source),"text/html");
    expect(doc.querySelectorAll("script")).toHaveLength(0);
    expect(doc.querySelectorAll("iframe")).toHaveLength(1);
    expect(doc.querySelector("iframe")?.getAttribute("srcdoc")).toBe(componentDocument(source));
    expect(doc.querySelector("meta[http-equiv]")?.getAttribute("content")).toContain("frame-src about:");
  });
  // Source comes from arbitrary student files; CSP must precede even a supplied
  // full HTML document, so imported scripts cannot execute before restrictions.
  it("places resource policy before student HTML and preserves inline Canvas code",()=>{
    const source='<!doctype html><html><head><script>window.demo=1</script></head><body><canvas></canvas></body></html>';
    const html=componentDocument(source);
    expect(html.indexOf('Content-Security-Policy')).toBeLessThan(html.indexOf('window.demo'));
    expect(html).toContain("connect-src 'none'");
    expect(html).toContain("frame-src 'none'");
    expect(html).toContain("form-action 'none'");
    expect(html).toContain("base-uri 'none'");
    expect(html).toContain(source);
  });
});
