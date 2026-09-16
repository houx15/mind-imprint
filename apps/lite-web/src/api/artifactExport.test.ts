import { describe, expect, it } from "vitest";
import type { Artifact } from "./artifacts";
import { codingAgentBrief, artifactEdits, artifactHTML, artifactMarkdown, reviewEntries } from "./artifactExport";
import type { ReviewPlan } from "./review";

const artifact: Artifact={id:"test",kind:"draft",title:"模拟展览",payload:{body:'【虚构记录】<script>alert("x")</script>\n未进行实地观察。'},guessed:["居民可能使用微信群，尚未证实。"],admits:["未完成访谈"],verdict:"revise",why:"保留模拟限定",settledAt:"2026-09-15T00:00:00Z",createdAt:"2026-09-15T00:00:00Z"};
describe("artifact export",()=>{
 it("exports material without appendices while retaining source qualifiers and review state",()=>{
  const review: ReviewPlan={marks:[],dimensions:[{id:"d",prompt:"检查",why:"",answer:"内部审核意见",ordinal:0}]};
  const edited={...artifact,payload:{...artifact.payload,baseArtifactId:"old",edits:[{old:"历史正文",new:"新正文"}]}};
  for (const output of [artifactMarkdown(edited,review,"material"),artifactHTML(edited,review,"material")]) {
   expect(output).toContain("模拟展览");
   expect(output).toContain("审核状态：待修改");
   expect(output).toContain("【虚构记录】");
   expect(output).toContain("未进行实地观察");
   for (const excluded of ["内部审核意见","历史正文","保留模拟限定","未完成访谈",artifact.guessed[0]]) expect(output).not.toContain(excluded);
  }
  expect(artifactMarkdown(edited,review)).toContain("内部审核意见");
  expect(artifactHTML(edited,review)).toContain("历史正文");
 });
 it("prints panels from the same source without rendering executable markup or a stale body",()=>{
  const panels=Array.from({length:6},(_,i)=>({title:`面${i+1}`,body:'待核对\n\n<script>alert(1)</script>'}));
  const html=artifactHTML({...artifact,payload:{body:"过时正文",printLayout:{format:"a4-accordion-six",panels}}});
  expect(html).not.toContain("过时正文");
  expect(html).not.toContain("<script>");
  expect(html.indexOf("面1")).toBeLessThan(html.indexOf("面6"));
  expect(html).toContain("size:A4 landscape");
  expect(html).toContain("背面留白");
 });
 it("preserves deletions in the change record and escapes replacement markup",()=>{
  const edited: Artifact={...artifact,payload:{body:"保留正文",baseArtifactId:"original",edits:[{old:"删除内容",new:""},{old:"原文",new:"<script>changed</script>"},null,{old:3,new:"无效"}]}};
  expect(artifactEdits(edited)).toHaveLength(2);
  expect(artifactMarkdown(edited)).toContain("修改后：（已删除）");
  expect(artifactHTML(edited)).toContain("&lt;script&gt;changed&lt;/script&gt;");
  expect(artifactHTML(edited)).not.toContain("<script>");
  expect(artifactEdits({...edited,payload:{edits:edited.payload.edits}})).toEqual([]);
 });
 it("renders an empty interview table without adding responses or executing markup",()=>{
  const html=artifactHTML({...artifact,payload:{body:'## 访谈记录\n\n尚未开展访谈。\n\n| 问题 | 受访者原话 | 访谈者理解 |\n| --- | --- | --- |\n| 第一个问题 | | |\n\n[危险链接](javascript:alert%281%29)\n\n![示意图](https://example.com/tracker.png)'}});
  expect(html).toContain('<h2>访谈记录</h2>');
  expect(html).toContain('<table>');
  expect(html.match(/<td><\/td>/g)).toHaveLength(2);
  expect(html).toContain('尚未开展访谈');
  expect(html).not.toContain('href="javascript:');
  expect(html).not.toContain('<img');
  expect(html).toContain('[图片：示意图]');
 });
 it("exports actual student judgments without treating unanswered questions as work",()=>{
  const review: ReviewPlan={marks:[{id:"m",part:"材料",partNote:"",quote:"虚构记录",question:"你标出来的地方",answer:"请保留虚构限定 <script>bad</script>",sessionId:null,ordinal:0,mine:true}],dimensions:[{id:"d",prompt:"调查方法",why:"",answer:"访谈也能收集第一手证据",ordinal:0},{id:"empty",prompt:"未回答的问题",why:"",answer:"",ordinal:1}]};
  const entries=reviewEntries(review);
  expect(entries).toHaveLength(2);
  expect(entries[0]?.source).toBe("学生标注");
  expect(artifactMarkdown(artifact,review)).toContain("访谈也能收集第一手证据");
  expect(artifactMarkdown(artifact,review)).not.toContain("未回答的问题");
  expect(artifactHTML(artifact,review)).toContain("请保留虚构限定 &lt;script&gt;bad&lt;/script&gt;");
  expect(artifactHTML(artifact,review)).not.toContain("<script>");
 });
 it("preserves evidence limits and actual review status",()=>{
  const md=artifactMarkdown(artifact);
  expect(md).toContain(artifact.payload.body);
  expect(md).toContain("审核状态：待修改");
  expect(md).toContain("未完成访谈");
  expect(md).toContain("保留模拟限定");
 });
 it("keeps user text inert in standalone HTML",()=>{
  const html=artifactHTML(artifact);
  expect(html).not.toContain("<script>");
  expect(html).toContain("&lt;script&gt;");
  expect(html).toContain("未进行实地观察");
 });
});


describe("coding handoff retains decision boundaries", () => {
 it("carries the exact source, review decisions and unanswered questions", () => {
  const review: ReviewPlan = { marks: [], dimensions: [
   {id:"open",prompt:"How will touch work?",why:"",answer:"",ordinal:0},
   {id:"answered",prompt:"Need sign in?",why:"",answer:"No sign in",ordinal:1},
  ] };
  const spec: Artifact = {...artifact, kind:"spec"};
  const brief = codingAgentBrief(spec, review);
  expect(brief).toContain(spec.payload.body);
  expect(brief).toContain(spec.id);
  expect(brief).toContain(spec.createdAt);
  expect(codingAgentBrief({...spec,id:"different-source-version"},review)).not.toBe(brief);
  expect(brief).toContain(spec.admits[0]);
  expect(brief).toContain(spec.guessed[0]);
  expect(brief).toContain("审核状态：待修改");
  expect(brief).toContain("How will touch work?");
  expect(brief).toContain("No sign in");
  expect(brief).toContain("未测试");
 });
 it("rejects a missing specification instead of exporting an empty build request", () => {
  const review: ReviewPlan = {marks:[],dimensions:[]};
  expect(() => codingAgentBrief(artifact, review)).toThrow();
  expect(() => codingAgentBrief({...artifact, kind:"spec", payload:{body:" "}}, review)).toThrow();
 });
});
