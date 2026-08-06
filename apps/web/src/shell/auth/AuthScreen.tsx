import { useState } from "react";
import { api, ApiError, type ApiClient, type MeUser } from "../../api";
import { Button, Illustration, Input, Pebble } from "@/ui";

/**
 * AuthScreen — platform shell rebuild (Task 10, restyle only).
 *
 * The 3-step state machine (login / register / bind) and every API call are
 * untouched from the pre-restyle version — only the markup and styling
 * changed, from inline `style={}` objects to `ui/` primitives + `mk-*`
 * tokens.
 */

type AuthClient = Pick<ApiClient, "signin" | "signup">;

function messageFor(e: unknown): string {
  if (e instanceof ApiError) return e.message;
  return "网络错误，请稍后再试";
}

function FieldLabel({ htmlFor, children }: { htmlFor: string; children: string }) {
  return (
    <label htmlFor={htmlFor} className="mb-1 block text-mk-small font-medium text-mk-secondary">
      {children}
    </label>
  );
}

export function AuthScreen({
  onAuthed,
  client = api,
}: {
  onAuthed: (user: MeUser) => void;
  client?: AuthClient;
}) {
  const [step, setStep] = useState<"login" | "register" | "bind">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [joinCode, setJoinCode] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function doLogin() {
    setErr(null);
    setBusy(true);
    try {
      onAuthed(await client.signin({ email, password }));
    } catch (e) {
      setErr(messageFor(e));
    } finally {
      setBusy(false);
    }
  }

  async function doRegister() {
    setErr(null);
    setBusy(true);
    try {
      await client.signup({ email, password, display_name: displayName, join_code: joinCode });
      onAuthed(await client.signin({ email, password }));
    } catch (e) {
      setErr(messageFor(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex h-full w-full items-center justify-center overflow-y-auto bg-mk-paper px-4 py-8">
      <div className="flex w-full max-w-[880px] flex-col overflow-hidden rounded-mk-lg shadow-mk-lg md:flex-row">
        {/* brand panel */}
        <div className="flex shrink-0 flex-col items-center justify-center gap-4 bg-mk-accent-50 px-8 py-10 text-center md:w-[320px]">
          <div className="flex items-center gap-2">
            <Pebble state="idle" size={36} />
            <span className="text-mk-h2 text-mk-ink">思维印记</span>
          </div>
          <Illustration name="bookLover" className="h-[150px] w-[150px]" alt="" />
          <p className="text-mk-body text-mk-secondary">
            带上你手头真实的功课，我们陪你把思考往深处走一走。
          </p>
        </div>

        {/* form panel */}
        <div className="flex flex-1 items-center justify-center bg-mk-surface px-6 py-10 sm:px-10">
          <div className="w-full max-w-[340px]">
            {step === "login" && (
              <>
                <h1 className="mb-5 text-mk-h1 text-mk-ink">登录</h1>
                <div className="mb-3">
                  <FieldLabel htmlFor="auth-email">邮箱 / 手机号</FieldLabel>
                  <Input id="auth-email" value={email} onChange={setEmail} />
                </div>
                <div className="mb-5">
                  <FieldLabel htmlFor="auth-password">密码</FieldLabel>
                  <Input
                    id="auth-password"
                    type="password"
                    value={password}
                    onChange={setPassword}
                    error={err ?? undefined}
                  />
                </div>
                <Button variant="primary" className="w-full" loading={busy} onClick={doLogin}>
                  登录
                </Button>
                <div className="mt-4 text-center text-mk-small text-mk-muted">
                  还没有账号？
                  <Button
                    variant="link"
                    size="sm"
                    className="ml-1 inline"
                    onClick={() => {
                      setStep("register");
                      setErr(null);
                    }}
                  >
                    注册
                  </Button>
                </div>
              </>
            )}

            {step === "register" && (
              <>
                <h1 className="mb-5 text-mk-h1 text-mk-ink">创建账号</h1>
                <div className="mb-3">
                  <FieldLabel htmlFor="auth-display-name">姓名</FieldLabel>
                  <Input id="auth-display-name" value={displayName} onChange={setDisplayName} />
                </div>
                <div className="mb-3">
                  <FieldLabel htmlFor="auth-reg-email">邮箱</FieldLabel>
                  <Input id="auth-reg-email" value={email} onChange={setEmail} />
                </div>
                <div className="mb-5">
                  <FieldLabel htmlFor="auth-reg-password">设置密码</FieldLabel>
                  <Input id="auth-reg-password" type="password" value={password} onChange={setPassword} />
                </div>
                <Button
                  variant="primary"
                  className="w-full"
                  onClick={() => {
                    setErr(null);
                    setStep("bind");
                  }}
                >
                  下一步 · 绑定班级
                </Button>
                <div className="mt-4 text-center text-mk-small text-mk-muted">
                  已有账号？
                  <Button
                    variant="link"
                    size="sm"
                    className="ml-1 inline"
                    onClick={() => {
                      setStep("login");
                      setErr(null);
                    }}
                  >
                    登录
                  </Button>
                </div>
              </>
            )}

            {step === "bind" && (
              <>
                <h1 className="mb-1 text-mk-h1 text-mk-ink">绑定你的班级</h1>
                <p className="mb-5 text-mk-small leading-relaxed text-mk-muted">
                  绑定后，老师布置的探究任务会出现在你的任务列表里。你的思考过程只属于你，老师看不到对话本身。
                </p>
                <div className="mb-5">
                  <FieldLabel htmlFor="auth-join-code">班级邀请码</FieldLabel>
                  <Input id="auth-join-code" value={joinCode} onChange={setJoinCode} error={err ?? undefined} />
                </div>
                <Button variant="primary" className="w-full" loading={busy} onClick={doRegister}>
                  完成，进入思维印记
                </Button>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
