/**
 * tree/types — 兴趣树这一层自己的类型。
 *
 * 这些类型原来住在 `eco/data/types.ts`，和原型的十几个 mock 数据结构挤在一个
 * 文件里。兴趣树现在是 lite 的真页面（`/tree`），它的类型必须住在真目录里：
 * `eco/` 的文件头上写着「原型做完就连同这个目录一起删掉」，而一个会被删掉的
 * 目录不能是生产代码的类型来源。
 *
 * 依赖方向只有一个方向：**原型可以引用这里，这里绝不引用原型。**
 */

/**
 * 树的七根主枝。
 *
 * `formal`（数学与形式）是第七根，2026-09-02 随真学科表加上的：原来的六根
 * **没有数学的位置**，而对一个服务 IB / A-Level / AP 学生的产品来说这是个洞，
 * 不是取舍——统计推断、微积分、逻辑、离散数学无处可落。它画成**树冠**：从树干
 * 的最顶端离开（y≈306，树干自身的路径止于 y=292），所以它读起来是这棵树的主梢，
 * 而不是硬塞进一个为六根枝设计的布局里的第七根。
 *
 * 必须与 `packages/contracts/src/discipline.ts` 的 `FIELD_IDS`、以及迁移 0116
 * 里 `interest_keyword.field` 的 CHECK 约束保持一致。
 */
export type FieldId =
  | "formal"
  | "humanities"
  | "science"
  | "society"
  | "making"
  | "arts"
  | "self";

export interface Field {
  id: FieldId;
  label: string;
  en: string;
  /** 一个 CSS 变量名——七根主枝的颜色定义在 `tree.css` 的 `.mk-grove` 里。 */
  hue: string;
  /** 枝从树干离开的角度，用树 SVG 自己的坐标系。 */
  angle: number;
}

export type SourceKind = "reading" | "writing" | "project" | "news" | "course";

export interface KeywordSource {
  kind: SourceKind;
  id: string;
  label: string;
  /** 她自己的话，或者让这个词出现的那一句。服务端保证非空。 */
  evidence?: string;
  date: string;
}

export interface Keyword {
  id: string;
  /**
   * interests.json 的 id。探索地图算「你可能还会感兴趣的」时靠它排除她已经有的
   * 词 —— 靠 id，不靠中文名。0134 之前种下、还没被重新采到的行是空串。
   */
  interestId: string;
  text: string;
  en: string;
  field: FieldId;
  /** 1..5。节点大小 + 强度读数。由来源条数推出来，不是等级。 */
  strength: number;
  /** 第一次出现在哪个成长刻度（GROWTH_STOPS 的下标）。 */
  bornAt: number;
  /** 印记对这个词之于她是什么的一句话。 */
  note: string;
  sources: KeywordSource[];
  /**
   * 这个领域扎在哪几门学科上（disciplines.json 的 id）。
   *
   * 由领域词表里写好的 `disciplines[]` 决定，服务端查表得到 —— 不是模型判定的，
   * 所以同一个词在每个学生身上连的是同一批学科。树上点一片叶子，亮起来的就是
   * 这几条根。
   */
  disciplineIds: string[];
  /** 高光时刻——她在这里做得特别好。树上的金星。 */
  shining?: { title: string; body: string; date: string };
  /**
   * 沿枝的位置，0（贴树干）..1（梢尖），加一个横向偏移让叶子不叠在一起。
   *
   * 🚨 **算出来的，不是手摆的**——见 `liveTree.ts`：同枝按第一次出现的时间
   * 排序均匀铺开，哈希只抖动横向幅度。原型里这两个数是手摆的，那是让 mock
   * 看起来像设计过的办法，真关键词没有这个待遇。
   */
  at: { t: number; spread: number };
}
