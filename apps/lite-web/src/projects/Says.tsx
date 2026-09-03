// Says.tsx —— 把印记说的一段话按它本来的形状排出来。
//
// 🚨 铁律③ 要求「有多个要点时分点列出」，prompt 里也这么写了，模型也照做了——
// 然后气泡用 `{m.content}` 直接把字符串塞进 div，换行被 CSS 折掉，三条要点
// 挤成一行拿「·」隔开。规矩写在 prompt 里、被界面吃掉，等于没写。
//
// 这里只做一件事：把纯文本还原成段落和列表。不引 markdown 渲染器——印记的
// 话里不该出现 **粗体** 和 # 标题，真出现了那是 prompt 的问题，不该由渲染器
// 兜着。
//
// 要点用 flex 的悬挂缩进，不用 white-space: pre-wrap：pre-wrap 下第二行会
// 缩回最左边，和下一条要点的记号对齐，反而比不分点更难读。

/**
 * 一条要点：`· 内容` / `- 内容` / `1. 内容` / `2、内容`。记号和内容分开拿。
 *
 * 🚨 `.` 后面必须跟空格，`、` 后面可以不跟。中文列表常写「2、四」不带空格，
 * 但「3.5 倍」如果也放行，一句话就会被当成一条要点拆掉。
 */
const BULLET = /^\s*(?:([·•\-*])\s+|(\d+[.)])\s+|(\d+、)\s*)(.*)$/;

type Block =
  | { kind: "p"; text: string }
  | { kind: "list"; items: { marker: string; text: string }[] };

/** 把一段纯文本切成段落和列表。导出只为可测。 */
export function toBlocks(content: string): Block[] {
  const out: Block[] = [];
  for (const raw of content.split("\n")) {
    const line = raw.trim();
    if (!line) continue;
    const m = BULLET.exec(line);
    if (m) {
      // 连着的要点归到同一个列表里；中间隔了段落就另起一个。
      const last = out[out.length - 1];
      const item = { marker: m[1] ?? m[2] ?? m[3] ?? "·", text: m[4] ?? "" };
      if (last?.kind === "list") last.items.push(item);
      else out.push({ kind: "list", items: [item] });
    } else {
      out.push({ kind: "p", text: line });
    }
  }
  return out;
}

/** 印记（或她自己）说的一段话。 */
export function Says({ content }: { content: string }) {
  const blocks = toBlocks(content);
  // 一整段没有换行时不额外包一层，省得给单行气泡多出上下间距。
  const only = blocks.length === 1 ? blocks[0] : undefined;
  if (only?.kind === "p") return <>{only.text}</>;
  return (
    <div className="flex flex-col gap-2">
      {blocks.map((b, i) =>
        b.kind === "p" ? (
          <p key={i}>{b.text}</p>
        ) : (
          <ul key={i} className="flex flex-col gap-1.5">
            {b.items.map((it, j) => (
              <li key={j} className="flex gap-2">
                {/* 数字要点保留她看到的编号；符号一律显示成「·」。 */}
                <span aria-hidden className="shrink-0 text-mk-secondary">
                  {/^\d/.test(it.marker) ? it.marker : "·"}
                </span>
                <span className="min-w-0">{it.text}</span>
              </li>
            ))}
          </ul>
        ),
      )}
    </div>
  );
}
