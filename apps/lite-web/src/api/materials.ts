import { listArtifacts, pendingArtifacts } from "./artifacts";
import { listSiteRefs } from "./siteRefs";
import { getCreativeDirection } from "./creativeDirection";
import { listCodeVersions } from "./codeVersions";
import { getSite } from "./site";
import { listDecisions } from "./decide";
import { listNotes } from "./notes";
import { listPblCourses } from "./pblCourses";
import { currentReframe, listReframes, reframeSentence } from "./reframe";
import { getTree } from "./tree";

/**
 * 这个项目攒下来的东西。
 *
 * 🚨 产品负责人 2026-09-03，照着 Claude Cowork 的分法：「by default, we have
 * small right sidebar, upper is the task todo list, and then materials list.
 * when we review something, the right side becomes the preview things …
 * and the task todo list disappears, close that then todo list appears again.」
 *
 * 我们原来只有上半截：右栏是计划，工具打开时盖在它上面，顶上留一排标签页。
 * 于是她做出来的东西——便签、问题陈述、方案、成果、结构、决定——**没有一个
 * 地方能看见全貌**。它们只在对话里那张卡片被点开的那一刻存在过，卡片一旦被
 * 后面的对话顶上去，那件东西就等于消失了。
 *
 * 材料清单就是那个地方：她攒了什么，一眼看得见，点一下就能回去看。
 */

export interface Material {
  /** 对应哪件工具。点一下开的就是它。 */
  tool: string;
  label: string;
  hue: string;
  /** 一句话说清楚现在有什么。空 = 还没有。 */
  detail: string;
  /** 有多少。0 = 还没开始，那一行淡着放。 */
  count: number;
  /** Optional tools can be opened before any material has been collected. */
  availableWhenEmpty?: boolean;
}

/**
 * 一次把六样都取回来。
 *
 * 用 allSettled 而不是 all：其中一个端点出错不该让整张清单空掉——她仍然该看见
 * 别的几样。取不到的那一样按"还没有"显示。
 */
export async function listMaterials(
  projectId: string,
  /** 项目类别。主页项目多一行「我的主页」——见下面那一段。 */
  projectKind = "",
): Promise<Material[]> {
  const [notes, reframes, artifacts, tree, decisions, courses] = await Promise.allSettled([
    listNotes(projectId),
    listReframes(projectId),
    listArtifacts(projectId),
    getTree(projectId),
    listDecisions(projectId),
    listPblCourses(projectId),
  ]);

  const ok = <T,>(r: PromiseSettledResult<T>, fallback: T): T =>
    r.status === "fulfilled" ? r.value : fallback;

  const allNotes = ok(notes, []);
  const ideas = allNotes.filter((n) => n.kind === "idea");
  const stickies = allNotes.filter((n) => n.kind !== "idea");
  const current = currentReframe(ok(reframes, []));
  const arts = ok(artifacts, []);
  const waiting = pendingArtifacts(arts).filter(a => !a.stale);
  const nodes = ok(tree, { tree: "main", nodes: [], checks: [] }).nodes;
  const decs = ok(decisions, []);
  const unsettled = decs.filter((d) => !d.settledAt);
  const crs = ok(courses, []);
  const unfinishedCourses = crs.filter((c) => !c.finishedAt);

  // 🚨 主页项目那一行：她随时点得开的「我的主页」。
  //
  // 发布不是终点（spec §4：上线之后项目进 keeping，她随时能回来改），所以上线
  // 那一屏不能只在印记递给她的那一刻存在。SiteStudio 被退役之后，这一行就是她
  // 自己回到那一页的入口。
  //
  // count 恒为 1：这一行永远点得开。别的几行是「攒了多少」，这一行是一个地方。
  const websiteRow: Material[] = [];
  if (projectKind === "website") {
    const [site, refs, creative, versions] = await Promise.all([getSite().catch(() => null), listSiteRefs(projectId).catch(() => null), getCreativeDirection(projectId).catch(() => null), listCodeVersions(projectId).catch(() => null)]);
    websiteRow.push({tool:"sites", label:"灵感采集", hue:"var(--mk-mist)", count:refs?.length ?? 0, availableWhenEmpty:true, detail:refs ? refs.length ? `已收藏 ${refs.length} 个参考 · 可选` : "可选 · 探索画面与互动效果" : "参考读取失败，点击重试"});
    websiteRow.push({
      tool: "ship",
      label: "我的主页",
      hue: "var(--mk-gold)",
      count: 1,
      detail: !site || !versions ? "主页状态读取失败，点击重试" : site.published ? "已上线 · 查看版本与更新" : versions.length ? `${versions.length} 个草稿版本 · 查看发布条件` : "查看主页与发布条件",
    });
    // The primary design entry follows the creative workflow, not the retired palette picker.
    const hasCreative = Boolean(creative && (creative.document.feeling.trim() || creative.document.motifs.length || creative.document.hero?.scene.trim()));
    websiteRow.push({
      tool: "creative",
      label: "主页创作构思",
      hue: "var(--mk-lake)",
      count: hasCreative ? 1 : 0,
      availableWhenEmpty: true,
      detail: !creative ? "构思读取失败，点击重试" : hasCreative ? "风格、意象与第一幕 · 继续编辑" : "从喜欢的感觉开始",
    });
  }

  return [
    ...websiteRow,
    {
      tool: "board",
      label: "便签板",
      hue: "var(--mk-mist)",
      count: stickies.length,
      detail: stickies.length ? `${stickies.length} 张便签` : "",
    },
    {
      tool: "reframe",
      label: "问题陈述",
      hue: "var(--mk-taro)",
      count: current ? 1 : 0,
      detail: current ? reframeSentence(current) : "",
    },
    {
      tool: "ideas",
      label: "解决方案",
      hue: "var(--mk-matcha)",
      count: ideas.length,
      detail: ideas.length ? `${ideas.length} 个办法` : "",
    },
    {
      tool: "review",
      label: "成果",
      hue: "var(--mk-peach)",
      count: arts.length,
      detail: arts.length
        ? waiting.length
          ? `${waiting.length} 份待审核`
          : `${arts.filter((a) => a.settledAt).length} 份已审 / ${arts.length} 份成果`
        : "",
    },
    {
      tool: "structure",
      label: "结构",
      hue: "var(--mk-lake)",
      count: nodes.length,
      detail: nodes.length ? `${nodes.length} 个节点` : "",
    },
    {
      tool: "course",
      label: "课程",
      hue: "var(--mk-butter)",
      count: crs.length,
      detail: crs.length
        ? unfinishedCourses.length
          ? `${unfinishedCourses.length} 门待上`
          : `${crs.length} 门已上完`
        : "",
    },
    {
      tool: "decide",
      label: "决定",
      hue: "var(--mk-berry)",
      count: decs.length,
      detail: decs.length
        ? unsettled.length
          ? `${unsettled.length} 个待定`
          : `${decs.length} 个已定`
        : "",
    },
  ];
}
