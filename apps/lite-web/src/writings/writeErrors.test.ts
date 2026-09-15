import { describe, expect, it, vi } from "vitest";
import { ApiError } from "../api/client";
import { apiErrorText } from "../api/errorText";
import { handleWriteError } from "./writeErrors";

describe("handleWriteError", () => {
  it("reloads on 403 writing_locked and shows nothing", () => {
    const onLocked = vi.fn();
    const setError = vi.fn();
    handleWriteError(new ApiError("writing_locked", "已过截止时间，作业已锁定", 403), onLocked, setError);
    expect(onLocked).toHaveBeenCalledTimes(1);
    expect(setError).not.toHaveBeenCalled();
  });

  // Another tab pressed 完成这篇 or 放弃修改: the writing is no longer being
  // revised, so this room is stale and every retry would fail the same way.
  it("reloads on 403 writing_finished too", () => {
    const onLocked = vi.fn();
    const setError = vi.fn();
    handleWriteError(new ApiError("writing_finished", "这篇已经完成", 403), onLocked, setError);
    expect(onLocked).toHaveBeenCalledTimes(1);
    expect(setError).not.toHaveBeenCalled();
  });

  it("shows the server's message for any other error", () => {
    const onLocked = vi.fn();
    const setError = vi.fn();
    const err = new ApiError("bad_request", "标题不能为空", 400);
    handleWriteError(err, onLocked, setError);
    expect(onLocked).not.toHaveBeenCalled();
    expect(setError).toHaveBeenCalledWith(apiErrorText(err));
  });

  it("does not reload on writing_finished at a status other than 403", () => {
    const onLocked = vi.fn();
    const setError = vi.fn();
    handleWriteError(new ApiError("writing_finished", "这篇已经完成", 409), onLocked, setError);
    expect(onLocked).not.toHaveBeenCalled();
    expect(setError).toHaveBeenCalledTimes(1);
  });

  // A caller with no reload path must still say something rather than drop
  // the error silently.
  it("shows the message when there is no onLocked", () => {
    const setError = vi.fn();
    const err = new ApiError("writing_locked", "已过截止时间，作业已锁定", 403);
    handleWriteError(err, undefined, setError);
    expect(setError).toHaveBeenCalledWith(apiErrorText(err));
  });
});
