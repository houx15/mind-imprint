import type { ReactNode } from "react";
import type { DemoMockKind } from "../types";
import { QuestionCardMock } from "./QuestionCardMock";

/** Registry of static real-UI mocks a `demoModal` tour step can show.
 *  Add a new `DemoMockKind` + case here to register another mock. */
export function demoMockFor(kind: DemoMockKind): ReactNode {
  switch (kind) {
    case "question-card":
      return <QuestionCardMock />;
    default: {
      const exhaustive: never = kind;
      return exhaustive;
    }
  }
}
