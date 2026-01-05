.PHONY: help build test dev dev-orchid dev-lotus dev-ivy dev-mocks air air-orchid air-lotus air-ivy up down clean infra infra-down infra-clean infra-volumes-clean vendor tidy wait-debezium wait-memgraph

# Default target
help:
	@echo "Meadow - Monorepo Commands"
	@echo ""
	@echo "Development:"
	@echo "  make dev           - Run Orchid, Lotus, and Ivy (go run)"
	@echo "  make dev-orchid    - Run Orchid only (port 3001)"
	@echo "  make dev-lotus     - Run Lotus only (port 3000)"
	@echo "  make dev-ivy       - Run Ivy only (port 3002)"
	@echo "  make dev-mocks     - Run mock APIs (OKTA + MS Graph, port 8090)"
	@echo "  make air           - Run all with Air hot-reload"
	@echo "  make air-orchid    - Run Orchid with Air"
	@echo "  make air-lotus     - Run Lotus with Air"
	@echo "  make air-ivy       - Run Ivy with Air"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build         - Build all projects"
	@echo "  make test          - Run all tests"
	@echo "  make test-e2e      - Run E2E tests (requires services)"
	@echo "  make clean         - Clean build artifacts"
	@echo ""
	@echo "Dependencies:"
	@echo "  make tidy          - Run go mod tidy on all modules"
	@echo "  make vendor        - Vendor workspace dependencies"
	@echo ""
	@echo "Infrastructure:"
	@echo "  make infra         - Start shared infrastructure"
	@echo "  make infra-down    - Stop shared infrastructure"
	@echo "  make infra-clean   - Remove infrastructure volumes"
	@echo "  make debezium-register-ivy - Register Debezium connector for Ivy tables (CDC)"
	@echo ""
	@echo "Manual Testing (use VS Code REST Client extension):"
	@echo "  test/http/mock-apis.http  - Test mock OKTA & MS Graph APIs"
	@echo "  test/http/orchid.http     - Orchid plans & executions"
	@echo "  test/http/lotus.http      - Lotus mappings & bindings"
	@echo "  test/http/ivy.http        - Ivy entities, rules & graph"

# =============================================================================
# Development
# =============================================================================

dev: dev-orchid dev-lotus dev-ivy

dev-orchid:
	@echo "Starting Orchid..."
	cd orchid && go run ./cmd/...

dev-lotus:
	@echo "Starting Lotus..."
	cd lotus && go run ./cmd/...

dev-ivy:
	@echo "Starting Ivy..."
	cd ivy && go run ./cmd/...

dev-mocks:
	@echo "Starting Mock API Server (OKTA + MS Graph) on :8090..."
	cd mocks && go run ./cmd/main.go

# Run all services with Air (requires multiple terminals)
air:
	@echo "Starting Air for Orchid, Lotus, and Ivy..."
	@echo "Run 'make air-orchid', 'make air-lotus', and 'make air-ivy' in separate terminals"

air-orchid:
	@echo "Starting Orchid with Air hot-reload..."
	cd orchid && air

air-lotus:
	@echo "Starting Lotus with Air hot-reload..."
	cd lotus && air

air-ivy:
	@echo "Starting Ivy with Air hot-reload..."
	cd ivy && air

# Debug mode with Delve
debug-orchid:
	@echo "Starting Orchid in debug mode (Delve on :2345)..."
	cd orchid && air -c .air.debug.toml

debug-lotus:
	@echo "Starting Lotus in debug mode (Delve on :2346)..."
	cd lotus && air -c .air.debug.toml

debug-ivy:
	@echo "Starting Ivy in debug mode (Delve on :2347)..."
	cd ivy && air -c .air.debug.toml

# =============================================================================
# Build & Test
# =============================================================================

build:
	@echo "Building all projects..."
	go build ./orchid/... ./lotus/... ./ivy/... ./stem/...

test:
	@echo "Running all unit tests..."
	go test ./orchid/... ./lotus/... ./ivy/... ./stem/...

test-v:
	@echo "Running all unit tests (verbose)..."
	go test -v ./orchid/... ./lotus/... ./ivy/... ./stem/...

test-orchid:
	@echo "Running Orchid tests..."
	go test -v ./orchid/...

