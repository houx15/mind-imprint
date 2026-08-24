import type { ReactNode } from "react";
import type { DemoMockKind } from "../types";
import { QuestionCardMock } from "./QuestionCardMock";
import { WriteModeChoiceMock } from "./WriteModeChoiceMock";

/** Registry of static real-UI mocks a `demoModal` tour step can show.
 *  Add a new `DemoMockKind` + case here to register another mock. */
export function demoMockFor(kind: DemoMockKind): ReactNode {
  switch (kind) {
    case "question-card":
      return <QuestionCardMock />;
    case "write-mode-choice":
      return <WriteModeChoiceMock />;
    default: {
      const exhaustive: never = kind;
      return exhaustive;
    }
  }
}
