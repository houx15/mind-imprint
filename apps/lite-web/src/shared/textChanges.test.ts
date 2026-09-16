import { describe, expect, it } from 'vitest';
import { textChanges, remarkTextChanges, type TextRange } from './textChanges';
const retain = (s: string, ranges: TextRange[]) => ranges.reduceRight((value, r) => value.slice(0,r.start)+value.slice(r.end), s);
describe('text change matching', () => {
  it('locates Chinese edits while retaining shared Markdown and repeated text', () => {
    for (const [a,b] of [
      ['**提问**：请看厨房\n记录时间','**提问**：请找相关面\n记录时间'],
      ['记录记录甲记录','记录乙记录记录'], ['😀原方案','😀新方案'], ['', '新增'], ['删除',''], ['相同','相同'],
    ] as const) {
      const result=textChanges(a,b);
      expect(retain(a,result.before) || '').toBe(retain(b,result.after) || '');
      expect(result.before.every(r=>r.start>=0 && r.end<=a.length)).toBe(true);
      expect(result.after.every(r=>r.start>=0 && r.end<=b.length)).toBe(true);
    }
    expect(textChanges('不提示房间名','不提示面名')).toEqual({before:[{start:3,end:5}],after:[{start:3,end:4}]});
  });
  it('bounds large rewrites without losing shared edges', () => {
    const a='相同\n'+'旧'.repeat(2000)+'\n结尾', b='相同\n'+'新'.repeat(2000)+'\n结尾';
    const changes=textChanges(a,b);
    expect(retain(a,changes.before)).toBe('相同\n\n结尾');
    expect(retain(b,changes.after)).toBe('相同\n\n结尾');
  });
  it('decorates text without changing its content or Markdown structure', () => {
    const tree={type:'root',children:[{type:'paragraph',children:[{type:'text',value:'前旧后',position:{start:{offset:0},end:{offset:3}}}]}]};
    remarkTextChanges({source:'前旧后',ranges:[{start:1,end:2}],removed:true})(tree);
    expect(tree.children[0]!.children).toEqual([{type:'text',value:'前'},{type:'delete',data:{hName:'del'},children:[{type:'text',value:'旧'}]},{type:'text',value:'后'}]);
  });
});
