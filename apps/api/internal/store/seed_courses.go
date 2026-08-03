package store

// seed_courses.go — Task 12: seeds the repo's two example courses (a-mid,
// b-mid) from the embedded course JSON (internal/store/seed/courses,
// package courses, embedded via courses.FS — see migration 0050's Task 2)
// into the course table. This is the same operation the admin publish path
// (internal/api/course_admin.go's postAdminUploadCourse) performs for one
// course pushed over HTTP; SeedCourses runs it for the two examples shipped
// in-repo, called from cmd/api/main.go's --migrate-up path right after
// RunMigrations succeeds — the deploy hook that used to apply the old raw-
// SQL course seed (migration 0012), now retired along with the phase-gated
// course model migration 0050 dropped.
//
// UpsertCourse is idempotent by slug (course.go's ON CONFLICT (slug) DO
// UPDATE), so re-running SeedCourses on every migrate-up is safe and keeps
// the two seeded courses in sync with whatever is committed to the repo —
// intended, not a bug: an example course's content can change between
// deploys and the seed step is how that reaches the database.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"mindimprint/api/internal/agent"
	"mindimprint/api/internal/cards"
	courses "mindimprint/api/internal/store/seed/courses"
	"mindimprint/api/internal/store/sqlc"
)

// courseSeed is one example course's catalog-card metadata plus the two
// embedded filenames it upserts from. Blurb/TimeLabel are authored here
// (courses.FS's structure JSON has no field for them — see
// coursestore.go's UpsertCourseInput doc); Title/StepCount are derived from
// the structure file itself so this list never has to duplicate them.
type courseSeed struct {
	Slug            string
	Branch          string
	Blurb           string
	TimeLabel       string
	CardIDs         []string
	StructureFile   string
	RenderCacheFile string
}

var courseSeeds = []courseSeed{
	{
		Slug:            "a-mid",
		Branch:          "A",
		Blurb:           "用 CRRAAB 六维框架，对机构、亲历者、专家三类信源做溯源体检。",
		TimeLabel:       "约 25 分钟",
		CardIDs:         []string{"craap"},
		StructureFile:   "a-mid.json",
		RenderCacheFile: "a-mid-render-cache.json",
	},
	{
		Slug:            "b-mid",
		Branch:          "B",
		Blurb:           "从一篇公众号推文出发，走完 SIFT 溯源、论证拆解到让步段写作的学术写作全流程。",
		TimeLabel:       "约 40 分钟",
		CardIDs:         []string{"sift", "craap", "argument-map", "concession", "metacognition"},
		StructureFile:   "b-mid.json",
		RenderCacheFile: "b-mid-render-cache.json",
	},
}

// courseSeedStructure is the narrow slice of a course's structure JSON this
// seed reads: title (for the catalog card) and step count (via len(Steps)).
// Everything else in the structure — purpose/teaching_flow/materials/… — is
// stored verbatim and never parsed here, per the border-validation rule
// course_admin.go and coursestore.go both follow.
type courseSeedStructure struct {
	Title string            `json:"title"`
	Steps []json.RawMessage `json:"steps"`
}

// SeedCourses upserts every entry in courseSeeds from its embedded JSON
// (courses.FS) into the course table, verifying every card_id resolves in
// the cards registry first (fails loudly, before touching the database, if
// one doesn't — a typo'd card id here would otherwise silently ship a
// course whose card summons never resolve). Returns the number of courses
// seeded (len(courseSeeds) on success) for the caller to log.
func SeedCourses(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	queries := sqlc.New(pool)
	agentStore := agent.NewSqlcAgentStore(queries, pool)

	for _, seed := range courseSeeds {
		for _, id := range seed.CardIDs {
			if _, ok := cards.ByID(id); !ok {
				return 0, fmt.Errorf("seed course %s: unknown card id %q", seed.Slug, id)
			}
		}

		raw, err := courses.FS.ReadFile(seed.StructureFile)
		if err != nil {
			return 0, fmt.Errorf("seed course %s: read %s: %w", seed.Slug, seed.StructureFile, err)
		}
		renderCache, err := courses.FS.ReadFile(seed.RenderCacheFile)
		if err != nil {
			return 0, fmt.Errorf("seed course %s: read %s: %w", seed.Slug, seed.RenderCacheFile, err)
		}

		var structure courseSeedStructure
		if err := json.Unmarshal(raw, &structure); err != nil {
			return 0, fmt.Errorf("seed course %s: parse structure: %w", seed.Slug, err)
		}
		if structure.Title == "" {
			return 0, fmt.Errorf("seed course %s: structure has no title", seed.Slug)
		}

		if err := agentStore.UpsertCourse(ctx, agent.UpsertCourseInput{
			Slug:        seed.Slug,
			Branch:      seed.Branch,
			Title:       structure.Title,
			Blurb:       seed.Blurb,
			TimeLabel:   seed.TimeLabel,
			CardIDs:     seed.CardIDs,
			Structure:   raw,
			RenderCache: renderCache,
			StepCount:   len(structure.Steps),
		}); err != nil {
			return 0, fmt.Errorf("seed course %s: upsert: %w", seed.Slug, err)
		}
	}

	return len(courseSeeds), nil
}
