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

// goldenCourseFile is the embedded golden CourseDefinition 2.0 course (Course
// Runtime Slice 8) — the coverage course copied from the contract test fixture.
// Seeded as ONE course row whose course_definition is the whole document, so the
// student platform has a real 2.0 course to route through the new runtime player
// before the (out-of-scope) generator emits 2.0 courses. Its media assets are
// placeholder paths (no real OSS files) — accepted for this plumbing slice.
const goldenCourseFile = "coverage-course.json"

// goldenCourseDoc is the narrow border slice of the golden 2.0 document: the
// schemaVersion (must be "2.0"), the course.id/title/estimatedMinutes used for
// the catalog row + slug, and parts[].slices[] — counted (shallow) for the
// catalog step_count (one slice = one step). The whole document is stored verbatim.
type goldenCourseDoc struct {
	SchemaVersion string `json:"schemaVersion"`
	Course        struct {
		ID               string `json:"id"`
		Title            string `json:"title"`
		EstimatedMinutes int    `json:"estimatedMinutes"`
		Parts            []struct {
			Slices []json.RawMessage `json:"slices"`
		} `json:"parts"`
	} `json:"course"`
}

// goldenStepCount sums the slices across the golden course's parts — the same
// "one slice = one step" rule the authoring publish path uses (courseStepCount
// in internal/api). Without this the seed loader re-upserts the golden course
// with step_count 0 on every boot, overwriting any backfill and re-introducing
// the "0 步" catalog bug for the seeded 2.0 course.
func goldenStepCount(doc goldenCourseDoc) int {
	n := 0
	for _, part := range doc.Course.Parts {
		n += len(part.Slices)
	}
	return n
}

// seedGoldenCourse upserts the golden 2.0 course as one course row (bare catalog
// fields + empty legacy structure/render_cache) and attaches its CourseDefinition
// 2.0 document. Border-validated (schemaVersion=="2.0", non-empty course.id)
// before writing so a malformed fixture fails the deploy loudly, not silently.
// Slug = the definition's course.id (stable, matches the document).
// goldenCourseStore is the narrow store surface seedGoldenCourse needs; the
// unexported *sqlcAgentStore returned by agent.NewSqlcAgentStore satisfies it
// (its concrete type can't be named from this package, so we take an interface).
type goldenCourseStore interface {
	UpsertCourse(ctx context.Context, in agent.UpsertCourseInput) error
	SetCourseDefinition(ctx context.Context, slug string, def []byte) error
}

func seedGoldenCourse(ctx context.Context, agentStore goldenCourseStore) error {
	raw, err := courses.FS.ReadFile(goldenCourseFile)
	if err != nil {
		return fmt.Errorf("seed golden course: read %s: %w", goldenCourseFile, err)
	}
	var doc goldenCourseDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("seed golden course: parse: %w", err)
	}
	if doc.SchemaVersion != "2.0" {
		return fmt.Errorf("seed golden course: schemaVersion = %q, want 2.0", doc.SchemaVersion)
	}
	if doc.Course.ID == "" || doc.Course.Title == "" {
		return fmt.Errorf("seed golden course: document has no course.id/title")
	}
	slug := doc.Course.ID
	timeLabel := fmt.Sprintf("约 %d 分钟", doc.Course.EstimatedMinutes)
	if err := agentStore.UpsertCourse(ctx, agent.UpsertCourseInput{
		Slug:        slug,
		Branch:      "Runtime",
		Title:       doc.Course.Title,
		Blurb:       "运行时 2.0 示例课程：判断两个论断能否直接比较。",
		TimeLabel:   timeLabel,
		CardIDs:     []string{},
		Structure:   []byte("{}"),
		RenderCache: []byte("{}"),
		StepCount:   goldenStepCount(doc),
	}); err != nil {
		return fmt.Errorf("seed golden course: upsert: %w", err)
	}
	if err := agentStore.SetCourseDefinition(ctx, slug, raw); err != nil {
		return fmt.Errorf("seed golden course: set definition: %w", err)
	}
	return nil
}

// SeedCourses upserts every entry in courseSeeds from its embedded JSON
// (courses.FS) into the course table, verifying every card_id resolves in
// the cards registry first (fails loudly, before touching the database, if
// one doesn't — a typo'd card id here would otherwise silently ship a
// course whose card summons never resolve). Returns the number of courses
// seeded (len(courseSeeds) on success) for the caller to log.
//
// synth/audioStore (course voice narration, Task 4) are passed straight
// through to agent.GenerateCourseAudio for each seeded course, so the same
// deploy step (cmd/api/main.go's --migrate-up path) that (re)seeds a-mid/
// b-mid also (re)generates their narration manifest. This package already
// imports agent (agentStore below), and agent never imports store, so
// widening this signature to agent's own narrow CourseAudioSynth/
// CourseAudioStore interfaces (rather than a callback) introduces no import
// cycle. Either argument may be nil (voice/OSS not configured, or a caller —
// e.g. tests — that doesn't care about narration): GenerateCourseAudio
// degrades to an empty manifest in that case, and seeding proceeds exactly
// as before this task.
func SeedCourses(ctx context.Context, pool *pgxpool.Pool, synth agent.CourseAudioSynth, audioStore agent.CourseAudioStore) (int, error) {
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

		audioManifest, err := agent.GenerateCourseAudio(ctx, synth, audioStore, seed.Slug, renderCache)
		if err != nil {
			return 0, fmt.Errorf("seed course %s: generate audio: %w", seed.Slug, err)
		}

		if err := agentStore.UpsertCourse(ctx, agent.UpsertCourseInput{
			Slug:          seed.Slug,
			Branch:        seed.Branch,
			Title:         structure.Title,
			Blurb:         seed.Blurb,
			TimeLabel:     seed.TimeLabel,
			CardIDs:       seed.CardIDs,
			Structure:     raw,
			RenderCache:   renderCache,
			StepCount:     len(structure.Steps),
			AudioManifest: audioManifest,
		}); err != nil {
			return 0, fmt.Errorf("seed course %s: upsert: %w", seed.Slug, err)
		}
	}

	// Course Runtime Slice 8: also seed the golden CourseDefinition 2.0 course so
	// the student platform has one 2.0 course to route through the new runtime
	// player (legacy render_cache courses above keep the old player).
	if err := seedGoldenCourse(ctx, agentStore); err != nil {
		return 0, err
	}

	return len(courseSeeds) + 1, nil
}
