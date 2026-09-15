import { studentArtwork } from "../learning/StudentArtwork";
export function StudioEmpty({ children, kind = "ideas" }: { children: React.ReactNode; kind?: keyof typeof studentArtwork }) {
  return <div className="teacher-empty"><img src={studentArtwork[kind]} alt="" /><p>{children}</p></div>;
}
export function StudioHeading({ label, title, kind = "writing" }: { label: string; title: string; kind?: keyof typeof studentArtwork }) {
  return <header className="teacher-hero teacher-hero-compact"><div><p className="teacher-eyebrow">{label}</p><h1>{title}</h1></div><img src={studentArtwork[kind]} alt="" /></header>;
}
