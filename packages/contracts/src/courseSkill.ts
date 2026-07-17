import { Skill } from "./skill";
import raw from "../skills/info-literacy-course.json";

// The authored info-literacy course skill (Slice 12), parsed once here so
// apps/web can read phase-authored `page`/`ask_chips`/`steps` off the SAME
// single source of truth the backend reads via `go:embed` (`make
// sync-skills`) — not a second copy of the content. Follows the exact
// precedent of registry.ts's card-JSON re-exports (CARD_REGISTRY).
export const INFO_LITERACY_COURSE_SKILL: Skill = Skill.parse(raw);
