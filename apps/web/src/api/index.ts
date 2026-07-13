import type { TraceEvent, Course, CourseSummary, CourseProgress, RenderedStep, StudioProjection, Anchor } from "@mind-imprint/contracts";
import { signup, verifyEmail, signin, signout, getMe, type MeUser } from "./auth";
import {
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  type ClassSummary, type RosterStudent, type ClassDetail, type Teacher,
} from "./classes";
import {
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  type Overview, type TeacherInvite, type ImportRow, type ImportResult,
} from "./admin";
import { listCourses, getCourse, getCourseProgress, saveCourseProgress, renderCourseStep } from "./courses";
import { listProjects, getProject, type ProjectListItem } from "./projects";
import { activateProjectCard, submitProjectCard, skipProjectCard } from "./projectCards";
import type { StudioTurnEvent } from "./studioTurn";

export type { MeUser, ClassSummary, RosterStudent, ClassDetail, Teacher, Overview, TeacherInvite, ImportRow, ImportResult, ProjectListItem, StudioTurnEvent };
export { ApiError } from "./client";

export interface ApiClient {
  signup(input: { email: string; password: string; display_name: string; join_code: string }): Promise<void>;
  verifyEmail(token: string): Promise<MeUser>;
  signin(input: { email: string; password: string }): Promise<MeUser>;
  signout(): Promise<void>;
  getMe(): Promise<MeUser>;
  listClasses(): Promise<ClassSummary[]>;
  createClass(input: { name: string; teacher_user_id?: string }): Promise<ClassSummary>;
  getClass(id: string): Promise<ClassDetail>;
  renameClass(id: string, name: string): Promise<ClassSummary>;
  regenerateJoinCode(id: string): Promise<ClassSummary>;
  removeEnrollment(id: string, userId: string): Promise<void>;
  getOverview(): Promise<Overview>;
  listTeacherInvites(): Promise<TeacherInvite[]>;
  createTeacherInvite(input: { email?: string; expires_days?: number }): Promise<{ code: string; expires_at: string }>;
  adminImport(rows: ImportRow[]): Promise<ImportResult>;
  listTeachers(): Promise<Teacher[]>;
  assignTeacher(classId: string, teacherUserId: string): Promise<{ teachers: Teacher[] }>;
  removeTeacher(classId: string, userId: string): Promise<void>;
  listCourses(): Promise<CourseSummary[]>;
  getCourse(id: string): Promise<Course>;
  getCourseProgress(id: string): Promise<CourseProgress>;
  saveCourseProgress(id: string, input: { current_ordinal: number; completed_ordinals: number[] }): Promise<CourseProgress>;
  renderCourseStep(courseId: string, ordinal: number): Promise<RenderedStep>;
  listProjects(): Promise<ProjectListItem[]>;
  getProject(id: string): Promise<StudioProjection>;
  activateProjectCard(projectId: string, cid: string): Promise<void>;
  submitProjectCard(projectId: string, cid: string, input: { field_values: Record<string, unknown>; event_trace: TraceEvent[]; anchors: Anchor[] }): AsyncGenerator<StudioTurnEvent>;
  skipProjectCard(projectId: string, cid: string, input: { event_trace: TraceEvent[] }): Promise<void>;
}

export const api: ApiClient = {
  signup, verifyEmail, signin, signout, getMe,
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  listCourses, getCourse, getCourseProgress, saveCourseProgress, renderCourseStep,
  listProjects, getProject,
  activateProjectCard, submitProjectCard, skipProjectCard,
};
