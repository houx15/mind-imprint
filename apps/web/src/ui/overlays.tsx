import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { IconButton } from "@/ui/Button";
import { X } from "@/ui/Icon";
import { MOTION } from "@/ui/tokens";

/**
 * Overlays (design-system foundation, Part 1 Task 8): Modal, Drawer, Menu,
 * toast + ToastHost (spec §12).
 *
 * Modal, Drawer, and ToastHost render their ENTIRE tree through a React
 * portal to `document.body`. Menu keeps its trigger in normal document flow
 * (so it composes with a caller-supplied `<button>`/icon without producing
 * nested interactive elements) but portals only its popover — positioned
 * from the trigger's `getBoundingClientRect()` — so the dropdown itself
 * still escapes whatever `overflow:hidden`/`transform` ancestor it's used
 * inside (e.g. a project card, spec §10).
 *
 * GOTCHA (carried from Task 5/7): Tailwind emits same-CSS-property utility
 * classes in ALPHABETICAL order in the compiled stylesheet, not className
 * order. Every element below applies exactly ONE class per competing CSS
 * property (bg-*, border-color, text-<color>, rounded-*) — never two
 * competing utilities stacked "for order".
 */

/** Join truthy class fragments with a single space; drops falsy/empty ones. */
function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

function useEscape(active: boolean, onEscape: () => void) {
  useEffect(() => {
    if (!active) return;
    function onKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onEscape();
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [active, onEscape]);
}

// ---------------------------------------------------------------------------
// Modal — centered panel, for destructive/confirm flows.
// ---------------------------------------------------------------------------

export interface ModalProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  className?: string;
}

export function Modal({ open, onClose, title, children, footer, className }: ModalProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  const titleId = useId();

  useEscape(open, onClose);

  useEffect(() => {
    if (open) panelRef.current?.focus();
  }, [open]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-mk-ink/40" onClick={onClose} aria-hidden="true" />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={title ? titleId : undefined}
        aria-label={title ? undefined : "Dialog"}
        tabIndex={-1}
        className={cx(
          "relative z-10 flex w-full max-w-[480px] flex-col rounded-mk-lg bg-mk-surface shadow-mk-lg focus:outline-none",
          className,
        )}
      >
        <div className="flex items-center justify-between gap-3 p-4">
          {title ? (
            <h2 id={titleId} className="text-mk-h3 text-mk-ink">
              {title}
            </h2>
          ) : (
            <span />
          )}
          <IconButton icon={X} label="Close" size="sm" onClick={onClose} />
        </div>
        <div className="px-4 pb-4 text-mk-body text-mk-ink">{children}</div>
        {footer && <div className="flex justify-end gap-2 border-t border-mk-border p-4">{footer}</div>}
      </div>
    </div>,
    document.body,
  );
}

// ---------------------------------------------------------------------------
// Drawer — off-canvas panel for sub-tasks / isolated context.
// ---------------------------------------------------------------------------

export type DrawerSide = "left" | "right";

export interface DrawerProps {
  open: boolean;
  onClose: () => void;
  side?: DrawerSide;
  children?: ReactNode;
  className?: string;
}

const DRAWER_SIDE: Record<DrawerSide, string> = {
  left: "left-0 rounded-r-mk-lg",
  right: "right-0 rounded-l-mk-lg",
};

// Off-canvas resting position per side (translated fully out of view before
// the enter transition, and again while playing the exit transition).
const DRAWER_HIDDEN: Record<DrawerSide, string> = {
  left: "-translate-x-full",
  right: "translate-x-full",
};

// Keep the panel mounted for this long after `open` flips false so the
// slide-out transition can play before it's removed from the DOM. Mirrors
// `MOTION.base` (the same `--mk-base` duration used in the transition class
// below) — kept as a plain number here since `setTimeout` can't read a CSS
// custom property.
const DRAWER_EXIT_MS = MOTION.base;

export function Drawer({ open, onClose, side = "right", children, className }: DrawerProps) {
  const panelRef = useRef<HTMLDivElement>(null);
  // `mounted` keeps the panel in the DOM through the exit animation;
  // `visible` toggles the translate class that actually drives the slide.
  const [mounted, setMounted] = useState(open);
  const [visible, setVisible] = useState(false);

  useEscape(open, onClose);

  // Mount immediately on open; on close, start the slide-out and unmount
  // only after the transition has had time to play.
  useEffect(() => {
    if (open) {
      setMounted(true);
      return;
    }
    setVisible(false);
    const timeout = window.setTimeout(() => setMounted(false), DRAWER_EXIT_MS);
    return () => window.clearTimeout(timeout);
  }, [open]);

  // Once mounted with `open`, flip to visible on the next frame so the
  // browser paints the off-canvas position first, then animates in.
  useEffect(() => {
    if (!mounted || !open) return;
    const raf = requestAnimationFrame(() => setVisible(true));
    return () => cancelAnimationFrame(raf);
  }, [mounted, open]);

  useEffect(() => {
    if (open) panelRef.current?.focus();
  }, [open]);

  if (!mounted || typeof document === "undefined") return null;

  return createPortal(
    <div className="fixed inset-0 z-50">
      <div
        className={cx(
          "absolute inset-0 bg-mk-ink/40 motion-safe:transition-opacity duration-[var(--mk-base)] ease-mk",
          visible ? "opacity-100" : "opacity-0",
        )}
        onClick={onClose}
        aria-hidden="true"
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label="Panel"
        tabIndex={-1}
        className={cx(
          "absolute inset-y-0 z-10 flex w-full max-w-[420px] flex-col bg-mk-surface shadow-mk-lg focus:outline-none",
          "motion-safe:transition-transform duration-[var(--mk-base)] ease-mk",
          DRAWER_SIDE[side],
          visible ? "translate-x-0" : DRAWER_HIDDEN[side],
          className,
        )}
      >
        <div className="flex-1 overflow-y-auto p-4">{children}</div>
      </div>
    </div>,
    document.body,
  );
}

