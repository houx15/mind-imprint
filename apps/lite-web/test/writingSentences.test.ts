import { describe, expect, it } from "vitest";
import { ROLE_BOARD_MAX, sentencesForBoard, splitSentences } from "../src/writings/sentences";

/**
 * 拆句是标注板的地基：板上的每一张卡片都是这个函数切出来的一句话。
 * 切错不会报错，只会让板上多一张两个字的卡、或者两句话挤在一起——
 * 那种毛病只有人眼能发现，所以这里逐条钉住。
 */
describe("splitSentences", () => {
  it("按中文句末标点断句，并保留标点", () => {
    expect(splitSentences("食堂每天倒掉很多饭。我觉得这件事值得写。")).toEqual([
      "食堂每天倒掉很多饭。",
      "我觉得这件事值得写。",
    ]);
  });

  it("问号、感叹号、分号都算句末", () => {
    expect(splitSentences("这算浪费吗？我觉得算！而且不止一点；很多。")).toEqual([
      "这算浪费吗？",
      "我觉得算！",
      "而且不止一点；",
      "很多。",
    ]);
  });

  it("连续的句末标点一起收尾", () => {
    expect(splitSentences("真的吗？！我不信。")).toEqual(["真的吗？！", "我不信。"]);
  });

  it("收尾的右引号跟着上一句走", () => {
    expect(splitSentences("他说：「我不去。」然后就走了。")).toEqual([
      "他说：「我不去。」",
      "然后就走了。",
    ]);
  });

  // 🚨 这一条是承重的：逗号不是句末标点。
  // 「因为食堂每天倒掉很多饭，我觉得这件事值得写」是一个完整的因果，
  // 而标注板要她分辨的恰好是这种关系 —— 按逗号切开，那个关系就没了。
  it("不按逗号断句", () => {
    expect(splitSentences("因为食堂每天倒掉很多饭，我觉得这件事值得写。")).toEqual([
      "因为食堂每天倒掉很多饭，我觉得这件事值得写。",
    ]);
  });

  it("英文句号后面要有空白才算句末", () => {
    expect(splitSentences("Buses are cheap. They are sometimes slow.")).toEqual([
      "Buses are cheap.",
      "They are sometimes slow.",
    ]);
  });

  it("小数点不断句", () => {
    expect(splitSentences("The average was 3.5 kg. That is a lot.")).toEqual([
      "The average was 3.5 kg.",
      "That is a lot.",
    ]);
  });

  it("常见缩写不断句", () => {
    expect(splitSentences("Dr. Chen disagreed. So did I.")).toEqual([
      "Dr. Chen disagreed.",
      "So did I.",
    ]);
    expect(splitSentences("Many animals (cats, dogs, etc.) are affected.")).toEqual([
      "Many animals (cats, dogs, etc.) are affected.",
    ]);
  });

  it("网址里的点不断句", () => {
    expect(splitSentences("See nasa.gov for the data. It is public.")).toEqual([
      "See nasa.gov for the data.",
      "It is public.",
    ]);
  });

  it("换行也断句 —— 她按回车分开的两句，在她眼里本来就是两句", () => {
    expect(splitSentences("第一句没有标点\n第二句也没有")).toEqual(["第一句没有标点", "第二句也没有"]);
  });

  it("最后一句没有标点也要收进来", () => {
    expect(splitSentences("写完了。还有一句没打标点")).toEqual(["写完了。", "还有一句没打标点"]);
  });

  it("空白和空串不产生卡片", () => {
    expect(splitSentences("")).toEqual([]);
    expect(splitSentences("   \n  \n ")).toEqual([]);
    expect(splitSentences("。。。")).toEqual(["。。。"]);
  });
});

describe("sentencesForBoard", () => {
  it("编号从 1 起，给她看的就是这个号", () => {
    const { sentences } = sentencesForBoard("第一句。第二句。");
    expect(sentences.map((s) => s.index)).toEqual([1, 2]);
    expect(sentences[0]?.text).toBe("第一句。");
  });

  // 🚨 超出上限时**只取前几句，并且要报出来**。悄悄少几句会让她以为自己写的
  // 东西丢了 —— 那是这个产品最不能有的那种失败。
  it("超过上限只取前几句，并且报告被截断了", () => {
    const long = Array.from({ length: ROLE_BOARD_MAX + 3 }, (_, i) => `第${i + 1}句。`).join("");
    const { sentences, truncated } = sentencesForBoard(long);
    expect(sentences).toHaveLength(ROLE_BOARD_MAX);
    expect(truncated).toBe(true);
  });

  it("没超出就不报截断", () => {
    const { sentences, truncated } = sentencesForBoard("一句。两句。");
    expect(sentences).toHaveLength(2);
    expect(truncated).toBe(false);
  });
});
