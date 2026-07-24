import type { TraceEvent, Course, CourseSummary, CourseProgress, RenderedStep, StudioProjection, Anchor, MaterialSource, DualAxisReport, ChatThread, ChatMessage, CourseSession, GrowthHistoryEntry, AbilityModel, CollectedCard } from "@mind-imprint/contracts";
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
import { listProjects, getProject, finishProject, createProject, submitOnboarding, submitSelfScore, submitReflection, submitFraming, submitPerspectives, reopenStation, type ProjectListItem } from "./projects";
import { getGrowthHistory } from "./growth";
import { getAbilityModel } from "./ability";
import { getGrowthCards } from "./cards";
import { activateProjectCard, submitProjectCard, skipProjectCard } from "./projectCards";
import { addMaterial, logSourceOpen, type AddMaterialBody } from "./materials";
import { putBuffer, commitSnapshot, orderReview, orderSpotCheck, attestGate, signDeclaration, type CommitSnapshotResult, type ReviewVoice } from "./writing";
import { postDisposition, type StudioTurnEvent } from "./studioTurn";
import { getAssessment } from "./assessment";
import { listThreads, createThread, getMessages, submitChatCard, skipChatCard, chatTurn, type ChatTurnEvent } from "./chat";
import { startCourseSession, getCourseSession, restartCourseSession, submitCourseCard, skipCourseCard, courseAsk, courseAdvance, type CourseTurnEvent } from "./courseSession";
import { getCourseAssessment, generateCourseAssessment } from "./courseAssessment";
import { getChatAssessment, generateChatAssessment } from "./chatAssessment";
import {
  getClassRosterReport, getStudentDetail, getStudentReport, getClassWeeklyReport, generateClassWeeklyProse,
  type RosterReportEntry, type StudentRecord, type StudentDetail, type TeacherReport, type WeeklyReport, type WeeklyCard,
} from "./teacher";

export type { MeUser, ClassSummary, RosterStudent, ClassDetail, Teacher, Overview, TeacherInvite, ImportRow, ImportResult, ProjectListItem, StudioTurnEvent, AddMaterialBody, CommitSnapshotResult, ReviewVoice, ChatTurnEvent, CourseTurnEvent, RosterReportEntry, StudentRecord, StudentDetail, TeacherReport, WeeklyReport, WeeklyCard };
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
  finishProject(id: string): Promise<DualAxisReport>;
  createProject(body: { title?: string; prompt: string }): Promise<{ id: string }>;
  submitOnboarding(projectId: string, body: { restate: string; weakPicks: number[] }): Promise<void>;
  submitSelfScore(projectId: string, body: { scores: { code: string; band: number }[] }): Promise<void>;
  submitReflection(projectId: string, body: { text: string }): Promise<void>;
  submitFraming(projectId: string, body: { terms: { term: string; definition: string }[]; answers: string[]; searchPlan: string[] }): Promise<void>;
  submitPerspectives(projectId: string, body: { perspectives: { text: string; level: string }[] }): Promise<void>;
  // N6-E Task 6: re-opens a `waived` station.
  reopenStation(projectId: string, code: string): Promise<void>;
  activateProjectCard(projectId: string, cid: string): Promise<void>;
  submitProjectCard(projectId: string, cid: string, input: { field_values: Record<string, unknown>; event_trace: TraceEvent[]; anchors: Anchor[] }): AsyncGenerator<StudioTurnEvent>;
  skipProjectCard(projectId: string, cid: string, input: { event_trace: TraceEvent[] }): Promise<void>;
  addMaterial(projectId: string, body: AddMaterialBody): Promise<MaterialSource>;
  logSourceOpen(projectId: string, materialId: string, timeSpentS: number): Promise<void>;
  putBuffer(projectId: string, content: string): Promise<void>;
  commitSnapshot(projectId: string, content: string): Promise<CommitSnapshotResult>;
  orderReview(projectId: string, snapshotId: string, voice: ReviewVoice): AsyncGenerator<StudioTurnEvent>;
  // N3f Task 7: S3/S4 station spot-check (信源体检 / 论证体检) — `contractId`
  // is `evaluate_sources` | `build_argument`, the only two contracts with a
  // spot-check defined.
  orderSpotCheck(projectId: string, contractId: string): Promise<void>;
  postDisposition(projectId: string, interventionId: string, action: "accept" | "rewrite" | "reject", reason: string): Promise<void>;
  attestGate(projectId: string, contractId: string, item: string, confirmed: boolean): Promise<void>;
  // N3f Task 9: signs the S6 AI 使用申报单 — no request body, no stream.
  signDeclaration(projectId: string): Promise<void>;
  getAssessment(projectId: string): Promise<DualAxisReport | null>;
  listThreads(): Promise<ChatThread[]>;
  createThread(title?: string): Promise<ChatThread>;
  getMessages(threadId: string): Promise<ChatMessage[]>;
  submitChatCard(threadId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void>;
  skipChatCard(threadId: string, cardInstanceId: string): Promise<void>;
  chatTurn(threadId: string, userInput: string): AsyncGenerator<ChatTurnEvent>;
  getChatAssessment(threadId: string): Promise<DualAxisReport | null>;
  generateChatAssessment(threadId: string): Promise<DualAxisReport>;
  startCourseSession(courseId: string): Promise<CourseSession>;
  getCourseSession(courseId: string): Promise<CourseSession>;
  restartCourseSession(courseId: string): Promise<CourseSession>;
  submitCourseCard(courseId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void>;
  skipCourseCard(courseId: string, cardInstanceId: string): Promise<void>;
  courseAsk(courseId: string, userInput: string): AsyncGenerator<CourseTurnEvent>;
  courseAdvance(courseId: string): AsyncGenerator<CourseTurnEvent>;
  getCourseAssessment(courseId: string): Promise<DualAxisReport | null>;
  generateCourseAssessment(courseId: string): Promise<DualAxisReport>;
  getGrowthHistory(): Promise<GrowthHistoryEntry[]>;
  getAbilityModel(): Promise<AbilityModel>;
  getGrowthCards(): Promise<CollectedCard[]>;
  getClassRosterReport(classId: string): Promise<RosterReportEntry[]>;
  getStudentDetail(classId: string, userId: string): Promise<StudentDetail>;
  getStudentReport(classId: string, userId: string, surface: string, scopeId: string): Promise<TeacherReport>;
  getClassWeeklyReport(classId: string): Promise<WeeklyReport>;
  generateClassWeeklyProse(classId: string): Promise<WeeklyReport>;
}

export const api: ApiClient = {
  signup, verifyEmail, signin, signout, getMe,
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  listCourses, getCourse, getCourseProgress, saveCourseProgress, renderCourseStep,
  listProjects, getProject, finishProject, createProject, submitOnboarding, submitSelfScore, submitReflection, submitFraming, submitPerspectives, reopenStation,
  activateProjectCard, submitProjectCard, skipProjectCard,
  addMaterial, logSourceOpen,
  putBuffer, commitSnapshot, orderReview, orderSpotCheck, postDisposition, attestGate, signDeclaration,
  getAssessment,
  listThreads, createThread, getMessages, submitChatCard, skipChatCard, chatTurn,
  getChatAssessment, generateChatAssessment,
  startCourseSession, getCourseSession, restartCourseSession, submitCourseCard, skipCourseCard, courseAsk, courseAdvance,
  getCourseAssessment, generateCourseAssessment,
  getGrowthHistory,
  getAbilityModel,
  getGrowthCards,
  getClassRosterReport, getStudentDetail, getStudentReport, getClassWeeklyReport, generateClassWeeklyProse,
};
