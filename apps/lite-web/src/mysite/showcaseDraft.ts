/** Preserve the input string in the editor; normalize only the derived keyword list. */
export function parseShowcaseInterests(text: string): { interests: string[]; error: string } {
  const interests = [...new Set(text.split(/[,，、\n]/).map(value => value.trim()).filter(Boolean))];
  if (interests.length > 12) return { interests, error: "兴趣关键词最多 12 个，请减少后保存。" };
  if (interests.some(value => [...value].length > 60)) return { interests, error: "每个兴趣关键词最多 60 个字符，请缩短后保存。" };
  return { interests, error: "" };
}
