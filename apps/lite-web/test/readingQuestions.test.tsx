import { render, screen, waitFor, cleanup, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ReadingQuestions, WRITING_IDEA_KEY } from "@lite/readings/ReadingQuestions";
import { WritingsLanding } from "@lite/writings/WritingsLanding";

/**
 * ReadingQuestions — what a finished reading leaves her with besides a full
 * stop, and the handoff that carries exactly one of them into 写作.
 *
 * The product ruling: *"just a suggestion, but suggestion is very important,
 * some interesting questions would grow from this reading."* A suggestion,
 * not a pre-seeded writing project — so these tests pin that ONLY the
 * question text crosses into WritingsLanding, and that the sessionStorage
 * key is read-AND-CLEAR (a lingering key would silently refill the box on a
 * later, unrelated visit).
 */

const READING_ID = "atom-1";

type Route = { status?: number; body?: unknown };
let routes: Record<string, Route>;
let calls: { method: string; url: string; body: unknown }[];

function key(method: string, url: string) {
  return `${method} ${url}`;
}

function jsonResponse(status: number, body: unknown): Response {
  return { ok: status >= 200 && status < 300, status, json: async () => body } as Response;
}

function stubFetch() {
  calls = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string, init?: RequestInit) => {
      const method = (init?.method ?? "GET").toUpperCase();
      const raw = init?.body;
      const body = typeof raw === "string" ? JSON.parse(raw) : undefined;
      calls.push({ method, url, body });
      const route = routes[key(method, url)];
      if (!route) return jsonResponse(404, { error: { code: "not_found", message: "资源不存在" } });
      return jsonResponse(route.status ?? 200, route.body ?? {});
    }),
  );
}

type Question = { id: string; text: string; anchorQuote: string; anchorBlock: string };

const QUESTION_1: Question = {
  id: "q1",
  text: "为什么太阳能装机量增长这么快，碳排放却没有跟着降下来？",
  anchorQuote: "中国的太阳能装机量在过去十年增长了十倍。",
  anchorBlock: "b1",
};

const QUESTION_2: Question = {
  id: "q2",
  text: "「装机量」和「实际发电量」是一回事吗？",
  anchorQuote: "但同一时期，中国的碳排放总量仍居全球第一。",
  anchorBlock: "b2",
};

const TWO_QUESTIONS: [Question, Question] = [QUESTION_1, QUESTION_2];

beforeEach(() => {
  routes = {};
  stubFetch();
  window.history.replaceState(null, "", "/readings/atom-1");
  try {
    sessionStorage.clear();
  } catch {
    /* not relevant to what's under test */
  }
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  try {
    sessionStorage.clear();
  } catch {
    /* not relevant to what's under test */
  }
});

describe("ReadingQuestions", () => {
  it("renders nothing at all when the server returns no questions", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}/questions`)] = { body: { questions: [] } };
    const { container } = render(<ReadingQuestions readingId={READING_ID} />);

    await waitFor(() =>
      expect(calls.some((c) => c.url === `/api/v1/readings/${READING_ID}/questions`)).toBe(true),
    );
    expect(container.innerHTML).toBe("");
  });

  it("shows each question with the sentence it grew from", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}/questions`)] = {
      body: { questions: TWO_QUESTIONS },
    };
    render(<ReadingQuestions readingId={READING_ID} />);

    expect(await screen.findByText(QUESTION_1.text)).toBeTruthy();
    expect(screen.getByText(QUESTION_2.text)).toBeTruthy();
    expect(screen.getByText(`从这句想到的：${QUESTION_1.anchorQuote}`)).toBeTruthy();
    expect(screen.getByText(`从这句想到的：${QUESTION_2.anchorQuote}`)).toBeTruthy();
  });

  it("去写一写 stashes the question and navigates to /writings", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}/questions`)] = {
      body: { questions: TWO_QUESTIONS },
    };
    render(<ReadingQuestions readingId={READING_ID} />);

    await screen.findByText(QUESTION_1.text);
    const buttons = screen.getAllByRole("button", { name: "去写一写" });
    fireEvent.click(buttons[0]!);

    expect(sessionStorage.getItem(WRITING_IDEA_KEY)).toBe(QUESTION_1.text);
    expect(window.location.pathname).toBe("/writings");
  });

  it("never carries the article or her notes — only the question text", async () => {
    routes[key("GET", `/api/v1/readings/${READING_ID}/questions`)] = {
      body: { questions: TWO_QUESTIONS },
    };
    render(<ReadingQuestions readingId={READING_ID} />);

    await screen.findByText(QUESTION_2.text);
    const buttons = screen.getAllByRole("button", { name: "去写一写" });
    fireEvent.click(buttons[1]!);

    expect(sessionStorage.getItem(WRITING_IDEA_KEY)).toBe(QUESTION_2.text);
    expect(sessionStorage.length).toBe(1);
  });
});

describe("the writings landing picks the stashed idea up", () => {
  beforeEach(() => {
    routes[key("GET", "/api/v1/writings")] = { body: { writings: [] } };
    window.history.replaceState(null, "", "/writings");
  });

  it("exactly once", async () => {
    sessionStorage.setItem(WRITING_IDEA_KEY, "为什么太阳能装机量增长这么快，碳排放却没有跟着降下来？");
    render(<WritingsLanding />);

    const textarea = await screen.findByLabelText("想写点什么");
    await waitFor(() =>
      expect((textarea as HTMLTextAreaElement).value).toBe(
        "为什么太阳能装机量增长这么快，碳排放却没有跟着降下来？",
      ),
    );
    expect(sessionStorage.getItem(WRITING_IDEA_KEY)).toBeNull();
  });

  it("leaves the box empty when nothing was stashed", async () => {
    render(<WritingsLanding />);
    const textarea = await screen.findByLabelText("想写点什么");
    expect((textarea as HTMLTextAreaElement).value).toBe("");
  });
});
