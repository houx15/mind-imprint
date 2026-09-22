import { useEffect, useRef, useState } from "react";
import { outlineKindLabel, type OutlineKind } from "./outlineKind";

/**
 * KindPicker —— 图上卡片那个小标题，点一下能改。
 *
 * # 为什么它是一个按钮（同事 2026-09-22 的意见 3）
 *
 *	「如图，黑心商家那个点感觉应该是和分论点并列的一个反面论证，而不是论据。
 *	  AI 也没有提醒学生进行修改，只能靠学生自己判断、修改。」
 *
 * 那个小标题原来只是一行字：印记判它是什么，她只能看着。而印记会判错。
 *
 * 🚨 在这之前她**没有任何办法**改它。图上能拖，但拖动只改深度，
 * `rekindForDepth` 把任何拖到深度 1 的东西一律变成 `point` ——
 * 也就是说「反方观点」这一种根本到不了：同事那条「黑心商家哪怕赚很多钱，
 * 也是失败」拖上去会变成分论点，那也是错的。闭表里十种，她碰得到的只有三种。
 *
 * 判错本身归提示词管（writing_plan.go 里那一节「这一块是什么，怎么判」）。
 * 这里管的是另一半：**判错了她自己改得动**，而且改完当场看得见。
 *
 * 不用原生 `<select>`（2026-09-17 的裁定：「default ones… don't look good」），
 * 就是一列按钮。
 */
export function KindPicker({
  current,
  label,
  choices,
  onPick,
  justDragged,
}: {
  current: OutlineKind;
  label: string;
  choices: OutlineKind[];
  onPick: (kind: OutlineKind) => void;
  /** 刚才那一下是拖不是点 —— 见 MindMap 里改文字那一处同样的判断。 */
  justDragged: () => boolean;
}) {
  const [open, setOpen] = useState(false);
  const boxRef = useRef<HTMLDivElement | null>(null);

  // 点别处就收起来。
  useEffect(() => {
    if (!open) return;
    function away(e: MouseEvent) {
      if (!boxRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", away);
    return () => document.removeEventListener("mousedown", away);
  }, [open]);

  return (
    <div ref={boxRef} className="relative">
      <button
        type="button"
        aria-haspopup="menu"
        aria-expanded={open}
        title="这一条是什么 —— 点一下可以改"
        onClick={() => {
          // 拖动几乎总是从卡片上起手；不挡这一下，她每挪一次都会弹出菜单。
          if (justDragged()) return;
          setOpen((v) => !v);
        }}
        className="w-fit text-mk-label underline decoration-dotted underline-offset-2"
        style={{ color: "var(--mk-accent-700)" }}
      >
        {label}
      </button>
      {open && (
        <div
          role="menu"
          className="absolute left-0 top-full z-20 mt-1 flex w-max flex-col rounded-mk-md border py-1"
          style={{
            borderColor: "var(--mk-border)",
            background: "var(--mk-surface)",
            boxShadow: "0 8px 24px color-mix(in srgb, var(--mk-ink) 12%, transparent)",
          }}
        >
          <span className="px-3 py-1 text-mk-label" style={{ color: "var(--mk-faint)" }}>
            这一条是什么
          </span>
          {choices.map((k) => (
            <button
              key={k}
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false);
                if (k !== current) onPick(k);
              }}
              className="px-3 py-1 text-left text-mk-small transition-colors duration-[120ms] ease-mk hover:bg-mk-accent-50"
              style={{ color: k === current ? "var(--mk-accent-700)" : "var(--mk-ink)" }}
            >
              {outlineKindLabel(k)}
              {k === current ? " ·" : ""}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
