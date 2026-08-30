import { go } from "../route";
import { cx } from "../ui";

/**
 * 世界 ⇄ 我的家 — the one control the whole product hangs off.
 *
 * It is a two-state pill rather than two tabs because the two views are not
 * peers in a list; they are two sides of the same thing (向外 / 向内). The
 * knob slides, the labels stay legible in both states, and the pill styles
 * itself for whichever ground it is sitting on.
 */
export function ViewSwitch({ view }: { view: "world" | "tree" }) {
  const dark = view === "world";
  return (
    <div
      className="relative flex items-center rounded-mk-full p-1"
      style={{
        background: dark ? "rgba(240,233,224,.08)" : "var(--mk-surface)",
        border: dark ? "1px solid rgba(240,233,224,.16)" : "1px solid var(--mk-border)",
        boxShadow: dark ? "none" : "var(--mk-shadow-xs)",
      }}
      role="tablist"
      aria-label="切换视图"
    >
      <span
        aria-hidden
        className="absolute top-1 h-[calc(100%-8px)] rounded-mk-full transition-all duration-[320ms] ease-mk"
        style={{
          width: "calc(50% - 4px)",
          left: view === "world" ? "4px" : "calc(50%)",
          background: dark
            ? "linear-gradient(140deg,#F3ECE3,#DCD1C3)"
            : "linear-gradient(140deg,var(--mk-accent-400),var(--mk-accent-600))",
        }}
      />
      {(
        [
          { key: "world", label: "世界", sub: "向外" },
          { key: "tree", label: "我的家", sub: "向内" },
        ] as const
      ).map((t) => {
        const active = view === t.key;
        return (
          <button
            key={t.key}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => go({ name: "home", view: t.key })}
            className={cx(
              "relative z-10 flex w-[118px] shrink-0 items-center justify-center gap-1.5 whitespace-nowrap",
              "rounded-mk-full px-3 py-2",
              "text-mk-body transition-colors duration-[200ms] ease-mk focus-visible:outline-none",
            )}
            style={{
              color: active
                ? dark
                  ? "#17130F"
                  : "#FFFFFF"
                : dark
                  ? "#B6A99A"
                  : "var(--mk-secondary)",
              fontWeight: active ? 600 : 400,
            }}
          >
            {t.label}
            <span className="eco-mono opacity-60">{t.sub}</span>
          </button>
        );
      })}
    </div>
  );
}
