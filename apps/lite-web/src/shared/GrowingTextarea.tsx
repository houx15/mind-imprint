import { useLayoutEffect, useRef, type TextareaHTMLAttributes } from "react";

/** Grow with typed, pasted or restored text; keep long drafts within the viewport. */
export function GrowingTextarea({ value, style, rows = 1, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const ref = useRef<HTMLTextAreaElement>(null);
  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    const resize = () => {
      element.style.height = "auto";
      const computed = getComputedStyle(element);
      const borders = parseFloat(computed.borderTopWidth) + parseFloat(computed.borderBottomWidth);
      element.style.height = `${element.scrollHeight + borders}px`;
    };
    resize();
    let width = element.clientWidth;
    const observer = new ResizeObserver(() => {
      if (element.clientWidth === width) return;
      width = element.clientWidth;
      resize();
    });
    observer.observe(element);
    return () => observer.disconnect();
  }, [value, rows]);
  return <textarea {...props} ref={ref} rows={rows} value={value} style={{ ...style, boxSizing: "border-box", resize: "none", maxHeight: "min(240px, 40dvh)", overflowY: "auto" }} />;
}
