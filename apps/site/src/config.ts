// Entrances into the actual product. The marketing site never holds secrets;
// it only links out to the app. Override with PUBLIC_APP_URL at build time.
const APP_URL = import.meta.env.PUBLIC_APP_URL ?? "http://localhost:5173";

export const appUrl = APP_URL;
export const loginUrl = APP_URL;
// Demo carries a hint the app can read to sign in as the seeded sample student.
// (Wiring the auto-demo-login handler in apps/web is a follow-up.)
export const demoUrl = `${APP_URL}/?demo=1`;
