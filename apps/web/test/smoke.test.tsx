import { render, screen } from "@testing-library/react";

function Hello() {
  return <span>hi</span>;
}

it("renders", () => {
  render(<Hello />);
  expect(screen.getByText("hi")).toBeInTheDocument();
});
