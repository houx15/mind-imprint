import type { TraceEvent, Course, CourseSummary, CourseProgress, RenderedStep, StudioProjection, Anchor, MaterialSource, Assessment, ChatThread, ChatMessage, CourseSession, GrowthHistoryEntry } from "@mind-imprint/contracts";
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
import { listProjects, getProject, finishProject, type ProjectListItem } from "./projects";
import { getGrowthHistory } from "./growth";
import { activateProjectCard, submitProjectCard, skipProjectCard } from "./projectCards";
import { addMaterial, logSourceOpen, type AddMaterialBody } from "./materials";
import { putBuffer, commitSnapshot, orderReview, attestGate, type CommitSnapshotResult, type ReviewVoice } from "./writing";
import { postDisposition, type StudioTurnEvent } from "./studioTurn";
import { getAssessment } from "./assessment";
import { listThreads, createThread, getMessages, submitChatCard, skipChatCard, chatTurn, type ChatTurnEvent } from "./chat";
import { startCourseSession, getCourseSession, submitCourseCard, skipCourseCard, courseAsk, courseAdvance, type CourseTurnEvent } from "./courseSession";
import { getCourseAssessment, generateCourseAssessment } from "./courseAssessment";
import { getChatAssessment, generateChatAssessment } from "./chatAssessment";

export type { MeUser, ClassSummary, RosterStudent, ClassDetail, Teacher, Overview, TeacherInvite, ImportRow, ImportResult, ProjectListItem, StudioTurnEvent, AddMaterialBody, CommitSnapshotResult, ReviewVoice, ChatTurnEvent, CourseTurnEvent };
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
  saveCourseProgress(id: string, input: { current_ordinal: number }): Promise<CourseProgress>;
  renderCourseStep(courseId: string, ordinal: number): Promise<RenderedStep>;
  listProjects(): Promise<ProjectListItem[]>;
  getProject(id: string): Promise<StudioProjection>;
  finishProject(id: string): Promise<Assessment>;
  activateProjectCard(projectId: string, cid: string): Promise<void>;
  submitProjectCard(projectId: string, cid: string, input: { field_values: Record<string, unknown>; event_trace: TraceEvent[]; anchors: Anchor[] }): AsyncGenerator<StudioTurnEvent>;
  skipProjectCard(projectId: string, cid: string, input: { event_trace: TraceEvent[] }): Promise<void>;
  addMaterial(projectId: string, body: AddMaterialBody): Promise<MaterialSource>;
  logSourceOpen(projectId: string, materialId: string, timeSpentS: number): Promise<void>;
  putBuffer(projectId: string, content: string): Promise<void>;
  commitSnapshot(projectId: string, content: string): Promise<CommitSnapshotResult>;
  orderReview(projectId: string, snapshotId: string, voice: ReviewVoice): AsyncGenerator<StudioTurnEvent>;
  postDisposition(projectId: string, interventionId: string, action: "accept" | "rewrite" | "reject", reason: string): Promise<void>;
  attestGate(projectId: string, contractId: string, item: string, confirmed: boolean): Promise<void>;
  getAssessment(projectId: string): Promise<Assessment | null>;
  listThreads(): Promise<ChatThread[]>;
  createThread(title?: string): Promise<ChatThread>;
  getMessages(threadId: string): Promise<ChatMessage[]>;
  submitChatCard(threadId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void>;
  skipChatCard(threadId: string, cardInstanceId: string): Promise<void>;
  chatTurn(threadId: string, userInput: string): AsyncGenerator<ChatTurnEvent>;
  getChatAssessment(threadId: string): Promise<Assessment | null>;
  generateChatAssessment(threadId: string): Promise<Assessment>;
  startCourseSession(courseId: string): Promise<CourseSession>;
  getCourseSession(courseId: string): Promise<CourseSession>;
  submitCourseCard(courseId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void>;
  skipCourseCard(courseId: string, cardInstanceId: string): Promise<void>;
  courseAsk(courseId: string, userInput: string): AsyncGenerator<CourseTurnEvent>;
  courseAdvance(courseId: string): AsyncGenerator<CourseTurnEvent>;
  getCourseAssessment(courseId: string): Promise<Assessment | null>;
  generateCourseAssessment(courseId: string): Promise<Assessment>;
  getGrowthHistory(): Promise<GrowthHistoryEntry[]>;
}

export const api: ApiClient = {
  signup, verifyEmail, signin, signout, getMe,
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  listCourses, getCourse, getCourseProgress, saveCourseProgress, renderCourseStep,
  listProjects, getProject, finishProject,
  activateProjectCard, submitProjectCard, skipProjectCard,
  addMaterial, logSourceOpen,
  putBuffer, commitSnapshot, orderReview, postDisposition, attestGate,
  getAssessment,
  listThreads, createThread, getMessages, submitChatCard, skipChatCard, chatTurn,
  getChatAssessment, generateChatAssessment,
  startCourseSession, getCourseSession, submitCourseCard, skipCourseCard, courseAsk, courseAdvance,
  getCourseAssessment, generateCourseAssessment,
  getGrowthHistory,
};
