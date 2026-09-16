import { summarizeOutput } from "./outputSummary";
import { Says } from "../projects/Says";

/** Read-only text summary. The original payload is retained, never executed as HTML. */
export function OutputRecord({ value, labels, markdown = false }: { value: unknown; labels?: Record<string, string>; markdown?: boolean }) {
  const lines = summarizeOutput(value, labels);
  return <div className="mt-2 min-w-0">
    {lines.length > 0 ? <dl className="flex flex-col gap-2">
      {lines.map((line, i) => <div key={i}>
        <dt className="text-mk-small text-mk-muted">{line.label}</dt>
        <dd className={`break-words text-mk-small text-mk-ink ${markdown ? "" : "whitespace-pre-wrap"}`}>{markdown ? <Says content={line.text} /> : line.text}</dd>
      </div>)}
    </dl> : <p className="text-mk-small text-mk-muted">{value == null ? "暂无记录" : "暂无文字摘要，可展开查看完整记录。"}</p>}
    {value != null && typeof value !== "string" && <details className="mt-2 text-mk-small text-mk-muted">
      <summary className="cursor-pointer">查看完整记录</summary>
      <pre className="mt-2 whitespace-pre-wrap break-words">{JSON.stringify(value, null, 2)}</pre>
    </details>}
  </div>;
}
