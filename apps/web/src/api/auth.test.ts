import { describe, it, expect, vi, beforeEach } from "vitest";
import { signup, signin, signout, getMe, verifyEmail } from "./auth";

const ME = {
  id: "u1", email: "p@d.local", display_name: "Phoebe", role: "student",
  avatar_color: "#7C9CF0", school: { id: "s1", name: "Demo" }, classes: [],
};

function mockFetch(status: number, body: unknown) {
  return vi.fn(async () => ({
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  })) as unknown as typeof fetch;
}

describe("api/auth", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("signin posts creds and returns the user", async () => {
    const f = mockFetch(200, { user: ME });
    vi.stubGlobal("fetch", f);
    const u = await signin({ email: "p@d.local", password: "pw" });
    expect(u.display_name).toBe("Phoebe");
    const calls = (f as unknown as { mock: { calls: unknown[][] } }).mock.calls;
    const [, init] = calls[0] as [string, RequestInit];
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ email: "p@d.local", password: "pw" });
  });

  it("signup posts and resolves void on 201", async () => {
    vi.stubGlobal("fetch", mockFetch(201, {}));
    await expect(signup({ email: "a@b.c", password: "pw123456", display_name: "A", join_code: "DEMO-0001" })).resolves.toBeUndefined();
  });

  it("getMe returns the user", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { user: ME }));
    expect((await getMe()).email).toBe("p@d.local");
  });

  it("verifyEmail returns the user", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { user: ME }));
    expect((await verifyEmail("tok")).id).toBe("u1");
  });

  it("signout resolves on 204", async () => {
    vi.stubGlobal("fetch", mockFetch(204, undefined));
    await expect(signout()).resolves.toBeUndefined();
  });

  it("signin surfaces ApiError on 401", async () => {
    vi.stubGlobal("fetch", mockFetch(401, { error: { code: "invalid_credentials", message: "邮箱或密码错误" } }));
    await expect(signin({ email: "x", password: "y" })).rejects.toMatchObject({ code: "invalid_credentials" });
  });
});
