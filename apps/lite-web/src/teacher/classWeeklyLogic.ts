// teacher/classWeeklyLogic.ts — where the class weekly page shows a student's
// lead and action (controller Ruling 14).
//
// The class prose has one lead and one action per student, but a student can
// have a watch card and a praise card. The text is shown once: on the watch
// card when there is one (需要建议), otherwise on the praise card (值得表扬).
// A praise card for a student who also has a watch card shows only name, tag
// and evidence.

/** Whether the card of `kind` for `userId` carries that student's lead and action. */
export function cardShowsProse(kind: string, userId: string, watchUserIds: ReadonlySet<string>): boolean {
  if (kind === "watch") return true;
  return !watchUserIds.has(userId);
}
