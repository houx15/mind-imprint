import { describe, it, expect, vi } from "vitest";
import {
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
} from "@/api/admin";

const ok = (body: unknown, status = 200) =>
  vi.fn(async () => new Response(status === 204 ? null : JSON.stringify(body), { status })) as any;
const callOf = (spy: any, i = 0) => spy.mock.calls[i] as unknown as [string, RequestInit];

describe("admin api", () => {
  it("getOverview returns counts + usage", async () => {
    vi.stubGlobal("fetch", ok({ counts: { student: 3, teacher: 1, class: 2, project: 5, evaluation: 1, active_student: 2 }, usage_by_tier: [{ tier: "chaperone", prompt_tokens: 10, completion_tokens: 20, cost: "0.01" }] }));
    const o = await getOverview();
    expect(o.counts.student).toBe(3);
    expect(o.usage_by_tier[0]!.cost).toBe("0.01");
  });
  it("listTeacherInvites unwraps {invites}", async () => {
    vi.stubGlobal("fetch", ok({ invites: [{ id: "i1", code: "T-AB", expires_at: "z", created_at: "z" }] }));
    expect((await listTeacherInvites())[0]!.code).toBe("T-AB");
  });
  it("createTeacherInvite POSTs body and returns {code,expires_at}", async () => {
    const spy = ok({ code: "T-NEW", expires_at: "z" });
    vi.stubGlobal("fetch", spy);
    const r = await createTeacherInvite({ email: "t@x", expires_days: 7 });
    expect(r.code).toBe("T-NEW");
    expect(callOf(spy)[1].method).toBe("POST");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ email: "t@x", expires_days: 7 }));
  });
  it("adminImport POSTs {rows} and returns code sheet", async () => {
    const spy = ok({ classes: [{ name: "A", join_code: "AB-CD" }], teacher_invites: [{ email: "t@x", code: "T-1" }] });
    vi.stubGlobal("fetch", spy);
    const r = await adminImport([{ class: "A", teacher_email: "t@x" }]);
    expect(r.classes[0]!.join_code).toBe("AB-CD");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ rows: [{ class: "A", teacher_email: "t@x" }] }));
  });
  it("listTeachers unwraps {teachers}", async () => {
    vi.stubGlobal("fetch", ok({ teachers: [{ id: "u1", display_name: "Mr A", email: "a@x" }] }));
    expect((await listTeachers())[0]!.display_name).toBe("Mr A");
  });
  it("assignTeacher POSTs teacher_user_id and returns {teachers}", async () => {
    const spy = ok({ teachers: [{ id: "u1", display_name: "Mr A", email: "a@x" }] });
    vi.stubGlobal("fetch", spy);
    const r = await assignTeacher("c1", "u1");
    expect(r.teachers).toHaveLength(1);
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1/teachers");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ teacher_user_id: "u1" }));
  });
  it("removeTeacher DELETEs the teacher path", async () => {
    const spy = ok(null, 204);
    vi.stubGlobal("fetch", spy);
    await removeTeacher("c1", "u1");
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1/teachers/u1");
    expect(callOf(spy)[1].method).toBe("DELETE");
  });
});
