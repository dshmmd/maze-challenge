.PHONY: check fmt vet build test run up down tidy

# check is the single verification gate: it must be green before any commit.
check: fmt vet build test

# fmt fails if any file is not gofmt-clean (CI-friendly, non-mutating).
fmt:
	@gofmt -l . | tee /dev/stderr | (! read)

vet:
	go vet ./...

build:
	go build ./...

test:
	go test ./...

# run starts the HTTP server locally (needs a reachable Postgres via
# DATABASE_URL; for a one-command stack use `make up`).
run:
	go run ./cmd/server

# up / down bring the full stack (app + Postgres) via docker-compose.
up:
	docker compose up --build

down:
	docker compose down -v

# qa runs the black-box scenario simulation against a running server (BASE,
# default http://localhost:8095). The target server should run on a fresh DB
# with a short AUCTION_WINDOW (e.g. 5s) — see scripts/qa.mjs.
qa:
	node scripts/qa.mjs

tidy:
	go mod tidy
