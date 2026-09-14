import bookmark from "../home/assets/yinji-bookmark.webp";

/** The same visual identity across student learning workspaces. */
export function StudentCoachHeading({ label = "AI 学习伙伴" }: { label?: string }) {
  return <div className="student-coach-heading"><img src={bookmark} alt="" /><div>印记<span>{label}</span></div></div>;
}
