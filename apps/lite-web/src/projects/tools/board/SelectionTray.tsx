import { ArrowDown, X } from "lucide-react";
import type { ReactNode } from "react";

/** Shows the actual materials an operation will affect, before it is applied. */
export function SelectionTray({ title, items, onRemove, children }: {
  title: string;
  items: { id: string; body: string }[];
  onRemove?: (id: string) => void;
  children?: ReactNode;
}) {
  if (!items.length) return null;
  return <section className="student-selection-tray" aria-label={title}>
    <header><strong>{title}</strong><span>{items.length} 条材料</span></header>
    <div className="student-selection-tray__items">
      {items.map((item, i) => <div key={item.id} className="student-selection-tray__item">
        <span className="student-selection-tray__number">{String(i + 1).padStart(2, "0")}</span>
        <p>{item.body}</p>
        {onRemove && <button type="button" onClick={() => onRemove(item.id)} aria-label={`取消选择第 ${i + 1} 条`}><X size={14} /></button>}
      </div>)}
    </div>
    {children && <><div className="student-selection-tray__connector" aria-hidden="true"><ArrowDown size={20} /></div><div className="student-selection-tray__actions">{children}</div></>}
  </section>;
}
