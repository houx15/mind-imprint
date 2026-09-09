import { describe, expect, it } from "vitest";
import { analyzeHtml } from "./referenceAnalysis";

describe("reference analysis", () => {
  it("extracts HTML structure instead of only a text summary", () => {
    const spec = analyzeHtml(`
      <html><head><title>Lin Portfolio</title><meta name="description" content="设计与研究作品集"></head>
      <body class="portfolio minimal"><header><nav><a>About</a><a>Work</a></nav></header>
      <main><section><h1>About me</h1><img src="hero.jpg"></section>
      <section class="project-grid"><h2>Selected work</h2><a>Case study</a><img src="work.jpg"></section></main>
      <style>.portfolio { color: #112233 } .project-grid { display:grid }</style></body></html>
    `, "reference.html");
    expect(spec.title).toBe("Lin Portfolio");
    expect(spec.sections).toHaveLength(2);
    expect(spec.sections[1]?.layout).toBe("网格或卡片");
    expect(spec.navigation.items).toEqual(["About", "Work"]);
    expect(spec.contentSignals.images).toBe(2);
    expect(spec.visualStyle.palette).toContain("#112233");
  });
});
