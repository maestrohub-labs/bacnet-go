# Library-only Makefile for maestrohub-labs/bacnet-go.
#
# No CI in this repo (per release policy); these targets are the
# manual-CI substitute documented in the fork plan § F. Treat them as
# the pre-release checklist.

GO ?= go

.PHONY: all test test-integration test-all coverage vet build \
        sim-up sim-down sim-logs license-scan release-check

all: vet build test

# ---- compile + lint ----

build:
	$(GO) build ./...

vet:
	$(GO) vet ./...

# ---- tests ----

test:
	$(GO) test -race ./...

test-integration:
	$(GO) test -tags=integration -race ./...

test-all: test test-integration

# ---- coverage ----

coverage:
	$(GO) test -tags=integration -race -coverprofile=cover.out ./...
	$(GO) tool cover -func=cover.out | tail -1

# ---- BACnet simulator (Docker) ----
#
# These targets manage the local BACnet/IP simulator container used by
# the integration tests. They assume a `docker-compose.yaml` lives one
# directory up (the standard MaestroHub temp/bacnet layout). Set
# COMPOSE_DIR if your layout differs.

COMPOSE_DIR ?= ..

sim-up:
	cd $(COMPOSE_DIR) && docker compose up -d

sim-down:
	cd $(COMPOSE_DIR) && docker compose down

sim-logs:
	cd $(COMPOSE_DIR) && docker compose logs -f

# ---- license scan ----
#
# Validates the transitive dep tree carries only permissive licenses.
# Requires `go install github.com/google/go-licenses@latest` once.

license-scan:
	go-licenses check ./...

# ---- release gate ----
#
# Walks the v0.1.0 pre-release checklist from the fork plan § H.1
# in one shot. Fail-fast: if any step exits non-zero, the release is
# not ready.

release-check: vet build test test-integration coverage
	@echo
	@echo "Release check OK. Reminders before tagging:"
	@echo "  - AUDIT.md is up to date"
	@echo "  - CHANGELOG.md has an entry for the new version"
	@echo "  - FORK.md reflects any new divergences"
	@echo "  - README.md status matrix matches the test suite"
