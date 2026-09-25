import type { ReactNode } from "react";
import { BookOpen } from "lucide-react";
import { Settings, type LucideIcon } from "@/ui";
import { navigate } from "../routing";
import "../home/learning.css";

export type RailLink = {
  key: string;
  label: string;
  icon: LucideIcon;
  active: boolean;
  onSelect: () => void;
};

/**
 * The lite navigation rail, shared by the student shell and the teacher shell:
 * paper background, 68px folded, 220px on hover or focus, accent-tinted active
 * item. Layout lives in `home/learning.css` (`learning-nav*`). The brand link
 * goes to `/`; each shell resolves `/` to its own landing page.
 *
 * `footer` sits at the bottom of the rail (the student's 收件箱, and the account
 * button for both shells).
 */
export function LearningRail({ links, footer }: { links: RailLink[]; footer: ReactNode }) {
  return (
    <div className="learning-nav-slot">
      <nav className="learning-nav" aria-label="主导航">
        <a
          href="/"
          className="learning-brand"
          onClick={(e) => {
            e.preventDefault();
            navigate("/");
          }}
        >
          <BookOpen size={29} />
          <span>
            思维印记<small>THE MARK OF THINKING</small>
          </span>
        </a>
        <div className="learning-nav-links">
          {links.map(({ key, label, icon: NavIcon, active, onSelect }) => (
            <button
              type="button"
              key={key}
              aria-label={label}
              aria-current={active ? "page" : undefined}
              onClick={onSelect}
            >
              <NavIcon size={21} strokeWidth={1.7} />
              <span>{label}</span>
            </button>
          ))}
        </div>
        <div className="learning-nav-bottom">{footer}</div>
      </nav>
    </div>
  );
}

/** The account button at the foot of the rail; it opens 设置. */
export function LearningAccountButton({
  name,
  image,
  active,
  onSelect,
}: {
  name: string;
  image: string;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      className="learning-account"
      type="button"
      aria-label="设置"
      aria-current={active ? "page" : undefined}
      onClick={onSelect}
    >
      <img src={image} alt="" />
      <span>
        {name || "我的账号"}
        <small>账号与设置</small>
      </span>
      <Settings size={17} />
    </button>
  );
}
