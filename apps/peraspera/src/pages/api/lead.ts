import type { APIRoute } from "astro";
import { getAdminClient } from "../../lib/supabase";

export const prerender = false;

const CONTACT_MAX = 500;
const TEXT_MAX = 2000;
const LOCALE_MAX = 16;
const BODY_MAX = 8192;
const INTEREST_OPTIONS = new Set(["sprint", "academy", "explore"]);

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
  // Body size guard (defense-in-depth on this unauthenticated endpoint).
  // Reject before reading/parsing when Content-Length declares an oversized body.
  const declaredLength = Number(request.headers.get("content-length"));
  if (Number.isFinite(declaredLength) && declaredLength > BODY_MAX) {
    return json(413, { ok: false });
  }

  let body: LeadPayload;
  try {
    const raw = await request.text();
    // Defensive cap when Content-Length was absent or understated.
    if (raw.length > BODY_MAX) {
      return json(413, { ok: false });
    }
    body = JSON.parse(raw);
  } catch {
    return json(400, { ok: false, error: "Invalid request body." });
  }

  if (typeof body !== "object" || body === null) {
    return json(400, { ok: false, error: "Invalid request body." });
  }

  // Honeypot: any non-null value that stringifies to non-empty is a bot.
  // Silently pretend success without inserting.
  if (body.hp != null && String(body.hp).trim().length > 0) {
    return json(200, { ok: true });
  }

  const contact = asTrimmedString(body.contact);
  if (!contact || contact.length > CONTACT_MAX) {
    return json(400, { ok: false, error: "A valid contact is required." });
  }

  const contactName = asTrimmedString(body.contact_name);
  if (contactName && contactName.length > TEXT_MAX) {
    return json(400, { ok: false, error: "contact_name is too long." });
  }

  const childAge = asTrimmedString(body.child_age);
  if (childAge && childAge.length > TEXT_MAX) {
    return json(400, { ok: false, error: "child_age is too long." });
  }

  // Constrain interest to the known option set; drop (null) anything else
  // rather than rejecting an otherwise-valid lead over an odd interest value.
  const interestRaw = asTrimmedString(body.interest);
  const interest = interestRaw && INTEREST_OPTIONS.has(interestRaw) ? interestRaw : undefined;

  const message = asTrimmedString(body.message);
  if (message && message.length > TEXT_MAX) {
    return json(400, { ok: false, error: "message is too long." });
  }

  const locale = asTrimmedString(body.locale);
  if (locale && locale.length > LOCALE_MAX) {
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
