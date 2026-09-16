export interface ComparisonText { feedback: string; observation: string }

/** The surrounding application owns this viewer; generated code stays isolated. */
export function ProcessComparison({comparison, source}: {comparison: ComparisonText; source: string}) {
  return <section id="process-comparison" aria-label="制作过程对比" style={{padding:"24px",background:"#f4f0e6",color:"#23201c"}}>
    <h2 style={{fontSize:24,margin:"0 0 16px"}}>我的修改过程</h2>
    <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fit,minmax(min(100%,320px),1fr))",gap:20}}>
      {(["before","after"] as const).map(side=><article key={side} style={{border:"1px solid #b8b1a4",borderRadius:16,overflow:"hidden",background:"white"}}>
        <h3 style={{padding:"0 16px"}}>{side==="before"?"修改前":"修改后"}</h3>
        <iframe title={side==="before"?"展示的修改前版本":"展示的修改后版本"} src={`${source}${source.includes("?")?"&":"?"}comparison=${side}`} sandbox="allow-scripts" referrerPolicy="no-referrer" allow="camera 'none'; microphone 'none'; geolocation 'none'; payment 'none'" style={{display:"block",width:"100%",height:520,border:0}}/>
      </article>)}
    </div>
    <h3>修改意见</h3><p style={{whiteSpace:"pre-wrap",overflowWrap:"anywhere"}}>{comparison.feedback}</p>
    <h3>试用判断</h3><p style={{whiteSpace:"pre-wrap",overflowWrap:"anywhere"}}>{comparison.observation}</p>
  </section>;
}
