import type { Task, CardInstance, Evaluation, TraceEvent } from "@mind-imprint/contracts";
import { listTasks, createTask, getTask, type TaskDetail } from "./tasks";
import { activateCard, submitCard, skipCard } from "./cards";
import { runEvaluation, getEvaluation } from "./evaluate";
import { runTurn, type TurnEvent } from "./turn";
import { signup, verifyEmail, signin, signout, getMe, type MeUser } from "./auth";
import {
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  type ClassSummary, type RosterStudent, type ClassDetail, type Teacher,
} from "./classes";
import {
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
  type Overview, type TeacherInvite, type ImportRow, type ImportResult,
} from "./admin";

export type { TaskDetail, TurnEvent, MeUser, ClassSummary, RosterStudent, ClassDetail, Teacher, Overview, TeacherInvite, ImportRow, ImportResult };
export { ApiError } from "./client";

export interface ApiClient {
  listTasks(): Promise<Task[]>;
  createTask(input: { title: string; seed: string | null }): Promise<Task>;
  getTask(id: string): Promise<TaskDetail>;
  activateCard(taskId: string, cardId: string): Promise<CardInstance>;
  submitCard(taskId: string, cardId: string, env: CardInstance): Promise<CardInstance>;
  skipCard(taskId: string, cardId: string, eventTrace: TraceEvent[]): Promise<CardInstance>;
  runEvaluation(taskId: string): Promise<Evaluation>;
  getEvaluation(taskId: string): Promise<Evaluation | null>;
  runTurn(taskId: string, userInput?: string): AsyncGenerator<TurnEvent>;
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
}

export const api: ApiClient = {
  listTasks, createTask, getTask, activateCard, submitCard, skipCard, runEvaluation, getEvaluation, runTurn,
  signup, verifyEmail, signin, signout, getMe,
  listClasses, createClass, getClass, renameClass, regenerateJoinCode, removeEnrollment,
  getOverview, listTeacherInvites, createTeacherInvite, adminImport, listTeachers, assignTeacher, removeTeacher,
};
