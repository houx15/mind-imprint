import { test, expect } from "@playwright/test";
import { login, PHOEBE } from "./helpers";

// J-voice — the Volcano TTS path (course 朗读本节 narration + studio dictation
// share this endpoint). Live: signs in, then POSTs /voice/tts and asserts real
// MP3 audio comes back. Config-gated: when the API has no Volcano creds
// (VOICE_APP_ID/VOICE_ACCESS_KEY unset) the endpoint returns 503 and this test
// SKIPS, so the suite still passes in environments without voice provisioned.
// The correct seed-tts-2.0 speaker (VOICE_TTS_VOICE) must be one activated on
// the Volcano app — see docs/2026-07-25-e2e-findings.md.
//
// ASR (WebSocket + live microphone PCM) is not covered here: driving real audio
// through getUserMedia → AudioWorklet in headless Chromium needs a fake-audio
// harness; documented as a manual/keyed step in the findings.
const API = "http://localhost:8080/api/v1";

test("J-voice: TTS returns real MP3 audio (skips if voice unconfigured)", async ({ page }) => {
  test.setTimeout(60_000);
  await login(page, PHOEBE.email, PHOEBE.password);

  const res = await page.request.post(`${API}/voice/tts`, {
    data: { text: "你好，这是一次语音合成测试。", speed: 1.0 },
    timeout: 30_000,
  });

  // 503 ⇒ the server has no Volcano credentials wired; nothing to assert.
  test.skip(res.status() === 503, "voice not configured on this API (no Volcano creds)");

  expect(res.status()).toBe(200);
  expect(res.headers()["content-type"]).toContain("audio/mpeg");
  const body = await res.body();
  // A real synthesis is well over a KB; an ID3 tag or MPEG frame-sync leads it.
  expect(body.length).toBeGreaterThan(1000);
  const isID3 = body[0] === 0x49 && body[1] === 0x44 && body[2] === 0x33; // "ID3"
  const isFrameSync = body[0] === 0xff && (body[1]! & 0xe0) === 0xe0;
  expect(isID3 || isFrameSync).toBeTruthy();
});
