import { z } from "zod";

// Where a log row came from: "auto" appended by platform actions, "me" written
// by the student.
export const LogSource = z.enum(["auto", "me"]);
export type LogSource = z.infer<typeof LogSource>;

// One activity-log entry. date is "MM-DD" for display.
export const LogEntry = z.object({
  id: z.string(),
  date: z.string(),
  text: z.string(),
  source: LogSource,
});
export type LogEntry = z.infer<typeof LogEntry>;
