import { render, screen, fireEvent, cleanup, act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EditionRedirectNotice } from "@/shell/edition/EditionRedirectNotice";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

const LITE_URL = "https://mind-lite.uni-robot.cn/?edition_redirect=1";

describe("EditionRedirectNotice", () => {
  it("names the edition the student actually belongs to before moving them", () => {
    const redirect = vi.fn();
    render(
      <EditionRedirectNotice
        decision={{ kind: "redirect", target: "lite", url: LITE_URL }}
        appEdition="pro"
        redirect={redirect}
      />,
    );
    // The whole point of showing a panel instead of jumping silently: the
    // student is told which version is theirs, so next time they can go
    // straight there.
    expect(screen.getByRole("dialog").textContent).toContain("轻量版");
    expect(redirect).not.toHaveBeenCalled();
  });

  it("goes immediately when the student presses the button", () => {
    const redirect = vi.fn();
    render(
      <EditionRedirectNotice
        decision={{ kind: "redirect", target: "lite", url: LITE_URL }}
        appEdition="pro"
        redirect={redirect}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /前往轻量版/ }));
    expect(redirect).toHaveBeenCalledWith(LITE_URL);
  });

  it("goes on its own if the student does nothing", () => {
    vi.useFakeTimers();
    const redirect = vi.fn();
    render(
      <EditionRedirectNotice
        decision={{ kind: "redirect", target: "lite", url: LITE_URL }}
        appEdition="pro"
        delayMs={4000}
        redirect={redirect}
      />,
    );
    expect(redirect).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(4000);
    });
    // Nobody may be reading. The panel explains, then moves — a student who
    // walked away must not come back to a screen that never went anywhere.
    expect(redirect).toHaveBeenCalledWith(LITE_URL);
  });

  // There is no way out of this panel but forward: behind it every request
  // this student makes answers 404. A close button or a dismissing backdrop
  // would only strand them in front of a broken app.
  it("offers no way to dismiss it and stay in the wrong app", () => {
    render(
      <EditionRedirectNotice
        decision={{ kind: "redirect", target: "lite", url: LITE_URL }}
        appEdition="pro"
        redirect={vi.fn()}
      />,
    );
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(1);
    expect(buttons[0]!.textContent).toContain("前往轻量版");
  });

  it("says so plainly, and offers no button, when there is nowhere to send them", () => {
    vi.useFakeTimers();
    const redirect = vi.fn();
    render(
      <EditionRedirectNotice
        decision={{ kind: "stranded", target: "lite" }}
        appEdition="pro"
        redirect={redirect}
      />,
    );
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByRole("dialog").textContent).toContain("老师或管理员");
    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    // No configured URL means no navigation at all — never a jump to a
    // half-built address.
    expect(redirect).not.toHaveBeenCalled();
  });
});
