import type { TraceEvent, CourseSummary, CoursePlayerPayload, CourseProgress, CourseReport, CourseAnswerReport, Anchor, MaterialSource, ProjectStatus, CardCatalogEntry, CoverTheme, SelectionEval, ReadingBrief, TakeawayDraft, Reference } from "@mind-imprint/contracts";
import { signup, verifyEmail, signin, signout, getMe, setAccent, setBackground, putOnboarding, submitFeedback, type MeUser } from "./auth";
import type { AccentId } from "../ui/accent";
import type { BackgroundId } from "../ui/background";
import {
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  type ClassSummary, type RosterStudent, type ClassDetail, type Teacher,
} from "./classes";
import {
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  type Overview, type TeacherInvite, type ImportRow, type ImportResult,
} from "./admin";
import { listCourses, getCourse, getCourseProgress, saveCourseProgress, answerCourseQuiz, getCourseReport, getCourseAnswerReport, restartCourse, getCourseHistory, courseAsk, type CourseAskEvent, type CourseHistoryItem } from "./courses";
import { listProjects, finishProject, createProject, renameProject, getProjectCovers, type ProjectListItem } from "./projects";
import { getCardsCatalog, setCardTheme } from "./cards";
import { activateProjectCard, submitProjectCard, skipProjectCard } from "./projectCards";
import { addMaterial, logSourceOpen, prepareSourceAnnotation, type AddMaterialBody } from "./materials";
import { uploadUserImage, resolveUrl } from "./oss";
import { putBuffer, commitSnapshot, orderReview, type CommitSnapshotResult, type ReviewVoice } from "./writing";
import { type StudioTurnEvent } from "./studioTurn";
import {
  getClassRosterReport, getStudentDetail, getStudentEvaluationReport, getClassWeeklyReport, generateClassWeeklyProse,
  type RosterEntry, type ClassLiveHeader, type ClassRoster, type StudentRecord, type StudentDetail, type WeeklyReport, type WeeklyCard, type EvalReportEnvelope,
} from "./teacher";
import { readTurn, summonCard, evaluateCardSelection, getOpenCard, putReadingBrief, getTakeawayDraft, postFinalizeReading } from "./reading";

export type { MeUser, ClassSummary, RosterStudent, ClassDetail, Teacher, Overview, TeacherInvite, ImportRow, ImportResult, ProjectListItem, StudioTurnEvent, AddMaterialBody, CommitSnapshotResult, ReviewVoice, CourseAskEvent, RosterEntry, ClassLiveHeader, ClassRoster, StudentRecord, StudentDetail, WeeklyReport, WeeklyCard, EvalReportEnvelope };
export { ApiError } from "./client";

