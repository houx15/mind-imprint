import { Blank, Ground, MONO, hair, textMix, type LayoutProps } from "./parts";
import type { SiteLayout } from "./types";

/** The confirmed outline is the page order. Planning notes never render here. */
export function StructuredSite({ site, theme, narrow, editing, heroUrl, layout }: LayoutProps & { layout: SiteLayout }) {
  const compact = layout === "ledger";
  return <Ground theme={theme}>
    {heroUrl && <img src={heroUrl} alt="" style={{ width: "100%", height: narrow ? 160 : 260, objectFit: "cover" }} />}
    <main style={{ maxWidth: compact ? 900 : 1000, margin: "0 auto", padding: narrow ? "24px" : "48px 64px", overflowWrap: "anywhere" }}>
      <header style={{ paddingBottom: compact ? 24 : 48, borderBottom: `1px solid ${hair(.2)}` }}>
        <p style={{ ...MONO, fontSize: 12, color: textMix(.6) }}>{site.name} {site.role && `· ${site.role}`}</p>
        {site.headline ? <h1 style={{ fontSize: narrow ? 32 : compact ? 40 : 56, lineHeight: 1.2, margin: "20px 0" }}>{site.headline}</h1> : <Blank what="首屏内容待补充" editing={editing} />}
        {site.lead && <p style={{ marginTop: 16, lineHeight: 1.9 }}>{site.lead}</p>}
        {site.about.map((text, i) => <p key={i} style={{ marginTop: 12, lineHeight: 1.9 }}>{text}</p>)}
      </header>
      {(site.sections ?? []).map((section, index) => <section key={section.key} style={{
        marginLeft: section.depth * (narrow ? 10 : 24),
        padding: compact ? "20px 0" : "32px 0",
        borderBottom: `1px solid ${hair(.12)}`,
        ...(layout === "magazine" && section.depth === 0 ? { borderTop: "3px solid var(--st-accent)", marginTop: 24 } : {}),
      }}>
        <div style={{ display: "flex", gap: 16, alignItems: "baseline" }}>
          {compact && <span style={{ ...MONO, color: textMix(.5), fontSize: 12 }}>{String(index + 1).padStart(2, "0")}</span>}
          {section.depth === 0 ? <h2 style={{ fontSize: narrow ? 24 : 30, fontWeight: 650 }}>{section.title}</h2> : <h3 style={{ fontSize: narrow ? 18 : 22, fontWeight: 600 }}>{section.title}</h3>}
        </div>
        {section.imageUrl && <img src={section.imageUrl} alt={section.title} style={{ display: "block", width: "100%", height: "auto", marginTop: 20, borderRadius: 8 }} />}
        {section.body ? <p style={{ whiteSpace: "pre-wrap", marginTop: 16, lineHeight: 1.95, color: textMix(.85) }}>{section.body}</p> :
          <div style={{ marginTop: 12 }}><Blank what={section.imageUrl ? "图片说明待补充" : "模块内容待补充"} editing={editing} /></div>}
      </section>)}
      {site.email && <a href={`mailto:${site.email}`} style={{ display: "inline-block", marginTop: 32, color: "var(--st-accent)" }}>{site.email}</a>}
      <footer style={{ ...MONO, marginTop: 48, fontSize: 11, color: textMix(.45) }}>{site.name} · 用思维印记搭建</footer>
    </main>
  </Ground>;
}
