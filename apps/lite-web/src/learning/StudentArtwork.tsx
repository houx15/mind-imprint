import type { ReactNode } from "react";
import { IllustrationImagesProvider } from "@/ui/Illustration";
import reading from "../home/assets/curious-learner-v2.webp";
import writing from "../home/assets/learning-together-v3.webp";
import project from "../home/assets/project-makers-v2.webp";
import quest from "./assets/reading-quest.webp";
import discovery from "./assets/discovery-observatory.webp";
import ideas from "./assets/ideas-workbench.webp";
import keepsake from "./assets/reading-keepsake.webp";
export const studentArtwork = { reading, writing, project, quest, discovery, ideas, keepsake };
const images = { bookLover: reading, reading, questions: discovery, emptyProjects: ideas, writing, focus: quest, completed: keepsake, sent: writing, loading: reading };
export function StudentArtwork({ children }: { children: ReactNode }) {
  return <IllustrationImagesProvider images={images}>{children}</IllustrationImagesProvider>;
}
