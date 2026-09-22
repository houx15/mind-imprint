export interface ShowcaseInterestBranch {
  label: string;
  keywords: string[];
}

export interface ShowcaseInterestSnapshot {
  mode: "tree" | "keywords";
  title?: string;
  keywords: string[];
  branches?: ShowcaseInterestBranch[];
}

export interface ShowcaseInterestTreeProps {
  interest?: ShowcaseInterestSnapshot;
}

function nonEmpty(values: string[]): string[] {
  return values.map((value) => value.trim()).filter(Boolean);
}

export function ShowcaseInterestTree({ interest }: ShowcaseInterestTreeProps) {
  if (!interest) return null;

  const title = interest.title?.trim() || "兴趣";
  const keywords = nonEmpty(interest.keywords);

  if (interest.mode === "keywords") {
    if (keywords.length === 0) return null;
    return (
      <section className="showcase-interest showcase-interest-keywords" aria-labelledby="showcase-interest-title">
        <p className="showcase-interest-label" id="showcase-interest-title">{title}</p>
        <ul>{keywords.map((keyword, index) => <li key={`${keyword}-${index}`}>{keyword}</li>)}</ul>
      </section>
    );
  }

  const branches = (interest.branches ?? [])
    .map((branch) => ({ label: branch.label.trim(), keywords: nonEmpty(branch.keywords) }))
    .filter((branch) => branch.label && branch.keywords.length > 0);
  if (branches.length === 0 && keywords.length === 0) return null;

  return (
    <section className="showcase-interest showcase-interest-tree" aria-labelledby="showcase-interest-title">
      <div className="showcase-interest-tree-heading">
        <p className="showcase-interest-label" id="showcase-interest-title">{title}</p>
        {keywords.length > 0 && <ul className="showcase-interest-roots" aria-label="兴趣关键词">{keywords.map((keyword, index) => <li key={`${keyword}-${index}`}>{keyword}</li>)}</ul>}
      </div>
      {branches.length > 0 && (
        <div className="showcase-interest-branches">
          <svg className="showcase-interest-lines" viewBox="0 0 100 46" preserveAspectRatio="none" aria-hidden="true">
            <path d="M50 0v14" />
            <path d="M8 40C22 21 37 17 50 14C63 17 78 21 92 40" />
          </svg>
          <ul className="showcase-interest-branch-list">
            {branches.map((branch, branchIndex) => (
              <li className="showcase-interest-branch" key={`${branch.label}-${branchIndex}`}>
                <h3>{branch.label}</h3>
                <ul>{branch.keywords.map((keyword, keywordIndex) => <li key={`${keyword}-${keywordIndex}`}>{keyword}</li>)}</ul>
              </li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
