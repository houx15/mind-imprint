import { existsSync, readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * 🚨 spec §15 的那条硬规矩，用一条测试钉住：
 *
 * > `site/` may not import the app's UI kit (`Btn`, `Panel`, any `mk-*`
 * > token). The moment it does, the page starts looking like the product that
 * > made it.
 *
 * 这条规矩靠人记是记不住的——它每次被破坏都只是一行看上去很合理的 import。所以
 * 让它变成一条会红的测试。
 *
 * 一个例外：`mk-site` 是 `Ground` 给自己那层容器起的类名，不是设计 token。
 */
// 🚨 不能用 `import.meta.url`：Vite 把它换成了服务端的虚拟路径（/src/site），
// readdirSync 拿它去查真实文件系统必然 ENOENT。从 cwd 出发找，两种常见的
// 运行位置（仓库根 / apps/lite-web）都覆盖到。
const DIR = [
  join(process.cwd(), "src/site"),
  join(process.cwd(), "apps/lite-web/src/site"),
].find((p) => existsSync(p))!;

function sourceFiles(): string[] {
  return readdirSync(DIR)
    .filter((f) => (f.endsWith(".tsx") || f.endsWith(".ts")) && !f.endsWith(".test.ts"))
    .map((f) => join(DIR, f));
}

describe("她的网站不许长得像做出它的那个产品", () => {
  it("有文件可查", () => {
    expect(sourceFiles().length).toBeGreaterThan(4);
  });

  it("不从 app 的 UI kit 里 import 任何东西", () => {
    const offenders: string[] = [];
    for (const path of sourceFiles()) {
      const src = readFileSync(path, "utf8");
      for (const line of src.split("\n")) {
        if (!line.trimStart().startsWith("import")) continue;
        if (/from\s+["'](@\/ui|.*\/ui["']|@\/components)/.test(line)) {
          offenders.push(`${path.split("/").pop()}: ${line.trim()}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  it("不用任何 mk-* 设计 token", () => {
    const offenders: string[] = [];
    for (const path of sourceFiles()) {
      const src = readFileSync(path, "utf8");
      // mk-site 是容器自己的类名，不是 token。
      const hits = src.match(/\bmk-(?!site\b)[a-z0-9-]+/g) ?? [];
      if (hits.length) offenders.push(`${path.split("/").pop()}: ${[...new Set(hits)].join(", ")}`);
    }
    expect(offenders).toEqual([]);
  });

  it("不用 Tailwind 的透明度写法碰 --st-* 变量", () => {
    // 这些变量是裸 CSS 变量，`text-[var(--st-ink)]/60` 一行 CSS 都不会生成，
    // 于是颜色静默失效。只能走 color-mix（parts.tsx 的 mix / hair）。
    const offenders: string[] = [];
    for (const path of sourceFiles()) {
      // 只看代码行。parts.tsx 的注释里就写着这个反例（那句警告本身），
      // 连注释一起扫会把警告当成违规。
      const code = readFileSync(path, "utf8")
        .split("\n")
        .filter((l) => {
          const t = l.trimStart();
          return !t.startsWith("//") && !t.startsWith("*") && !t.startsWith("/*");
        })
        .join("\n");
      if (/var\(--st-[a-z]+\)\]\/\d/.test(code)) offenders.push(path.split("/").pop()!);
    }
    expect(offenders).toEqual([]);
  });
});
