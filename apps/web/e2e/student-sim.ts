import { readFileSync } from "node:fs";
import { join } from "node:path";

// Test-only LLM "student simulator": reads the coach's latest message and
// returns ONE short, concrete, on-topic Chinese reply that genuinely engages,
// so the course/project coach's engagement gate (soft_condition) is satisfied
// and the conversation can progress. This is HARNESS code — it drives the
// student side of a live conversation the way a real student would. It is NOT
// the client app (the app still never calls a model directly); it calls
// DeepSeek from the test process to play the human.

const DEEPSEEK_URL = "https://api.deepseek.com/v1/chat/completions";
const MODEL = "deepseek-v4-pro";

let cachedKey: string | null = null;
function deepseekKey(): string {
  if (cachedKey) return cachedKey;
  // playwright runs with cwd = apps/web ⇒ ../api/.env.local
  const envPath = join(process.cwd(), "..", "api", ".env.local");
  const raw = readFileSync(envPath, "utf8");
  const m = raw.match(/^DEEPSEEK_API_KEY=(.+)$/m);
  if (!m) throw new Error("DEEPSEEK_API_KEY not found in apps/api/.env.local");
  cachedKey = m[1].trim();
  return cachedKey;
}

const SYSTEM = [
  "你在扮演一个配合、认真、学得很快的中学生，正在上一门批判性思维课，和 AI 老师一对一对话。",
  "老师刚对你说了下面这段话。请用一到两句自然、具体的中文话直接回应老师的要求。",
  "要点：直接给出老师想要的答案；绝不回避、不要只空谈方法、不要重复讲同一个故事；",
  "如果老师要你说出某个词，就说那个词；如果老师让你举例，就给一个具体的小例子。",
  "重要：你是来把这一节学完并往下走的。当你已经回答了老师的问题、或已经明白他讲的道理时，",
  "要明确说出你的理解并表示你准备好继续了（例如「我明白了，随手一信的风险就是容易被误导、白花钱，我记住要先停一下核实。我准备好继续下一部分了」），",
  "而不是不停地反问。口语、简短。",
].join("");

// One student turn. Retries once on transient failure; falls back to a safe
// cooperative line so a single hiccup never sinks a whole journey.
export async function studentReply(coachText: string): Promise<string> {
  const body = JSON.stringify({
    model: MODEL,
    messages: [
      { role: "system", content: SYSTEM },
      { role: "user", content: coachText || "请继续。" },
    ],
    stream: false,
    max_tokens: 200,
    temperature: 0.7,
  });
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      const res = await fetch(DEEPSEEK_URL, {
        method: "POST",
        headers: { "content-type": "application/json", authorization: `Bearer ${deepseekKey()}` },
        body,
      });
      if (!res.ok) throw new Error(`deepseek ${res.status}`);
      const data = (await res.json()) as { choices?: { message?: { content?: string } }[] };
      const text = (data.choices?.[0]?.message?.content ?? "").trim();
      if (text) return text.replace(/\s+/g, " ").slice(0, 400);
    } catch {
      // fall through to retry
    }
  }
  return "好的，我明白你的意思了，我来试着回答。";
}
