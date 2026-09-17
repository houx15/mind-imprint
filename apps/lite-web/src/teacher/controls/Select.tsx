import { useCallback, useId, useRef, useState, type KeyboardEvent } from "react";
import { Check, ChevronDown } from "lucide-react";
import { Popover } from "./Popover";
import "./controls.css";

// teacher/controls/Select.tsx — the teacher forms' dropdown, drawn with the
// lite tokens instead of the browser's <select>. A button opens a listbox;
// the keyboard works as on a native select (arrows, Home/End, Enter/Space,
// Escape, Tab).

export interface SelectOption<T extends string> {
  value: T;
  label: string;
}

export function Select<T extends string>({
  value,
  options,
  onChange,
  ariaLabel,
  placeholder = "请选择",
  disabled = false,
  size = "md",
  className = "",
}: {
  value: T;
  options: SelectOption<T>[];
  onChange: (value: T) => void;
  /** Needed when no <label> wraps the field. */
  ariaLabel?: string;
  placeholder?: string;
  disabled?: boolean;
  /** `sm`: the filter bars above a list. */
  size?: "md" | "sm";
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listId = useId();
  const selectedIndex = options.findIndex((o) => o.value === value);
  const selected = selectedIndex >= 0 ? options[selectedIndex] : undefined;

  const close = useCallback((refocus: boolean) => {
    setOpen(false);
    if (refocus) triggerRef.current?.focus();
  }, []);
  const closeOutside = useCallback(() => close(false), [close]);

  function openList() {
    if (disabled || options.length === 0) return;
    setActive(Math.max(0, selectedIndex));
    setOpen(true);
    requestAnimationFrame(() => {
      listRef.current?.focus();
      listRef.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest" });
    });
  }

  function pick(i: number) {
    const o = options[i];
    if (!o) return;
    if (o.value !== value) onChange(o.value);
    close(true);
  }

  function moveTo(i: number) {
    const next = Math.min(options.length - 1, Math.max(0, i));
    setActive(next);
    listRef.current?.querySelectorAll('[role="option"]')[next]?.scrollIntoView({ block: "nearest" });
  }

  function onTriggerKey(e: KeyboardEvent) {
    if (["ArrowDown", "ArrowUp", "Enter", " "].includes(e.key)) {
      e.preventDefault();
      openList();
    }
  }

  function onListKey(e: KeyboardEvent) {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        moveTo(active + 1);
        break;
      case "ArrowUp":
        e.preventDefault();
        moveTo(active - 1);
        break;
      case "Home":
        e.preventDefault();
        moveTo(0);
        break;
      case "End":
        e.preventDefault();
        moveTo(options.length - 1);
        break;
      case "Enter":
      case " ":
        e.preventDefault();
        pick(active);
        break;
      case "Escape":
        // Kept here: a dialog around this field closes on Escape too.
        e.preventDefault();
        e.stopPropagation();
        close(true);
        break;
      case "Tab":
        close(false);
        break;
    }
  }

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={`tc-field tc-select ${size === "sm" ? "tc-field--sm" : ""} ${className}`}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? listId : undefined}
        aria-label={ariaLabel}
        disabled={disabled}
        data-open={open || undefined}
        onClick={() => (open ? close(false) : openList())}
        onKeyDown={onTriggerKey}
      >
        <span className={selected ? "tc-select-value" : "tc-select-value tc-placeholder"}>{selected?.label ?? placeholder}</span>
        <ChevronDown size={16} aria-hidden="true" className="tc-chevron" />
      </button>
      <Popover anchor={triggerRef} open={open} onClose={closeOutside}>
        <ul
          ref={listRef}
          id={listId}
          role="listbox"
          tabIndex={-1}
          aria-label={ariaLabel}
          aria-activedescendant={`${listId}-${active}`}
          className="tc-listbox"
          onKeyDown={onListKey}
        >
          {options.map((o, i) => (
            <li
              key={o.value}
              id={`${listId}-${i}`}
              role="option"
              aria-selected={o.value === value}
              data-active={i === active || undefined}
              className="tc-option"
              onMouseEnter={() => setActive(i)}
              onClick={() => pick(i)}
            >
              <span>{o.label}</span>
              {o.value === value && <Check size={15} aria-hidden="true" />}
            </li>
          ))}
        </ul>
      </Popover>
    </>
  );
}
