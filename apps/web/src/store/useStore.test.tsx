import { describe, it, expect } from "vitest";
import { render, screen, act } from "@testing-library/react";
import { createStore } from "./createStore";
import { makeMemoryStorage } from "./storage";
import { useStore } from "./useStore";

function Counter({ store }: { store: ReturnType<typeof createStore> }) {
  const snap = useStore(store);
  return <span data-testid="count">{snap.tasks.length}</span>;
}

describe("useStore", () => {
  it("re-renders when the store gains a task", () => {
    const store = createStore({ storage: makeMemoryStorage() });
    render(<Counter store={store} />);
    expect(screen.getByTestId("count").textContent).toBe("0");
    act(() => {
      store.createTask({ title: "气候", seed: null });
    });
    expect(screen.getByTestId("count").textContent).toBe("1");
  });
});
