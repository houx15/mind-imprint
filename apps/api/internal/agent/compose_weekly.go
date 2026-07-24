package agent

// WeeklyFacts is the deterministic fact sheet the weekly-report composer is
// allowed to see. teacher.BuildWeeklyFacts (internal/teacher/weekly.go) is the
// only producer: every name, tag, and number in here was decided by the rule
// layer, never by the model that will later turn it into prose. Defined here
// (not in internal/teacher) because teacher imports agent, and the reverse
// would be an import cycle.
type WeeklyFacts struct {
	ClassName     string               `json:"className"`
	ClassSize     int                  `json:"classSize"`
	WeekLabel     string               `json:"weekLabel"`
	DepthBuckets  map[string]int       `json:"depthBuckets"`
	RatedCount    int                  `json:"ratedCount"`
	AutonomyMean  string               `json:"autonomyMean"`
	AutonomyDelta string               `json:"autonomyDelta"`
	BucketChanges []WeeklyBucketChange `json:"bucketChanges"`
	Cards         []WeeklyFactCard     `json:"cards"`
}

// WeeklyBucketChange is one student's depth-bucket move this week (e.g.
// L2 → L3), used to narrate the class distribution's movement.
type WeeklyBucketChange struct {
	Name string `json:"name"`
	From string `json:"from"`
	To   string `json:"to"`
}

// WeeklyFactCard is one 值得表扬 / 需要建议 card as the rule layer produced it:
// the student, the tag, and the verbatim evidence quote.
type WeeklyFactCard struct {
	UserID   string `json:"userId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	TagCode  string `json:"tagCode"`
	TagLabel string `json:"tagLabel"`
	Evidence string `json:"evidence"`
}
