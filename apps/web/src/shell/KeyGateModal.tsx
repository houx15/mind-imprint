import type { LlmConfig, ChatRequest, ChatResult } from "../llm/types";
import { LlmConfigForm } from "./settings/LlmConfigForm";

export type ChatFn = (config: Partial<LlmConfig>, req: ChatRequest) => Promise<ChatResult>;

export function KeyGateModal({
  chat,
  onPass,
}: {
  chat?: ChatFn;
  onPass: () => void;
}) {
  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(20,30,60,.45)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 50,
      }}
      onClick={() => {
        /* non-dismissable: scrim click does nothing */
      }}
    >
      <div
        style={{
          background: "#fff",
          borderRadius: 18,
          padding: "36px 40px",
          maxWidth: 560,
          width: "100%",
          boxShadow: "0 8px 48px rgba(20,30,60,.18)",
          display: "flex",
          flexDirection: "column",
          gap: 16,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2
          style={{
            margin: 0,
            fontSize: 22,
            fontWeight: 700,
            color: "#141E3C",
          }}
        >
          先连接你的模型
        </h2>
        <p
          style={{
            margin: 0,
            fontSize: 14,
            color: "#4B5168",
            lineHeight: 1.6,
          }}
        >
          思维印记需要你自己的模型 API Key 才能开始。它只存在你的浏览器里，不上传任何服务器。
        </p>
        <LlmConfigForm chat={chat} onVerified={onPass} />
      </div>
    </div>
  );
}
