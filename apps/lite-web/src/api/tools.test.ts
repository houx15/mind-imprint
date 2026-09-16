import {expect,it} from 'vitest';
import {toolsForSession,type ToolInstance} from './tools';
it('keeps invitations and accepted work in their originating discussion without dropping main legacy tools',()=>{
 const make=(id:string,sessionId?:string|null)=>({id,sessionId,status:'summoned'} as ToolInstance);
 const tools=[make('legacy'),make('main',null),make('a','session-a'),make('b','session-b')];
 expect(toolsForSession(tools,null).map(t=>t.id)).toEqual(['legacy','main']);
 expect(toolsForSession(tools,'session-a').map(t=>t.id)).toEqual(['a']);
 expect(toolsForSession(tools,'new-session')).toEqual([]);
 expect(tools).toHaveLength(4);
});
