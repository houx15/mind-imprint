import { useEffect, type CSSProperties } from "react";
import { useAccent } from "@/ui";
import { useBackground } from "@/ui/background";

/**
 * The lite theme scope, shared by the student shell and the teacher shell.
 *
 * `ui/themes/lite.css` maps every `--mk-accent-*` onto `--mk-theme-accent-*`
 * inside `.lite-student` / `body.lite-student-theme`, so a shell that mounts in
 * that scope must also supply those variables. The returned `themeStyle` goes
 * on the shell's root (next to the `lite-student` class, `data-accent` and
 * `data-background`). Portals (dialogs, the inbox panel) render under `body`,
 * so the class, the accent variables and the background are mirrored onto
 * `body` too and restored on unmount.
 */
export function useLiteTheme(): { accent: string; background: string; themeStyle: CSSProperties } {
  const { id: accent, presets } = useAccent();
  const palette = presets.find((p) => p.id === accent) ?? presets[0]!;
  const themeStyle = Object.fromEntries(
    Object.entries(palette.scale).map(([step, value]) => [`--mk-theme-accent-${step}`, value]),
  ) as CSSProperties;
  const { id: background } = useBackground();

  useEffect(() => {
    const body = document.body;
    const hadClass = body.classList.contains("lite-student-theme");
    body.classList.add("lite-student-theme");
    return () => {
      if (!hadClass) body.classList.remove("lite-student-theme");
    };
  }, []);

  useEffect(() => {
    const body = document.body;
    const previousBackground = body.getAttribute("data-background");
    const previous = Object.keys(palette.scale).map((step) => {
      const key = `--mk-theme-accent-${step}`;
      return [key, body.style.getPropertyValue(key)] as const;
    });
    body.dataset.background = background;
    for (const [step, value] of Object.entries(palette.scale)) body.style.setProperty(`--mk-theme-accent-${step}`, value);
    return () => {
      if (previousBackground === null) body.removeAttribute("data-background");
      else body.setAttribute("data-background", previousBackground);
      for (const [key, value] of previous) {
        if (value) body.style.setProperty(key, value);
        else body.style.removeProperty(key);
      }
    };
  }, [palette, background]);

  return { accent, background, themeStyle };
}
