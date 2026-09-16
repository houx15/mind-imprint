import {describe,expect,it} from "vitest";
import {paperHTML,readPaperLayout,type PaperLayout} from "./paperLayout";
const layout:PaperLayout={format:"a4-portrait",title:"<img src=x>",notice:"未验证",elements:[{kind:"text",x:12,y:35,width:180,height:6,fontSize:3,text:'</text><script>alert(1)</script>'}]};
describe("inert printable paper",()=>{
 it("escapes model text rather than interpreting it as markup",()=>{
  const html=paperHTML(layout);
  expect(html).not.toContain("<script>");expect(html).not.toContain("<img src=x>");
  expect(html).toContain("&lt;script&gt;");expect(html).toContain("size:A4 portrait");
 });
 it("places labels above opaque shapes regardless of model order",()=>{
  const html=paperHTML({...layout,elements:[layout.elements[0]!,{kind:"rect",x:12,y:35,width:80,height:20,fill:"white"}]});
  expect(html.indexOf('width="80"')).toBeLessThan(html.indexOf('paint-order="stroke fill"'));
 });
 it("rejects unsupported fill values",()=>{
  expect(readPaperLayout({...layout,elements:[{...layout.elements[0],fill:"toString"}]})).toBeNull();
 });
 it("rejects nonfinite and off-page geometry",()=>{
  for(const x of [NaN,Infinity,199]) expect(readPaperLayout({...layout,elements:[{...layout.elements[0],x}]})).toBeNull();
 });
});
