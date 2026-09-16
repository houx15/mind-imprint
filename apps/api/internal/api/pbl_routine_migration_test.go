package api_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestHomepageRoutineMigrationPreservesStudentPlans(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy_homepage_routine.json")
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		Summary, Reason string
		Steps           json.RawMessage
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../store/migrations/0167_homepage_creative_routine.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(migration), "-- +goose Down")[0]
	for _, scenario := range []string{"pristine", "approved", "edited", "progress", "newer"} {
		t.Run(scenario, func(t *testing.T) {
			h, c, _, pool := liteHandler(t)
			pid := decodePblProject(t, siteReq(t, h, c, "POST", "/api/v1/pbl/site/project", ""))["id"].(string)
			ctx := context.Background()
			var version string
			if err := pool.QueryRow(ctx, `SELECT id FROM pbl_plan_version WHERE atom_id=$1`, pid).Scan(&version); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE pbl_plan_version SET summary=$2,reason=$3 WHERE id=$1`, version, legacy.Summary, legacy.Reason); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `UPDATE pbl_plan_step p SET title=s.title,blurb=s.blurb,goal=s.goal,you_bring=s.you_bring,i_bring=s.i_bring,decide=s.decide,then_bring=s.then_bring,status=s.status FROM jsonb_to_recordset($2::jsonb) AS s(ordinal integer,title text,blurb text,goal text,you_bring text,i_bring text,decide text,then_bring text,status text) WHERE p.version_id=$1 AND p.ordinal=s.ordinal`, version, string(legacy.Steps)); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "approved":
				_, err = pool.Exec(ctx, `UPDATE pbl_plan_version SET approved_at=now() WHERE id=$1`, version)
			case "edited":
				_, err = pool.Exec(ctx, `UPDATE pbl_plan_step SET you_bring='student custom material' WHERE version_id=$1 AND ordinal=2`, version)
			case "progress":
				_, err = pool.Exec(ctx, `UPDATE pbl_plan_step SET status='done' WHERE version_id=$1 AND ordinal=1`, version)
			case "newer":
				_, err = pool.Exec(ctx, `INSERT INTO pbl_plan_version(atom_id,version,summary) VALUES($1,2,'student revised plan')`, pid)
			}
			if err != nil {
				t.Fatal(err)
			}
			var before string
			if err := pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(s) ORDER BY ordinal)::text FROM pbl_plan_step s WHERE version_id=$1`, version).Scan(&before); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				tx, err := pool.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(ctx, up); err != nil {
					_ = tx.Rollback(ctx)
					t.Fatal(err)
				}
				if err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			}
			var after string
			var count int
			if err := pool.QueryRow(ctx, `SELECT jsonb_agg(to_jsonb(s) ORDER BY ordinal)::text FROM pbl_plan_step s WHERE version_id=$1`, version).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("historical steps changed")
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pbl_plan_version WHERE atom_id=$1`, pid).Scan(&count); err != nil {
				t.Fatal(err)
			}
			want := 1
			if scenario == "pristine" || scenario == "newer" {
				want = 2
			}
			if count != want {
				t.Fatalf("version count %d, want %d", count, want)
			}
			if scenario == "pristine" {
				var title string
				var approved bool
				if err := pool.QueryRow(ctx, `SELECT s.title,v.approved_at IS NOT NULL FROM pbl_plan_version v JOIN pbl_plan_step s ON s.version_id=v.id WHERE v.atom_id=$1 AND v.version=2 AND s.ordinal=2`, pid).Scan(&title, &approved); err != nil {
					t.Fatal(err)
				}
				if title != "构思与试用第一幕" || approved {
					t.Fatalf("invalid replacement: %s approved=%v", title, approved)
				}
			}
		})
	}
}
