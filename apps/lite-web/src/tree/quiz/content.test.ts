import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";
import type { QuizResult } from "../../api/interestQuiz";
import { quizExitResult } from "./AwakeningQuiz";
import {
  CHALLENGE_OPTIONS,
  HOOKS,
  MAX_REASON,
  MAX_WORK,
  MISSION_STEPS,
  NAVIGATORS,
  PRINCIPLES,
  PRINCIPLE_REPLY,
  STEPS,
  missionIndex,
  nextStep,
  prevStep,
  type Step,
} from "./content";

// content.test —— 觉醒协议内容里「读代码看不出对错」的那几条。
//
// 这里不测屏幕长什么样。测的是四类会静默出错的东西：钩子 id 和后端对不对得上、
// 压力测试有没有恰好一个正确项、每个错误项有没有带上说明差在哪的那句话、
// 以及那台状态机走不走得到头。

describe("测试退出结果", () => {
  it("完成前退出不携带待定位词", () => {
    expect(quizExitResult(null)).toEqual({ grew: false, interestIds: [] });
  });

  it("结果页的所有退出入口都携带真正种下的闭表 id", () => {
    const result = {
      keywords: [
        { interestId: "climate", textZh: "气候", textEn: "Climate", field: "science", note: "", evidence: "气候变化" },
        { interestId: "", textZh: "", textEn: "", field: "self", note: "", evidence: "" },
      ],
    } as QuizResult;
    expect(quizExitResult(result)).toEqual({ grew: true, interestIds: ["climate"] });
  });

  it("顶栏与结果按钮都使用同一份退出结果", () => {
    const source = readFileSync(resolve(process.cwd(), "src/tree/quiz/AwakeningQuiz.tsx"), "utf8");
    expect(source.match(/onExit\(exitResult\)/g)).toHaveLength(2);
    expect(source).not.toContain("onExit({ grew: false, interestIds: [] })");
  });
});

describe("兴趣钩子", () => {
  // 🚨 这三个字符串是**跨语言的契约**：Go 侧 interest.Hook 只认这三个值，
  // 数据库 hook 列上还有 CHECK 约束。打错一个字母的后果是她选完钩子、走完
  // 全程、然后结果页一片透镜都没有——而且没有任何报错。
  it("id 与 Go 侧 interest.Hook 逐字一致", () => {
    expect(HOOKS.map((h) => h.key)).toEqual(["character", "craft", "society"]);
  });

  it("每个钩子都是一个问句——它问的是「哪种问题你想追下去」", () => {
    for (const h of HOOKS) {
      expect(h.question.endsWith("？")).toBe(true);
      expect(h.body.length).toBeGreaterThan(6);
    }
  });
});

describe("反方压力测试", () => {
  // 恰好一个。零个 = 她永远过不去；两个 = 那道题什么也没测到。
  it("恰好有一个正确项", () => {
    expect(CHALLENGE_OPTIONS.filter((o) => o.correct)).toHaveLength(1);
  });

  // 一句「再试一次」教不会任何人。每个错误项都要说清楚它差在哪——这道题的
  // 全部意义就在这几句话里。
  it("每个错误项都带一句说明差在哪的话", () => {
    for (const o of CHALLENGE_OPTIONS.filter((o) => !o.correct)) {
      expect(o.feedback.length).toBeGreaterThan(12);
    }
  });

  it("正确项不需要纠正的话", () => {
    expect(CHALLENGE_OPTIONS.find((o) => o.correct)!.feedback).toBe("");
  });

  it("选项 key 不重复——它要被存进 challenge_choice", () => {
    expect(new Set(CHALLENGE_OPTIONS.map((o) => o.key)).size).toBe(CHALLENGE_OPTIONS.length);
  });
});

describe("行动原则", () => {
  it("每一条都有对应的回应——包括「把大脑交出去」那一条", () => {
    for (const p of PRINCIPLES) {
      expect(PRINCIPLE_REPLY[p.key]).toBeTruthy();
    }
  });

  // 选了「交给 AI」不是失败。这一屏是提醒，不是筛选——一个被判定为失败的
  // 开场，会让本来最该留下的那个学生直接关掉。
  it("有且只有一条被标成 risky，而两条都能继续", () => {
    expect(PRINCIPLES.filter((p) => p.risky)).toHaveLength(1);
    expect(PRINCIPLES.length).toBeGreaterThanOrEqual(2);
  });
});

describe("导航员", () => {
  it("三个，名字各不相同——名字会被原样存进库里", () => {
    expect(NAVIGATORS).toHaveLength(3);
    expect(new Set(NAVIGATORS.map((n) => n.name)).size).toBe(3);
  });

  it("每个都有一句自己的话", () => {
    for (const n of NAVIGATORS) {
      expect(n.quote.length).toBeGreaterThan(4);
      expect(n.accent).toMatch(/^#[0-9a-f]{6}$/i);
    }
  });
});

describe("长度上限", () => {
  // 与服务端 interest.maxWorkRunes / maxReasonRunes 对齐。前端写大了，她就会
  // 眼看着自己写的一段话在结果里被默默截短。
  it("与服务端一致", () => {
    expect(MAX_WORK).toBe(40);
    expect(MAX_REASON).toBe(180);
  });
});

describe("状态机", () => {
  it("七屏，boot 开头 result 结尾", () => {
    expect(STEPS[0]).toBe("boot");
    expect(STEPS[STEPS.length - 1]).toBe("result");
    expect(STEPS).toHaveLength(7);
  });

  it("从头一路 next 走得到结果页", () => {
    let s: Step = STEPS[0]!;
    for (let i = 0; i < 20 && s !== "result"; i += 1) s = nextStep(s);
    expect(s).toBe("result");
  });

  it("结果页再 next 停在原地，不绕回开头", () => {
    expect(nextStep("result")).toBe("result");
  });

  it("第一屏再 prev 停在原地", () => {
    expect(prevStep("boot")).toBe("boot");
  });

  it("进度条只数三个 MISSION，序章与结果不在其中", () => {
    expect(MISSION_STEPS).toEqual(["interest", "lens", "challenge"]);
    expect(missionIndex("interest")).toBe(0);
    expect(missionIndex("challenge")).toBe(2);
    expect(missionIndex("boot")).toBe(-1);
    expect(missionIndex("result")).toBe(-1);
  });
});
