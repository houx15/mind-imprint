import { z } from "zod";

// The review-room reflection: answers indexed to the 5 dimensioned prompts,
// plus whether the student has marked the reflection done.
export const ReflectionDoc = z.object({
  answers: z.array(z.string()),
  done: z.boolean(),
});
export type ReflectionDoc = z.infer<typeof ReflectionDoc>;
