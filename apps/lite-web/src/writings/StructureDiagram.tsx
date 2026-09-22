/** Illustrates relationships only; names and teaching definitions remain in the API vocabulary. */
export function StructureDiagram({ kind }: { kind: string }) {
  const nodes: [number, number][] = kind === "struct_progressive"
    ? [[12, 52], [72, 32], [132, 12]]
    : kind === "struct_contrast"
      ? [[24, 12], [24, 52], [132, 32]]
      : kind === "struct_parallel"
        ? [[12, 12], [72, 12], [132, 12], [72, 52]]
        : [[72, 12], [12, 52], [72, 52], [132, 52]];
  const paths = kind === "struct_progressive" ? ["M44 60H60V40H72", "M104 40H120V20H132"]
    : kind === "struct_contrast" ? ["M56 20H96V40H132", "M56 60H96V40"]
    : kind === "struct_parallel" ? ["M28 28V40H148V28", "M88 28V52"]
    : ["M88 28V40H28V52", "M88 40H148V52", "M88 40V52"];
  const anchor = kind === "struct_parallel" ? 3 : kind === "struct_contrast" ? 2 : 0;
  return <svg className="writing-structure__diagram" viewBox="0 0 180 80" aria-hidden="true" fill="none">
    {paths.map((d) => <path key={d} d={d} stroke="currentColor" strokeWidth="1.6" opacity=".5" />)}
    {nodes.map(([x, y], i) => <g key={i}>
      <rect x={x} y={y} width="32" height="16" rx="5" fill="currentColor" opacity={i === anchor ? ".9" : ".15"} />
      <path d={`M${x + 9} ${y + 8}h14`} stroke={i === anchor ? "var(--mk-on-accent, white)" : "currentColor"} strokeWidth="2" strokeLinecap="round" />
    </g>)}
  </svg>;
}
