package evalreport

type Ref struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	TS    string `json:"ts,omitempty"`
}

type EvidenceItem struct {
	ID          string `json:"id"`
	TS          string `json:"ts,omitempty"`
	Stage       string `json:"stage,omitempty"`
	Quote       string `json:"quote"`
	Observation string `json:"observation"`
	Boundary    string `json:"boundary,omitempty"`
}

type Milestones struct {
	Started           *string `json:"started"`
	FrameworkFinished *string `json:"frameworkFinished"`
	ProposalFinished  *string `json:"proposalFinished"`
	WritingFinished   *string `json:"writingFinished"`
	ProjectFinished   *string `json:"projectFinished"`
}

type Counters struct {
	AITurns        int `json:"aiTurns"`
	MaterialsRead  int `json:"materialsRead"`
	WordsWritten   int `json:"wordsWritten"`
	AICommentCount int `json:"aiCommentCount"`
	EditCount      int `json:"editCount"`
}

type Basics struct {
	Title      string     `json:"title"`
	Type       string     `json:"type"`
	StartDate  string     `json:"startDate"`
	EndDate    *string    `json:"endDate"`
	Milestones Milestones `json:"milestones"`
	Counters   Counters   `json:"counters"`
}

type RecommendedCourse struct {
	CourseID string `json:"courseId"`
	Reason   string `json:"reason"`
}

type Abstract struct {
	Overview            string              `json:"overview"`
	MaterialSentence    string              `json:"materialSentence"`
	WritingSentence     string              `json:"writingSentence"`
	AISentence          string              `json:"aiSentence"`
	SuggestionParagraph string              `json:"suggestionParagraph"`
	SuggestionSentences []string            `json:"suggestionSentences"`
	RecommendedCourses  []RecommendedCourse `json:"recommendedCourses"`
}

type EventEntry struct {
	TS      string `json:"ts"`
	Kind    string `json:"kind"`
	Summary string `json:"summary"`
	AITurns int    `json:"aiTurns"`
	Ref     *Ref   `json:"ref,omitempty"`
}

type MaterialEntry struct {
	MaterialID    string `json:"materialId"`
	AddedAt       string `json:"addedAt"`
	Source        string `json:"source"`
	URL           *string `json:"url"`
	UsedIn        *Ref   `json:"usedIn"`
	FinalStatus   string `json:"finalStatus"`
	Comment       string `json:"comment"`
	CannotSupport string `json:"cannotSupport"`
}

type DepthDimResult struct {
	ID         string         `json:"id"`
	Level      int            `json:"level"`
	Summary    string         `json:"summary"`
	Evidence   []EvidenceItem `json:"evidence"`
	Suggestion string         `json:"suggestion"`
}

type AutonomyDimResult struct {
	ID         string         `json:"id"`
	Band       int            `json:"band"`
	Summary    string         `json:"summary"`
	Evidence   []EvidenceItem `json:"evidence"`
	Suggestion string         `json:"suggestion"`
}

type PromptItem struct {
	Stage          string   `json:"stage"`
	Quote          string   `json:"quote"`
	Ref            Ref      `json:"ref"`
	Observation    string   `json:"observation"`
	RelatedDomains []string `json:"relatedDomains"`
	Attention      bool     `json:"attention"`
}

type PromptLens struct {
	Summary string       `json:"summary"`
	Prompts []PromptItem `json:"prompts"`
}

type ToolUsageEntry struct {
	ToolID  string `json:"toolId"`
	Name    string `json:"name"`
	Stage   string `json:"stage"`
	Purpose string `json:"purpose"`
	Summary string `json:"summary"`
}

type RiskEntry struct {
	Type      string `json:"type"`
	Behaviour string `json:"behaviour"`
	Ref       *Ref   `json:"ref,omitempty"`
	Suggestion string `json:"suggestion"`
}

type Report struct {
	Version   int    `json:"version"`
	ReportID  string `json:"reportId"`
	ProjectID string `json:"projectId"`
	Student   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"student"`
	Basics     Basics              `json:"basics"`
	Abstract   Abstract            `json:"abstract"`
	Events     []EventEntry        `json:"events"`
	Materials  []MaterialEntry     `json:"materials"`
	Depth      []DepthDimResult    `json:"depth"`
	Autonomy   []AutonomyDimResult `json:"autonomy"`
	PromptLens PromptLens          `json:"promptLens"`
	ToolUsage  []ToolUsageEntry    `json:"toolUsage"`
	Risks      []RiskEntry         `json:"risks"`
	GeneratedAt string             `json:"generatedAt"`
}
