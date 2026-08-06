import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { IconButton } from "@/ui/Button";
import { X } from "@/ui/Icon";

/**
 * Overlays (design-system foundation, Part 1 Task 8): Modal, Drawer, Menu,
 * toast + ToastHost (spec §12).
 *
 * All four render through a React portal to `document.body` so they escape
 * whatever `overflow`/`transform` ancestors happen to be in play.
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

export function Drawer({ open, onClose, side = "right", children, className }: DrawerProps) {
  const panelRef = useRef<HTMLDivElement>(null);

  useEscape(open, onClose);

  useEffect(() => {
    if (open) panelRef.current?.focus();
  }, [open]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-mk-ink/40" onClick={onClose} aria-hidden="true" />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label="Panel"
        tabIndex={-1}
        className={cx(
          "absolute inset-y-0 z-10 flex w-full max-w-[420px] flex-col bg-mk-surface shadow-mk-lg focus:outline-none",
          DRAWER_SIDE[side],
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
  const rootRef = useRef<HTMLDivElement>(null);

  useEscape(open, () => setOpen(false));

  useEffect(() => {
    if (!open) return;
    function onDocMouseDown(e: MouseEvent) {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", onDocMouseDown);
    return () => document.removeEventListener("mousedown", onDocMouseDown);
  }, [open]);

  return (
    <div ref={rootRef} className={cx("relative inline-block", className)}>
      <div onClick={() => setOpen((o) => !o)}>{trigger}</div>
      {open && (
        <div
          role="menu"
          className="absolute right-0 z-20 mt-2 min-w-[160px] rounded-mk-md bg-mk-surface p-1 shadow-mk-lg"
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
        </div>
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