// ---------------------------------------------------------------------------
// Menu — trigger + popover, click-toggle, click-outside/Esc close.
// ---------------------------------------------------------------------------

export interface MenuItem {
  key: string;
  label: ReactNode;
  onSelect: () => void;
  tone?: "danger";
}

export interface MenuProps {
  trigger: ReactNode;
  items: MenuItem[];
  className?: string;
}

export function Menu({ trigger, items, className }: MenuProps) {
  const [open, setOpen] = useState(false);
  // `null` until the trigger has been measured on open; the popover only
  // renders once we know where to place it.
  const [coords, setCoords] = useState<{ top: number; left: number } | null>(null);
  const triggerRef = useRef<HTMLDivElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  useEscape(open, () => setOpen(false));

  // Click-outside must treat BOTH the trigger wrapper and the portaled
  // popover as "inside" — they're siblings in the DOM once portaled, not
  // ancestor/descendant, so a single ref/contains check isn't enough.
  useEffect(() => {
    if (!open) return;
    function onDocMouseDown(e: MouseEvent) {
      const target = e.target as Node;
      if (triggerRef.current?.contains(target)) return;
      if (popoverRef.current?.contains(target)) return;
      setOpen(false);
    }
    document.addEventListener("mousedown", onDocMouseDown);
    return () => document.removeEventListener("mousedown", onDocMouseDown);
  }, [open]);

  function handleTriggerClick() {
    // Simple below-the-trigger placement, computed fresh each time the
    // trigger opens the menu — no flip/collision logic (spec doesn't ask
    // for it), so this is just the trigger's own bottom-left corner + gap.
    if (triggerRef.current) {
      const rect = triggerRef.current.getBoundingClientRect();
      setCoords({ top: rect.bottom + 8, left: rect.left });
    }
    setOpen((o) => !o);
  }

  return (
    <div ref={triggerRef} className={cx("inline-block", className)}>
      <div onClick={handleTriggerClick}>{trigger}</div>
      {open &&
        coords &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            ref={popoverRef}
            role="menu"
            style={{ position: "fixed", top: coords.top, left: coords.left }}
            className="z-20 min-w-[160px] rounded-mk-md bg-mk-surface p-1 shadow-mk-lg"
          >
            {items.map((item) => (
              <button
                key={item.key}
                type="button"
                role="menuitem"
                onClick={() => {
                  item.onSelect();
                  setOpen(false);
                }}
                className={cx(
                  "block w-full rounded-mk-sm px-3 py-2 text-left text-mk-body transition-colors duration-[120ms] ease-mk hover:bg-mk-paper",
                  item.tone === "danger" ? "text-mk-danger" : "text-mk-ink",
                )}
              >
                {item.label}
              </button>
            ))}
          </div>,
          document.body,
        )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// toast() + <ToastHost/> — imperative dark bubble, auto-dismiss.
// ---------------------------------------------------------------------------

export interface ToastOptions {
  duration?: number;
}

interface ToastItem {
  id: number;
  message: ReactNode;
}

type ToastListener = (items: ToastItem[]) => void;

let toastItems: ToastItem[] = [];
let toastListeners: ToastListener[] = [];
let toastNextId = 0;

function emitToasts() {
  toastListeners.forEach((listener) => listener(toastItems));
}

/** Imperative toast trigger. Mount `<ToastHost/>` once (e.g. in the app shell). */
export function toast(message: ReactNode, opts?: ToastOptions): number {
  const id = ++toastNextId;
  const duration = opts?.duration ?? 3000;
  toastItems = [...toastItems, { id, message }];
  emitToasts();
  if (typeof window !== "undefined") {
    window.setTimeout(() => {
      toastItems = toastItems.filter((item) => item.id !== id);
      emitToasts();
    }, duration);
  }
  return id;
}

export function ToastHost() {
  const [items, setItems] = useState<ToastItem[]>(toastItems);

  useEffect(() => {
    toastListeners.push(setItems);
    return () => {
      toastListeners = toastListeners.filter((l) => l !== setItems);
    };
  }, []);

  if (typeof document === "undefined") return null;

  return createPortal(
    <div className="pointer-events-none fixed inset-x-0 bottom-6 z-50 flex flex-col items-center gap-2">
      {items.map((item) => (
        <div
          key={item.id}
          role="status"
          className="pointer-events-auto rounded-[11px] bg-mk-ink px-4 py-2 text-mk-body text-white shadow-mk-lg"
        >
          {item.message}
        </div>
      ))}
    </div>,
    document.body,
  );
}
