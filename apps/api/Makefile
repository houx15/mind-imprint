.PHONY: sync-cards sync-skills sync-rubric sqlc migrate-up test run

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
