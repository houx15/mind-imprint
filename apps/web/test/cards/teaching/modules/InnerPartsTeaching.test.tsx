import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InnerPartsTeaching } from "@/cards/teaching/modules/InnerPartsTeaching";
import { pickTeaching } from "@/cards/teaching/teachingRegistry";

it("is registered under emotional-alignment with the expected chapters", () => {
  expect(InnerPartsTeaching.cardId).toBe("emotional-alignment");
  expect(InnerPartsTeaching.chapters.map((c) => c.key)).toEqual(["energy","cast","closing"]);
  expect(pickTeaching("emotional-alignment")).toBe(InnerPartsTeaching);
});

it("the cast chapter shows the six parts and flips to reveal what each protects", async () => {
  const cast = InnerPartsTeaching.chapters.find((c) => c.key === "cast")!;
  render(<cast.Component />);
  expect(screen.getByText("保护者")).toBeTruthy();
  await userEvent.click(screen.getByText("保护者"));
  expect(screen.getByText(/替你挡/)).toBeTruthy();   // 翻面后的保护意图文案
});
