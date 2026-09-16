export interface ArtifactTrial {
  mode: "self" | "other" | "not_tested";
  version: string;
  task: string;
  expected: string;
  actual: string;
  next: string;
}

export function emptyArtifactTrial(): ArtifactTrial {
  return { mode: "self", version: "", task: "", expected: "", actual: "", next: "" };
}

export interface ArtifactTrialDraft { document: ArtifactTrial; revision: number }
