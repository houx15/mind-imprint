import {expect,it} from 'vitest';
import {pendingArtifacts,type Artifact} from './artifacts';
import {artifactStatus} from './artifactExport';
it('keeps superseded history outside pending work without rewriting old decisions',()=>{
 const old={id:'old',superseded:true,settledAt:null} as Artifact;
 const latest={id:'latest',settledAt:null} as Artifact;
 const reviewed={id:'reviewed',superseded:true,settledAt:'date',verdict:'revise'} as Artifact;
 expect(pendingArtifacts([old,latest,reviewed]).map(a=>a.id)).toEqual(['latest']);
 expect(artifactStatus(old)).toBe('已有新版');
 expect(artifactStatus(reviewed)).toBe('待修改');
});
