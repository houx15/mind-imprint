import type { ReactNode } from "react";
import { IllustrationImagesProvider } from "@/ui/Illustration";
import reading from "../home/assets/curious-learner-v2.webp";
import writing from "../home/assets/learning-together-v3.webp";
import project from "../home/assets/project-makers-v2.webp";
export const studentArtwork = { reading, writing, project };
const images = { bookLover: reading, reading, questions: reading, emptyProjects: project, writing, focus: writing, completed: project, sent: writing, loading: reading };
export function StudentArtwork({ children }: { children: ReactNode }) {
  return <IllustrationImagesProvider images={images}>{children}</IllustrationImagesProvider>;
}
