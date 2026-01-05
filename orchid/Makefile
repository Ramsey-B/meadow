# Shared infrastructure (recommended for local dev)
.PHONY: infra
infra:
	@echo "Starting shared infrastructure..."
	@cd ../meadow && make up

.PHONY: infra-down
infra-down:
	@cd ../meadow && make down

.PHONY: infra-clean
infra-clean:
	@cd ../meadow && make clean

.PHONY: kafka-ui
kafka-ui:
	@echo "Kafka UI available at http://localhost:8080"

# Legacy standalone infrastructure (if not using meadow)
.PHONY: database
database:
	docker compose -f docker-compose.dev.yml up -d coordinator worker

.PHONY: redis
redis:
	docker compose -f docker-compose.dev.yml up -d redis

.PHONY: kafka
kafka:
	docker compose -f docker-compose.dev.yml up -d zookeeper kafka

.PHONY: keycloak
keycloak:
	docker compose -f docker-compose.dev.yml up -d keycloak

.PHONY: down
down:
	docker compose -f docker-compose.dev.yml down --volumes

.PHONY: deps
deps:
	make infra

.PHONY: build
build:
	docker compose -f docker-compose.dev.yml build

.PHONY: run
run:
	docker compose -f docker-compose.dev.yml up -d --build orchid

.PHONY: stop
stop:
	docker compose -f docker-compose.dev.yml down

.PHONY: clean
clean:
	docker compose -f docker-compose.dev.yml down --volumes --remove-orphans
	docker compose -f docker-compose.dev.yml rm -f

.PHONY: logs
logs:
	docker compose -f docker-compose.dev.yml logs -f

.PHONY: ps
ps:
	docker compose -f docker-compose.dev.yml ps

.PHONY: lint
lint:
	golangci-lint run -c .golangci.yml

.PHONY: test
test:
	go test -v ./...

.PHONY: test-integration
test-integration:
	go test -v ./test/integration/... -timeout 120s

.PHONY: start
start:
	air -c .air.toml

.PHONY: dev
dev:
	go run ./cmd/...
