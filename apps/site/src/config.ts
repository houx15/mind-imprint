// Entrances into the actual product. The marketing site never holds secrets;
// it only links out to the app. Override with PUBLIC_APP_URL at build time.
const APP_URL = import.meta.env.PUBLIC_APP_URL ?? "http://localhost:5173";

export const appUrl = APP_URL;
export const loginUrl = APP_URL;
// Demo deep-links into the app with ?trial=1, which auto-signs-in as the seeded
// sample student (handled in apps/web AppShell). Note: ?demo is a different,
// pre-existing surface (the dev card gallery), so the trial param is distinct.
export const demoUrl = `${APP_URL}/?trial=1`;