export interface ApiClient {
  signup(input: { email: string; password: string; display_name: string; join_code: string }): Promise<void>;
  verifyEmail(token: string): Promise<MeUser>;
  signin(input: { email: string; password: string }): Promise<MeUser>;
  signout(): Promise<void>;
  getMe(): Promise<MeUser>;
  setAccent(accent: AccentId): Promise<void>;
  setBackground(background: BackgroundId): Promise<void>;
  putOnboarding(): Promise<void>;
  submitFeedback(text: string): Promise<void>;
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
  getCourse(slug: string): Promise<CoursePlayerPayload>;
  getCourseProgress(slug: string): Promise<CourseProgress>;
  saveCourseProgress(slug: string, input: { current_ordinal: number; completed_ordinal?: number; active_seconds_delta?: number }): Promise<CourseProgress>;
  answerCourseQuiz(slug: string, body: { stepId: string; interactionId: string; selected: string[]; correct: boolean }): Promise<void>;
  courseAsk(slug: string, input: string, ordinal: number): AsyncGenerator<CourseAskEvent>;
  getCourseReport(slug: string, attemptId?: string): Promise<CourseReport>;
  getCourseAnswerReport(slug: string, attemptId?: string): Promise<CourseAnswerReport>;
  restartCourse(slug: string): Promise<void>;
  getCourseHistory(): Promise<CourseHistoryItem[]>;
  listProjects(): Promise<ProjectListItem[]>;
  finishProject(id: string): Promise<{ status: ProjectStatus }>;
  createProject(body: { title?: string; prompt: string; projectType?: string; writingLanguage?: "en" | "zh" | "bilingual"; cover?: string }): Promise<{ id: string }>;
  renameProject(id: string, title: string): Promise<{ title: string }>;
  // Task 3: cover picker options for the create-project drawer.
  getProjectCovers(): Promise<{ key: string; url: string }[]>;
  activateProjectCard(projectId: string, cid: string): Promise<void>;
  submitProjectCard(projectId: string, cid: string, input: { field_values: Record<string, unknown>; event_trace: TraceEvent[]; anchors: Anchor[] }): AsyncGenerator<StudioTurnEvent>;
  skipProjectCard(projectId: string, cid: string, input: { event_trace: TraceEvent[] }): Promise<void>;
  addMaterial(projectId: string, body: AddMaterialBody): Promise<MaterialSource>;
  logSourceOpen(projectId: string, materialId: string, timeSpentS: number): Promise<void>;
  prepareSourceAnnotation(projectId: string, materialId: string): Promise<boolean>;
  readTurn(projectId: string, materialId: string, body: { student_text: string; focused_spans: { block_id: string; quote: string }[] }): AsyncGenerator<StudioTurnEvent>;
  summonCard(projectId: string, materialId: string, cardId: string): AsyncGenerator<StudioTurnEvent>;
  evaluateCardSelection(projectId: string, cid: string, body: { block_id: string; start: number; end: number; quote: string; dimension: string }): Promise<SelectionEval>;
  getOpenCard(projectId: string, materialId: string): Promise<{ cardInstanceId: string; cardId: string; status: "proposed" | "active"; anchors: Anchor[] } | null>;
  // S2: reading brief-in + takeaway finalize (Task 9) — rid is the reference
  // id, not the material id source.id above.
  putReadingBrief(projectId: string, rid: string, brief: ReadingBrief): Promise<void>;
  getTakeawayDraft(projectId: string, rid: string): Promise<TakeawayDraft>;
  postFinalizeReading(projectId: string, rid: string, body: { newLeads: string[]; proposalImpact: string }): Promise<Reference>;
  putBuffer(projectId: string, content: string): Promise<void>;
  commitSnapshot(projectId: string, content: string): Promise<CommitSnapshotResult>;
  orderReview(projectId: string, snapshotId: string, voice: ReviewVoice): AsyncGenerator<StudioTurnEvent>;
  getCardsCatalog(theme?: CoverTheme): Promise<{ cards: CardCatalogEntry[]; theme: CoverTheme }>;
  setCardTheme(theme: CoverTheme): Promise<CoverTheme>;
  getClassRosterReport(classId: string): Promise<ClassRoster>;
  getStudentDetail(classId: string, userId: string): Promise<StudentDetail>;
  getStudentEvaluationReport(classId: string, userId: string, projectId: string): Promise<EvalReportEnvelope | null>;
  getClassWeeklyReport(classId: string, weekStart?: string): Promise<WeeklyReport>;
  generateClassWeeklyProse(classId: string, weekStart?: string): Promise<WeeklyReport>;
  // OSS storage: upload a user image (returns its object key), resolve a key to
  // a short-lived signed GET URL.
  uploadUserImage(file: File): Promise<string>;
  resolveUrl(objectKey: string): Promise<string>;
}

export const api: ApiClient = {
  signup, verifyEmail, signin, signout, getMe, setAccent, setBackground, putOnboarding, submitFeedback,
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  listCourses, getCourse, getCourseProgress, saveCourseProgress, answerCourseQuiz, getCourseReport, getCourseAnswerReport, restartCourse, getCourseHistory, courseAsk,
  listProjects, finishProject, createProject, renameProject, getProjectCovers,
  activateProjectCard, submitProjectCard, skipProjectCard,
  addMaterial, logSourceOpen, prepareSourceAnnotation,
  readTurn, summonCard, evaluateCardSelection, getOpenCard,
  putReadingBrief, getTakeawayDraft, postFinalizeReading,
  putBuffer, commitSnapshot, orderReview,
  getCardsCatalog, setCardTheme,
  getClassRosterReport, getStudentDetail, getStudentEvaluationReport, getClassWeeklyReport, generateClassWeeklyProse,
  uploadUserImage, resolveUrl,
};
