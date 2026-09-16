import { Button } from "@/ui";
import { studentArtwork } from "../learning/StudentArtwork";

/** 教师端的空状态：插图 + 一句话 + 可选的一颗按钮。
 *  只给占整页或整段的空状态用；嵌在小面板、表格单元、下拉里的空状态保持一行字。 */
export function StudioEmpty({
  children,
  kind = "ideas",
  action,
}: {
  children: React.ReactNode;
  kind?: keyof typeof studentArtwork;
  action?: { label: string; onClick: () => void };
}) {
  return (
    <div className="teacher-empty">
      <img src={studentArtwork[kind]} alt="" />
      <p>{children}</p>
      {action && (
        <Button variant="primary" size="sm" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </div>
  );
}
export function StudioHeading({ label, title, kind = "writing" }: { label: string; title: string; kind?: keyof typeof studentArtwork }) {
  return <header className="teacher-hero teacher-hero-compact"><div><p className="teacher-eyebrow">{label}</p><h1>{title}</h1></div><img src={studentArtwork[kind]} alt="" /></header>;
}
