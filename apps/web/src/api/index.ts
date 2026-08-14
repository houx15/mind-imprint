import type { TraceEvent, CourseSummary, CoursePlayerPayload, CourseProgress, Anchor, MaterialSource, ProjectStatus, ChatThread, ChatMessage, AbilityModel, CollectedCard, CardCatalogEntry, CoverTheme, SelectionEval, ReadingBrief, TakeawayDraft, Reference } from "@mind-imprint/contracts";
import { signup, verifyEmail, signin, signout, getMe, setAccent, type MeUser } from "./auth";
import type { AccentId } from "../ui/accent";
import {
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  type ClassSummary, type RosterStudent, type ClassDetail, type Teacher,
} from "./classes";
import {
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  type Overview, type TeacherInvite, type ImportRow, type ImportResult,
} from "./admin";
import { listCourses, getCourse, getCourseProgress, saveCourseProgress, answerCourseQuiz, courseAsk, type CourseAskEvent } from "./courses";
import { listProjects, finishProject, createProject, getProjectCovers, submitOnboarding, submitSelfScore, submitReflection, submitFraming, submitPerspectives, reopenStation, type ProjectListItem } from "./projects";
import { getAbilityModel } from "./ability";
import { getGrowthCards, getCardsCatalog, setCardTheme } from "./cards";
import { activateProjectCard, submitProjectCard, skipProjectCard } from "./projectCards";
import { addMaterial, logSourceOpen, prepareSourceAnnotation, type AddMaterialBody } from "./materials";
import { uploadUserImage, resolveUrl } from "./oss";
import { putBuffer, commitSnapshot, orderReview, orderSpotCheck, attestGate, signDeclaration, type CommitSnapshotResult, type ReviewVoice } from "./writing";
import { postDisposition, type StudioTurnEvent } from "./studioTurn";
import { listThreads, createThread, getMessages, submitChatCard, skipChatCard, chatTurn, type ChatTurnEvent } from "./chat";
import {
  getClassRosterReport, getStudentDetail, getStudentEvaluationReport, getClassWeeklyReport, generateClassWeeklyProse,
  type RosterReportEntry, type StudentRecord, type StudentDetail, type WeeklyReport, type WeeklyCard, type EvalReportEnvelope,
} from "./teacher";
import { readTurn, summonCard, evaluateCardSelection, getOpenCard, putReadingBrief, getTakeawayDraft, postFinalizeReading } from "./reading";

export type { MeUser, ClassSummary, RosterStudent, ClassDetail, Teacher, Overview, TeacherInvite, ImportRow, ImportResult, ProjectListItem, StudioTurnEvent, AddMaterialBody, CommitSnapshotResult, ReviewVoice, ChatTurnEvent, CourseAskEvent, RosterReportEntry, StudentRecord, StudentDetail, WeeklyReport, WeeklyCard, EvalReportEnvelope };
export { ApiError } from "./client";

export interface ApiClient {
  signup(input: { email: string; password: string; display_name: string; join_code: string }): Promise<void>;
  verifyEmail(token: string): Promise<MeUser>;
  signin(input: { email: string; password: string }): Promise<MeUser>;
  signout(): Promise<void>;
  getMe(): Promise<MeUser>;
  setAccent(accent: AccentId): Promise<void>;
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
  listProjects(): Promise<ProjectListItem[]>;
  finishProject(id: string): Promise<{ status: ProjectStatus }>;
  createProject(body: { title?: string; prompt: string; projectType?: string; writingLanguage?: "en" | "zh" | "bilingual"; cover?: string }): Promise<{ id: string }>;
  // Task 3: cover picker options for the create-project drawer.
  getProjectCovers(): Promise<{ key: string; url: string }[]>;
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
  // N3f Task 7: S3/S4 station spot-check (信源体检 / 论证体检) — `contractId`
  // is `evaluate_sources` | `build_argument`, the only two contracts with a
  // spot-check defined.
  orderSpotCheck(projectId: string, contractId: string): Promise<void>;
  postDisposition(projectId: string, interventionId: string, action: "accept" | "rewrite" | "reject", reason: string): Promise<void>;
  attestGate(projectId: string, contractId: string, item: string, confirmed: boolean): Promise<void>;
  // N3f Task 9: signs the S6 AI 使用申报单 — no request body, no stream.
  signDeclaration(projectId: string): Promise<void>;
  listThreads(): Promise<ChatThread[]>;
  createThread(title?: string): Promise<ChatThread>;
  getMessages(threadId: string): Promise<ChatMessage[]>;
  submitChatCard(threadId: string, cardInstanceId: string, payload: { field_values: unknown; event_trace: unknown; anchors: unknown }): Promise<void>;
  skipChatCard(threadId: string, cardInstanceId: string): Promise<void>;
  chatTurn(threadId: string, userInput: string): AsyncGenerator<ChatTurnEvent>;
  getAbilityModel(): Promise<AbilityModel>;
  getGrowthCards(): Promise<CollectedCard[]>;
  getCardsCatalog(theme?: CoverTheme): Promise<{ cards: CardCatalogEntry[]; theme: CoverTheme }>;
  setCardTheme(theme: CoverTheme): Promise<CoverTheme>;
  getClassRosterReport(classId: string): Promise<RosterReportEntry[]>;
  getStudentDetail(classId: string, userId: string): Promise<StudentDetail>;
  getStudentEvaluationReport(classId: string, userId: string, projectId: string): Promise<EvalReportEnvelope | null>;
  getClassWeeklyReport(classId: string): Promise<WeeklyReport>;
  generateClassWeeklyProse(classId: string): Promise<WeeklyReport>;
  // OSS storage: upload a user image (returns its object key), resolve a key to
  // a short-lived signed GET URL.
  uploadUserImage(file: File): Promise<string>;
  resolveUrl(objectKey: string): Promise<string>;
}

export const api: ApiClient = {
  signup, verifyEmail, signin, signout, getMe, setAccent,
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  listCourses, getCourse, getCourseProgress, saveCourseProgress, answerCourseQuiz, courseAsk,
  listProjects, finishProject, createProject, getProjectCovers, submitOnboarding, submitSelfScore, submitReflection, submitFraming, submitPerspectives, reopenStation,
  activateProjectCard, submitProjectCard, skipProjectCard,
  addMaterial, logSourceOpen, prepareSourceAnnotation,
  readTurn, summonCard, evaluateCardSelection, getOpenCard,
  putReadingBrief, getTakeawayDraft, postFinalizeReading,
  putBuffer, commitSnapshot, orderReview, orderSpotCheck, postDisposition, attestGate, signDeclaration,
  listThreads, createThread, getMessages, submitChatCard, skipChatCard, chatTurn,
  getAbilityModel,
  getGrowthCards, getCardsCatalog, setCardTheme,
  getClassRosterReport, getStudentDetail, getStudentEvaluationReport, getClassWeeklyReport, generateClassWeeklyProse,
  uploadUserImage, resolveUrl,
};