test-lotus:
	@echo "Running Lotus tests..."
	go test -v ./lotus/...

test-ivy:
	@echo "Running Ivy tests..."
	go test -v ./ivy/...

test-ivy-integration:
	@echo "Running Ivy integration tests..."
	go test -v ./ivy/test/integration/...

test-e2e:
	@echo "Running E2E tests (requires infra + services running)..."
	go test -v -count=1 ./test/e2e/...

test-e2e-ivy:
	@echo "Running Ivy E2E tests (requires infra + all services running)..."
	go test -v -count=1 -run "TestIvy|TestFullPipeline" ./test/e2e/...

test-e2e-short:
	@echo "Running E2E tests (skipping integration tests)..."
	go test -short -v ./test/e2e/...

clean:
	@echo "Cleaning build artifacts..."
	rm -rf orchid/tmp lotus/tmp ivy/tmp stem/tmp
	rm -rf orchid/bin lotus/bin ivy/bin stem/bin

# =============================================================================
# Dependencies
# =============================================================================

tidy:
	@echo "Syncing workspace dependencies..."
	go work sync
	@echo "Done! Use 'make tidy-individual' if you need to tidy each module separately."

# For individual module tidy (requires stem to be published or uses replace directives)
tidy-individual:
	@echo "Running go mod tidy on each module (requires workspace context)..."
	cd stem && go mod tidy
	cd orchid && GOWORK=$(PWD)/go.work go mod tidy
	cd lotus && GOWORK=$(PWD)/go.work go mod tidy
	cd ivy && GOWORK=$(PWD)/go.work go mod tidy

vendor:
	@echo "Vendoring workspace dependencies..."
	go work vendor

# =============================================================================
# Infrastructure
# =============================================================================

# Core infrastructure commands (delegated to shared-infra)
up:
	docker compose up -d

down:
	docker compose down

infra-volumes-clean:
	docker compose down -v

# Convenience aliases
infra: up
	@$(MAKE) wait-debezium
	@$(MAKE) wait-memgraph
	@$(MAKE) create-cdc-topics
	@$(MAKE) memgraph-setup-streams
	@echo ""
	@echo "=== Infrastructure ready ==="
	@echo "1. Run 'make dev-mocks' in a terminal"
	@echo "2. Run 'make dev-orchid', 'make dev-lotus', 'make dev-ivy' in separate terminals"
	@echo "3. Run 'make setup-cdc' after Ivy has started (one-time setup for CDC)"

# Run after Ivy has started to set up CDC (requires Ivy tables to exist)
setup-cdc:
	@$(MAKE) create-ivy-publication
	@$(MAKE) debezium-register-ivy
	@echo "CDC setup complete."

migrate-all: migrate-orchid migrate-lotus migrate-ivy

infra-down: down
	@echo "Infrastructure stopped"

infra-clean: infra-volumes-clean
	@echo "Infrastructure cleaned"

# =============================================================================
# Wait helpers
# =============================================================================

wait-debezium:
	@echo "Waiting for Debezium Connect on :$${DEBEZIUM_CONNECT_PORT:-8083}..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do \
		if curl -sS http://localhost:$${DEBEZIUM_CONNECT_PORT:-8083}/connectors > /dev/null 2>&1; then \
			echo "Debezium Connect is up."; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Debezium Connect did not become ready in time."; \
	exit 1

wait-memgraph:
	@echo "Waiting for Memgraph to accept Bolt connections..."
	@for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30; do \
		if docker compose exec -T memgraph sh -lc "echo 'RETURN 1;' | mgconsole -username \"$${MEMGRAPH_USER:-user}\" -password \"$${MEMGRAPH_PASSWORD:-password}\" >/dev/null 2>&1"; then \
			echo "Memgraph is up."; \
			exit 0; \
		fi; \
		sleep 1; \
	done; \
	echo "Memgraph did not become ready in time."; \
	exit 1

# =============================================================================
# CDC (Debezium)
# =============================================================================

