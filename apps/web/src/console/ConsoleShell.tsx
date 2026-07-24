import { useState } from "react";
import type { ApiClient } from "../api";
import type { SessionStore } from "../shell/session";
import { SettingsView } from "../shell/settings/SettingsView";
import { ConsoleRail, type ConsoleTab } from "./ConsoleRail";
import { ClassesView } from "./ClassesView";
import { ClassDetailView } from "./ClassDetailView";
import { StudentDetailView } from "./StudentDetailView";
import { OverviewView } from "./OverviewView";
import { TeachersView } from "./TeachersView";
import { ImportView } from "./ImportView";

export type ConsoleClient = Pick<
  ApiClient,
  | "listClasses" | "createClass" | "getClass" | "renameClass" | "regenerateJoinCode" | "removeEnrollment"
  | "getOverview" | "listTeacherInvites" | "createTeacherInvite" | "adminImport"
  | "listTeachers" | "assignTeacher" | "removeTeacher"
  | "getClassRosterReport" | "getStudentDetail" | "getStudentReport"
>;

export function ConsoleShell({
  session,
  client,
  onLogout,
}: {
  session: SessionStore;
  client: ConsoleClient;
  onLogout: () => void;
}) {
  const user = session.getUser();
  const role = user?.role ?? "admin";
  const [tab, setTab] = useState<ConsoleTab>(role === "admin" ? "overview" : "classes");
  const [openClassId, setOpenClassId] = useState<string | null>(null);
  const [openStudentId, setOpenStudentId] = useState<string | null>(null);
  const [openReport, setOpenReport] = useState<{ surface: string; scopeId: string } | null>(null);

  return (
    <div style={{ display: "flex", height: "100%", width: "100%", background: "#F3F4F8", overflow: "hidden" }}>
      <ConsoleRail role={role} tab={tab} onTab={(t) => { setTab(t); if (t === "classes") { setOpenClassId(null); setOpenStudentId(null); setOpenReport(null); } }} />
      <div style={{ flex: 1, overflow: "hidden", position: "relative", display: "flex" }}>
        {tab === "overview" && <OverviewView client={client} />}
        {tab === "classes" && openClassId == null && (
          <ClassesView client={client} role={role} onOpenClass={setOpenClassId} />
        )}
        {tab === "classes" && openClassId != null && openStudentId == null && (
          <ClassDetailView
            client={client}
            classId={openClassId}
            role={role}
            onBack={() => { setOpenClassId(null); setOpenStudentId(null); setOpenReport(null); }}
            onOpenStudent={setOpenStudentId}
          />
        )}
        {tab === "classes" && openClassId != null && openStudentId != null && openReport == null && (
          <StudentDetailView
            client={client}
            classId={openClassId}
            userId={openStudentId}
            onBack={() => { setOpenStudentId(null); setOpenReport(null); }}
            onOpenReport={(surface, scopeId) => setOpenReport({ surface, scopeId })}
          />
        )}
        {tab === "teachers" && <TeachersView client={client} />}
        {tab === "import" && <ImportView client={client} />}
        {tab === "settings" && <SettingsView session={session} user={user} onLogout={onLogout} />}
      </div>
    </div>
  );
}
