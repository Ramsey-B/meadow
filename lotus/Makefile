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

# Legacy standalone infrastructure (if not using meadow)
.PHONY: database
database:
	docker compose -f docker-compose.dev.yml up -d coordinator worker

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
	docker compose build

.PHONY: run
run:
	docker compose up -d --build lotus

.PHONY: stop
stop:
	docker compose down

.PHONY: clean
clean:
	docker compose down --volumes
	docker compose rm -f
	docker compose down --volumes
	docker compose rm -f
	docker compose down --volumes
	docker compose rm -f

.PHONY: lint
lint:
	golangci-lint run -c .golangci.yml

.PHONY: test
test:
	go test -v ./...

.PHONY: start
start:
	air -c .air.toml

.PHONY: dev
dev:
	go run ./cmd/...
