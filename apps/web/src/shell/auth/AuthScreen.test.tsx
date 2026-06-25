import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { AuthScreen } from "./AuthScreen";
import { ApiError } from "../../api";

const ME = {
  id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
  avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [],
};

describe("AuthScreen", () => {
  it("signs in and calls onAuthed with the user", async () => {
    const onAuthed = vi.fn();
    const client = { signin: vi.fn(async () => ME), signup: vi.fn() };
    const { container } = render(<AuthScreen onAuthed={onAuthed} client={client} />);
    const inputs = container.querySelectorAll("input");
    fireEvent.change(inputs[0]!, { target: { value: "p@d.local" } });
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    await waitFor(() => expect(onAuthed).toHaveBeenCalledWith(ME));
  });

  it("shows an inline error on bad credentials", async () => {
    const onAuthed = vi.fn();
    const client = {
      signin: vi.fn(async () => { throw new ApiError("invalid_credentials", "邮箱或密码错误", 401); }),
      signup: vi.fn(),
    };
    render(<AuthScreen onAuthed={onAuthed} client={client} />);
    fireEvent.click(screen.getByRole("button", { name: "登录" }));
    await waitFor(() => expect(screen.getByText("邮箱或密码错误")).toBeInTheDocument());
    expect(onAuthed).not.toHaveBeenCalled();
  });

  it("register → bind → signup+signin → onAuthed", async () => {
    const onAuthed = vi.fn();
    const client = { signin: vi.fn(async () => ME), signup: vi.fn(async () => undefined) };
    render(<AuthScreen onAuthed={onAuthed} client={client} />);
    fireEvent.click(screen.getByText("注册"));
    fireEvent.click(screen.getByRole("button", { name: /下一步/ }));
    fireEvent.click(screen.getByRole("button", { name: /完成/ }));
    await waitFor(() => expect(client.signup).toHaveBeenCalledOnce());
    await waitFor(() => expect(onAuthed).toHaveBeenCalledWith(ME));
  });
});
