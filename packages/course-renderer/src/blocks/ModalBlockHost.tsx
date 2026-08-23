import { useEffect, useRef, useState, type ReactNode } from "react";
import type { BlockDefinition, BlockSessionState } from "@mind-imprint/course-contract";
import { InteractionModal } from "./media/InteractionModal";

export interface ModalBlockHostProps {
  block: BlockDefinition;
  state: BlockSessionState;
  /** The block's own renderer element, rendered inside the dialog. */
  children: ReactNode;
}

/** The authored prompt, used as the dialog's accessible name and the launcher label. */
function promptOf(block: BlockDefinition): string {
  return "prompt" in block && typeof block.prompt === "string" ? block.prompt : "这道题";
}

/**
 * §9.8 — hosts an assessment block declared `presentation: "modal"`.
 *
 * The slide's FIGURE keeps the whole slot; the question arrives on top of it in
 * the same dialog the video cue already uses. The slot still carries a compact
 * launcher, so the question is never a dead end and never invisible: the student
 * can dismiss the dialog to study the figure and reopen it from the launcher as
 * often as they like.
 *
 * The dialog opens by itself the first time the block becomes enabled, and
 * closes by itself once the block completes so the figure is unobstructed
 * again. Reopening after completion REMOUNTS the renderer, which is why
 * SlicePlayer passes `enabled: false` to a completed modal block — a remounted
 * assessment would otherwise have lost its local `locked` flag and could emit a
 * second `block.completed`, driving the Workflow somewhere the author never
 * authored. The trade is that the reopened dialog shows the question rather
 * than the feedback text; the launcher carries the 已完成 state instead.
 */
export function ModalBlockHost({ block, state, children }: ModalBlockHostProps) {
  const [open, setOpen] = useState(false);
  const autoOpened = useRef(false);
  const prompt = promptOf(block);
  const { visible, enabled, completed } = state;

  // Open once, when the Workflow first makes the question live.
  useEffect(() => {
    if (visible && enabled && !completed && !autoOpened.current) {
      autoOpened.current = true;
      setOpen(true);
    }
  }, [visible, enabled, completed]);

  // Completing the question hands the slide back to the figure.
  useEffect(() => {
    if (completed) setOpen(false);
  }, [completed]);

  // A hidden block has no launcher and no dialog (§10 — the slot still exists).
  if (!visible) {
    return <div data-block-id={block.id} data-modal-host hidden aria-hidden="true" className="course-block" />;
  }

  return (
    <div data-block-id={block.id} data-modal-host className="course-block course-block--modal-host">
      <button
        type="button"
        className="course-modal-host__launcher"
        data-modal-launcher
        data-completed={completed ? "true" : "false"}
        disabled={!enabled && !completed}
        onClick={() => setOpen(true)}
      >
        <span className="course-modal-host__prompt">{prompt}</span>
        <span className="course-modal-host__cta">{completed ? "已完成 · 再看一次" : "回答这道题"}</span>
      </button>
      {open ? (
        <InteractionModal ariaLabel={prompt} onSkip={() => setOpen(false)} dismissLabel="关闭">
          {children}
        </InteractionModal>
      ) : null}
    </div>
  );
}
