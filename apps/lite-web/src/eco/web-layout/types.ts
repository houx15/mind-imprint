import type { SiteLayout, SitePalette } from "../../site/types";
import type { ReferenceSpec } from "./referenceAnalysis";

export interface SiteReference {
  id: string;
  kind: "url" | "html" | "image";
  name: string;
  summary: string;
  preview?: string;
  spec?: ReferenceSpec;
}

export interface WebBrief {
  kind: string;
  style: string;
  topic: string;
  goal: string;
  audience: string[];
  requiredContent: string[];
  avoid: string[];
}

export interface DesignDirection {
  key: string;
  name: string;
  reason: string;
  headline: string;
  description: string;
  cta: string;
  layout: SiteLayout;
  templateId?: string;
  templateName?: string;
  templateTag?: string;
  palette?: SitePalette;
}

export interface Recommendation {
  brief: WebBrief;
  directions: DesignDirection[];
  archetype: SiteArchetype;
  sourceRequirement: string;
  provider?: string;
  model?: string;
}

export type SiteArchetype = "personal" | "project" | "organization" | "event" | "knowledge";

export type WireframeKind = "section" | "text" | "image" | "button" | "divider" | "pen";

export interface WireframePoint {
  x: number;
  y: number;
}

export interface WireframeNode {
  id: string;
  kind: WireframeKind;
  x: number;
  y: number;
  width: number;
  height: number;
  text?: string;
  points?: WireframePoint[];
}

export interface WireframeDocument {
  viewport: "desktop" | "phone";
  width: number;
  height: number;
  nodes: WireframeNode[];
}

export type ComponentKind = "heading" | "text" | "button" | "image" | "color" | "cards";

export interface PageComponent {
  id: string;
  type: ComponentKind;
  text: string | string[];
  span: number;
  height?: number;
  color?: string;
  src?: string;
}

export interface PageBlock {
  id: string;
  name: string;
  items: PageComponent[];
}

export interface PageDocument {
  layout: SiteLayout;
  templateId?: string;
  palette?: SitePalette;
  title: string;
  siteName?: string;
  navItems?: string[];
  background?: string;
  blocks: PageBlock[];
}

export interface AnnotationScope {
  type: "page" | "block" | "component";
  id: string;
  label: string;
}

export interface PatchChange {
  componentId: string;
  action:
    | "replace_text"
    | "resize"
    | "resize_height"
    | "set_color"
    | "set_page_background"
    | "move_component"
    | "add_component"
    | "delete_component";
  text?: string;
  span?: number;
  height?: number;
  color?: string;
  blockId?: string;
  index?: number;
  component?: PageComponent;
}

export interface PatchProposal {
  summary: string;
  before: string;
  after: string;
  changes: PatchChange[];
  provider?: string;
  model?: string;
}

export interface AnnotationRecord {
  id: string;
  scopeId: string;
  scopeLabel: string;
  instruction: string;
  summary: string;
  status: "accepted" | "rejected" | "question";
}

export interface ChatRow {
  id: string;
  role: "assistant" | "student";
  text: string;
}
