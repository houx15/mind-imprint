export type TextRange = { start: number; end: number };
export type TextChanges = { before: TextRange[]; after: TextRange[] };

/** Match lines first, then characters within changed runs; bound the matrix size. */
export function textChanges(before: string, after: string): TextChanges {
  const result: TextChanges = { before: [], after: [] };
  function match(a: string[], b: string[], offsetA: number, offsetB: number, refine: boolean) {
    const width = b.length + 1;
    if (a.length * b.length > 1_000_000) {
      // Large rewrites remain exact: mark the unmatched middle, retain shared edges.
      let left = 0, right = 0;
      while (left < a.length && left < b.length && a[left] === b[left]) left++;
      while (right < a.length-left && right < b.length-left && a[a.length-1-right] === b[b.length-1-right]) right++;
      changed(a.slice(left,a.length-right).join(""), b.slice(left,b.length-right).join(""), offsetA+a.slice(0,left).join("").length, offsetB+b.slice(0,left).join("").length, false);
      return;
    }
    const dp = new Uint32Array((a.length+1)*width);
    for (let i=a.length-1;i>=0;i--) for (let j=b.length-1;j>=0;j--) {
      dp[i*width+j] = a[i]===b[j] ? 1+dp[(i+1)*width+j+1]! : Math.max(dp[(i+1)*width+j]!,dp[i*width+j+1]!);
    }
    let i=0,j=0,oldRun="",newRun="",x=offsetA,y=offsetB;
    const flush = () => { changed(oldRun,newRun,x,y,refine); x+=oldRun.length; y+=newRun.length; oldRun=""; newRun=""; };
    while(i<a.length || j<b.length) {
      if(i<a.length && j<b.length && a[i]===b[j]) { flush(); x+=a[i]!.length;y+=b[j]!.length;i++;j++; }
      else if(i<a.length && (j===b.length || dp[(i+1)*width+j]!>=dp[i*width+j+1]!)) oldRun+=a[i++];
      else newRun+=b[j++];
    }
    flush();
  }
  function changed(a:string,b:string,x:number,y:number,refine:boolean) {
    if (!a && !b) return;
    if(refine && a && b) { match(Array.from(a),Array.from(b),x,y,false); return; }
    if(a) result.before.push({start:x,end:x+a.length});
    if(b) result.after.push({start:y,end:y+b.length});
  }
  match(before.match(/[^\n]*\n|[^\n]+$/g) ?? [], after.match(/[^\n]*\n|[^\n]+$/g) ?? [],0,0,true);
  return result;
}

type Node = { type: string; value?: string; children?: Node[]; data?: object; position?: { start: { offset?: number }; end: { offset?: number } } };

/** Decorate parsed Markdown text, keeping its paragraphs, emphasis and lists. */
export function remarkTextChanges({ source, ranges, removed }: { source: string; ranges: TextRange[]; removed: boolean }) {
  return (tree: Node) => {
    const visit = (parent: Node) => {
      if (!parent.children) return;
      parent.children = parent.children.flatMap((node): Node[] => {
        if (node.type !== "text" || !node.value || node.position?.start.offset === undefined || node.position.end.offset === undefined) { visit(node); return [node]; }
        const start=node.position.start.offset, end=node.position.end.offset;
        let intersections=ranges.filter(r=>r.start<end && r.end>start).map(r=>({start:Math.max(start,r.start)-start,end:Math.min(end,r.end)-start}));
        if (!intersections.length) return [node];
        // Escaped/entity text has different source offsets; mark the whole text node.
        if(source.slice(start,end)!==node.value) intersections=[{start:0,end:node.value.length}];
        const pieces:Node[]=[]; let cursor=0;
        for(const range of intersections) {
          if(range.start>cursor) pieces.push({type:"text",value:node.value.slice(cursor,range.start)});
          pieces.push({type:removed?"delete":"strong",data:{hName:removed?"del":"mark"},children:[{type:"text",value:node.value.slice(range.start,range.end)}]});
          cursor=range.end;
        }
        if(cursor<node.value.length) pieces.push({type:"text",value:node.value.slice(cursor)});
        return pieces;
      });
    };
    visit(tree);
  };
}
