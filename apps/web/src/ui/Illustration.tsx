import { createContext, useContext, type ReactNode } from "react";
import { Button } from "@/ui/Button";

import bookLoverUrl from "../assets/illustrations/undraw_book-lover_m9n3.svg";
import specsUrl from "../assets/illustrations/undraw_specs_2nnl.svg";
import readingNotesUrl from "../assets/illustrations/undraw_reading-notes_dg9z.svg";
import mcpServerUrl from "../assets/illustrations/undraw_mcp-server_7kvc.svg";
import notebookUrl from "../assets/illustrations/undraw_notebook_8ihb.svg";
import deepWorkUrl from "../assets/illustrations/undraw_deep-work_muov.svg";
import projectCompletedUrl from "../assets/illustrations/undraw_project-completed_ug9i.svg";
import loadingUrl from "../assets/illustrations/undraw_loading_3kqt.svg";
import messageSentUrl from "../assets/illustrations/undraw_message-sent_iyz6.svg";
import questionsUrl from "../assets/illustrations/undraw_questions_52ic.svg";
import standing20Url from "../assets/illustrations/standing-20.svg";
import sitting2Url from "../assets/illustrations/sitting-2.svg";
import sitting4Url from "../assets/illustrations/sitting-4.svg";
import sitting5Url from "../assets/illustrations/sitting-5.svg";
import sitting6Url from "../assets/illustrations/sitting-6.svg";
import sitting8Url from "../assets/illustrations/sitting-8.svg";

/**
 * Illustration + EmptyState (design-system foundation, Part 3 Task 12).
 *
 * 16 bundled SVGs (10 undraw + 6 Humaaans) live in
 * `apps/web/src/assets/illustrations/`; this module imports every one as a
 * Vite asset URL (spec §16) and exposes them under friendly `name` keys.
 *
 * `tone` is purely descriptive today: it is stamped as `data-tone` so a
 * future recolor build-step can pick it up. The SVGs are consumed as plain
 * `<img>` sources here, which cannot be recolored via CSS — do NOT attempt
 * runtime SVG recoloring in this component.
 */

export type IllustrationName =
  | "bookLover"
  | "emptyProjects"
  | "reading"
  | "warren"
  | "writing"
  | "focus"
  | "completed"
  | "loading"
  | "sent"
  | "questions"
  | "standing20"
  | "sitting2"
  | "sitting4"
  | "sitting5"
  | "sitting6"
  | "sitting8";

const ILLUSTRATIONS: Record<IllustrationName, string> = {
  bookLover: bookLoverUrl,
  emptyProjects: specsUrl,
  reading: readingNotesUrl,
  warren: mcpServerUrl,
  writing: notebookUrl,
  focus: deepWorkUrl,
  completed: projectCompletedUrl,
  loading: loadingUrl,
  sent: messageSentUrl,
  questions: questionsUrl,
  standing20: standing20Url,
  sitting2: sitting2Url,
  sitting4: sitting4Url,
  sitting5: sitting5Url,
  sitting6: sitting6Url,
  sitting8: sitting8Url,
};

export interface IllustrationProps {
  name: IllustrationName;
  /** Macaron/accent tone name (e.g. "peach", "matcha", "vermilion"). Descriptive only — see module doc. */
  tone?: string;
  className?: string;
  alt?: string;
}

const IllustrationImages = createContext<Partial<Record<IllustrationName, string>>>({});
export function IllustrationImagesProvider({ images, children }: { images: Partial<Record<IllustrationName, string>>; children: ReactNode }) {
  return <IllustrationImages.Provider value={images}>{children}</IllustrationImages.Provider>;
}

export function Illustration({ name, tone, className, alt }: IllustrationProps) {
  const images = useContext(IllustrationImages);
  return (
    <img
      src={images[name] ?? ILLUSTRATIONS[name]}
      alt={alt ?? ""}
      data-tone={tone}
      className={["max-w-full", className].filter(Boolean).join(" ")}
    />
  );
}

export interface EmptyStateAction {
  label: string;
  onClick: () => void;
}

export interface EmptyStateProps {
  illustration: IllustrationName;
  title: string;
  body: string;
  action?: EmptyStateAction;
  className?: string;
}

/** Illustration + title + one line + optional primary action (spec §16). */
export function EmptyState({ illustration, title, body, action, className }: EmptyStateProps) {
  return (
    <div className={["flex flex-col items-center gap-3 py-10 text-center", className].filter(Boolean).join(" ")}>
      <Illustration name={illustration} className="h-[180px] w-[180px]" />
      <h2 className="text-mk-h2 font-semibold text-mk-ink">{title}</h2>
      <p className="text-mk-body text-mk-muted">{body}</p>
      {action && (
        <Button variant="primary" onClick={action.onClick} className="mt-2">
          {action.label}
        </Button>
      )}
    </div>
  );
}
