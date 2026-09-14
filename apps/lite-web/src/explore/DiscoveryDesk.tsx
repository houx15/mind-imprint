import { useState } from "react";
import { ArrowRight, ArrowLeft, Compass } from "lucide-react";
import type { ExplorePlanet } from "../api/explore";
import { fieldById } from "../tree/geometry";
import type { FieldId } from "../tree/types";
import { studentArtwork } from "../learning/StudentArtwork";

/** A browsable edition of today's real stories. Previewing never saves a reading. */
export function DiscoveryDesk({ planets, lang, seen, onOpen, connections, onWord }: {
  planets: ExplorePlanet[];
  lang: "zh" | "en";
  seen: string[];
  onOpen: (id: string) => void;
  connections: { planetId: string; to: { id: string; zh: string } }[];
  onWord: (id: string) => void;
}) {
  const [selected, setSelected] = useState<string | null>(null);
  const index = Math.max(0, planets.findIndex(p => p.id === selected));
  const item = planets[index];
  if (!item) return null;
  const field = fieldById(item.field as FieldId);
  const illustration = item.field === "making" || item.field === "society" ? studentArtwork.ideas : item.field === "self" || item.field === "humanities" || item.field === "arts" ? studentArtwork.writing : studentArtwork.discovery;
  const related = connections.filter(t => t.planetId === item.id);
  const title = lang === "en" ? item.titleEn || item.titleZh : item.titleZh;
  function step(delta: number) { setSelected(planets[(index + delta + planets.length) % planets.length]!.id); }
  return <div className="discovery-desk" style={{ ["--discovery-hue" as string]: field.hue }}>
    <div className="discovery-intro"><span>THE DAILY DISCOVERY</span><h1>世界里，<br />还有哪些新发现？</h1><p>从一个问题开始，探索今天的科学与生活。</p></div>
    <div className="discovery-spread">
      <nav className="discovery-index" aria-label="今日发现">
        <p>今日选题 <span>{planets.length}</span></p>
        {planets.map((p,i) => <button type="button" key={p.id} aria-pressed={p.id===item.id} onClick={()=>setSelected(p.id)}>
          <span className="discovery-index__number">{String(i+1).padStart(2,"0")}</span>
          <span><strong>{lang==="en" ? p.titleEn || p.titleZh : p.titleZh}</strong><small>{fieldById(p.field as FieldId).label}{p.finished ? " · 已读完" : p.saved ? " · 在阅读室" : seen.includes(p.id) ? " · 已浏览" : ""}</small></span>
          <ArrowRight size={17} />
        </button>)}
      </nav>
      <section className="discovery-feature" aria-label="选题预览" key={item.id}>
        <div className="discovery-art" aria-hidden="true"><span className="discovery-orbit" /><span className="discovery-art__label"><Compass size={18}/>{field.label}</span><img src={illustration} alt="" /><span className="discovery-art__number">{String(index+1).padStart(2,"0")}</span></div>
        <div className="discovery-story"><span className="discovery-kicker">{item.source || field.label}</span><h2>{title}</h2><p className="discovery-question">{item.hook}</p><p className="discovery-summary">{item.summary}</p>
          <button type="button" className="discovery-open" onClick={()=>onOpen(item.id)}>探索这件事 <ArrowRight size={20}/></button>
          {related.length>0 && <div className="discovery-connections"><span>与你的兴趣相连</span>{related.map(t=><button type="button" key={t.to.id} onClick={()=>onWord(t.to.id)}>{t.to.zh} ↗</button>)}</div>}
        </div>
      </section>
    </div>
    <footer className="discovery-footer"><span>发现 {index+1} / {planets.length}</span><div><button type="button" onClick={()=>step(-1)} aria-label="上一条发现"><ArrowLeft size={18}/></button><button type="button" onClick={()=>step(1)} aria-label="下一条发现"><ArrowRight size={18}/></button></div></footer>
  </div>;
}
