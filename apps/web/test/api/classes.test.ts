import { describe, it, expect, vi } from "vitest";
import {
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  setClassGrade, CLASS_GRADE_OPTIONS,
} from "@/api/classes";

const ok = (body: unknown, status = 200) =>
  vi.fn(async () => new Response(status === 204 ? null : JSON.stringify(body), { status })) as any;
const callOf = (spy: any, i = 0) =>
  spy.mock.calls[i] as unknown as [string, RequestInit];

describe("classes api", () => {
  it("listClasses unwraps {classes}", async () => {
    vi.stubGlobal("fetch", ok({ classes: [{ id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "2026-06-26T00:00:00Z" }] }));
    const out = await listClasses();
    expect(out).toHaveLength(1);
    expect(out[0]!.id).toBe("c1");
  });

  it("createClass POSTs {name} and unwraps {class}", async () => {
    const spy = ok({ class: { id: "c2", name: "New", join_code: "EF-GH", school_id: "s1", created_at: "z" } });
    vi.stubGlobal("fetch", spy);
    const c = await createClass({ name: "New" });
    expect(c.join_code).toBe("EF-GH");
    expect(callOf(spy)[0]).toContain("/api/v1/classes");
    expect(callOf(spy)[1].method).toBe("POST");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ name: "New" }));
  });

  it("getClass returns {class, roster}", async () => {
    vi.stubGlobal("fetch", ok({
      class: { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "z" },
      roster: [{ id: "u1", display_name: "Phoebe", email: "p@d", last_active_at: null, project_count: 0, evaluation_count: 0, card_count: 0 }],
    }));
    const d = await getClass("c1");
    expect(d.roster[0]!.last_active_at).toBeNull();
  });

  it("renameClass PATCHes {name}", async () => {
    const spy = ok({ class: { id: "c1", name: "Renamed", join_code: "AB-CD", school_id: "s1", created_at: "z" } });
    vi.stubGlobal("fetch", spy);
    const c = await renameClass("c1", "Renamed");
    expect(c.name).toBe("Renamed");
    expect(callOf(spy)[1].method).toBe("PATCH");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ name: "Renamed" }));
  });

  it("regenerateJoinCode PATCHes {regenerate_join_code:true}", async () => {
    const spy = ok({ class: { id: "c1", name: "11A", join_code: "ZZ-ZZ", school_id: "s1", created_at: "z" } });
    vi.stubGlobal("fetch", spy);
    const c = await regenerateJoinCode("c1");
    expect(c.join_code).toBe("ZZ-ZZ");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ regenerate_join_code: true }));
  });

  it("removeEnrollment DELETEs the enrollment path", async () => {
    const spy = ok(null, 204);
    vi.stubGlobal("fetch", spy);
    await removeEnrollment("c1", "u1");
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1/enrollments/u1");
    expect(callOf(spy)[1].method).toBe("DELETE");
  });

  it("setClassGrade PATCHes {grade}", async () => {
    const spy = ok({ class: { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "z", grade: "junior2", grade_label: "初二" } });
    vi.stubGlobal("fetch", spy);
    const c = await setClassGrade("c1", "junior2");
    expect(c.grade_label).toBe("初二");
    expect(callOf(spy)[0]).toContain("/api/v1/classes/c1");
    expect(callOf(spy)[1].method).toBe("PATCH");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ grade: "junior2" }));
  });

  // 🚨 空串是合法值，它的意思是「清掉年级」。写成 `if (grade)` 提前 return
  // 就会让「改回未填写」这个动作静默失败。
  it("setClassGrade sends an empty string to clear the grade", async () => {
    const spy = ok({ class: { id: "c1", name: "11A", join_code: "AB-CD", school_id: "s1", created_at: "z", grade: "", grade_label: "" } });
    vi.stubGlobal("fetch", spy);
    await setClassGrade("c1", "");
    expect(callOf(spy)[1].body).toBe(JSON.stringify({ grade: "" }));
  });

  it("CLASS_GRADE_OPTIONS starts with the empty option and covers the closed set", () => {
    expect(CLASS_GRADE_OPTIONS[0]).toEqual({ value: "", label: "未填写" });
    expect(CLASS_GRADE_OPTIONS.map((o) => o.value)).toEqual(
      ["", "junior1", "junior2", "junior3", "senior1", "senior2", "senior3"],
    );
  });
});
