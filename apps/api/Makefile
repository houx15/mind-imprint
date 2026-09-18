.PHONY: sync-cards sync-skills sync-rubric sqlc migrate-up test run prompt-gate-capture prompt-gate

SUITE ?=
OUT_DIR ?=
BEFORE_DIR ?=
PROMPT_SAMPLES ?= 2
PROMPT_REPEATS ?= 1

# Mirror the canonical card JSON specs into the embeddable directory.
sync-cards:
	go run ./tools/synccards

# Mirror the canonical skill JSON specs into the embeddable directory.
sync-skills:
	go run ./tools/syncskills

# Mirror the canonical CT rubric JSON into the embeddable directory.
sync-rubric:
	go run ./tools/syncrubric

# Regenerate the type-safe sqlc store from queries/ + migrations/.
# CGO_ENABLED=0 forces the WASM-based pg_query parser (avoids macOS build issue
# with the native pganalyze/pg_query_go C library).
sqlc:
	CGO_ENABLED=0 go tool sqlc generate

# Apply goose migrations against DATABASE_URL (env or .env.local).
migrate-up:
	go run ./cmd/api -migrate-up

# Run the full Go test suite (unit + testcontainers integration).
test:
	go test ./...

# Run the API server locally.
run:
	go run ./cmd/api

# Paid, manual prompt-regression evidence. Use a fresh OUT_DIR for each run.
prompt-gate-capture:
	test -n "$(SUITE)"
	test -n "$(OUT_DIR)"
	test ! -e "$(OUT_DIR)/routebench.json"
	test ! -e "$(OUT_DIR)/coachwalk.json"
	mkdir -p "$(OUT_DIR)"
	go run ./cmd/routebench -config cmd/routebench/routebench.json -suite "$(SUITE)" -bound -samples "$(PROMPT_SAMPLES)" -out "$(OUT_DIR)/routebench.md" -json-out "$(OUT_DIR)/routebench.json" -check
	go run ./cmd/coachwalk -suite "$(SUITE)" -bound -scripted -repeats "$(PROMPT_REPEATS)" -out "$(OUT_DIR)/coachwalk.md" -json-out "$(OUT_DIR)/coachwalk.json" -check

# Capture a new run and compare it with an earlier run from this experiment.
# Quality differences are review notes; only technical failures stop the run.
prompt-gate:
	test -n "$(SUITE)"
	test -n "$(OUT_DIR)"
	test -n "$(BEFORE_DIR)"
	test "$(OUT_DIR)" != "$(BEFORE_DIR)"
	test -f "$(BEFORE_DIR)/routebench.json"
	test -f "$(BEFORE_DIR)/coachwalk.json"
	test ! -e "$(OUT_DIR)/routebench.json"
	test ! -e "$(OUT_DIR)/coachwalk.json"
	mkdir -p "$(OUT_DIR)"
	@status=0; \
	go run ./cmd/routebench -config cmd/routebench/routebench.json -suite "$(SUITE)" -bound -samples "$(PROMPT_SAMPLES)" -out "$(OUT_DIR)/routebench.md" -json-out "$(OUT_DIR)/routebench.json" -compare "$(BEFORE_DIR)/routebench.json" -check || status=$$?; \
	go run ./cmd/coachwalk -suite "$(SUITE)" -bound -scripted -repeats "$(PROMPT_REPEATS)" -out "$(OUT_DIR)/coachwalk.md" -json-out "$(OUT_DIR)/coachwalk.json" -compare "$(BEFORE_DIR)/coachwalk.json" -check || status=$$?; \
	exit $$status
