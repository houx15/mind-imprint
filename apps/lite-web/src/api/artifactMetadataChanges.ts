import type {Artifact} from './artifacts';

/** Compare stored versions only; model-authored before-values are not evidence. */
export function artifactMetadataChanges(current: Artifact, previous?: Artifact) {
  const sourceId = current.payload.baseArtifactId ?? current.payload.replacesArtifactId;
  if (!previous || previous.id !== sourceId) return [];
  return ([['guessed', 'AI 标注的假设'], ['admits', '待核实与局限']] as const)
    .filter(([key]) => JSON.stringify(previous[key]) !== JSON.stringify(current[key]))
    .map(([key, label]) => ({key, label, before: previous[key], after: current[key]}));
}
