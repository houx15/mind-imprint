import type { ComponentType } from "react";

export type TeachingChapter = {
  key: string;
  label: string;
  badge?: string;
  Component: ComponentType;
};

export type TeachingModule = {
  cardId: string;
  title: string;
  category: string;
  chapters: TeachingChapter[];
};
