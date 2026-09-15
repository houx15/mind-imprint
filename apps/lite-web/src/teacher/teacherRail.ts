import type { TeacherRoute } from "./teacherRouting";

export type TeacherRailKey = "overview" | "classes" | "assignments" | "parentReports" | "teachers" | "import";

export type TeacherRailItem = { key: TeacherRailKey; label: string };

const TEACHER_ITEMS: TeacherRailItem[] = [
  { key: "classes", label: "班级" },
  { key: "assignments", label: "作业" },
  { key: "parentReports", label: "家长报告" },
];

const ADMIN_ITEMS: TeacherRailItem[] = [
  { key: "overview", label: "概览" },
  { key: "classes", label: "班级" },
  { key: "assignments", label: "作业" },
  { key: "parentReports", label: "家长报告" },
  { key: "teachers", label: "教师" },
  { key: "import", label: "导入" },
];

/** The rail's items for a role, in rail order. 设置 is the account button at the foot, not an item. */
export function teacherRailItems(role: string): TeacherRailItem[] {
  return role === "admin" ? ADMIN_ITEMS : TEACHER_ITEMS;
}

/** Whether a rail item is the current one. A page below an item keeps that item selected. */
export function isTeacherRailActive(key: TeacherRailKey, route: TeacherRoute): boolean {
  switch (key) {
    case "classes":
      return (
        route.view === "classes" ||
        route.view === "class" ||
        route.view === "classWeekly" ||
        route.view === "student" ||
        route.view === "item"
      );
    case "assignments":
      return (
        route.view === "assignments" || route.view === "assignmentNew" || route.view === "assignment" || route.view === "grading"
      );
    case "parentReports":
      return route.view === "parentReports" || route.view === "parentReport";
    default:
      return route.view === key;
  }
}
