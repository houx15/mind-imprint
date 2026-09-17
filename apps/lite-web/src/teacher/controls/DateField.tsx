import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from "react";
import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import { Popover } from "./Popover";
import {
  DEFAULT_DUE_TIME,
  HOURS,
  dayAllowed,
  dayKey,
  dueShortcuts,
  formatValue,
  minuteOptions,
  monthGrid,
  pad2,
  parseValue,
  shiftMonth,
  todayBeijing,
  valueLabel,
  withDay,
  type Day,
} from "./dateFieldLogic";
import "./controls.css";

// teacher/controls/DateField.tsx — the date / deadline field of the teacher
// forms, in place of the browser's date and datetime-local inputs. The value
// keeps the shape those inputs had (`YYYY-MM-DD`, or `YYYY-MM-DDTHH:mm` with
// `withTime`), so no caller's parsing changes. Dates are Beijing's.

const WEEK_HEAD = ["一", "二", "三", "四", "五", "六", "日"];

export function DateField({
  value,
  onChange,
  withTime = false,
  shortcuts = false,
  min,
  max,
  disabled = false,
  ariaLabel,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  withTime?: boolean;
  /** Deadline shortcuts (今天 / 明天 / 本周五 / 下周一). */
  shortcuts?: boolean;
  /** `YYYY-MM-DD`; days outside are shown but cannot be picked. */
  min?: string;
  max?: string;
  disabled?: boolean;
  ariaLabel?: string;
  placeholder?: string;
}) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const today = todayBeijing(Date.now());
  const parsed = parseValue(value);
  const [month, setMonth] = useState(() => ({ y: parsed?.y ?? today.y, m: parsed?.m ?? today.m }));

  const close = useCallback((refocus: boolean) => {
    setOpen(false);
    if (refocus) triggerRef.current?.focus();
  }, []);
  const closeOutside = useCallback(() => close(false), [close]);

  function toggle() {
    if (disabled) return;
    if (open) return close(false);
    const p = parseValue(value);
    setMonth({ y: p?.y ?? today.y, m: p?.m ?? today.m });
    setOpen(true);
  }

  useEffect(() => {
    if (!open) return;
    requestAnimationFrame(() => {
      panelRef.current?.querySelector<HTMLElement>('.tc-day[aria-pressed="true"], .tc-day[data-today]')?.focus();
      panelRef.current?.querySelectorAll('.tc-time-col [aria-pressed="true"]').forEach((el) => el.scrollIntoView({ block: "center" }));
    });
  }, [open]);

  function pickDay(day: Day) {
    if (!dayAllowed(day, min, max)) return;
    onChange(formatValue(withDay(value, day, DEFAULT_DUE_TIME), withTime));
    if (!withTime) close(true);
  }

  function pickTime(hh: number, mi: number) {
    const base = parseValue(value) ?? { ...today, hh, mi };
    onChange(formatValue({ ...base, hh, mi }, true));
  }

  function onPanelKey(e: KeyboardEvent) {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      close(true);
      return;
    }
    // Arrow keys move between the day buttons.
    const cell = (e.target as HTMLElement).closest<HTMLElement>("[data-day]");
    if (!cell) return;
    const step = { ArrowLeft: -1, ArrowRight: 1, ArrowUp: -7, ArrowDown: 7 }[e.key];
    if (step === undefined) return;
    e.preventDefault();
    const cells = Array.from(panelRef.current?.querySelectorAll<HTMLElement>("[data-day]") ?? []);
    const i = cells.indexOf(cell) + step;
    if (i >= 0 && i < cells.length) cells[i]!.focus();
    else setMonth((cur) => shiftMonth(cur.y, cur.m, step > 0 ? 1 : -1));
  }

  const label = valueLabel(value, withTime, today);
  const grid = monthGrid(month.y, month.m);
  const selectedKey = parsed ? dayKey(parsed) : "";
  const todayKey = dayKey(today);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className="tc-field tc-date"
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-label={ariaLabel ? `${ariaLabel}${label ? `：${label}` : ""}` : undefined}
        disabled={disabled}
        data-open={open || undefined}
        onClick={toggle}
      >
        <CalendarDays size={16} aria-hidden="true" className="tc-date-icon" />
        <span className={label ? "tc-select-value" : "tc-select-value tc-placeholder"}>
          {label || placeholder || (withTime ? "请选择日期和时间" : "请选择日期")}
        </span>
      </button>
      <Popover anchor={triggerRef} open={open} onClose={closeOutside} width={withTime ? 440 : 320}>
        <div ref={panelRef} role="dialog" aria-label={ariaLabel ?? "选择日期"} className="tc-date-panel" onKeyDown={onPanelKey}>
          {shortcuts && (
            <div className="tc-date-shortcuts">
              {dueShortcuts(Date.now()).map((s) => (
                <button
                  key={s.label}
                  type="button"
                  className="tc-chip"
                  aria-pressed={s.value === value}
                  onClick={() => {
                    onChange(s.value);
                    const p = parseValue(s.value);
                    if (p) setMonth({ y: p.y, m: p.m });
                  }}
                >
                  {s.label}
                </button>
              ))}
            </div>
          )}
          <div className="tc-date-body">
            <div className="tc-calendar">
              <div className="tc-calendar-head">
                <button type="button" className="tc-icon-btn" aria-label="上个月" onClick={() => setMonth((c) => shiftMonth(c.y, c.m, -1))}>
                  <ChevronLeft size={16} aria-hidden="true" />
                </button>
                <span>
                  {month.y}年{month.m}月
                </span>
                <button type="button" className="tc-icon-btn" aria-label="下个月" onClick={() => setMonth((c) => shiftMonth(c.y, c.m, 1))}>
                  <ChevronRight size={16} aria-hidden="true" />
                </button>
              </div>
              <div className="tc-calendar-grid">
                {WEEK_HEAD.map((w) => (
                  <span key={w} className="tc-weekhead" aria-hidden="true">
                    {w}
                  </span>
                ))}
                {grid.map((c) => {
                  const key = dayKey(c);
                  const allowed = dayAllowed(c, min, max);
                  return (
                    <button
                      key={key}
                      type="button"
                      data-day={key}
                      data-today={key === todayKey || undefined}
                      data-outside={!c.inMonth || undefined}
                      aria-pressed={key === selectedKey}
                      aria-label={`${c.m}月${c.d}日`}
                      disabled={!allowed}
                      tabIndex={key === selectedKey || (!selectedKey && key === todayKey) ? 0 : -1}
                      className="tc-day"
                      onClick={() => pickDay(c)}
                    >
                      {c.d}
                    </button>
                  );
                })}
              </div>
            </div>
            {withTime && (
              <div className="tc-time" aria-label="时间">
                <p className="tc-time-title">{parsed ? `${pad2(parsed.hh)}:${pad2(parsed.mi)}` : "时间"}</p>
                <div className="tc-time-cols">
                  <div className="tc-time-col mk-scroll" role="group" aria-label="时">
                    {HOURS.map((h) => (
                      <button
                        key={h}
                        type="button"
                        aria-pressed={parsed?.hh === h}
                        className="tc-time-cell"
                        onClick={() => pickTime(h, parsed?.mi ?? 0)}
                      >
                        {pad2(h)}
                      </button>
                    ))}
                  </div>
                  <div className="tc-time-col mk-scroll" role="group" aria-label="分">
                    {minuteOptions(parsed?.mi ?? null).map((mi) => (
                      <button
                        key={mi}
                        type="button"
                        aria-pressed={parsed?.mi === mi}
                        className="tc-time-cell"
                        onClick={() => pickTime(parsed?.hh ?? DEFAULT_DUE_TIME.hh, mi)}
                      >
                        {pad2(mi)}
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>
          {withTime && (
            <div className="tc-date-foot">
              <span>北京时间</span>
              <button type="button" className="tc-done" onClick={() => close(true)}>
                完成
              </button>
            </div>
          )}
        </div>
      </Popover>
    </>
  );
}
