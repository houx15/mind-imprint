import {expect,it} from 'vitest';
import type {Artifact} from './artifacts';
import {artifactMetadataChanges} from './artifactMetadataChanges';
const source={id:'source',payload:{body:'原文'},guessed:['未经验证的假设'],admits:['旧局限']} as Artifact;
it('compares real linked versions, including deletions, and ignores forged before-values',()=>{
 const current={...source,id:'new',payload:{body:'原文',baseArtifactId:'source',previousAdmits:['伪造原文']},admits:[]};
 expect(artifactMetadataChanges(current,source)).toEqual([{key:'admits',label:'待核实与局限',before:['旧局限'],after:[]}]);
 expect(artifactMetadataChanges(current)).toEqual([]);
 expect(artifactMetadataChanges(current,{...source,id:'unrelated'})).toEqual([]);
 expect(artifactMetadataChanges({...current,admits:source.admits},source)).toEqual([]);
});
it('supports full revisions and preserves the order of claims',()=>{
 const current={...source,id:'new',payload:{replacesArtifactId:'source'},guessed:['第二项','第一项']};
 expect(artifactMetadataChanges(current,source)[0]?.after).toEqual(['第二项','第一项']);
});