debezium-register-ivy:
	@echo "Registering Debezium connector (ivy-postgres-ivy)..."
	@if curl -sS http://localhost:$${DEBEZIUM_CONNECT_PORT:-8083}/connectors/ivy-postgres-ivy 2>/dev/null | grep -q '"name"'; then \
		echo "Connector already exists, restarting task..."; \
		curl -sS -X POST http://localhost:$${DEBEZIUM_CONNECT_PORT:-8083}/connectors/ivy-postgres-ivy/tasks/0/restart 2>/dev/null || true; \
	else \
		echo "Creating new connector..."; \
		curl -sS -X POST http://localhost:$${DEBEZIUM_CONNECT_PORT:-8083}/connectors \
			-H "Content-Type: application/json" \
			--data @debezium/ivy-connector.json | cat; \
	fi
	@echo ""

# Create Postgres publication for Debezium (requires tables to exist - run migrate-ivy first)
create-ivy-publication:
	@echo "Creating Postgres publication for Debezium CDC..."
	@docker compose exec postgres psql -U user -d ivy -c "DROP PUBLICATION IF EXISTS dbz_publication; CREATE PUBLICATION dbz_publication FOR TABLE staged_entities, staged_relationships, merged_entities, merged_relationships;" 2>/dev/null || true
	@echo "Done."

# Pre-create Kafka topics for Debezium CDC (required before Memgraph streams can connect)
create-cdc-topics:
	@echo "Creating Kafka topics for Ivy CDC..."
	@docker compose exec kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic ivy.public.staged_entities --partitions 1 --replication-factor 1 2>/dev/null || true
	@docker compose exec kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic ivy.public.staged_relationships --partitions 1 --replication-factor 1 2>/dev/null || true
	@docker compose exec kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic ivy.public.merged_entities --partitions 1 --replication-factor 1 2>/dev/null || true
	@docker compose exec kafka kafka-topics --bootstrap-server localhost:9092 --create --if-not-exists --topic ivy.public.merged_relationships --partitions 1 --replication-factor 1 2>/dev/null || true
	@echo "Done."

memgraph-setup-streams:
	@echo "Setting up Memgraph Kafka streams for Ivy CDC..."
	@docker compose exec -T memgraph sh -lc "echo 'CALL mg.load_all();' | mgconsole -username \"$${MEMGRAPH_USER:-user}\" -password \"$${MEMGRAPH_PASSWORD:-password}\"" 2>/dev/null || true
	@docker compose exec -T memgraph sh -lc "echo 'DROP STREAM ivy_merged_entities;' | mgconsole -username \"$${MEMGRAPH_USER:-user}\" -password \"$${MEMGRAPH_PASSWORD:-password}\"" 2>/dev/null || true
	@docker compose exec -T memgraph sh -lc "echo 'DROP STREAM ivy_merged_relationships;' | mgconsole -username \"$${MEMGRAPH_USER:-user}\" -password \"$${MEMGRAPH_PASSWORD:-password}\"" 2>/dev/null || true
	@cat memgraph/streams/ivy_debezium_streams.cypher | docker compose exec -T memgraph sh -lc "mgconsole -username \"$${MEMGRAPH_USER:-user}\" -password \"$${MEMGRAPH_PASSWORD:-password}\""
	@echo "Done."

# Legacy alias
memgraph-start-cdc-streams: memgraph-setup-streams

# =============================================================================
# Database Migrations
# =============================================================================

migrate-orchid:
	@echo "Running Orchid migrations..."
	cd orchid && go run ./cmd/... migrate

migrate-lotus:
	@echo "Running Lotus migrations..."
	cd lotus && go run ./cmd/... migrate

migrate-ivy:
	@echo "Running Ivy migrations..."
	cd ivy && go run ./cmd/... migrate

# =============================================================================
# Quick Start
# =============================================================================

# Start everything: infra + all services
start: infra
	@echo "Waiting for infrastructure to be ready..."
	@sleep 5
	@echo "Starting services..."
	@$(MAKE) -j3 dev-orchid dev-lotus dev-ivy

# Start with Air hot-reload
start-air: infra
	@echo "Waiting for infrastructure to be ready..."
	@sleep 5
	@echo "Run 'make air-orchid', 'make air-lotus', and 'make air-ivy' in separate terminals"

clear-ports: 
	@echo "Clearing ports 3000, 3001, 3002, and 8090..."
	@lsof -ti:3000,3001,3002,8090 | xargs kill -9 && echo "Ports cleared"