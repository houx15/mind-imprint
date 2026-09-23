import { useState } from "react";
import { createRoot } from "react-dom/client";
import { GenrePicker } from "../../src/writings/GenrePicker";
import { useLiteTheme } from "../../src/shared/useLiteTheme";
import { AccentProvider } from "@/ui";
import { LITE_ACCENT_PRESETS } from "@/ui/themes/lite";
import "../../src/index.css";
import "@/ui/themes/lite.css";
import "../../src/learning/student-surfaces.css";

/**
 * 「这一篇按什么文体在教」那一块的看图台。
 *
 * 产品负责人 2026-09-23：「for writing, maybe we need to let the students
 * select/talk with ai about what genre they are going to write.」
 *
 * 🚨 为什么在真浏览器里按：memory `control-that-is-not-wired-2026-09-22`。
 * 这一块要证三件事：
 *   1. 「印记按 X 在教」和「你定的是 X」**分得开**（把推断说成是她的选择，
 *      是替她做主之后再赖给她）；
 *   2. 每一种后面那句是「什么时候选它」，不是定义；
 *   3. 换完之后上面那一行真的变了 —— 按钮接线了。
 *
 * 🚨 主题照 LiteApp 的真做法装（AccentProvider + useLiteTheme）。
 *
 * 服务端在这里是假的：台上自己接一个 fetch，返回和 writing_genre_route.go
 * **同一个形状**的 JSON（choices 的措辞逐字抄自 writingGenreChoices）。
 */

const CHOICES = [
  { id: "argument", label: "议论文", blurb: "要说清一个看法，并且给出理由和材料。" },
  { id: "narrative", label: "记叙文", blurb: "写一件真实发生过的事，写出当时的场景和你的变化。" },
  { id: "letter", label: "书信", blurb: "写给一个具体的人，要让他知道什么、或者请他做什么。" },
  { id: "prose", label: "散文", blurb: "几件不连着的小事，靠一样东西串起来，写出一点体会。" },
];

// 台上的「服务端」：推断出来是议论文，她选过之后按她选的。
let stored = "";
const real = window.fetch.bind(window);
window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = String(typeof input === "string" ? input : input instanceof URL ? input.href : input.url);
  if (url.includes("/genre")) {
    if (init?.method === "PUT") {
      const body = JSON.parse(String(init.body ?? "{}")) as { genre?: string };
      stored = CHOICES.some((c) => c.id === body.genre) ? (body.genre ?? "") : "";
    }
    return new Response(
      JSON.stringify({ genre: stored || "argument", chosen: stored !== "", choices: CHOICES }),
      { status: 200, headers: { "content-type": "application/json" } },
    );
  }
  return real(input, init);
};

function Stage() {
  const { themeStyle } = useLiteTheme();
  const [genre, setGenre] = useState("");
  return (
    <div style={{ ...themeStyle, padding: 24, maxWidth: 420, background: "var(--mk-paper)" }}>
      <p data-room-genre style={{ fontSize: 12, color: "var(--mk-muted)" }}>{genre}</p>
      <GenrePicker writingId="w1" outlineVersion={0} onGenre={setGenre} />
    </div>
  );
}

createRoot(document.getElementById("root")!).render(
  <AccentProvider presets={LITE_ACCENT_PRESETS} initialAccent={LITE_ACCENT_PRESETS[0]!.id}>
    <Stage />
  </AccentProvider>,
);
