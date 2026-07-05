import type { APIRoute } from "astro";
import { getAdminClient } from "../../lib/supabase";

export const prerender = false;

const SHORT_MAX = 500;
const LONG_MAX = 2000;

interface LeadPayload {
  contact_name?: unknown;
  contact?: unknown;
  child_age?: unknown;
  interest?: unknown;
  message?: unknown;
  locale?: unknown;
  hp?: unknown;
}

function asTrimmedString(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

export const POST: APIRoute = async ({ request }) => {
  let body: LeadPayload;
  try {
    body = await request.json();
  } catch {
    return json(400, { ok: false, error: "Invalid request body." });
  }

  if (typeof body !== "object" || body === null) {
    return json(400, { ok: false, error: "Invalid request body." });
  }

  // Honeypot: if filled in, silently pretend success without inserting.
  if (typeof body.hp === "string" && body.hp.trim().length > 0) {
    return json(200, { ok: true });
  }

  const contact = asTrimmedString(body.contact);
  if (!contact || contact.length > SHORT_MAX) {
    return json(400, { ok: false, error: "A valid contact is required." });
  }

  const contactName = asTrimmedString(body.contact_name);
  if (contactName && contactName.length > SHORT_MAX) {
    return json(400, { ok: false, error: "contact_name is too long." });
  }

  const childAge = asTrimmedString(body.child_age);
  if (childAge && childAge.length > SHORT_MAX) {
    return json(400, { ok: false, error: "child_age is too long." });
  }

  const interest = asTrimmedString(body.interest);
  if (interest && interest.length > LONG_MAX) {
    return json(400, { ok: false, error: "interest is too long." });
  }

  const message = asTrimmedString(body.message);
  if (message && message.length > LONG_MAX) {
    return json(400, { ok: false, error: "message is too long." });
  }

  const locale = asTrimmedString(body.locale);
  if (locale && locale.length > SHORT_MAX) {
    return json(400, { ok: false, error: "locale is too long." });
  }

  const userAgent = request.headers.get("user-agent") ?? undefined;

  try {
    const supabase = getAdminClient();
    const { error } = await supabase.from("leads").insert({
      contact_name: contactName ?? null,
      contact,
      child_age: childAge ?? null,
      interest: interest ?? null,
      message: message ?? null,
      locale: locale ?? null,
      source: "web",
      user_agent: userAgent ?? null,
    });

    if (error) {
      return json(500, { ok: false });
    }

    return json(200, { ok: true });
  } catch {
    return json(500, { ok: false });
  }
};

function json(status: number, body: Record<string, unknown>): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}
