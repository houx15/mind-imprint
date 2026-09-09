import {
  PatchSpecV1,
  VisualDesignSpecV1,
  type PatchSpecV1 as PatchSpec,
  type VisualDesignSpecV1 as VisualDesignSpec,
} from "@mind-imprint/contracts";

export function applyPatch(design: VisualDesignSpec, input: PatchSpec): VisualDesignSpec {
  const patch = PatchSpecV1.parse(input);
  if (patch.designId !== design.designId || patch.baseRevision !== design.revision) throw new Error("补丁对应的设计版本已经失效");
  let next: VisualDesignSpec = structuredClone(design);
  for (const operation of patch.operations) {
    if (operation.action === "set-theme") {
      next.theme = { ...next.theme, ...operation.theme, palette: operation.theme.palette ?? next.theme.palette };
      const background = operation.theme.palette?.background[0];
      if (background) next.pages.forEach((page) => { page.background = background; });
      continue;
    }
    if (operation.action === "upsert-interaction") {
      next.interactions = [...next.interactions.filter((item) => item.id !== operation.interaction.id), operation.interaction];
      continue;
    }
    if (operation.action === "remove-interaction") {
      next.interactions = next.interactions.filter((item) => item.id !== operation.interactionId);
      next.pages.forEach((page) => page.nodes.forEach((node) => { node.interactionIds = node.interactionIds.filter((id) => id !== operation.interactionId); }));
      continue;
    }
    if (operation.action === "add-node") {
      const page = next.pages.find((item) => item.id === operation.pageId);
      if (!page) throw new Error(`找不到页面 ${operation.pageId}`);
      page.nodes.splice(Math.min(operation.index, page.nodes.length), 0, { ...operation.node, parentId: operation.parentId });
      continue;
    }
    if (operation.action === "reorder-children") {
      const page = next.pages.find((item) => item.id === operation.pageId);
      if (!page) throw new Error(`找不到页面 ${operation.pageId}`);
      const rank = new Map(operation.childIds.map((id, index) => [id, index]));
      page.nodes.sort((a, b) => (rank.get(a.id) ?? a.order + operation.childIds.length) - (rank.get(b.id) ?? b.order + operation.childIds.length));
      page.nodes.forEach((node, index) => { node.order = index; });
      continue;
    }
    const page = next.pages.find((item) => item.nodes.some((node) => node.id === ("nodeId" in operation ? operation.nodeId : "")));
    if (!page) throw new Error("补丁指向了不存在的节点");
    const nodeIndex = page.nodes.findIndex((node) => node.id === ("nodeId" in operation ? operation.nodeId : ""));
    const node = page.nodes[nodeIndex]!;
    if (operation.action === "remove-node") {
      const removed = new Set<string>([node.id]);
      let changed = true;
      while (changed) { changed = false; page.nodes.forEach((item) => { if (item.parentId && removed.has(item.parentId) && !removed.has(item.id)) { removed.add(item.id); changed = true; } }); }
      page.nodes = page.nodes.filter((item) => !removed.has(item.id));
      next.interactions = next.interactions.filter((item) => !removed.has(item.sourceNodeId) && (!item.targetNodeId || !removed.has(item.targetNodeId)));
    } else if (operation.action === "move-node") {
      const target = next.pages.find((item) => item.id === operation.targetPageId);
      if (!target) throw new Error(`找不到目标页面 ${operation.targetPageId}`);
      page.nodes.splice(nodeIndex, 1);
      node.parentId = operation.targetParentId;
      if (operation.position) node.bounds = { ...node.bounds, ...operation.position };
      target.nodes.splice(Math.min(operation.index, target.nodes.length), 0, node);
    } else if (operation.action === "resize-node") node.bounds = { ...node.bounds, ...operation.bounds };
    else if (operation.action === "replace-content") node.content = operation.content;
    else if (operation.action === "set-style") node.style = { ...node.style, ...operation.style };
    else if (operation.action === "set-layout") node.layout = operation.layout;
  }
  next.revision += 1;
  return VisualDesignSpecV1.parse(next);
}

export function manualPatch(
  design: VisualDesignSpec,
  nodeId: string,
  instruction: string,
  operation: PatchSpec["operations"][number],
): PatchSpec {
  return PatchSpecV1.parse({ schema: "visual-pbl-local-patch", version: "1.0", patchId: `patch.manual.${crypto.randomUUID()}`, designId: design.designId, baseRevision: design.revision, scope: { type: "node", id: nodeId }, instruction, summary: instruction, operations: [operation], referenceIds: [], author: "student" });
}

export function rebasePatch(patch: PatchSpec, design: VisualDesignSpec): PatchSpec {
  return PatchSpecV1.parse({ ...patch, patchId: `patch.ai.${crypto.randomUUID()}`, designId: design.designId, baseRevision: design.revision });
}
