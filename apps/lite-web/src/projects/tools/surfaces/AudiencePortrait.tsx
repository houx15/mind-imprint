import portraits from "../../assets/audience-portraits-v1.png";

/** The four original portraits share one image; CSS displays one quadrant. */
export function AudiencePortrait({ role, className = "" }: { role: string; className?: string }) {
  const position = ({ 父母: "0% 0%", 老师: "100% 0%", 同学: "0% 100%", 陌生人: "100% 100%" } as Record<string, string>)[role] ?? "100% 100%";
  return <div aria-hidden="true" className={`aspect-square ${className}`} style={{ backgroundImage: `url(${portraits})`, backgroundSize: "200% 200%", backgroundPosition: position, backgroundRepeat: "no-repeat" }} />;
}
