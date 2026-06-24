import { render } from "@testing-library/react";
import { SiftCraapTeaching } from "./SiftCraapTeaching";
import { pickTeaching } from "../teachingRegistry";

it("is a 5-chapter module registered under sift_craap", () => {
  expect(SiftCraapTeaching.cardId).toBe("sift_craap");
  expect(SiftCraapTeaching.chapters.map((c) => c.key)).toEqual(
    ["stop", "investigate", "find", "trace", "craap"]
  );
  expect(pickTeaching("sift_craap")).toBe(SiftCraapTeaching);
});

it("each chapter component renders without crashing", () => {
  for (const ch of SiftCraapTeaching.chapters) {
    const { unmount } = render(<ch.Component />);
    unmount();
  }
});
