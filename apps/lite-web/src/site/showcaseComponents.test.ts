import { describe, expect, it } from "vitest";
import { componentDocument } from "./ShowcaseComponents";

describe("component document isolation",()=>{
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
