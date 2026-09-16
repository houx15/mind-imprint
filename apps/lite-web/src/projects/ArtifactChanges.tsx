import { PaperPreview } from "./PaperPreview";
import { readPaperLayout } from "../api/paperLayout";
import { ReviewMarkdown } from "./tools/surfaces/ReviewMarkdown";
import type { Artifact } from "../api/artifacts";
import { artifactMetadataChanges } from "../api/artifactMetadataChanges";
import { artifactEdits } from "../api/artifactExport";

/** Show the persisted replacement, not the coach's description of its intent. */
export function ArtifactChanges({artifact, previousArtifact, showMetadata = true}: {artifact: Artifact; previousArtifact?: Artifact; showMetadata?: boolean}) {
  const changes = artifactMetadataChanges(artifact, previousArtifact);
  const oldPaper = readPaperLayout(previousArtifact?.payload.paperLayout);
  const newPaper = readPaperLayout(artifact.payload.paperLayout);
  const paperChanged = oldPaper && newPaper && JSON.stringify(oldPaper) !== JSON.stringify(newPaper);
  return <>
    <BodyChanges artifact={artifact} />
    {paperChanged && previousArtifact && <section aria-label="图形版本对照" className="my-4 rounded-mk-md border border-mk-border p-4">
      <h3 className="font-semibold">图形版本对照</h3>
      <p className="mt-2 text-mk-small text-mk-secondary">请对照页面，检查位置、文字和留白是否符合修改要求。</p>
      {previousArtifact.why && <p className="mt-2 whitespace-pre-wrap text-mk-small">审核意见：{previousArtifact.why}</p>}
      <details className="mt-3">
        <summary className="cursor-pointer text-mk-small">展开修改前后图形</summary>
        <div className="grid gap-4" style={{gridTemplateColumns:"repeat(auto-fit, minmax(min(100%, 300px), 1fr))"}}>
          <PaperPreview key={`before-${previousArtifact.id}`} artifact={previousArtifact} heading="修改前" exportable={false}/>
          <PaperPreview key={`after-${artifact.id}`} artifact={artifact} heading="修改后" exportable={false}/>
        </div>
      </details>
    </section>}
    {showMetadata && changes.length > 0 && <section aria-label="假设与局限修改对照" className="my-4 rounded-mk-md border border-mk-border p-4 space-y-4">
      <h3 className="font-semibold">假设与局限修改对照</h3>
      {previousArtifact?.payload.body === artifact.payload.body && <p className="text-mk-small text-mk-secondary">正文未修改</p>}
      {changes.map(change => <div key={change.key}>
        <h4 className="mb-3 font-semibold">{change.label}</h4>
        <div className="grid gap-3" style={{gridTemplateColumns:"repeat(auto-fit, minmax(min(100%, 260px), 1fr))"}}>
          {([["修改前",change.before],["修改后",change.after]] as const).map(([label,items]) => <div key={label} className="min-w-0 rounded border border-mk-border p-3">
            <h5 className="mb-2 text-mk-small font-semibold">{label}</h5>
            {items.length ? <ul className="list-disc space-y-2 pl-5 text-mk-small break-words">{items.map((item,i)=><li key={i}>{item}</li>)}</ul> : <p className="text-mk-small text-mk-secondary">无</p>}
          </div>)}
        </div>
      </div>)}
    </section>}
  </>;
}

function BodyChanges({artifact}: {artifact: Artifact}) {
  const edits = artifactEdits(artifact);
  const previous = typeof artifact.payload.previousBody === "string" ? artifact.payload.previousBody : null;
  if (typeof artifact.payload.replacesArtifactId === "string" && previous !== null) {
    return <section className="my-4 rounded-mk-md border border-mk-border p-4" aria-label="版本对照">
      <h3 className="font-semibold">版本对照</h3>
      {typeof artifact.payload.revisionRequest === "string" && artifact.payload.revisionRequest && <p className="my-3 whitespace-pre-wrap text-mk-small">修改要求：{artifact.payload.revisionRequest}</p>}
      <details className="mt-3">
        <summary className="cursor-pointer text-mk-small">展开修改前后全文</summary>
        <div className="mt-3 grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 260px), 1fr))" }}>
          <div className="min-w-0 rounded border border-mk-border p-4"><h4 className="mb-3 font-semibold">修改前</h4><div className="max-h-[36rem] overflow-auto break-words text-mk-small"><ReviewMarkdown text={previous} marks={[]} onOpen={() => {}} /></div></div>
          <div className="min-w-0 rounded border border-mk-border p-4"><h4 className="mb-3 font-semibold">修改后</h4><div className="max-h-[36rem] overflow-auto break-words text-mk-small"><ReviewMarkdown text={artifact.payload.body ?? ""} marks={[]} onOpen={() => {}} /></div></div>
        </div>
      </details>
    </section>;
  }
  if (!edits.length) return null;
  return <section className="my-4 rounded-mk-md border border-mk-border p-3" aria-label="本次修改">
    <h3 className="font-semibold">本次修改 · {edits.length} 处</h3>
    <p className="my-2 text-mk-small">AI 可能扩大修改范围。请对照实际替换内容，检查是否符合修改要求。</p>
    {edits.map((edit,index) => <div key={index} className="mt-3 grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 260px), 1fr))" }}>
      <div className="rounded border border-mk-border p-3"><h4 className="mb-2 text-mk-small font-semibold">修改前</h4><p className="whitespace-pre-wrap break-words">{edit.old}</p></div>
      <div className="rounded border border-mk-border p-3"><h4 className="mb-2 text-mk-small font-semibold">修改后</h4><p className="whitespace-pre-wrap break-words">{edit.new || "（已删除）"}</p></div>
    </div>)}
  </section>;
}
