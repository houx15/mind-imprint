// 成长报告 — the rail slot exists (binding design), the surface does not yet.
// The old task-based 我的评估 was retired with the task model in Slice 5d; the
// project-backed report lands in Slice 9 (评估 view) + Slice 10 (growth report).
export function GrowthPlaceholder() {
  return (
    <div style={{ height: "100%", display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 10, color: "#6B7384" }}>
      <div style={{ fontSize: 16, fontWeight: 700, color: "#3A4256" }}>成长报告正在重建</div>
      <div style={{ fontSize: 13.5, lineHeight: 1.7, maxWidth: 420, textAlign: "center" }}>
        我们正在按新的过程模型重做评估与成长报告。你在工作室里的每一步都在被记录，报告回来时它们都在。
      </div>
    </div>
  );
}
